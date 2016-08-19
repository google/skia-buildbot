package ptracestore

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

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
	setupStoreDir(t)
	defer cleanup()

	d := New(tmpDir)
	commitID := &CommitID{
		Offset: COMMITS_PER_TILE + 1,
		Source: "master",
	}
	values := map[string]float32{
		",config=565,test=foo,":  1.23,
		",config=8888,test=foo,": 3.21,
	}
	err := d.Add(commitID, values, "gs://skia-perf/nano-json-v1/blah/blah.json")
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

	assert.Equal(t, 1, d.cache.Len())

	// Add new values that would go into a different tile.
	commitID2 := &CommitID{
		Offset: 2*COMMITS_PER_TILE + 1,
		Source: "master",
	}
	values2 := map[string]float32{
		",config=565,test=foo,":  3.14,
		",config=8888,test=foo,": 3.15,
	}
	err = d.Add(commitID2, values2, "gs://skia-perf/nano-json-v1/blah2/blah.json")
	assert.NoError(t, err)

	assert.Equal(t, 2, d.cache.Len())

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
	err = d.Add(commitID2, values2, "gs://skia-perf/nano-json-v1/blah3/blah.json")
	assert.NoError(t, err)

	// Confirm we get the last values written.
	source, value, err = d.Details(commitID2, ",config=565,test=foo,")
	assert.NoError(t, err)
	assert.Equal(t, "gs://skia-perf/nano-json-v1/blah3/blah.json", source)
	assert.Equal(t, float32(9.99), value)

}
