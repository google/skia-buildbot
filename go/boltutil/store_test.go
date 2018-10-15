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
	TEST_DATA_DIR    = "./testdata"
	TEST_INDEX_ONE   = "testindex-1"
	TEST_INDEX_TWO   = "testindex-2"
	TEST_BUCKET_NAME = "testbucket"
)

var currentTestIndices []string

// exampleRec is a test implementation of Record.
type exampleRec struct {
	ID    string
	Val_1 string
	Val_2 string
}

func newExample(id, val_1, val_2 string) *exampleRec {
	return &exampleRec{
		ID:    id,
		Val_1: val_1,
		Val_2: val_2,
	}
}

func (e *exampleRec) Key() string {
	return e.ID
}

func (e *exampleRec) IndexValues() map[string][]string {
	ret := map[string][]string{}
	for _, idx := range currentTestIndices {
		switch idx {
		case TEST_INDEX_ONE:
			ret[idx] = []string{e.Val_1}
		case TEST_INDEX_TWO:
			ret[idx] = []string{e.Val_2}
		}
	}
	return ret
}

func TestKeyConflicts(t *testing.T) {
	testutils.MediumTest(t)

	testIndices := []string{TEST_INDEX_ONE}
	ib, baseDir, _ := newIndexedBucket(t, testIndices)
	defer testutils.RemoveAll(t, baseDir)
	defer func() { assert.NoError(t, ib.DB.Close()) }()

	currentTestIndices = util.CopyStringSlice(testIndices)
	assert.NoError(t, ib.Insert([]Record{
		newExample("a", "aaa", ""),
		newExample("aa", "aa", ""),
		newExample("aaa", "a", ""),
	}))

	found, err := ib.ReadIndex(TEST_INDEX_ONE, []string{"a", "aa", "aaa"})
	assert.NoError(t, err)
	assert.Equal(t, map[string][]string{"a": {"aaa"}, "aa": {"aa"}, "aaa": {"a"}}, found)

	assert.NoError(t, ib.Delete([]string{"a", "aa", "aaa"}))

	assert.NoError(t, ib.Insert([]Record{
		newExample("a", "aaa", ""),
		newExample("ab", "aa", ""),
		newExample("aac", "a", ""),
	}))

	found, err = ib.ReadIndex(TEST_INDEX_ONE, []string{"a", "aa", "aaa"})
	assert.NoError(t, err)
	assert.Equal(t, map[string][]string{"a": {"aac"}, "aa": {"ab"}, "aaa": {"a"}}, found)
}

