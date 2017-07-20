package resultstore

import (
	"io/ioutil"
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/diff"
)

const (
	// Temporary directory used to store diff metrics db.
	TEST_DIFF_DIR = "diffs"

	// Temporary boltDB instance containing diff metrics data.
	TEST_DIFF_DB = "diffs.db"

	// Test data for ResultRec.
	TEST_RUN_ID        = "lchoi20170719123456"
	TEST_URL           = "http://www.google.com"
	TEST_RANK          = 1
	TEST_NOPATCH_IMG   = "lchoi20170719123456/nopatch/1/http___www_google_com"
	TEST_WITHPATCH_IMG = "lchoi20170719123456/withpatch/1/http___www_google_com"
)

// Tests the IsReadyForDiff() func and returns a ResultRec with test data.
func testResultRec(t *testing.T) *ResultRec {
	rec := &ResultRec{}
	assert.False(t, rec.IsReadyForDiff())

	rec.RunID = TEST_RUN_ID
	rec.URL = TEST_URL
	rec.Rank = TEST_RANK
	rec.NoPatchImg = TEST_NOPATCH_IMG
	rec.WithPatchImg = TEST_WITHPATCH_IMG
	rec.DiffMetrics = &diff.DiffMetrics{
		NumDiffPixels:    1158328,
		PixelDiffPercent: 98.65482,
		MaxRGBADiffs:     []int{19, 52, 87, 0},
		DimDiffer:        false,
		Diffs: map[string]float32{
			"percent":  98.65482,
			"pixel":    1.158328e+06,
			"combined": 4.4663033,
		},
	}
	assert.True(t, rec.IsReadyForDiff())

	return rec
}

func TestResultStore(t *testing.T) {
	testutils.SmallTest(t)
	rec := testResultRec(t)

	// Initialize the ResultStore.
	diffDir, err := ioutil.TempDir("", TEST_DIFF_DIR)
	assert.NoError(t, err)
	resultStore, err := NewBoltResultStore(diffDir, TEST_DIFF_DB)
	assert.NoError(t, err)

	// Add the ResultRec.
	err = resultStore.Add(TEST_RUN_ID, TEST_URL, rec)
	assert.NoError(t, err)

	// Get the ResultRec verify it's equivalent to what was added.
	storedRec, err := resultStore.Get(TEST_RUN_ID, TEST_URL)
	assert.NoError(t, err)
	assert.Equal(t, rec, storedRec)

	// Remove the run.
	err = resultStore.RemoveRun(TEST_RUN_ID)
	assert.NoError(t, err)
	// Trying to remove a second time should return an error.
	err = resultStore.RemoveRun(TEST_RUN_ID)
	assert.Error(t, err)
	// Calling Get on an empty ResultStore should also return an error.
	_, err = resultStore.Get(TEST_RUN_ID, TEST_URL)
	assert.Error(t, err)
}
