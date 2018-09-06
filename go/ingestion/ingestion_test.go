package ingestion

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"sort"
	"sync"
	"testing"
	"time"

	"go.skia.org/infra/go/bt"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/config"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	LOCAL_STATUS_DIR   = "./ingestion_status"
	RFLOCATION_CONTENT = "result file content"
)

func TestPollingIngester(t *testing.T) {
	testutils.LargeTest(t)
	testIngester(t, LOCAL_STATUS_DIR+"-polling", nil)
}

func TestPollingIngesterWithStore(t *testing.T) {
	testutils.LargeTest(t)
	assert.NoError(t, bt.InitBigtable(projectID, instanceID, BigTableConfig))
	store, err := NewBTIStore(projectID, instanceID, nameSpace)
	assert.NoError(t, err)
	assert.NotNil(t, store)

	// Clear the store to make sure the ingester adds all the data.
	assert.NoError(t, store.Clear())
	testIngester(t, LOCAL_STATUS_DIR+"-polling", store)
}

func testIngester(t *testing.T, statusDir string, ingestionStore IngestionStore) {
	defer util.RemoveAll(statusDir)

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

	sources := []Source{MockSource(t, vcs)}

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
		RunEvery:  config.Duration{Duration: 1 * time.Second},
		NCommits:  totalCommits / 2,
		MinDays:   3,
		StatusDir: statusDir,
	}

	// Instantiate ingester and start it.
	ingester, err := NewIngester("test-ingester", conf, vcs, sources, processor, ingestionStore)
	assert.NoError(t, err)
	assert.NoError(t, ingester.Start(ctx))

	// Wait until we have collected the desired result, but no more than two seconds.
	startTime := time.Now()
	for {
		mutex.Lock()
		colen := len(collected)
		mutex.Unlock()
		if colen >= (totalCommits/2) || (time.Now().Sub(startTime) > (time.Second * 10)) {
			break
		}
		time.Sleep(time.Millisecond * 100)
	}

	assert.Equal(t, totalCommits/2, len(collected))
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
	md5         string
	lastUpdated int64
}

func (m *mockRFLocation) Open() (io.ReadCloser, error) { return nil, nil }
func (m *mockRFLocation) Name() string                 { return m.path }
func (m *mockRFLocation) MD5() string                  { return m.md5 }
func (m *mockRFLocation) TimeStamp() int64             { return m.lastUpdated }
func (m *mockRFLocation) Content() []byte              { return []byte(RFLOCATION_CONTENT) }

func rfLocation(t time.Time, fname string) ResultFileLocation {
	path := fmt.Sprintf("root/%d/%d/%d/%d/%d/%s", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), fname)
	return &mockRFLocation{
		path:        path,
		md5:         fmt.Sprintf("%x", md5.Sum([]byte(path))),
		lastUpdated: t.Unix(),
	}
}

// mock source
type mockSource struct {
	data []ResultFileLocation
}

func MockSource(t *testing.T, vcs vcsinfo.VCS) Source {
	hashes := vcs.From(time.Unix(0, 0))
	ret := make([]ResultFileLocation, 0, len(hashes))
	for _, h := range hashes {
		detail, err := vcs.Details(context.Background(), h, true)
		assert.NoError(t, err)
		ret = append(ret, rfLocation(detail.Timestamp, fmt.Sprintf("result-file-%s", h)))
	}
	return &mockSource{
		data: ret,
	}
}

func (m *mockSource) Poll(startTime, endTime int64) ([]ResultFileLocation, error) {
	startIdx := sort.Search(len(m.data), func(i int) bool { return m.data[i].TimeStamp() >= startTime })
	endIdx := startIdx
	for ; (endIdx < len(m.data)) && (m.data[endIdx].TimeStamp() <= endTime); endIdx++ {
	}
	return m.data[startIdx:endIdx], nil
}

func (m mockSource) ID() string {
	return "test-source"
}

func (m *mockSource) SetEventChannel(resultCh chan<- []ResultFileLocation) error {
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

func TestRflQueue(t *testing.T) {
	testutils.SmallTest(t)
	locs := []ResultFileLocation{
		rfLocation(time.Now(), "1"),
		rfLocation(time.Now(), "2"),
		rfLocation(time.Now(), "3"),
		rfLocation(time.Now(), "4"),
		rfLocation(time.Now(), "5"),
	}

	queue := rflQueue([]ResultFileLocation{})
	queue.push(locs[0:3])
	queue.push(locs[3:])

	assert.Equal(t, locs, []ResultFileLocation(queue))
	queue.clear()
	assert.Equal(t, 0, len(queue))
}

func TestIngesterNilVcs(t *testing.T) {
	testutils.SmallTest(t)

	// Instantiate ingester config.
	conf := &sharedconfig.IngesterConfig{
		MinDays: 3,
	}

	// Instantiate ingester and call getCommitRangeOfInterest.
	ctx := context.Background()
	ingester, err := NewIngester("test-ingester", conf, nil, nil, nil, nil)
	start, end, err := ingester.getCommitRangeOfInterest(ctx)
	assert.NoError(t, err)

	// Verify that start = end - MinDays.
	delta := -time.Duration(conf.MinDays) * time.Hour * 24
	assert.Equal(t, time.Unix(end, 0).Add(delta).Unix(), start)
}
