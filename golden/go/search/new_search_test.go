package search

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/storage"
)

const (
	NEW_SEARCH_TEST_TILE = "./new_search_testdata/skia.tile"
	// 	NEW_SEARCH_TEST_TILE = "./new_search_testdata/pdfium.tile"

	Q = "fdiffmax=-1&fref=false&frgbamax=-1&head=true&include=false&limit=50&match=gamma_correct&match=name&metric=combined&neg=false&pos=false&query=source_type%3Dgm&sort=desc&unt=true"
)

func TestNewSearchAPI(t *testing.T) {
	// Load the storage layer.
	storages, ixr := getStoragesAndIndexerFromTile(t, NEW_SEARCH_TEST_TILE)

	api, err := NewSearchAPI(storages, ixr)
	assert.NoError(t, err)
	idx := ixr.GetIndex()

	testQuery(t, api, idx, Q)
}

func getTargetDigests(q *Query, tile *tiling.Tile) util.StringSet {
	return nil
}

func testQuery(t *testing.T, api *SearchAPI, idx *indexer.SearchIndex, qStr string) {
	q := &Query{}
	assert.NoError(t, clearParseQuery(q, qStr))

	resp, err := api.Search(q)
	assert.NoError(t, err)
	expDigests := getTargetDigests(q, idx.GetTile(q.IncludeIgnores))

	foundDigests := util.StringSet{}
	for _, digestRec := range resp.Digests {
		foundDigests[digestRec.Digest] = true
	}
	assert.Equal(t, expDigests, foundDigests)

	// testFilterTile(t, q, api)
	// testGetRefDiffs(t, q, api)
	// testSortAndLimitDigests(t, q, api)
	// testAddParamsAndTraces(t, q, api)

	// Assemble the expected response.

	// 1. get the traces of the first query.

	// 1a group by test and figure out the digests to compare against

	// 1b get the differences for each group.

	// filter the difference.

	// 2. Verify that they are correct.

	// 3. Verify the sort order and the pagination

	// 4. Validate the trace info is correct.

	// // Blaming
	// BlameGroupID string `json:"blame"`

	// // Image classification
	// Pos            bool `json:"pos"`
	// Neg            bool `json:"neg"`
	// Head           bool `json:"head"`
	// Unt            bool `json:"unt"`
	// IncludeIgnores bool `json:"include"`

	// // URL encoded query string
	// QueryStr string     `json:"query"`
	// Query    url.Values `json:"-"`

	// // Trybot support.
	// Issue         string   `json:"issue"`
	// PatchsetsStr  string   `json:"patchsets"` // Comma-separated list of patchsets.
	// Patchsets     []string `json:"-"`
	// IncludeMaster bool     `json:"master"` // Include digests also contained in master when searching Rietveld issues.

	// // Filtering.
	// FCommitBegin string  `json:"fbegin"`     // Start commit
	// FCommitEnd   string  `json:"fend"`       // End commit
	// FRGBAMin     int32   `json:"frgbamin"`   // Min RGBA delta
	// FRGBAMax     int32   `json:"frgbamax"`   // Max RGBA delta
	// FDiffMax     float32 `json:"fdiffmax"`   // Max diff according to metric
	// FGroupTest   string  `json:"fgrouptest"` // Op within grouped by test.
	// FRef         bool    `json:"fref"`       // Only digests with reference.

	// // Pagination.
	// Offset int `json:"offset"`
	// Limit  int `json:"limit"`
}

// func testFilterTile(t *testing.T, q *Query, api *SearchAPI) {
// 	idx := api.ixr.GetIndex()
// 	inter, err := api.filterTile(q, idx)

// 	tile := idx.GetTile(q.IncludeIgnores)
// 	for testName, testMap := range inter {
// 		for

// 	}

// }

func testGetRefDiffs(t *testing.T, q *Query, api *SearchAPI) {

}

func testSortAndLimitDigests(t *testing.T, q *Query, api *SearchAPI) {

}

func testAddParamsAndTraces(t *testing.T, q *Query, api *SearchAPI) {

}

func getStoragesAndIndexerFromTile(t *testing.T, path string) (*storage.Storage, *indexer.Indexer) {
	loadTimer := timer.New("Loading sample tile")
	sampledState := loadSample(t, path)
	tileBuilder := mocks.NewMockTileBuilderFromTile(t, sampledState.Tile)
	eventBus := eventbus.New()
	expStore := expstorage.NewMemExpectationsStore(eventBus)
	loadTimer.Stop()

	// ?????
	// err = expStore.AddChange(sample.Expectations.Tests, "testuser")
	// assert.NoError(t, err)

	storages := &storage.Storage{
		ExpectationsStore: expStore,
		MasterTileBuilder: tileBuilder,
		DigestStore: &mocks.MockDigestStore{
			FirstSeen: time.Now().Unix(),
			OkValue:   true,
		},
		DiffStore: mocks.NewMockDiffStore(),
		EventBus:  eventBus,
	}

	ixr, err := indexer.New(storages, 60*time.Minute)
	assert.NoError(t, err)
	return storages, ixr
}
