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

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/config"
	"go.skia.org/infra/go/eventbus"
	mockeventbus "go.skia.org/infra/go/eventbus/mocks"
	"go.skia.org/infra/go/ingestion/mocks"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
	mockvcs "go.skia.org/infra/go/vcsinfo/mocks"
)

const (
	RFLOCATION_CONTENT = "result file content"

	TEST_BUCKET_ID = "test-bucket"
)

func TestPollingIngester(t *testing.T) {
	unittest.LargeTest(t)

	eventBus := eventbus.New()
	ctx := context.Background()
	now := time.Now()
	beginningOfTime := now.Add(-time.Hour * 24 * 10).Unix()
	const totalCommits = 100

	mis := &mocks.IngestionStore{}
	defer mis.AssertExpectations(t)
	mis.On("ContainsResultFileHash", mock.Anything, mock.Anything).Return(false, nil)
	mis.On("SetResultFileHash", mock.Anything, mock.Anything).Return(nil)

	// Instantiate mock VCS and the source.
	vcs := getVCS(beginningOfTime, now.Unix(), totalCommits)
	hashes := vcs.From(time.Unix(0, 0))
	require.Equal(t, totalCommits, len(hashes))
	for _, h := range hashes {
		require.NotEqual(t, "", h)
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
		RunEvery: config.Duration{Duration: 5 * time.Second},
		NCommits: totalCommits / 2,
		MinDays:  3,
	}

	// Instantiate ingester and start it.
	ingester, err := NewIngester("test-ingester", conf, vcs, sources, processor, mis, eventBus)
	require.NoError(t, err)
	require.NoError(t, ingester.Start(ctx))

	// Clean up the ingester at the end.
	defer testutils.AssertCloses(t, ingester)

	require.NoError(t, testutils.EventuallyConsistent(5*time.Second, func() error {
		mutex.Lock()
		colen := len(collected)
		mutex.Unlock()
		if colen >= (totalCommits / 2) {
			return nil
		}
		return testutils.TryAgainErr
	}))

	for _, count := range collected {
		require.Equal(t, 1, count)
	}
	for _, result := range sources[0].(*mockSource).data[totalCommits/2:] {
		_, ok := collected[result.Name()]
		require.True(t, ok)
	}
}

// TestGetStartTimeOfInterestDays checks that we compute the time to start
// polling for commits properly in the case that the commits returned in
// last 3 days exceeds the NCommits we want to scan.
func TestGetStartTimeOfInterestDays(t *testing.T) {
	unittest.SmallTest(t)
	// We have to provide NewIngester non-nil eventbus and ingestionstore.
	meb := &mockeventbus.EventBus{}
	mis := &mocks.IngestionStore{}
	mvs := &mockvcs.VCS{}

	defer meb.AssertExpectations(t)
	defer mis.AssertExpectations(t)
	defer mvs.AssertExpectations(t)

	// arbitrary date
	now := time.Date(2019, 8, 5, 11, 20, 0, 0, time.UTC)
	threeDaysAgo := now.Add(-3 * 24 * time.Hour)
	alphaTime := time.Date(2019, 8, 2, 17, 35, 0, 0, time.UTC)

	hashes := []string{"alpha", "beta", "gamma", "delta", "epsilon"}

	mvs.On("Update", testutils.AnyContext, true, false).Return(nil)
	mvs.On("From", threeDaysAgo).Return(hashes)
	mvs.On("Details", testutils.AnyContext, "alpha", false).Return(&vcsinfo.LongCommit{
		// The function only cares about the timestamp
		Timestamp: alphaTime,
	}, nil)

	conf := &sharedconfig.IngesterConfig{
		NCommits: 2,
		MinDays:  3,
	}

	i, err := NewIngester("test-ingester-1", conf, mvs, nil, nil, mis, meb)
	require.NoError(t, err)

	ts, err := i.getStartTimeOfInterest(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, alphaTime, ts)
}

