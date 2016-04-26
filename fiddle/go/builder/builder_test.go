package builder

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

func allAvailable(t *testing.T, testData []string) {
	tempDir, err := ioutil.TempDir("", "builder_test_")
	assert.NoError(t, err)
	defer util.RemoveAll(tempDir)
	fi, err := os.Create(filepath.Join(tempDir, GOOD_BUILDS_FILENAME))
	assert.NoError(t, err)
	fmt.Fprintf(fi, strings.Join(testData, "\n"))
	err = fi.Close()
	assert.NoError(t, err)

	b := New(tempDir, "")
	lst, err := b.AvailableBuilds()
	assert.NoError(t, err)
	assert.Equal(t, len(testData), len(lst))
	assert.Equal(t, len(testData), len(lst))

	reversed := []string{}
	for _, r := range testData {
		reversed = append(reversed, r)
	}
	assert.Equal(t, reversed, testData)
}

func TestAllAvailable(t *testing.T) {
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

func (m *mockVcs) Update(pull, allBranches bool) error { return nil }
func (m *mockVcs) From(start time.Time) []string       { return nil }

// Details returns the full commit information for the given hash.
// If includeBranchInfo is true the Branches field of the returned
// result will contain all branches that contain the given commit,
// otherwise Branches will be empty.
func (m *mockVcs) Details(hash string, includeBranchInfo bool) (*vcsinfo.LongCommit, error) {
	if c, ok := m.commits[hash]; ok {
		return c, nil
	} else {
		return nil, fmt.Errorf("Not found")
	}
}

func TestDecimate(t *testing.T) {
	now := time.Now()
	mock := &mockVcs{
		commits: map[string]*vcsinfo.LongCommit{
			"aaa": &vcsinfo.LongCommit{
				Timestamp: now.Add(time.Second),
			},
			"bbb": &vcsinfo.LongCommit{
				Timestamp: now.Add(2 * time.Second),
			},
			"ccc": &vcsinfo.LongCommit{
				Timestamp: now.Add(3 * time.Second),
			},
			"ddd": &vcsinfo.LongCommit{
				Timestamp: now.Add(4 * time.Second),
			},
			"eee": &vcsinfo.LongCommit{
				Timestamp: now.Add(5 * time.Second),
			},
			"fff": &vcsinfo.LongCommit{
				Timestamp: now.Add(31 * 24 * time.Hour),
			},
			"ggg": &vcsinfo.LongCommit{
				Timestamp: now.Add(62 * 24 * time.Hour),
			},
		},
	}

	// No change if number if items < limit.
	keep, remove, err := decimate([]string{"aaa", "bbb", "ccc"}, mock, 4)
	assert.NoError(t, err)
	assert.Equal(t, keep, []string{"aaa", "bbb", "ccc"}, "")
	assert.Equal(t, remove, []string{})

	// Proper decimation if items == limit.
	keep, remove, err = decimate([]string{"aaa", "bbb", "ccc", "ddd"}, mock, 4)
	assert.NoError(t, err)
	assert.Equal(t, keep, []string{"aaa", "ccc"})
	assert.Equal(t, remove, []string{"bbb", "ddd"})

	// Proper decimation if items > limit.
	keep, remove, err = decimate([]string{"aaa", "bbb", "ccc", "ddd", "eee"}, mock, 4)
	assert.NoError(t, err)
	assert.Equal(t, keep, []string{"aaa", "ccc", "eee"})
	assert.Equal(t, remove, []string{"bbb", "ddd"})

	// Proper decimation (none) if we end up with less than 'limit' items after removing keepers.
	keep, remove, err = decimate([]string{"aaa", "bbb", "ccc", "fff"}, mock, 4)
	assert.NoError(t, err)
	assert.Equal(t, []string{"aaa", "bbb", "ccc", "fff"}, keep)
	assert.Equal(t, []string{}, remove)

	// Proper decimation (none) if we end up with less than 'limit' items after removing keepers.
	// "ccc", "fff", and "ggg" are keepers, leaving just 3 to decimate.
	keep, remove, err = decimate([]string{"aaa", "bbb", "ccc", "fff", "ggg"}, mock, 4)
	assert.NoError(t, err)
	assert.Equal(t, []string{"aaa", "bbb", "ccc", "fff", "ggg"}, keep)
	assert.Equal(t, []string{}, remove)

	// Proper decimation if we end up with enough 'limit' items after removing keepers.
	keep, remove, err = decimate([]string{"aaa", "bbb", "ccc", "ddd", "eee", "fff", "ggg"}, mock, 4)
	assert.NoError(t, err)
	assert.Equal(t, []string{"aaa", "ccc", "eee", "fff", "ggg"}, keep)
	assert.Equal(t, []string{"bbb", "ddd"}, remove)
}
