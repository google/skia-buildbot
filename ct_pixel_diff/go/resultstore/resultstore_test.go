package resultstore

import (
	"io/ioutil"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/ct_pixel_diff/go/dynamicdiff"
	"go.skia.org/infra/go/testutils"
)

const (
	// Temporary directory used to store diff metrics db.
	TEST_DIFF_DIR = "diffs"

	// Temporary boltDB instance containing diff metrics data.
	TEST_DIFF_DB = "diffs.db"

	// Test runIDs.
	TEST_RUN_ID         = "lchoi-20170719123456"
	TEST_RUN_ID_TWO     = "lchoi-20170721123456"
	TEST_NOEXIST_RUN_ID = "lchoi-00000000000000"

	// Test URLs.
	TEST_URL       = "http://www.google.com"
	TEST_URL_TWO   = "http://www.youtube.com"
	TEST_URL_THREE = "http://www.facebook.com"
)

// Tests the IsReadyForDiff func and returns a ResultRec with test data.
func createResultRec(t *testing.T) *ResultRec {
	rec := &ResultRec{}
	assert.False(t, rec.HasBothImages())

	rec.RunID = TEST_RUN_ID
	rec.URL = TEST_URL
	rec.Rank = 1
	rec.NoPatchImg = "lchoi20170719123456/nopatch/1/http___www_google_com"
	rec.WithPatchImg = "lchoi20170719123456/withpatch/1/http___www_google_com"
	rec.DiffMetrics = &dynamicdiff.DynamicDiffMetrics{}
	assert.True(t, rec.HasBothImages())

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
	testutils.MediumTest(t)

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
	updateRec.DiffMetrics = &dynamicdiff.DynamicDiffMetrics{
		NumDiffPixels:    1158328,
		PixelDiffPercent: 98.65482,
		MaxRGBDiffs:      []int{19, 52, 87, 0},
		NumStaticPixels:  100,
		NumDynamicPixels: 200,
	}
	err = resultStore.Put(TEST_RUN_ID, TEST_URL, updateRec)
	assert.NoError(t, err)
	storedRec, err = resultStore.Get(TEST_RUN_ID, TEST_URL)
	assert.NoError(t, err)
	assert.Equal(t, updateRec, storedRec)
}

func TestGetAll(t *testing.T) {
	testutils.MediumTest(t)

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
	testutils.MediumTest(t)

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

	// Add them to the ResultStore under different runIDs.
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
	testutils.MediumTest(t)

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

func TestFillCache(t *testing.T) {
	testutils.MediumTest(t)

	// Initialize the ResultStore.
	resultStore := createBoltResultStore(t)

	// Add the ResultRec.
	rec := createResultRec(t)
	err := resultStore.Put(TEST_RUN_ID, TEST_URL, rec)
	assert.NoError(t, err)

	// Clear the cache and fill it.
	boltStore := resultStore.(*BoltResultStore)
	boltStore.cache = map[string][]*ResultRec{}
	err = boltStore.fillCache()
	assert.NoError(t, err)
	assert.Equal(t, rec, boltStore.cache[TEST_RUN_ID][0])
}

func TestGetFiltered(t *testing.T) {
	testutils.MediumTest(t)

	// Initialize the ResultStore.
	resultStore := createBoltResultStore(t)

	// Create two ResultRecs and modify the URL of the second one.
	recOne := createResultRec(t)
	recOne.DiffMetrics = &dynamicdiff.DynamicDiffMetrics{
		PixelDiffPercent: 50,
	}
	recTwo := createResultRec(t)
	recTwo.DiffMetrics = &dynamicdiff.DynamicDiffMetrics{
		PixelDiffPercent: 100,
	}
	recTwo.URL = TEST_URL_TWO

	// Put them under different URLs so there are multiple entries associated
	// with a run.
	err := resultStore.Put(TEST_RUN_ID, TEST_URL, recOne)
	assert.NoError(t, err)
	err = resultStore.Put(TEST_RUN_ID, TEST_URL_TWO, recTwo)
	assert.NoError(t, err)

	// Verify the cache contains the right data.
	results, _, err := resultStore.GetFiltered(TEST_RUN_ID, 0, 0, 100)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(results))
	assert.Equal(t, recOne, results[0])
	assert.Equal(t, recTwo, results[1])

	results, _, err = resultStore.GetFiltered(TEST_RUN_ID, 0, 0, 50)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(results))
	assert.Equal(t, recOne, results[0])

	results, _, err = resultStore.GetFiltered(TEST_RUN_ID, 0, 51, 100)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(results))
	assert.Equal(t, recTwo, results[0])
}

