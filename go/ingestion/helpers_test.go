package ingestion

import (
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs/gcs_testutils"
	"go.skia.org/infra/go/testutils"
)

const (
	// TEST_GCS_DIR is the directory from where to fetch GCS test data.
	TEST_GCS_DIR = "ingest-testdata/dm-json-v1"

	// TEST_DATA_DIR  is the directory with data used for local ingest.
	TEST_DATA_DIR = "./testdata/local-ingest-test"

	// TEST_DATA_STORAGE_PATH is the folder in the test data bucket.
	// See go/testutils for details.
	TEST_DATA_STORAGE_PATH = "ingest-testdata/local-new-ingestion.tar.gz"

	// TEST_DATA_FILE_INDEX contains the list of files contained in the locally
	// ingested file and the Google storage bucket.
	TEST_DATA_INDEX_FILE = "./testdata/local_ingest_files.txt"
)

var (
	BEGINNING_OF_TIME = time.Date(2015, time.June, 1, 0, 0, 0, 0, time.UTC).Unix()
	END_OF_TIME       = time.Date(2015, time.October, 30, 0, 0, 0, 0, time.UTC).Unix()
	START_TIME        = time.Date(2015, time.October, 1, 0, 0, 0, 0, time.UTC).Unix()
	END_TIME          = time.Date(2015, time.October, 1, 23, 59, 59, 0, time.UTC).Unix()
)

func TestGoogleStorageSource(t *testing.T) {
	testutils.LargeTest(t)

	src, err := NewGoogleStorageSource("gs-test-src", gcs_testutils.TEST_DATA_BUCKET, TEST_GCS_DIR, http.DefaultClient, nil)
	assert.NoError(t, err)
	testSource(t, src)
}

func TestFileSystemResultFileLocations(t *testing.T) {
	testutils.LargeTest(t)

	err := gcs_testutils.DownloadTestDataArchive(t, gcs_testutils.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_DIR)
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, TEST_DATA_DIR)

	src, err := NewFileSystemSource("test-fs-source", TEST_DATA_DIR)
	assert.NoError(t, err)
	testSource(t, src)
}

func TestCompareSources(t *testing.T) {
	testutils.LargeTest(t)

	gsSource, err := NewGoogleStorageSource("gs-test-src", gcs_testutils.TEST_DATA_BUCKET, TEST_GCS_DIR, http.DefaultClient, nil)
	assert.NoError(t, err)

	err = gcs_testutils.DownloadTestDataArchive(t, gcs_testutils.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_DIR)
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, TEST_DATA_DIR)

	fsSource, err := NewFileSystemSource("test-fs-source", TEST_DATA_DIR)
	assert.NoError(t, err)

	gsResults := drainPollChannel(gsSource.Poll(START_TIME, END_TIME))
	fsResults := drainPollChannel(fsSource.Poll(START_TIME, END_TIME))

	assert.Equal(t, len(gsResults), len(fsResults))
	sort.Sort(rflSlice(gsResults))
	sort.Sort(rflSlice(fsResults))

	// Only compare a subset of all files to limit the runtime of this test.
	gsResults = gsResults[:10]
	fsResults = fsResults[:10]

	for idx, result := range gsResults {
		// Check if the MD5 from GS and the file system match.
		assert.Equal(t, result.MD5(), fsResults[idx].MD5())

		// Check if the content is the same for both sources.
		file, err := result.Open()
		assert.NoError(t, err)
		c1, err := ioutil.ReadAll(file)
		assert.NoError(t, err)
		file, err = fsResults[idx].Open()
		c2, err := ioutil.ReadAll(file)
		assert.NoError(t, err)
		assert.Equal(t, string(c1), string(c2))
	}
}

func testSource(t *testing.T, src Source) {
	testFilePaths := readTestFileNames(t)

	resultFileLocations := drainPollChannel(src.Poll(START_TIME, END_TIME))

	assert.Equal(t, len(testFilePaths), len(resultFileLocations))
	sort.Sort(rflSlice(resultFileLocations))

	for idx, result := range resultFileLocations {
		assert.True(t, strings.HasSuffix(result.Name(), testFilePaths[idx]))
	}

	// Make sure the narrow and wide time range produce the same result.
	allResultFileLocations := drainPollChannel(src.Poll(BEGINNING_OF_TIME, END_OF_TIME))
	sort.Sort(rflSlice(allResultFileLocations))

	assert.Equal(t, len(resultFileLocations), len(allResultFileLocations))
	for idx, result := range resultFileLocations {
		assert.Equal(t, result.MD5(), allResultFileLocations[idx].MD5())
		assert.Equal(t, result.Name(), allResultFileLocations[idx].Name())
	}
}

func readTestFileNames(t *testing.T) []string {
	// Read the expected list of files and compare them.
	content, err := ioutil.ReadFile("./testdata/filelist_2015_10_01.txt")
	assert.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	sort.Strings(lines)
	return lines
}

type rflSlice []ResultFileLocation

func (r rflSlice) Len() int           { return len(r) }
func (r rflSlice) Less(i, j int) bool { return r[i].Name() < r[j].Name() }
func (r rflSlice) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }

func drainPollChannel(ch <-chan ResultFileLocation) []ResultFileLocation {
	ret := []ResultFileLocation{}
	for rf := range ch {
		ret = append(ret, rf)
	}
	return ret
}
