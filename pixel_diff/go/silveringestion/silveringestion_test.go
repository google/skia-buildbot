package silveringestion

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/boltdb/bolt"
	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	// Name of the input file containing the test data.
	TEST_INGESTION_FILE = "testdata/test.json"

	// Temporary directory used to store screenshots db.
	TEST_SCREENSHOTS_DIR = "screenshots"

	// Temporary boltDB instance containing screenshot data.
	TEST_SCREENSHOTS_DB = "screenshots.db"
)

var (
	TEST_COMMITS = []*vcsinfo.LongCommit{}
)

// Tests parsing and processing of a single file.
func TestCTResults(t *testing.T) {
	testutils.SmallTest(t)

	// Load the sample data file as CTResults.
	r, err := os.Open(TEST_INGESTION_FILE)
	assert.NoError(t, err)

	// Parse the JSON file.
	ctResults, err := ParseCTResultsFromReader(r, TEST_INGESTION_FILE)
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
	testutils.SmallTest(t)

	// Set up mock VCS and IngesterConfig.
	vcs := ingestion.MockVCS(TEST_COMMITS)
	screenshotsDir, err := ioutil.TempDir("", TEST_SCREENSHOTS_DIR)
	ingesterConf := &sharedconfig.IngesterConfig{
		ExtraParams: map[string]string{
			CONFIG_BOLTDIR: screenshotsDir,
			CONFIG_BOLTNAME: TEST_SCREENSHOTS_DB,
		},
	}

	defer testutils.RemoveAll(t, screenshotsDir)

	// Set up the processor.
	processor, err := newSilverProcessor(vcs, ingesterConf, nil)
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
	err = screenshots.View(func(tx *bolt.Tx) error {
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
	})
	assert.NoError(t, err)

	// Expected results.
	expected := map[string][]bool {
		"http___www_google_com" : []bool{true, false},
		"http___www_youtube_com" : []bool{false, true},
	}
	assert.Equal(t, expected, results)
}
