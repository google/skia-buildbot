package boltutil

import (
	"io/ioutil"
	"path"
	"testing"

	"github.com/boltdb/bolt"
	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

const (
	TEST_DATA_DIR   = "./testdata"
	TEST_INDEX_NAME = "testindex"
)

func TestKeyConflicts(t *testing.T) {
	testutils.MediumTest(t)

	// Add a number of issues
	baseDir, err := ioutil.TempDir("", "temp-boltutil")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, TEST_DATA_DIR)

	db, err := bolt.Open(path.Join(baseDir, "test.db"), 0600, nil)
	assert.NoError(t, err)
	defer db.Close()

	ib, err := NewIndexedBucket(&Config{
		DB:      db,
		Name:    "testbucket",
		Indices: []string{TEST_INDEX_NAME},
		Codec:   util.JSONCodec(&exampleRec{}),
	})
	assert.NoError(t, err)

	assert.NoError(t, ib.Insert([]Record{newExample("a", "aaa")}))
	assert.NoError(t, ib.Insert([]Record{newExample("aa", "aa")}))
	assert.NoError(t, ib.Insert([]Record{newExample("aaa", "a")}))

	found, err := ib.ReadIndex(TEST_INDEX_NAME, []string{"a", "aa", "aaa"})
	assert.NoError(t, err)
	assert.Equal(t, map[string][]string{"a": {"aaa"}, "aa": {"aa"}, "aaa": {"a"}}, found)

	assert.NoError(t, ib.Delete([]string{"a", "aa", "aaa"}))

	assert.NoError(t, ib.Insert([]Record{newExample("a", "aaa")}))
	assert.NoError(t, ib.Insert([]Record{newExample("ab", "aa")}))
	assert.NoError(t, ib.Insert([]Record{newExample("aac", "a")}))

	found, err = ib.ReadIndex(TEST_INDEX_NAME, []string{"a", "aa", "aaa"})
	assert.NoError(t, err)
	assert.Equal(t, map[string][]string{"a": {"aac"}, "aa": {"ab"}, "aaa": {"a"}}, found)
}

type exampleRec struct {
	ID  string
	Val string
}

func newExample(id, val string) *exampleRec {
	return &exampleRec{
		ID:  id,
		Val: val,
	}
}

func (e *exampleRec) Key() string {
	return e.ID
}

func (e *exampleRec) IndexValues() map[string][]string {
	return map[string][]string{TEST_INDEX_NAME: []string{e.Val}}
}
