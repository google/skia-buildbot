package ptracestore

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"sort"
	"testing"
	"time"

	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/constants"
	"go.skia.org/infra/perf/go/types"

	"github.com/stretchr/testify/assert"
)

var (
	tmpDir = ""
)

func setupStoreDir(t *testing.T) {
	var err error
	tmpDir, err = ioutil.TempDir("", "ptracestore")
	assert.NoError(t, err)
}

func cleanup() {
	if err := os.RemoveAll(tmpDir); err != nil {
		fmt.Printf("Failed to clean up %q: %s", tmpDir, err)
	}
}

func TestAdd(t *testing.T) {
	testutils.MediumTest(t)
	setupStoreDir(t)
	defer cleanup()
	now := time.Now()

	d, err := New(tmpDir)
	assert.NoError(t, err)
	commitID := &cid.CommitID{
		Offset: constants.COMMITS_PER_TILE + 1,
		Source: "master",
	}
	values := map[string]float32{
		",config=565,test=foo,":  1.23,
		",config=8888,test=foo,": 3.21,
	}
	err = d.Add(commitID, values, "gs://skia-perf/nano-json-v1/blah/blah.json", now)
	assert.NoError(t, err)

	source, value, err := d.Details(commitID, ",config=565,test=foo,")
	assert.NoError(t, err)
	assert.Equal(t, "gs://skia-perf/nano-json-v1/blah/blah.json", source)
	assert.Equal(t, float32(1.23), value)

	source, value, err = d.Details(commitID, ",config=8888,test=foo,")
	assert.NoError(t, err)
	assert.Equal(t, "gs://skia-perf/nano-json-v1/blah/blah.json", source)
	assert.Equal(t, float32(3.21), value)

	source, value, err = d.Details(commitID, ",something=unknown,")
	assert.Error(t, err)

	assert.Equal(t, 1, len(d.cache))

	// Add new values that would go into a different tile.
	commitID2 := &cid.CommitID{
		Offset: 2*constants.COMMITS_PER_TILE + 1,
		Source: "master",
	}
	values2 := map[string]float32{
		",config=565,test=foo,":  3.14,
		",config=8888,test=foo,": 3.15,
	}
	err = d.Add(commitID2, values2, "gs://skia-perf/nano-json-v1/blah2/blah.json", now)
	assert.NoError(t, err)

	assert.Equal(t, 2, len(d.cache))

	source, value, err = d.Details(commitID2, ",config=565,test=foo,")
	assert.NoError(t, err)
	assert.Equal(t, "gs://skia-perf/nano-json-v1/blah2/blah.json", source)
	assert.Equal(t, float32(3.14), value)

	source, value, err = d.Details(commitID2, ",something=unknown,")
	assert.Error(t, err)

	// Now overwrite values we've already written.
	values2 = map[string]float32{
		",config=565,test=foo,":  9.99,
		",config=8888,test=foo,": 9.98,
	}
	err = d.Add(commitID2, values2, "gs://skia-perf/nano-json-v1/blah3/blah.json", now)
	assert.NoError(t, err)

	// Confirm we get the last values written.
	source, value, err = d.Details(commitID2, ",config=565,test=foo,")
	assert.NoError(t, err)
	assert.Equal(t, "gs://skia-perf/nano-json-v1/blah3/blah.json", source)
	assert.Equal(t, float32(9.99), value)
}

func TestBuildMapper(t *testing.T) {
	testutils.SmallTest(t)
	commitIDs := []*cid.CommitID{
		{
			Source: "master",
			Offset: 49,
		},
		{
			Source: "master",
			Offset: 50,
		},
		{
			Source: "master",
			Offset: 51,
		},
	}

	want := map[string]*tileMap{
		"master-000000.bdb": {
			commitID: &cid.CommitID{
				Source: "master",
				Offset: 49,
			},
			idxmap: map[int]int{
				49: 0,
			},
		},
		"master-000001.bdb": {
			commitID: &cid.CommitID{
				Source: "master",
				Offset: 50,
			},
			idxmap: map[int]int{
				0: 1,
				1: 2,
			},
		},
	}
	got := buildMapper(commitIDs)
	assert.Equal(t, got, want)

	commitIDs = []*cid.CommitID{}
	want = map[string]*tileMap{}
	got = buildMapper(commitIDs)
	assert.Equal(t, got, want)
}

