package dmstore

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/boltdb/bolt"
	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore"
	"go.skia.org/infra/golden/go/mocks"
)

const (
	// Temporary directory representing GS basedir.
	TEST_BASE_DIR = "images"

	// GS bucket.
	TEST_GS_BUCKET = "cluster-telemetry"

	//GS image directory.
	TEST_GS_IMAGE_DIR = "tasks/benchmark_runs"

	// Temporary directory used to store diff metrics db.
	TEST_DIFF_DIR = "diffs"

	// Temporary boltDB instance containing diff metrics data.
	TEST_DIFF_DB = "diffs.db"
)

var (
	// Temporary directories.
	baseDir = ""
	diffDir = ""
)

func setUpDMStore(t *testing.T) *DMStore {
	// Set up the DiffStore used to instantiate the DMStore.
	client := mocks.GetHTTPClient(t)
	baseDir, err := ioutil.TempDir("", TEST_BASE_DIR)
	assert.NoError(t, err)
	diffStore, err := diffstore.NewMemDiffStore(client, baseDir, []string{TEST_GS_BUCKET}, TEST_GS_IMAGE_DIR, 10, diffstore.GetCommonRunUrl, diffstore.GetCommonRunUrlImgName, diffstore.GetNoAndWithPatch)
	assert.NoError(t, err)

	// Create the DMStore.
	diffDir, err = ioutil.TempDir("", TEST_DIFF_DIR)
	assert.NoError(t, err)
	dmStore, err := NewDMStore(diffStore, diffDir, TEST_DIFF_DB)
	assert.NoError(t, err)
	return dmStore
}

// Removes the temporary directories.
func cleanUp(t *testing.T) {
	err := os.RemoveAll(baseDir)
	assert.NoError(t, err)

	err = os.RemoveAll(diffDir)
	assert.NoError(t, err)
}

func TestAdd(t *testing.T) {
	testutils.MediumTest(t)

	dmStore := setUpDMStore(t)
	defer cleanUp(t)
	err := dmStore.Add("rmistry-20170623184523", "http___www_google_com")
	assert.NoError(t, err)

	// Get the boltDB used by the DMStore to verify the results.
	diffs := dmStore.db
	results := &diff.DiffMetrics{}
	err = diffs.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("rmistry-20170623184523"))
		bytes := b.Get([]byte("http___www_google_com"))
		if err := json.Unmarshal(bytes, &results); err != nil {
			return err
		}
		return nil
	})
	assert.NoError(t, err)

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
	assert.Equal(t, expected, results)
}

func TestRemove(t *testing.T) {
	testutils.MediumTest(t)
	dmStore := setUpDMStore(t)
	defer cleanUp(t)

	// DMStore is empty, so Remove should cause error.
	err := dmStore.Remove("rmistry-20170623184523")
	assert.Error(t, err)

	err = dmStore.Add("rmistry-20170623184523", "http___www_google_com")
	assert.NoError(t, err)
	err = dmStore.Remove("rmistry-20170623184523")
	assert.NoError(t, err)

	// Get the boltDB used by the DMStore to verify removal.
	diffs := dmStore.db
	err = diffs.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("rmistry-20170623184523"))
		assert.Nil(t, b)
		return nil
	})
}
