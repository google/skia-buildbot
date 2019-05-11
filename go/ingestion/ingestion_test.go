package ingestion

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"regexp"
	"sort"
	"sync"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/config"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	LOCAL_STATUS_DIR   = "./ingestion_status"
	RFLOCATION_CONTENT = "result file content"

	PROJECT_ID      = "test-project-ingestion"
	TEST_TOPIC      = "test-topic-ingestion-testing"
	TEST_SUBSCRIBER = "test-subscriber-ingestion-testing"

	TEST_BUCKET_ID = "test-bucket"
)

func TestPollingIngester(t *testing.T) {
	unittest.LargeTest(t)
	testIngester(t, LOCAL_STATUS_DIR+"-polling", nil)
}

func TestPollingIngesterWithStore(t *testing.T) {
	unittest.LargeTest(t)

	// Delete and recreate the BT tables to make sure there are no residual data.
	assert.NoError(t, bt.DeleteTables(projectID, instanceID, TABLE_FILES_PROCESSED))
	assert.NoError(t, InitBT(projectID, instanceID, TABLE_FILES_PROCESSED))

	// Create the BT ingestion store.
	store, err := NewBTIStore(projectID, instanceID, nameSpace)
	assert.NoError(t, err)
	assert.NotNil(t, store)

	testIngester(t, LOCAL_STATUS_DIR+"-polling", store)
}

// testIngester test an ingester by using a polling source. Since the implementation
// generates (synthetic) storage events, we are also testing the event driven ingester.
func testIngester(t *testing.T, statusDir string, ingestionStore IngestionStore) {
	defer util.RemoveAll(statusDir)

	eventBus := eventbus.New()
	ctx := context.Background()
	now := time.Now()
	beginningOfTime := now.Add(-time.Hour * 24 * 10).Unix()
	const totalCommits = 100

	// Instantiate mock VCS and the source.
	vcs := getVCS(beginningOfTime, now.Unix(), totalCommits)
	hashes := vcs.From(time.Unix(0, 0))
	assert.Equal(t, totalCommits, len(hashes))
	for _, h := range hashes {
		assert.NotEqual(t, "", h)
	}

	sources := []Source{MockSource(t, TEST_BUCKET_ID, "root", vcs, eventBus)}

	// Instantiate the mock processor.
	collected := map[string]int{}
	var mutex sync.Mutex

	resultFiles := []ResultFileLocation{}
	processFn := func(result ResultFileLocation) error {
		mutex.Lock()
		defer mutex.Unlock()
		collected[result.Name()] += 1
		resultFiles = append(resultFiles, result)
		return nil
	}

	processor := MockProcessor(processFn)

	// Instantiate ingesterConf
	conf := &sharedconfig.IngesterConfig{
		RunEvery:  config.Duration{Duration: 5 * time.Second},
		NCommits:  totalCommits / 2,
		MinDays:   3,
		StatusDir: statusDir,
	}

	// Instantiate ingester and start it.
	ingester, err := NewIngester("test-ingester", conf, vcs, sources, processor, ingestionStore, eventBus)
	assert.NoError(t, err)
	assert.NoError(t, ingester.Start(ctx))

	// Clean up the ingester at the end.
	defer testutils.AssertCloses(t, ingester)

	assert.NoError(t, testutils.EventuallyConsistent(5*time.Second, func() error {
		mutex.Lock()
		colen := len(collected)
		mutex.Unlock()
		if colen >= (totalCommits / 2) {
			return nil
		}
		return testutils.TryAgainErr
	}))

	for _, count := range collected {
		assert.Equal(t, 1, count)
	}
	for _, result := range sources[0].(*mockSource).data[totalCommits/2:] {
		_, ok := collected[result.Name()]
		assert.True(t, ok)
	}
}

// mock processor
type mockProcessor struct {
	process func(ResultFileLocation) error
}

func MockProcessor(process func(ResultFileLocation) error) Processor {
	return &mockProcessor{
		process: process,
	}
}