func TestIndexedBucket(t *testing.T) {
	testutils.MediumTest(t)

	ib, baseDir, dbFileName := newIndexedBucket(t, []string{TEST_INDEX_ONE})
	defer testutils.RemoveAll(t, baseDir)

	currentTestIndices = []string{TEST_INDEX_ONE}
	inputRecs := []Record{
		newExample("id_01", "val_01", "val_11"),
		newExample("id_02", "val_01", "val_11"),
		newExample("id_03", "val_02", "val_12"),
		newExample("id_04", "val_02", "val_12"),
		newExample("id_05", "val_03", "val_12"),
	}
	assert.NoError(t, ib.Insert(inputRecs))

	// Read from the first index.
	found, err := ib.ReadIndex(TEST_INDEX_ONE, []string{"val_01", "val_02", "val_03"})
	assert.NoError(t, err)
	assert.Equal(t, map[string][]string{
		"val_01": {"id_01", "id_02"},
		"val_02": {"id_03", "id_04"},
		"val_03": {"id_05"}}, found)

	// Retrieve all records.
	foundAll, total, err := ib.List(0, -1)
	assert.NoError(t, err)
	assert.Equal(t, len(inputRecs), total)
	assert.Equal(t, len(inputRecs), len(foundAll))
	for i, rec := range inputRecs {
		assert.Equal(t, rec, foundAll[i])
	}

	// Delete a record and make sure Read works correctly.
	assert.NoError(t, ib.Delete([]string{"id_03"}))
	foundRec, err := ib.Read([]string{"id_03", "id_01"})
	assert.NoError(t, err)
	assert.Equal(t, []Record{nil, inputRecs[0]}, foundRec)

	// Read the raw record and make sure they are correct.
	foundBytes, err := ib.ReadRaw("id_01")
	assert.NoError(t, err)
	decodedRec, err := ib.codec.Decode(foundBytes)
	assert.NoError(t, err)
	assert.Equal(t, inputRecs[0], decodedRec)

	foundBytes, err = ib.ReadRaw("id_03")
	assert.NoError(t, err)
	assert.Nil(t, foundBytes)

	found, err = ib.ReadIndex(TEST_INDEX_ONE, []string{"val_01", "val_02", "val_03"})
	assert.NoError(t, err)
	compReadIndex(t, map[string][]string{
		"val_01": {"id_01", "id_02"},
		"val_02": {"id_04"},
		"val_03": {"id_05"}}, found)

	// Update an existing record.
	assert.NoError(t, ib.Update(inputRecs[:1], func(tx *bolt.Tx, recs []Record) error {
		rec := recs[0].(*exampleRec)
		rec.Val_1 = rec.Val_1 + "--1"
		return nil
	}))

	found, err = ib.ReadIndex(TEST_INDEX_ONE, []string{"val_01--1", "val_01"})
	assert.NoError(t, err)
	assert.Equal(t, map[string][]string{
		"val_01--1": {"id_01"},
		"val_01":    {"id_02"}}, found)

	// And an index to that database.
	assert.NoError(t, ib.DB.Close())

	// Make IndexValues return values for both indices.
	currentTestIndices = []string{TEST_INDEX_ONE, TEST_INDEX_TWO}

	db, err := bolt.Open(dbFileName, 0600, nil)
	assert.NoError(t, err)

	ib, err = NewIndexedBucket(&Config{
		DB:      db,
		Name:    TEST_BUCKET_NAME,
		Indices: util.CopyStringSlice(currentTestIndices),
		Codec:   util.JSONCodec(&exampleRec{}),
	})

	expectedIDs := map[string][]string{
		"val_11": {"id_01", "id_02"},
		"val_12": {"id_04", "id_05"},
	}
	found, err = ib.ReadIndex(TEST_INDEX_TWO, []string{"val_11", "val_12"})
	assert.NoError(t, err)
	compReadIndex(t, expectedIDs, found)

	// Delete all indices by hand and re-index the db.
	_ = ib.DB.Update(func(tx *bolt.Tx) error {
		assert.NoError(t, tx.DeleteBucket([]byte(TEST_INDEX_ONE)))
		assert.NoError(t, tx.DeleteBucket([]byte(TEST_INDEX_TWO)))
		return nil
	})

	assert.Panics(t, func() { _, _ = ib.ReadIndex(TEST_INDEX_TWO, []string{"val_11", "val_12"}) })
	assert.NoError(t, ib.ReIndex())
	found, err = ib.ReadIndex(TEST_INDEX_TWO, []string{"val_11", "val_12"})
	assert.NoError(t, err)
	assert.Equal(t, expectedIDs, found)

	assert.NoError(t, db.Close())

	// Remove an index which will also force reading the meta data.
	currentTestIndices = []string{TEST_INDEX_ONE}

	db, err = bolt.Open(dbFileName, 0600, nil)
	assert.NoError(t, err)

	ib, err = NewIndexedBucket(&Config{
		DB:      db,
		Name:    TEST_BUCKET_NAME,
		Indices: util.CopyStringSlice(currentTestIndices),
		Codec:   util.JSONCodec(&exampleRec{}),
	})

	found, err = ib.ReadIndex(TEST_INDEX_ONE, []string{"val_01--1", "val_01"})
	assert.NoError(t, err)
	assert.Equal(t, map[string][]string{
		"val_01--1": {"id_01"},
		"val_01":    {"id_02"}}, found)
	assert.NoError(t, db.Close())
}

func compReadIndex(t *testing.T, exp map[string][]string, actual map[string][]string) {
	assert.Equal(t, len(exp), len(actual))
	for key, vals := range exp {
		assert.NotNil(t, actual[key])
		assert.Equal(t, util.NewStringSet(vals), util.NewStringSet(actual[key]))
	}
}

func newIndexedBucket(t *testing.T, indices []string) (*IndexedBucket, string, string) {
	baseDir, err := ioutil.TempDir(".", TEST_DATA_DIR)
	assert.NoError(t, err)

	dbFileName := path.Join(baseDir, "test.db")
	db, err := bolt.Open(dbFileName, 0600, nil)
	assert.NoError(t, err)

	ib, err := NewIndexedBucket(&Config{
		DB:      db,
		Name:    TEST_BUCKET_NAME,
		Indices: indices,
		Codec:   util.JSONCodec(&exampleRec{}),
	})
	assert.NoError(t, err)
	return ib, baseDir, dbFileName
}
