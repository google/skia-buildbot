package search

import (
	"bytes"
	"fmt"
	"net/url"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs/gcs_testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Directory with testdata.
	TEST_DATA_DIR = "./testdata"

	// Local file location of the test data.
	TEST_DATA_PATH = TEST_DATA_DIR + "/10-test-sample-4bytes.tile"

	// Folder in the testdata bucket. See go/testutils for details.
	TEST_DATA_STORAGE_PATH = "gold-testdata/10-test-sample-4bytes.tile"
)

func TestSearch(t *testing.T) {
	unittest.MediumTest(t)

	storages, idx, tile, ixr := getStoragesIndexTile(t, gcs_testutils.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH, false)

	api, err := NewSearchAPI(storages, ixr)
	assert.NoError(t, err)
	exp, err := storages.ExpectationsStore.Get()
	assert.NoError(t, err)
	var buf bytes.Buffer

	// test basic search
	paramQuery := url.QueryEscape("source_type=gm")
	qStr := fmt.Sprintf("query=%s&unt=true&pos=true&neg=true&head=true", paramQuery)
	checkQuery(t, api, idx, qStr, exp, &buf)

	// test restricting to a commit range.
	commits := tile.Commits[0 : tile.LastCommitIndex()+1]
	middle := len(commits) / 2
	beginIdx := middle - 2
	endIdx := middle + 2
	fBegin := commits[beginIdx].Hash
	fEnd := commits[endIdx].Hash

	testQueryCommitRange(t, api, idx, tile, exp, fBegin, fEnd)
	for i := 0; i < tile.LastCommitIndex(); i++ {
		testQueryCommitRange(t, api, idx, tile, exp, commits[i].Hash, commits[i].Hash)
	}
}

func testQueryCommitRange(t assert.TestingT, api *SearchAPI, idx *indexer.SearchIndex, tile *tiling.Tile, exp types.TestExpBuilder, startHash, endHash string) {
	var buf bytes.Buffer
	paramQuery := url.QueryEscape("source_type=gm")
	qStr := fmt.Sprintf("query=%s&fbegin=%s&fend=%s&unt=true&pos=true&neg=true&head=true", paramQuery, startHash, endHash)
	checkQuery(t, api, idx, qStr, exp, &buf)
}
