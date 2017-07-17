package dmstore

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/boltdb/bolt"
	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/diff"
)

const (
	// Temporary directory used to store diff metrics db.
	TEST_DIFF_DIR = "diffs"

	// Temporary boltDB instance containing diff metrics data.
	TEST_DIFF_DB = "diffs.db"

	// runID and URL filename for testing.
	TEST_RUN_ID = "rmistry-20170623184523"
	TEST_FILENAME = "http___www_google_com"
)

// Tests the DMStore, namely the constructor, Add, and Remove.
func TestDMStore(t *testing.T) {
	testutils.SmallTest(t)

	// Set up the DMStore.
	diffDir, err := ioutil.TempDir("", TEST_DIFF_DIR)
	assert.NoError(t, err)
	dmStore, err := NewDMStore(diffDir, TEST_DIFF_DB)
	assert.NoError(t, err)

	// Add the diff metrics to the boltDB.
	expected := &diff.DiffMetrics{
		NumDiffPixels: 1158328,
		PixelDiffPercent: 98.65482,
		MaxRGBADiffs: []int{19, 52, 87, 0},
		DimDiffer: false,
		Diffs: map[string]float32{
			"percent":98.65482,
			"pixel": 1.158328e+06,
			"combined":4.4663033,
		},
	}
	err = dmStore.Add(TEST_RUN_ID, TEST_FILENAME, expected)
	assert.NoError(t, err)

	// Get the boltDB used by the DMStore to verify the results.
	diffs := dmStore.diffs

	// Deserialize the diff metrics stored in the boltDB and check that they are
	// equivalent to what was added.
	result := &diff.DiffMetrics{}
	viewFn := func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(TEST_RUN_ID))
		bytes := b.Get([]byte(TEST_FILENAME))
		if err := json.Unmarshal(bytes, &result); err != nil {
			return err
		}
		return nil
	}
	err = diffs.View(viewFn)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)

	// Remove the bucket and check that it no longer exists.
	err = dmStore.Remove(TEST_RUN_ID)
	assert.NoError(t, err)
	viewFn = func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(TEST_RUN_ID))
		assert.Nil(t, b)
		return nil
	}
	diffs.View(viewFn)

	// DMStore is now empty, so Remove should return an error.
	err = dmStore.Remove(TEST_RUN_ID)
	assert.Error(t, err)
}