func (m *mockProcessor) Process(ctx context.Context, resultsFile ResultFileLocation) error {
	return m.process(resultsFile)
}

type mockRFLocation struct {
	path        string
	bucketID    string
	objectID    string
	md5         string
	lastUpdated int64
}

func (m *mockRFLocation) Open() (io.ReadCloser, error) { return nil, nil }
func (m *mockRFLocation) Name() string                 { return m.path }
func (m *mockRFLocation) StorageIDs() (string, string) { return m.bucketID, m.objectID }
func (m *mockRFLocation) MD5() string                  { return m.md5 }
func (m *mockRFLocation) TimeStamp() int64             { return m.lastUpdated }
func (m *mockRFLocation) Content() []byte              { return []byte(RFLOCATION_CONTENT) }

func rfLocation(timeStamp int64, bucketID, objectID string) ResultFileLocation {
	path := bucketID + "/" + objectID
	return &mockRFLocation{
		bucketID:    bucketID,
		objectID:    objectID,
		path:        path,
		md5:         fmt.Sprintf("%x", md5.Sum([]byte(path))),
		lastUpdated: timeStamp,
	}
}

// mock source
type mockSource struct {
	data         []ResultFileLocation
	eventBus     eventbus.EventBus
	bucketID     string
	objectPrefix string
	regExp       *regexp.Regexp
}

func MockSource(t *testing.T, bucketID string, objectPrefix string, vcs vcsinfo.VCS, eventBus eventbus.EventBus) Source {
	hashes := vcs.From(time.Unix(0, 0))
	ret := make([]ResultFileLocation, 0, len(hashes))
	for _, h := range hashes {
		detail, err := vcs.Details(context.Background(), h, false)
		assert.NoError(t, err)
		t := detail.Timestamp
		objPrefix := fmt.Sprintf("%s/%d/%d/%d/%d/%d", objectPrefix, t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute())
		objectID := fmt.Sprintf("%s/result-file-%s", objPrefix, h)
		ret = append(ret, rfLocation(detail.Timestamp.Unix(), bucketID, objectID))
	}
	return &mockSource{
		data:         ret,
		bucketID:     bucketID,
		objectPrefix: objectPrefix,
		eventBus:     eventBus,
	}
}

func (m *mockSource) Poll(startTime, endTime int64) <-chan ResultFileLocation {
	ch := make(chan ResultFileLocation)
	go func() {
		startIdx := sort.Search(len(m.data), func(i int) bool { return m.data[i].TimeStamp() >= startTime })
		endIdx := startIdx
		for ; (endIdx < len(m.data)) && (m.data[endIdx].TimeStamp() <= endTime); endIdx++ {
		}
		for _, entry := range m.data[startIdx:endIdx] {
			ch <- entry
		}
		close(ch)
	}()
	return ch
}

func (m mockSource) ID() string {
	return "test-source"
}

func (m *mockSource) SetEventChannel(resultCh chan<- ResultFileLocation) error {
	eventType, err := m.eventBus.RegisterStorageEvents(m.bucketID, m.objectPrefix, m.regExp, nil)
	if err != nil {
		return err
	}
	m.eventBus.SubscribeAsync(eventType, func(evData interface{}) {
		file := evData.(*eventbus.StorageEvent)
		resultCh <- rfLocation(file.TimeStamp, file.BucketID, file.ObjectID)
	})
	return nil
}

// return a mock vcs
func getVCS(start, end int64, nCommits int) vcsinfo.VCS {
	commits := make([]*vcsinfo.LongCommit, 0, nCommits)
	inc := (end - start - 3600) / int64(nCommits)
	t := start
	for i := 0; i < nCommits; i++ {
		commits = append(commits, &vcsinfo.LongCommit{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    fmt.Sprintf("hash-%d", i),
				Subject: fmt.Sprintf("Commit #%d", i),
			},
			Timestamp: time.Unix(t, 0),
		})
		t += inc
	}
	return MockVCS(commits, nil, nil)
}
