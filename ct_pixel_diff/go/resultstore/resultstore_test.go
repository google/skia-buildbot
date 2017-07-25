package resultstore

import (
	"io/ioutil"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/diff"
)

const (
	// Temporary directory used to store diff metrics db.
	TEST_DIFF_DIR = "diffs"

	// Temporary boltDB instance containing diff metrics data.
	TEST_DIFF_DB = "diffs.db"

	// Test runID and url for querying and updating the db.
	TEST_RUN_ID = "lchoi-20170719123456"
	TEST_URL    = "http://www.google.com"

	// Test runID for querying runs that do not exist in the db.
	TEST_NOEXIST_RUN_ID = "lchoi-00000000000000"

	// Test url for querying urls that do not exist in a bucket, and for testing
	// GetAll.
	TEST_URL_TWO = "http://www.youtube.com"

	// Secondary runID for testing GetRunIDs.
	TEST_RUN_ID_TWO = "lchoi-20170721123456"
)

// Tests the IsReadyForDiff func and returns a ResultRec with test data.
func createResultRec(t *testing.T) *ResultRec {
	rec := &ResultRec{}
	assert.False(t, rec.IsReadyForDiff())

	rec.RunID = TEST_RUN_ID
	rec.URL = TEST_URL
	rec.Rank = 1
	rec.NoPatchImg = "lchoi20170719123456/nopatch/1/http___www_google_com"
	rec.WithPatchImg = "lchoi20170719123456/withpatch/1/http___www_google_com"
	assert.True(t, rec.IsReadyForDiff())

	return rec
}

// Tests the NewBoltResultStore constructor and returns a ResultStore.
func createBoltResultStore(t *testing.T) ResultStore {
	// Set up the temporary directory and create the ResultStore.
	diffDir, err := ioutil.TempDir("", TEST_DIFF_DIR)
	assert.NoError(t, err)
	resultStore, err := NewBoltResultStore(diffDir, TEST_DIFF_DB)
	assert.NoError(t, err)
	return resultStore
}

func TestGetAndPut(t *testing.T) {
	testutils.SmallTest(t)

	// Initialize the ResultStore.
	resultStore := createBoltResultStore(t)

	// Querying an empty database should return nil.
	nilRecord, err := resultStore.Get(TEST_RUN_ID, TEST_URL)
	assert.NoError(t, err)
	assert.Nil(t, nilRecord)

	// Add the ResultRec.
	rec := createResultRec(t)
	err = resultStore.Put(TEST_RUN_ID, TEST_URL, rec)
	assert.NoError(t, err)

	// Get the ResultRec and verify it's equivalent to what was inserted.
	storedRec, err := resultStore.Get(TEST_RUN_ID, TEST_URL)
	assert.NoError(t, err)
	assert.Equal(t, rec, storedRec)

	// If the run doesn't exist in the database, Get should return nil.
	nilRecord, err = resultStore.Get(TEST_NOEXIST_RUN_ID, TEST_URL)
	assert.NoError(t, err)
	assert.Nil(t, nilRecord)

	// If the url doesn't exist in the run bucket, Get should return nil.
	nilRecord, err = resultStore.Get(TEST_RUN_ID, TEST_URL_TWO)
	assert.NoError(t, err)
	assert.Nil(t, nilRecord)

	// Getting a ResultRec, updating it, and putting it back should overwrite the
	// current record.
	updateRec := rec
	updateRec.DiffMetrics = &diff.DiffMetrics{
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
	err = resultStore.Put(TEST_RUN_ID, TEST_URL, updateRec)
	assert.NoError(t, err)
	storedRec, err = resultStore.Get(TEST_RUN_ID, TEST_URL)
	assert.NoError(t, err)
	assert.Equal(t, updateRec, storedRec)
}

func TestGetAll(t *testing.T) {
	testutils.SmallTest(t)

	// Initialize the ResultStore.
	resultStore := createBoltResultStore(t)

	// Create two ResultRecs and modify the URL of the second one.
	recOne := createResultRec(t)
	recTwo := createResultRec(t)
	recTwo.URL = TEST_URL_TWO

	// Put them under different URLs so there are multiple entries associated
	// with a run.
	err := resultStore.Put(TEST_RUN_ID, TEST_URL, recOne)
	assert.NoError(t, err)
	err = resultStore.Put(TEST_RUN_ID, TEST_URL_TWO, recTwo)
	assert.NoError(t, err)

	// Verify that returned slice has proper length and the entries are equivalent
	// to what was added.
	recs, err := resultStore.GetAll(TEST_RUN_ID)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(recs))
	assert.Equal(t, recOne, recs[0])
	assert.Equal(t, recTwo, recs[1])

	// If the run doesn't exist in the database, GetAll should return an empty
	// list.
	recs, err = resultStore.GetAll(TEST_NOEXIST_RUN_ID)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(recs))
}

func TestGetRunIDs(t *testing.T) {
	testutils.SmallTest(t)

	// Initialize the ResultStore.
	resultStore := createBoltResultStore(t)

	// Calling GetRunIDs on an empty ResultStore should return an empty list.
	runIDs, err := resultStore.GetRunIDs(time.Time{}, time.Time{})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(runIDs))

	// Create two ResultRecs and modify the runID of the second one.
	recOne := createResultRec(t)
	recTwo := createResultRec(t)
	recTwo.RunID = TEST_RUN_ID_TWO

	// Add them to the ResultStore.
	err = resultStore.Put(TEST_RUN_ID, TEST_URL, recOne)
	assert.NoError(t, err)
	err = resultStore.Put(TEST_RUN_ID_TWO, TEST_URL, recTwo)
	assert.NoError(t, err)

	// Specifying BeginningOfTime and time.Now() should return all the runIDs.
	runIDs, err = resultStore.GetRunIDs(BeginningOfTime, time.Now())
	assert.NoError(t, err)
	assert.Equal(t, 2, len(runIDs))
	assert.Equal(t, TEST_RUN_ID, runIDs[0])
	assert.Equal(t, TEST_RUN_ID_TWO, runIDs[1])

	// Falls in between the timestamps of the two runIDs.
	splitTime := time.Date(2017, time.July, 20, 0, 0, 0, 0, time.UTC)

	runIDs, err = resultStore.GetRunIDs(BeginningOfTime, splitTime)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(runIDs))
	assert.Equal(t, TEST_RUN_ID, runIDs[0])

	runIDs, err = resultStore.GetRunIDs(splitTime, time.Now())
	assert.NoError(t, err)
	assert.Equal(t, 1, len(runIDs))
	assert.Equal(t, TEST_RUN_ID_TWO, runIDs[0])
}

func TestRemoveRun(t *testing.T) {
	testutils.SmallTest(t)

	// Initialize the ResultStore.
	resultStore := createBoltResultStore(t)

	// Add the ResultRec.
	rec := createResultRec(t)
	err := resultStore.Put(TEST_RUN_ID, TEST_URL, rec)
	assert.NoError(t, err)

	// If the run doesn't exist in the database, RemoveRun should return an error.
	err = resultStore.RemoveRun(TEST_NOEXIST_RUN_ID)
	assert.Error(t, err)

	// Calling RemoveRun on a valid run should work, and calling it on an empty db
	// should return an error.
	err = resultStore.RemoveRun(TEST_RUN_ID)
	assert.NoError(t, err)
	err = resultStore.RemoveRun(TEST_RUN_ID)
	assert.Error(t, err)
}
