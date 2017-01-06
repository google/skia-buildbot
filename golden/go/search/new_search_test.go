package search

import (
	"fmt"
	"testing"

	"github.com/davecgh/go-spew/spew"
	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/storage"
)

func TestSearchAPI(t *testing.T) {
	testutils.MediumTest(t)

	storages, _, _, ixr := getStoragesIndexTile(t, gs.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH)
	api, err := NewSearchAPI(storages, ixr)
	assert.NoError(t, err)
	idx := ixr.GetIndex()

	// testNameSet, total := map[string]map[string]paramtools.ParamSet{}
	q1 := &Query{
		Pos:            true,
		Neg:            true,
		Unt:            true,
		Head:           true,
		IncludeIgnores: false,
		Limit:          0,
		Match:          []string{"name", "gamma_correct"},
		Metric:         diff.METRIC_PIXEL,
	}

	testGivenQuery(t, api, q1, storages, idx)

	// Add a filter to only get the max count in each test.
	// q1.Filter = &Filter{
	// 	GroupTest: GROUP_TEST_MAX_COUNT,
	// }
	// testGivenQuery(t, api, q1, storages, idx)
}

func testGivenQuery(t *testing.T, api *SearchAPI, q *Query, storages *storage.Storage, idx *indexer.SearchIndex) {
	tile := idx.GetTile(q.IncludeIgnores)
	testNameSet, total := findTests(tile, q.Head)

	resp, err := api.Search(q)
	assert.NoError(t, err)
	assert.Equal(t, total, len(resp.Digests))

	foundTests := map[string]util.StringSet{}

	exp, err := storages.ExpectationsStore.Get()
	assert.NoError(t, err)

	noRef := 0
	var currMin float32 = -1.0
	for _, digest := range resp.Digests {
		// make sure they increase monotonically.
		if digest.ClosestRef != "" {
			diffVal := digest.RefDiffs[digest.ClosestRef].Diffs[diff.METRIC_PIXEL]
			assert.True(t, diffVal >= currMin, fmt.Sprintf("Not Increasing: %f <= %f", digest.RefDiffs[digest.ClosestRef].Diffs[diff.METRIC_PIXEL], currMin))
			currMin = diffVal
		} else {
			_, ok := exp.Tests[digest.Test]
			// fmt.Printf("FOUND exp: %s\n", spew.Sprint(found))
			fmt.Printf("PARAMS: %s\n", spew.Sprint(digest.ParamSet))

			assert.False(t, ok)
			noRef++
		}

		if _, ok := foundTests[digest.Test]; !ok {
			foundTests[digest.Test] = util.StringSet{}
		}
		foundTests[digest.Test][digest.Digest] = true
	}

	fmt.Printf("NO REFS: %d/%d\n", noRef, total)

	assert.Equal(t, len(testNameSet), len(foundTests))

	for test, digests := range foundTests {
		_, ok := testNameSet[test]
		assert.True(t, ok, fmt.Sprintf("Could not find %s", test))

		assert.Equal(t, testNameSet[test], digests)
	}

	// assert.Equal(t, testNameSet, foundTests)
}