func TestMatch(t *testing.T) {
	testutils.MediumTest(t)
	setupStoreDir(t)
	defer cleanup()

	now := time.Now()
	d, err := New(tmpDir)
	assert.NoError(t, err)
	commitID1 := &cid.CommitID{
		Offset: 1,
		Source: "master",
	}
	values := map[string]float32{
		",config=565,test=foo,":        1.23,
		",config=8888,test=foo,":       3.21,
		",arch=x86,source_type=image,": 5.55,
	}
	err = d.Add(commitID1, values, "gs://foo", now)
	assert.NoError(t, err)

	commitID2 := &cid.CommitID{
		Offset: 2,
		Source: "master",
	}
	values = map[string]float32{
		",config=565,test=foo,":        2.34,
		",config=8888,test=foo,":       5.43,
		",arch=x86,source_type=image,": 6.66,
	}
	err = d.Add(commitID2, values, "gs://foo", now)
	assert.NoError(t, err)

	commitID3 := &cid.CommitID{
		Offset: constants.COMMITS_PER_TILE + 3,
		Source: "master",
	}
	values = map[string]float32{
		",config=565,test=foo,":        3.45,
		",config=8888,test=foo,":       9.10,
		",arch=x86,source_type=image,": 7.77,
	}
	err = d.Add(commitID3, values, "gs://foo", now)
	assert.NoError(t, err)

	// A commit with no data.
	commitID4 := &cid.CommitID{
		Offset: constants.COMMITS_PER_TILE + 5,
		Source: "master",
	}

	_, value, err := d.Details(commitID1, ",config=565,test=foo,")
	assert.NoError(t, err)
	assert.Equal(t, float32(1.23), value)

	// Query that matches just one trace.
	q, err := query.New(url.Values{
		"config": []string{"565"},
	})
	assert.NoError(t, err)
	commits := []*cid.CommitID{commitID1, commitID2, commitID3, commitID4}
	traces, err := d.Match(commits, q.Matches, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(traces))
	assert.Equal(t, 4, len(traces[",config=565,test=foo,"]))
	assert.Equal(t, types.Trace{1.23, 2.34, 3.45, vec32.MISSING_DATA_SENTINEL}, traces[",config=565,test=foo,"])

	// Match both traces.
	q, err = query.New(url.Values{
		"test": []string{"foo"},
	})
	assert.NoError(t, err)
	traces, err = d.Match(commits, q.Matches, nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(traces))
	assert.Equal(t, 4, len(traces[",config=565,test=foo,"]))
	assert.Equal(t, types.Trace{1.23, 2.34, 3.45, vec32.MISSING_DATA_SENTINEL}, traces[",config=565,test=foo,"])
	assert.Equal(t, types.Trace{3.21, 5.43, 9.10, vec32.MISSING_DATA_SENTINEL}, traces[",config=8888,test=foo,"])

	// Query that returns only missing values, including a tile that doesn't exist.
	commitID5 := &cid.CommitID{
		Offset: 2*constants.COMMITS_PER_TILE + 6,
		Source: "master",
	}
	commits = []*cid.CommitID{commitID4, commitID5}
	traces, err = d.Match(commits, q.Matches, nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(traces))
	assert.Equal(t, 2, len(traces[",config=565,test=foo,"]))
	assert.Equal(t, types.Trace{vec32.MISSING_DATA_SENTINEL, vec32.MISSING_DATA_SENTINEL}, traces[",config=565,test=foo,"])
	assert.Equal(t, types.Trace{vec32.MISSING_DATA_SENTINEL, vec32.MISSING_DATA_SENTINEL}, traces[",config=8888,test=foo,"])

	// Match all traces with an empty query.
	q, err = query.New(url.Values{})
	assert.NoError(t, err)
	commits = []*cid.CommitID{commitID1, commitID2, commitID3, commitID4}
	traces, err = d.Match(commits, q.Matches, nil)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(traces))
	assert.Equal(t, 4, len(traces[",config=565,test=foo,"]))
	assert.Equal(t, types.Trace{1.23, 2.34, 3.45, vec32.MISSING_DATA_SENTINEL}, traces[",config=565,test=foo,"])
	assert.Equal(t, types.Trace{3.21, 5.43, 9.10, vec32.MISSING_DATA_SENTINEL}, traces[",config=8888,test=foo,"])
	assert.Equal(t, types.Trace{5.55, 6.66, 7.77, vec32.MISSING_DATA_SENTINEL}, traces[",arch=x86,source_type=image,"])

	// Match none of the traces.
	q, err = query.New(url.Values{"bar": []string{"baz"}})
	assert.NoError(t, err)
	commits = []*cid.CommitID{commitID1, commitID2, commitID3, commitID4}
	traces, err = d.Match(commits, q.Matches, nil)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(traces))

	// Match exact.
	commits = []*cid.CommitID{commitID1, commitID2, commitID3, commitID4}
	keys := []string{",config=565,test=foo,", ",config=8888,test=foo,"}
	sort.Strings(keys)
	matches := func(key string) bool {
		i := sort.SearchStrings(keys, key)
		if i > len(keys)-1 {
			return false
		}
		return keys[i] == key
	}
	traces, err = d.Match(commits, matches, nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(traces))
	assert.Equal(t, types.Trace{1.23, 2.34, 3.45, vec32.MISSING_DATA_SENTINEL}, traces[",config=565,test=foo,"])
	assert.Equal(t, types.Trace{3.21, 5.43, 9.10, vec32.MISSING_DATA_SENTINEL}, traces[",config=8888,test=foo,"])
}
