package ingester

import (
	"io/ioutil"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	storage "google.golang.org/api/storage/v1"
)

func init() {
	Init(nil)
}

func TestGetGSResultFileLocations(t *testing.T) {
	testutils.SkipIfShort(t)
	storage, err := storage.New(http.DefaultClient)
	assert.Nil(t, err)

	startTS := time.Date(2014, time.December, 10, 0, 0, 0, 0, time.UTC).Unix()
	endTS := time.Date(2014, time.December, 10, 23, 59, 59, 0, time.UTC).Unix()

	// TODO(stephana): Switch this to a dedicated test bucket, so we are not
	// in danger of removing it.
	resultFiles, err := getGSResultsFileLocations(startTS, endTS, storage, "chromium-skia-gm", "dm-json-v1")
	assert.Nil(t, err)

	// Read the expected list of files and compare them.
	content, err := ioutil.ReadFile("./testdata/filelist_dec_10.txt")
	assert.Nil(t, err)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	sort.Strings(lines)

	resultNames := make([]string, len(resultFiles))
	for idx, rf := range resultFiles {
		resultNames[idx] = rf.Name
	}
	sort.Strings(resultNames)
	assert.Equal(t, len(lines), len(resultNames))
	assert.Equal(t, lines, resultNames)
}

const (
	// TEST_DATA_DIR  is the directory with data used for local ingest.
	TEST_DATA_DIR = "./testdata/local-ingest-test"

	// TEST_DATA_STORAGE_PATH is the folder in the test data bucket.
	// See go/testutils for details.
	TEST_DATA_STORAGE_PATH = "ingest-testdata/local-ingest.tar.gz"
)

func TestGetLocalResultFileLocations(t *testing.T) {
	testutils.SkipIfShort(t)

	err := testutils.DownloadTestDataArchive(t, TEST_DATA_STORAGE_PATH, TEST_DATA_DIR)
	assert.Nil(t, err)

	startTS := time.Date(2015, time.May, 5, 0, 0, 0, 0, time.UTC).Unix()
	endTS := time.Date(2015, time.May, 17, 23, 59, 59, 0, time.UTC).Unix()

	resultFiles, err := getLocalResultsFileLocations(startTS, endTS, filepath.Join(TEST_DATA_DIR, "nano-json-v1"))
	assert.Nil(t, err)

	// Read the expected list of files and compare them.
	content, err := ioutil.ReadFile("./testdata/local_ingest_files.txt")
	assert.Nil(t, err)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	sort.Strings(lines)

	resultNames := make([]string, len(resultFiles))
	for idx, rf := range resultFiles {
		resultNames[idx] = rf.Name
	}
	sort.Strings(resultNames)
	assert.Equal(t, len(lines), len(resultNames))
	assert.Equal(t, lines, resultNames)
}
