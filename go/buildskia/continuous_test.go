package buildskia

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

func setupTemp(t *testing.T, testData []string, repo vcsinfo.VCS) (*ContinuousBuilder, func()) {
	tempDir, err := ioutil.TempDir("", "builder_test_")
	assert.NoError(t, err)
	fi, err := os.Create(filepath.Join(tempDir, GOOD_BUILDS_FILENAME))
	assert.NoError(t, err)
	_, err = fmt.Fprintf(fi, strings.Join(testData, "\n"))
	assert.NoError(t, err)
	err = fi.Close()
	assert.NoError(t, err)

	return New(context.Background(), tempDir, "", repo, nil, 2, time.Hour, false), func() {
		util.RemoveAll(tempDir)
	}
}

func allAvailable(t *testing.T, testData []string) {
	now := time.Now()
	mockRepo := &mockVcs{
		commits: map[string]*vcsinfo.LongCommit{
			"aaa": {
				Timestamp: now.Add(time.Second),
			},
		},
	}
	b, cleanup := setupTemp(t, testData, mockRepo)
	defer cleanup()
	lst, err := b.AvailableBuilds()
	if len(testData) > 0 {
		assert.NoError(t, err)
	}
	assert.Equal(t, len(testData), len(lst))

	reversed := []string{}
	for _, r := range testData {
		reversed = append(reversed, r)
	}
	assert.Equal(t, reversed, testData)
}

func TestAllAvailable(t *testing.T) {
	unittest.SmallTest(t)
	allAvailable(t, []string{
		"fea7de6c1459cb26c9e0a0c72033e9ccaea56530",
		"4d51f64ff18e2e15c40fec0c374d89879ba273bc",
	})
	allAvailable(t, []string{
		"fea7de6c1459cb26c9e0a0c72033e9ccaea56530",
	})
	allAvailable(t, []string{})
}

type mockVcs struct {
	commits map[string]*vcsinfo.LongCommit
}

func (m *mockVcs) GetBranch() string                                               { return git.DefaultBranch }
func (m *mockVcs) LastNIndex(N int) []*vcsinfo.IndexCommit                         { return nil }
func (m *mockVcs) Update(ctx context.Context, pull, allBranches bool) error        { return nil }
func (m *mockVcs) From(start time.Time) []string                                   { return nil }
func (m *mockVcs) Range(begin, end time.Time) []*vcsinfo.IndexCommit               { return nil }
func (m *mockVcs) IndexOf(ctx context.Context, hash string) (int, error)           { return 0, nil }
func (m *mockVcs) ByIndex(ctx context.Context, N int) (*vcsinfo.LongCommit, error) { return nil, nil }
func (m *mockVcs) GetFile(ctx context.Context, fileName, commitHash string) (string, error) {
	return "", nil
}

// Details returns the full commit information for the given hash.
// If includeBranchInfo is true the Branches field of the returned
// result will contain all branches that contain the given commit,
// otherwise Branches will be empty.
func (m *mockVcs) Details(ctx context.Context, hash string, includeBranchInfo bool) (*vcsinfo.LongCommit, error) {
	if c, ok := m.commits[hash]; ok {
		return c, nil
	} else {
		return nil, fmt.Errorf("Not found")
	}
}
func (m *mockVcs) DetailsMulti(ctx context.Context, hashes []string, includeBranchInfo bool) ([]*vcsinfo.LongCommit, error) {
	return nil, nil
}

func TestDecimate(t *testing.T) {
	unittest.SmallTest(t)
	now := time.Now()
	mock := &mockVcs{
		commits: map[string]*vcsinfo.LongCommit{
			"aaa": {
				Timestamp: now.Add(-62 * 24 * time.Hour),
			},
			"bbb": {
				Timestamp: now.Add(-31 * 24 * time.Hour),
			},
			"ccc": {
				Timestamp: now.Add(-5 * time.Second),
			},
			"ddd": {
				Timestamp: now.Add(-4 * time.Second),
			},
			"eee": {
				Timestamp: now.Add(-3 * time.Second),
			},
			"fff": {
				Timestamp: now.Add(-2 * time.Second),
			},
			"ggg": {
				Timestamp: now.Add(time.Second),
			},
		},
	}
	ctx := context.Background()

	// No change if number if items < limit.
	keep, remove, err := decimate(ctx, []string{"eee", "fff", "ggg"}, mock, 4)
	assert.NoError(t, err)
	assert.Equal(t, keep, []string{"eee", "fff", "ggg"}, "")
	assert.Equal(t, remove, []string{})

	// Proper decimation if items == limit.
	keep, remove, err = decimate(ctx, []string{"ddd", "eee", "fff", "ggg"}, mock, 4)
	assert.NoError(t, err)
	assert.Equal(t, keep, []string{"ddd", "fff", "ggg"})
	assert.Equal(t, remove, []string{"eee"})

	// Proper decimation if items > limit.
	keep, remove, err = decimate(ctx, []string{"ccc", "ddd", "eee", "fff", "ggg"}, mock, 4)
	assert.NoError(t, err)
	assert.Equal(t, keep, []string{"ccc", "eee", "ggg"})
	assert.Equal(t, remove, []string{"ddd", "fff"})

	// Proper decimation (none) if we end up with less than 'limit' items after removing keepers.
	keep, remove, err = decimate(ctx, []string{"bbb", "ddd", "eee"}, mock, 4)
	assert.NoError(t, err)
	assert.Equal(t, []string{"bbb", "ddd", "eee"}, keep)
	assert.Equal(t, []string{}, remove)

	// Proper decimation (none) if we end up with less than 'limit' items after removing keepers.
	// "ccc", "fff", and "ggg" are keepers, leaving just 3 to decimate.
	keep, remove, err = decimate(ctx, []string{"aaa", "bbb", "ccc", "fff", "ggg"}, mock, 4)
	assert.NoError(t, err)
	assert.Equal(t, []string{"aaa", "bbb", "ccc", "fff", "ggg"}, keep)
	assert.Equal(t, []string{}, remove)

	// Proper decimation if we end up with enough 'limit' items after removing keepers.
	keep, remove, err = decimate(ctx, []string{"aaa", "bbb", "ccc", "ddd", "eee", "fff", "ggg"}, mock, 4)
	assert.NoError(t, err)
	assert.Equal(t, []string{"aaa", "bbb", "ccc", "eee", "ggg"}, keep)
	assert.Equal(t, []string{"ddd", "fff"}, remove)
}

func TestCurrent(t *testing.T) {
	unittest.SmallTest(t)
	now := time.Now()
	mockRepo := &mockVcs{
		commits: map[string]*vcsinfo.LongCommit{
			"aaa": {
				ShortCommit: &vcsinfo.ShortCommit{
					Hash: "aaa",
				},
				Timestamp: now.Add(time.Second),
			},
		},
	}
	testData := []string{
		"aaa",
	}
	b, cleanup := setupTemp(t, testData, mockRepo)
	defer cleanup()
	assert.Equal(t, "aaa", b.Current().Hash)
}

func TestCurrentNoBuilds(t *testing.T) {
	unittest.SmallTest(t)
	mockRepo := &mockVcs{}
	testData := []string{}
	b, cleanup := setupTemp(t, testData, mockRepo)
	defer cleanup()
	assert.Equal(t, "unknown", b.Current().Hash)
}