func TestSortRun(t *testing.T) {
	testutils.MediumTest(t)

	// Initialize the ResultStore.
	resultStore := createBoltResultStore(t)

	// Create two ResultRecs.
	recOne := createResultRec(t)
	recOne.DiffMetrics = &dynamicdiff.DynamicDiffMetrics{
		NumDiffPixels:    2,
		PixelDiffPercent: 25,
		MaxRGBDiffs:      []int{0, 128, 255},
	}
	recTwo := createResultRec(t)
	recTwo.URL = TEST_URL_TWO
	recTwo.Rank = 2
	recTwo.DiffMetrics = &dynamicdiff.DynamicDiffMetrics{
		NumDiffPixels:    1,
		PixelDiffPercent: 50,
		MaxRGBDiffs:      []int{7, 128, 248},
	}

	// Put them under different URLs so there are multiple entries associated
	// with a run.
	err := resultStore.Put(TEST_RUN_ID, TEST_URL, recOne)
	assert.NoError(t, err)
	err = resultStore.Put(TEST_RUN_ID, TEST_URL_TWO, recTwo)
	assert.NoError(t, err)

	// Sort the cache and verify.
	err = resultStore.SortRun(TEST_RUN_ID, "rank", "ascending")
	assert.NoError(t, err)
	results, _, err := resultStore.GetFiltered(TEST_RUN_ID, 0, 0, 100)
	assert.NoError(t, err)
	assert.Equal(t, recTwo, results[0])
	assert.Equal(t, recOne, results[1])

	err = resultStore.SortRun(TEST_RUN_ID, "numDiff", "ascending")
	assert.NoError(t, err)
	results, _, err = resultStore.GetFiltered(TEST_RUN_ID, 0, 0, 100)
	assert.NoError(t, err)
	assert.Equal(t, recTwo, results[0])
	assert.Equal(t, recOne, results[1])

	err = resultStore.SortRun(TEST_RUN_ID, "percentDiff", "descending")
	assert.NoError(t, err)
	results, _, err = resultStore.GetFiltered(TEST_RUN_ID, 0, 0, 100)
	assert.Equal(t, recTwo, results[0])
	assert.Equal(t, recOne, results[1])

	err = resultStore.SortRun(TEST_RUN_ID, "redDiff", "descending")
	assert.NoError(t, err)
	results, _, err = resultStore.GetFiltered(TEST_RUN_ID, 0, 0, 100)
	assert.Equal(t, recTwo, results[0])
	assert.Equal(t, recOne, results[1])

	// Ties should be broken by URL.
	err = resultStore.SortRun(TEST_RUN_ID, "greenDiff", "ascending")
	assert.NoError(t, err)
	results, _, err = resultStore.GetFiltered(TEST_RUN_ID, 0, 0, 100)
	assert.Equal(t, recOne, results[0])
	assert.Equal(t, recTwo, results[1])

	err = resultStore.SortRun(TEST_RUN_ID, "blueDiff", "ascending")
	assert.NoError(t, err)
	results, _, err = resultStore.GetFiltered(TEST_RUN_ID, 0, 0, 100)
	assert.Equal(t, recTwo, results[0])
	assert.Equal(t, recOne, results[1])
}

func TestGetURLs(t *testing.T) {
	testutils.MediumTest(t)

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

	// Verify the correct URLs are returned.
	urls, err := resultStore.GetURLs(TEST_RUN_ID)
	assert.NoError(t, err)
	expectedOne := map[string]string{
		"text":  "google.com",
		"value": "http://www.",
	}
	expectedTwo := map[string]string{
		"text":  "youtube.com",
		"value": "http://www.",
	}
	assert.Equal(t, expectedOne, urls[0])
	assert.Equal(t, expectedTwo, urls[1])
}

func TestGetStats(t *testing.T) {
	testutils.MediumTest(t)

	// Initialize the ResultStore.
	resultStore := createBoltResultStore(t)

	// Create three ResultRecs.
	recOne := createResultRec(t)
	recTwo := createResultRec(t)
	recThree := createResultRec(t)
	recTwo.URL = TEST_URL_TWO
	recTwo.Rank = 2
	recTwo.DiffMetrics = &dynamicdiff.DynamicDiffMetrics{
		PixelDiffPercent: 100,
		NumDynamicPixels: 1,
	}
	recThree.URL = TEST_URL_THREE
	recThree.Rank = 3
	recThree.DiffMetrics = &dynamicdiff.DynamicDiffMetrics{
		PixelDiffPercent: 100,
		NumDynamicPixels: 2,
	}
	// Put them under different URLs so there are multiple entries associated
	// with a run.
	err := resultStore.Put(TEST_RUN_ID, TEST_URL, recOne)
	assert.NoError(t, err)
	err = resultStore.Put(TEST_RUN_ID, TEST_URL_TWO, recTwo)
	assert.NoError(t, err)
	err = resultStore.Put(TEST_RUN_ID, TEST_URL_THREE, recThree)
	assert.NoError(t, err)

	// Verify the correct statistics are returned.
	stats, histogram, err := resultStore.GetStats(TEST_RUN_ID)
	assert.NoError(t, err)

	expectedStats := map[string]int{
		NUM_TOTAL_RESULTS:   3,
		NUM_DYNAMIC_CONTENT: 2,
		NUM_ZERO_DIFF:       1,
	}
	assert.Equal(t, expectedStats, stats)

	expectedHistogram := map[string]int{
		BUCKET_0: 1,
		BUCKET_1: 0,
		BUCKET_2: 0,
		BUCKET_3: 0,
		BUCKET_4: 0,
		BUCKET_5: 0,
		BUCKET_6: 0,
		BUCKET_7: 0,
		BUCKET_8: 0,
		BUCKET_9: 2,
	}
	assert.Equal(t, expectedHistogram, histogram)
}
