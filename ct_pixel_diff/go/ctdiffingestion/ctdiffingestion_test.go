package ctdiffingestion

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/ct_pixel_diff/go/dynamicdiff"
	"go.skia.org/infra/ct_pixel_diff/go/resultstore"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/diffstore"
	"go.skia.org/infra/golden/go/mocks"
)

const (
	// Name of the input file containing the test JSON data.
	TEST_INGESTION_FILE = "testdata/test.json"

	// DiffStore parameters.
	TEST_BASE_DIR     = "img"
	TEST_GS_BUCKET    = "cluster-telemetry"
	TEST_GS_IMAGE_DIR = "tasks/pixel_diff_runs"

	// ResultStore arguments.
	TEST_DIFF_DIR = "diffs"
	TEST_DIFF_DB  = "diffs.db"

	// Test data for processing CT results and querying/updating the ResultStore.
	TEST_RUN_ID        = "rmistry-20170802211123"
	TEST_URL_ONE       = "http://www.google.com"
	TEST_URL_TWO       = "http://www.youtube.com"
	TEST_NOPATCH_ONE   = "rmistry-20170802211123/nopatch/1/http___www_google_com"
	TEST_WITHPATCH_ONE = "rmistry-20170802211123/withpatch/1/http___www_google_com"
	TEST_WITHPATCH_TWO = "rmistry-20170802211123/withpatch/2/http___www_youtube_com"
)

// Tests parsing and processing of a single CT output JSON file.
func TestCTResults(t *testing.T) {
	testutils.SmallTest(t)

	// Load the sample data file as CTResults.
	r, err := os.Open(TEST_INGESTION_FILE)
	assert.NoError(t, err)

	// Parse the JSON file.
	results, err := parseCTResultsFromReader(r, TEST_INGESTION_FILE)
	assert.NoError(t, err)

	// Expected results.
	expScreenshots := []*Screenshot{
		{Type: "nopatch", Rank: 1, Filename: "http___www_google_com.png", URL: TEST_URL_ONE},
		{Type: "withpatch", Rank: 2, Filename: "http___www_youtube_com.png", URL: TEST_URL_TWO},
		{Type: "withpatch", Rank: 1, Filename: "http___www_google_com.png", URL: TEST_URL_ONE}}
	expected := &CTResults{
		RunID:         TEST_RUN_ID,
		ChromiumPatch: "https://chromium-review.googlesource.com/c/000000",
		SkiaPatch:     "https://skia-review.googlesource.com/c/000000",
		Screenshots:   expScreenshots,
		name:          TEST_INGESTION_FILE,
	}
	assert.Equal(t, expected, results)
}

func TestPixelDiffProcessor(t *testing.T) {
	testutils.MediumTest(t)
	ctx := context.Background()

	// Set up the DiffStore.
	client := mocks.GetHTTPClient(t)
	baseDir, err := ioutil.TempDir("", TEST_BASE_DIR)
	assert.NoError(t, err)
	mapper := dynamicdiff.NewPixelDiffStoreMapper(&dynamicdiff.DynamicDiffMetrics{})
	diffStore, err := diffstore.NewMemDiffStore(client, baseDir, []string{TEST_GS_BUCKET}, TEST_GS_IMAGE_DIR, 10, mapper)
	assert.NoError(t, err)

	// Set up the ResultStore.
	diffDir, err := ioutil.TempDir("", TEST_DIFF_DIR)
	assert.NoError(t, err)
	resultStore, err := resultstore.NewBoltResultStore(diffDir, TEST_DIFF_DB)
	assert.NoError(t, err)

	// Initialize the processor.
	processor, err := NewPixelDiffProcessor(diffStore, resultStore)
	assert.NoError(t, err)

	// Load the test JSON file and process it.
	fsResult, err := ingestion.FileSystemResult(TEST_INGESTION_FILE, "./")
	assert.NoError(t, err)
	err = processor.Process(ctx, fsResult)
	assert.NoError(t, err)

	// Verify that the first entry in the ResultStore is correct.
	expectedRecOne := &resultstore.ResultRec{
		RunID:        TEST_RUN_ID,
		URL:          TEST_URL_ONE,
		Rank:         1,
		NoPatchImg:   TEST_NOPATCH_ONE,
		WithPatchImg: TEST_WITHPATCH_ONE,
		DiffMetrics: &dynamicdiff.DynamicDiffMetrics{
			NumDiffPixels:    363,
			PixelDiffPercent: 0.034480203,
			MaxRGBDiffs:      []int{49, 49, 49},
			NumStaticPixels:  1052778,
			NumDynamicPixels: 121344,
		},
	}
	recOne, err := resultStore.Get(TEST_RUN_ID, TEST_URL_ONE)
	assert.NoError(t, err)
	assert.Equal(t, expectedRecOne, recOne)

	// Verify that the second entry in the ResultStore is correct. There should
	// be no data for NoPatchImg and DiffMetrics.
	expectedRecTwo := &resultstore.ResultRec{
		RunID:        TEST_RUN_ID,
		URL:          TEST_URL_TWO,
		Rank:         2,
		WithPatchImg: TEST_WITHPATCH_TWO,
	}
	recTwo, err := resultStore.Get(TEST_RUN_ID, TEST_URL_TWO)
	assert.NoError(t, err)
	assert.Equal(t, expectedRecTwo, recTwo)
}
