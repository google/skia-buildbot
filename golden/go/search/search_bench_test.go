package search

import (
	"bytes"
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs"
)

const (
	// TEST_STORAGE_DIR_SEARCH_API is the path in the testdata bucket where
	// the test data files are stored.
	TEST_STORAGE_DIR_SEARCH_API = "gold-testdata"

	// TEST_DATA_DIR_SEARCH_API is the local directory where the local copy
	// of the test data are stored.
	TEST_DATA_DIR_SEARCH_API = "testdata_searchapi"

	// SAMPLED_TILE_FNAME is the filename that contains an entire snapshot of the
	// state of Gold at a point in time.
	SAMPLED_TILE_FNAME = "total_skia.sample"

	// QUERIES_FNAME_SEARCH_API contains the file name of the list of queries
	// that were extracted from the Gold application log.
	QUERIES_FNAME_SEARCH_API = "live_queries.txt"

	// STOP_AFTER_N_EMPTY_QUERIES sets the number of non-empty of queries after
	// which to stop. Change during profiling to shorten runs. -1 means to
	// run all queries.
	STOP_AFTER_N_EMPTY_QUERIES = -1
)

func BenchmarkNewSearchAPI(b *testing.B) {
	cloudTilePath := TEST_STORAGE_DIR_SEARCH_API + "/" + SAMPLED_TILE_FNAME + ".gz"
	cloudQueriesPath := TEST_STORAGE_DIR_SEARCH_API + "/" + QUERIES_FNAME_SEARCH_API + ".gz"

	localTilePath := TEST_DATA_DIR_SEARCH_API + "/" + SAMPLED_TILE_FNAME
	localQueriesPath := TEST_DATA_DIR_SEARCH_API + "/" + QUERIES_FNAME_SEARCH_API

	if !fileutil.FileExists(localTilePath) {
		assert.NoError(b, gcs.DownloadTestDataFile(b, gcs.TEST_DATA_BUCKET, cloudTilePath, localTilePath))
	}

	if !fileutil.FileExists(localQueriesPath) {
		assert.NoError(b, gcs.DownloadTestDataFile(b, gcs.TEST_DATA_BUCKET, cloudQueriesPath, localQueriesPath))
	}

	// Load the storage layer.
	storages, _, _, ixr := getStoragesAndIndexerFromTile(b, localTilePath, true)
	exp, err := storages.ExpectationsStore.Get()
	assert.NoError(b, err)

	api, err := NewSearchAPI(storages, ixr)
	assert.NoError(b, err)
	idx := ixr.GetIndex()

	qStrings, err := fileutil.ReadLines(localQueriesPath)
	assert.NoError(b, err)

	var buf bytes.Buffer
	nonEmpty := 0
	total := 0
	for _, qStr := range qStrings {
		nonEmpty += checkQuery(b, api, idx, qStr, exp, &buf)
		total++
		fmt.Printf("Queries (non-empty / total): %d / %d\n", nonEmpty, total)

		if (STOP_AFTER_N_EMPTY_QUERIES > 0) && (nonEmpty > STOP_AFTER_N_EMPTY_QUERIES) {
			break
		}
	}
}