// TestGetStartTimeOfInterestCommits checks that we compute the time to start
// polling for commits properly in the case that the commits returned in
// last 3 days does not exceed the NCommits we want to scan.
func TestGetStartTimeOfInterestCommits(t *testing.T) {
	unittest.SmallTest(t)
	// We have to provide NewIngester non-nil eventbus and ingestionstore.
	meb := &mockeventbus.EventBus{}
	mis := &mocks.IngestionStore{}
	mvs := &mockvcs.VCS{}

	defer meb.AssertExpectations(t)
	defer mis.AssertExpectations(t)
	defer mvs.AssertExpectations(t)

	// arbitrary date
	now := time.Date(2019, 8, 5, 11, 20, 0, 0, time.UTC)
	threeDaysAgo := now.Add(-3 * 24 * time.Hour)
	sixDaysAgo := now.Add(-6 * 24 * time.Hour)
	betaTime := time.Date(2019, 8, 1, 17, 35, 0, 0, time.UTC)

	hashes := []string{"alpha", "beta", "gamma", "delta", "epsilon"}

	mvs.On("Update", testutils.AnyContext, true, false).Return(nil)
	mvs.On("From", threeDaysAgo).Return(hashes[3:])
	mvs.On("From", sixDaysAgo).Return(hashes)
	// Since we retrieve 5 commits, the algorithm trims it to NCommits
	// when it has to query more.
	mvs.On("Details", testutils.AnyContext, "beta", false).Return(&vcsinfo.LongCommit{
		// The function only cares about the timestamp
		Timestamp: betaTime,
	}, nil)

	conf := &sharedconfig.IngesterConfig{
		NCommits: 4,
		MinDays:  3,
	}

	i, err := NewIngester("test-ingester-2", conf, mvs, nil, nil, mis, meb)
	require.NoError(t, err)

	ts, err := i.getStartTimeOfInterest(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, betaTime, ts)
}

// TestGetStartTimeOfInterestTryJobs checks that we compute the time to start
// polling for commits properly in the case of TryJobs, where NCommits is 0 and
// the time is short enough that we haven't seen new commits in that time.
func TestGetStartTimeOfInterestTryJobs(t *testing.T) {
	unittest.SmallTest(t)
	// We have to provide NewIngester non-nil eventbus and ingestionstore.
	meb := &mockeventbus.EventBus{}
	mis := &mocks.IngestionStore{}

	defer meb.AssertExpectations(t)
	defer mis.AssertExpectations(t)

	// arbitrary date
	now := time.Date(2019, 8, 5, 11, 20, 0, 0, time.UTC)
	oneHourAgo := now.Add(-1 * time.Hour)

	conf := &sharedconfig.IngesterConfig{
		MinHours: 1,
	}

	i, err := NewIngester("test-ingester-1", conf, nil, nil, nil, mis, meb)
	require.NoError(t, err)

	ts, err := i.getStartTimeOfInterest(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, oneHourAgo, ts)
}

// TestGetStartTimeOfInterestNotEnough makes sure we don't loop infinitely
// if there are not enough commits in the repo to fulfill the NCommits.
func TestGetStartTimeOfInterestNotEnough(t *testing.T) {
	unittest.SmallTest(t)
	// We have to provide NewIngester non-nil eventbus and ingestionstore.
	meb := &mockeventbus.EventBus{}
	mis := &mocks.IngestionStore{}
	mvs := &mockvcs.VCS{}

	defer meb.AssertExpectations(t)
	defer mis.AssertExpectations(t)
	defer mvs.AssertExpectations(t)

	// arbitrary date
	now := time.Date(2019, 8, 5, 11, 20, 0, 0, time.UTC)
	alphaTime := time.Date(2019, 8, 2, 17, 35, 0, 0, time.UTC)

	hashes := []string{"alpha", "beta", "gamma", "delta", "epsilon"}

	mvs.On("Update", testutils.AnyContext, true, false).Return(nil)
	mvs.On("From", mock.MatchedBy(func(then time.Time) bool {
		return then.Before(now) && then.After(now.Add(-365*24*time.Hour))
	})).Return(hashes)
	mvs.On("Details", testutils.AnyContext, "alpha", false).Return(&vcsinfo.LongCommit{
		// The function only cares about the timestamp
		Timestamp: alphaTime,
	}, nil)

	conf := &sharedconfig.IngesterConfig{
		NCommits: 100,
		MinDays:  3,
	}

	i, err := NewIngester("test-ingester-3", conf, mvs, nil, nil, mis, meb)
	require.NoError(t, err)

	ts, err := i.getStartTimeOfInterest(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, alphaTime, ts)
}

// TODO(kjlubick): replace these with mockery-based mocks

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
		require.NoError(t, err)
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
	return mockvcs.DeprecatedMockVCS(commits, nil, nil)
}
