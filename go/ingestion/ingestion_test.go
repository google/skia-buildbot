package ingestion

import (
	"crypto/md5"
	"fmt"
	"io"
	"sort"
	"sync"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

const LOCAL_STATUS_DIR = "./ingestion_status"

func TestIngester(t *testing.T) {
	defer util.RemoveAll(LOCAL_STATUS_DIR)

	now := time.Now()
	beginningOfTime := now.Add(-time.Hour * 24 * 10).Unix()
	const totalCommits = 100

	// Instantiate mock VCS and the source.
	vcs := MockVCS(beginningOfTime, now.Unix(), totalCommits)
	hashes := vcs.From(time.Unix(0, 0))
	assert.Equal(t, totalCommits, len(hashes))
	for _, h := range hashes {
		assert.NotEqual(t, "", h)
	}

	sources := []Source{MockSource(t, vcs)}

	// Instantiate the mock processor.
	collected := map[string]int{}
	var mutex sync.Mutex

	processFn := func(result ResultFileLocation) error {
		mutex.Lock()
		defer mutex.Unlock()
		collected[result.Name()] += 1
		return nil
	}

	finishFn := func() error { return nil }
	processor := MockProcessor(processFn, finishFn)

	// Instantiate ingesterConf
	conf := &sharedconfig.IngesterConfig{
		RunEvery:  sharedconfig.TomlDuration{Duration: 1 * time.Second},
		NCommits:  50,
		MinDays:   3,
		StatusDir: LOCAL_STATUS_DIR,
	}

	// Instantiate ingester and start it.
	ingester, err := NewIngester("test-ingester", conf, vcs, sources, processor)
	assert.Nil(t, err)
	ingester.Start()

	// Give it enough time to run a few ingestion cycles and to shut down.
	time.Sleep(5 * time.Second)
	ingester.stop()
	time.Sleep(5 * time.Second)

	assert.Equal(t, totalCommits/2, len(collected))
	for _, count := range collected {
		assert.Equal(t, 1, count)
	}
}

// mock processor
type mockProcessor struct {
	process func(ResultFileLocation) error
	finish  func() error
}

func MockProcessor(process func(ResultFileLocation) error, finish func() error) Processor {
	return &mockProcessor{
		process: process,
		finish:  finish,
	}
}

func (m *mockProcessor) Process(resultsFile ResultFileLocation) error {
	return m.process(resultsFile)
}

func (m *mockProcessor) BatchFinished() error {
	return m.finish()
}

type mockRFLocation struct {
	t    int64
	path string
	md5  string
}

func (m *mockRFLocation) Open() (io.ReadCloser, error) { return nil, nil }
func (m *mockRFLocation) Name() string                 { return m.path }
func (m *mockRFLocation) MD5() string                  { return m.md5 }

func rfLocation(t time.Time, fname string) ResultFileLocation {
	path := fmt.Sprintf("root/%d/%d/%d/%d/%d/%s", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), fname)
	return &mockRFLocation{
		t:    t.Unix(),
		path: path,
		md5:  fmt.Sprintf("%x", md5.Sum([]byte(path))),
	}
}

// mock source
type mockSource []ResultFileLocation

func MockSource(t *testing.T, vcs vcsinfo.VCS) Source {
	hashes := vcs.From(time.Unix(0, 0))
	ret := make([]ResultFileLocation, 0, len(hashes))
	for _, h := range hashes {
		detail, err := vcs.Details(h)
		assert.Nil(t, err)
		ret = append(ret, rfLocation(detail.Timestamp, fmt.Sprintf("result-file-%s", h)))
	}
	return mockSource(ret)
}

func (m mockSource) Poll(startTime, endTime int64) ([]ResultFileLocation, error) {
	startIdx := sort.Search(len(m), func(i int) bool { return m[i].(*mockRFLocation).t >= startTime })
	endIdx := startIdx
	for ; (endIdx < len(m)) && (m[endIdx].(*mockRFLocation).t <= endTime); endIdx++ {
	}
	return m[startIdx:endIdx], nil
}

func (m mockSource) EventChan() <-chan []ResultFileLocation {
	return nil
}

func (m mockSource) ID() string {
	return "test-source"
}

// mockVCS
type mockVCS []*vcsinfo.LongCommit

func MockVCS(start, end int64, nCommits int) vcsinfo.VCS {
	ret := make([]*vcsinfo.LongCommit, 0, nCommits)
	inc := (end - start - 3600) / int64(nCommits)
	t := start
	for i := 0; i < nCommits; i++ {
		ret = append(ret, &vcsinfo.LongCommit{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    fmt.Sprintf("hash-%d", i),
				Subject: fmt.Sprintf("Commit #%d", i),
			},
			Timestamp: time.Unix(t, 0),
		})
		t += inc
	}
	return mockVCS(ret)
}

func (m mockVCS) Update(pull, allBranches bool) error { return nil }
func (m mockVCS) From(start time.Time) []string {
	idx := sort.Search(len(m), func(i int) bool { return m[i].Timestamp.Unix() >= start.Unix() })

	ret := make([]string, 0, len(m)-idx)
	for _, commit := range m[idx:len(m)] {
		ret = append(ret, commit.Hash)
	}
	return ret
}

func (m mockVCS) Details(hash string) (*vcsinfo.LongCommit, error) {
	for _, commit := range m {
		if commit.Hash == hash {
			return commit, nil
		}
	}
	return nil, fmt.Errorf("Unable to find commit")
}
