package search

import (
	"fmt"
	"math"
	"net/url"
	"os"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/serialize"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Directory with testdata.
	TEST_DATA_DIR = "./testdata"

	// Local file location of the test data.
	TEST_DATA_PATH = TEST_DATA_DIR + "/10-test-sample.tile"

	// Folder in the testdata bucket. See go/testutils for details.
	TEST_DATA_STORAGE_PATH = "gold-testdata/10-test-sample.tile"

	// REPO_URL is the url of the repo to check out.
	REPO_URL = "https://skia.googlesource.com/skia"

	// REPO_DIR contains the location of where to check out Skia for benchmarks.
	REPO_DIR = "./skia"

	// N_COMMITS is the number of commits used in benchmarks.
	N_COMMITS = 50

	// Database user used by benchmarks.
	DB_USER = "readwrite"
)

func TestCompareTests(t *testing.T) {
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)

	const MAX_DIM = 999999999999
	var HEAD = true

	storages, idx, tile := getStoragesIndexTile(t, gcs.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH)

	// testNameSet collects all test names and the set of digests for
	// each test to establish a ground truth for the search below.
	testNameSet := map[string]util.StringSet{}
	for _, trace := range tile.Traces {
		testName := trace.Params()[types.PRIMARY_KEY_FIELD]
		if _, ok := testNameSet[testName]; !ok {
			testNameSet[testName] = util.StringSet{}
		}
		vals := trace.(*types.GoldenTrace).Values
		if HEAD {
			foundVals := []string{}
			for i := len(vals) - 1; i >= 0; i-- {
				if vals[i] != types.MISSING_DIGEST {
					foundVals = vals[i:]
					break
				}
			}
			vals = foundVals
		}

		testNameSet[testName].AddLists(vals)
		delete(testNameSet[testName], types.MISSING_DIGEST)
	}

	ctQuery := &CTQuery{
		RowQuery: &Query{
			Pos:            true,
			Neg:            true,
			Unt:            true,
			Head:           HEAD,
			IncludeIgnores: false,
			Limit:          MAX_DIM,
		},
		ColumnQuery: &Query{
			Pos:            true,
			Neg:            true,
			Unt:            true,
			Head:           HEAD,
			IncludeIgnores: false,
			Limit:          MAX_DIM,
		},
		Match:       []string{types.PRIMARY_KEY_FIELD},
		SortRows:    SORT_FIELD_COUNT,
		SortColumns: SORT_FIELD_DIFF,
		RowsDir:     SORT_DESC,
		ColumnsDir:  SORT_ASC,
		Metric:      diff.METRIC_COMBINED,
	}

	// Test individual tests.
	for testName := range testNameSet {
		// Make sure the query searches for the current testName. It is assumed
		// that all tests in the tile have source_type == 'gm'.
		q, err := url.ParseQuery(fmt.Sprintf("source_type=gm&name=%s", testName))
		assert.NoError(t, err)
		ctQuery.RowQuery.Query = q
		ctQuery.ColumnQuery.Query = q
		testCompTest(t, MAX_DIM, MAX_DIM, testNameSet, ctQuery, idx, storages, 1)
	}

	// test across all tests.
	q, err := url.ParseQuery("source_type=gm")
	assert.NoError(t, err)
	ctQuery.RowQuery.Query = q
	ctQuery.ColumnQuery.Query = q
	ctQuery.RowQuery.Limit = 1000000
	ctQuery.ColumnQuery.Limit = 1000000
	testCompTest(t, ctQuery.ColumnQuery.Limit, ctQuery.ColumnQuery.Limit, testNameSet, ctQuery, idx, storages, len(testNameSet))
}

func testCompTest(t *testing.T, maxLimit, maxRowLimit int, testNameSet map[string]util.StringSet, ctQuery *CTQuery, idx *indexer.SearchIndex, storages *storage.Storage, nTests int) {
	ret, err := CompareTest(ctQuery, storages, idx)
	assert.NoError(t, err)

	// Make sure the rows are as expected.
	assert.True(t, len(ret.Grid.Rows) <= maxLimit)
	uniqueTests := util.StringSet{}
	lastCount := math.MaxInt64
	for idx, row := range ret.Grid.Rows {
		digestSet, ok := testNameSet[row.TestName]
		assert.True(t, ok)

		uniqueTests[row.TestName] = true
		expectedCellsPerRow := util.MinInt(len(digestSet)-1, maxRowLimit)

		// Make sure the count is monotonically increasing.
		assert.True(t, lastCount >= row.N)
		lastCount = row.N
		foundDigestSet := util.StringSet{}
		for _, cell := range row.Values {
			foundDigestSet[cell.Digest] = true
		}

		// Make sure there are not duplicate digest in a row.
		assert.Equal(t, len(row.Values), len(foundDigestSet))

		// Make sure the 'row' digest is not in the row, i.e. compared to itself.
		assert.False(t, foundDigestSet[ret.Grid.Rows[idx].Digest])

		// Make sure we get the expected number of digests in this row.
		assert.Equal(t, expectedCellsPerRow, len(row.Values))

		// Make sure the found digests are fully contained in the whole set.
		assert.Equal(t, digestSet.Intersect(foundDigestSet), foundDigestSet)

		// Make sure the values are monotonically increasing.
		lastVal := row.Values[0].Diffs[ctQuery.Metric]
		for _, val := range row.Values[1:] {
			_, ok := val.Diffs[ctQuery.Metric]
			assert.True(t, ok)
			assert.True(t, lastVal <= val.Diffs[ctQuery.Metric])
			lastVal = val.Diffs[ctQuery.Metric]
		}
	}
	assert.Equal(t, nTests, len(uniqueTests))

	if len(uniqueTests) == 1 {
		assert.Equal(t, 1, len(uniqueTests))
		expectedRowCount := util.MinInt(len(testNameSet[uniqueTests.Keys()[0]]), maxLimit)
		assert.Equal(t, expectedRowCount, len(ret.Grid.Rows))
	}
}

func getStoragesIndexTile(t *testing.T, bucket, storagePath, outputPath string) (*storage.Storage, *indexer.SearchIndex, *tiling.Tile) {
	err := gcs.DownloadTestDataFile(t, bucket, storagePath, outputPath)
	assert.NoError(t, err, "Unable to download testdata.")
	defer testutils.RemoveAll(t, TEST_DATA_DIR)

	sample := loadSample(t, TEST_DATA_PATH)

	tileBuilder := mocks.NewMockTileBuilderFromTile(t, sample.Tile)
	eventBus := eventbus.New(nil)
	expStore := expstorage.NewMemExpectationsStore(eventBus)
	err = expStore.AddChange(sample.Expectations.Tests, "testuser")
	assert.NoError(t, err)

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

	ixr, err := indexer.New(storages, 10*time.Minute)
	assert.NoError(t, err)
	idx := ixr.GetIndex()
	tile := idx.GetTile(false)
	return storages, idx, tile
}

func loadSample(t assert.TestingT, fileName string) *serialize.Sample {
	file, err := os.Open(fileName)
	assert.NoError(t, err)

	sample, err := serialize.DeserializeSample(file)
	assert.NoError(t, err)

	return sample
}
