package silveringestion

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/boltdb/bolt"
	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore"
	"go.skia.org/infra/golden/go/mocks"
)

const (
	// Name of the input file containing the test JSON data.
	TEST_INGESTION_FILE = "testdata/test.json"

	// Temporary directory used to store screenshots db.
	TEST_SCREENSHOTS_DIR = "screenshots"

	// Temporary boltDB instance containing screenshot data.
	TEST_SCREENSHOTS_DB = "screenshots.db"

	// Temporary directory representing GS basedir.
	TEST_BASE_DIR = "images"

	// GS bucket.
	TEST_GS_BUCKET = "cluster-telemetry"

	// GS image directory.
	TEST_GS_IMAGE_DIR = "tasks/benchmark_runs"
)

// Tests parsing and processing of a single CT output JSON file.
func TestCTResults(t *testing.T) {
	testutils.SmallTest(t)

	// Load the sample data file as CTResults.
	r, err := os.Open(TEST_INGESTION_FILE)
	assert.NoError(t, err)

	// Parse the JSON file.
	ctResults, err := parseCTResultsFromReader(r, TEST_INGESTION_FILE)
	assert.NoError(t, err)

	// Expected results.
	expScreenshots := []*Screenshot{
		&Screenshot{Type: "nopatch", Filename:"http___www_google_com"},
		&Screenshot{Type: "withpatch", Filename:"http___www_youtube_com"}}
	expected := &CTResults{
		RunID: "lchoi-20170711123456",
		Patch: "https://chromium-review.googlesource.com/c/000000",
		Screenshots: expScreenshots,
		name: TEST_INGESTION_FILE,
	}
	assert.Equal(t, expected, ctResults)
}

// Tests the processor.
func TestSilverProcessor(t *testing.T) {
	testutils.MediumTest(t)

	// Set up the DiffStore used to instantiate the silverProcessor.
	client := mocks.GetHTTPClient(t)
	baseDir, err := ioutil.TempDir("", TEST_BASE_DIR)
	assert.NoError(t, err)
	diffStore, err := diffstore.NewMemDiffStore(client, baseDir, []string{TEST_GS_BUCKET}, TEST_GS_IMAGE_DIR, 10)
	assert.NoError(t, err)

	// Set up the processor.
	screenshotsDir, err := ioutil.TempDir("", TEST_SCREENSHOTS_DIR)
	assert.NoError(t, err)
	processor, err := newSilverProcessor(diffStore, screenshotsDir, TEST_SCREENSHOTS_DB)
	assert.NoError(t, err)

	// Load the test JSON file and process it.
	fsResult, err := ingestion.FileSystemResult(TEST_INGESTION_FILE, "./")
	assert.NoError(t, err)
	err = processor.Process(fsResult)
	assert.NoError(t, err)

	// Get the boltDB used by the processor to verify the results.
	screenshots := processor.(*silverProcessor).screenshots

	// Put the boltDB records into a map.
	results := map[string][]bool{}
	viewFn := func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("lchoi-20170711123456"))
		err = b.ForEach(func(k, v []byte) error {
			processed := make([]bool, 2)
			if err := json.Unmarshal(v, &processed); err != nil {
				return err
			}
			results[string(k)] = processed
			return nil
		})
		assert.NoError(t, err)
		return err
	}
	err = screenshots.View(viewFn)
	assert.NoError(t, err)

	// Expected results.
	expected := map[string][]bool {
		"http___www_google_com" : []bool{true, false},
		"http___www_youtube_com" : []bool{false, true},
	}
	assert.Equal(t, expected, results)

	// Verify the DiffStore calculates diff metrics correctly.
	noPatchImg, withPatchImg := getNoAndWithPatch("rmistry-20170623184523", "http___www_google_com")
	ds := processor.(*silverProcessor).diffStore
	diffs, err := ds.Get(diff.PRIORITY_NOW, noPatchImg, []string{withPatchImg})
	assert.NoError(t, err)
	diffMetrics := diffs[withPatchImg]
	expectedDiffMetrics := &diff.DiffMetrics {
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
	assert.Equal(t, expectedDiffMetrics, diffMetrics)

	// TODO(lchoi): Add tests that verify the nopatch, withpatch, and diff images
	// were downloaded to the correct local file paths by the DiffStore after the
	// image path configuration patch is landed.
}
