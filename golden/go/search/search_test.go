package search

import (
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/skia-dev/glog"
	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
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
	testutils.SkipIfShort(t)

	err := gs.DownloadTestDataFile(t, gs.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH)
	assert.NoError(t, err, "Unable to download testdata.")
	//	defer testutils.RemoveAll(t, TEST_DATA_DIR)

	sample := loadSample(t, TEST_DATA_PATH)

	tileBuilder := mocks.NewMockTileBuilderFromTile(t, sample.Tile)
	eventBus := eventbus.New(nil)
	expStore := expstorage.NewMemExpectationsStore(eventBus)
	expStore.AddChange(sample.Expectations.Tests, "testuser")

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

	ixr, err := indexer.New(storages, time.Minute)
	assert.NoError(t, err)
	tile := ixr.GetIndex().GetTile(false)

	testNameSet := map[string]util.StringSet{}
	for _, trace := range tile.Traces {
		testName := trace.Params()[types.PRIMARY_KEY_FIELD]
		if _, ok := testNameSet[testName]; !ok {
			testNameSet[testName] = util.StringSet{}
		}
		testNameSet[testName].AddLists(trace.(*types.GoldenTrace).Values)
	}

	const MAX_DIM = 5
	for testName, digestSet := range testNameSet {
		limit := util.MinInt(len(digestSet), MAX_DIM)
		q, err := url.ParseQuery(fmt.Sprintf("source_type=gm&name=%s", testName))
		assert.NoError(t, err)
		ctQuery := &CTQuery{
			Test: testName,
			RowQuery: &Query{
				Pos:            true,
				Neg:            true,
				Unt:            true,
				Head:           true,
				IncludeIgnores: false,
				Query:          q,
				Limit:          limit,
			},
			ColumnQuery: &Query{
				Pos:            true,
				Neg:            true,
				Unt:            true,
				Head:           true,
				IncludeIgnores: false,
				Query:          q,
				Limit:          limit,
			},
			Match:       []string{},
			SortRows:    "n",
			SortColumns: "diff",
			RowsDir:     "desc",
			ColumnsDir:  "asc",
		}
		ret, err := CompareTest(ctQuery, storages, ixr.GetIndex())
		assert.NoError(t, err)

		// Make sure the grid is a 5x5
		for i, row := range ret.Grid.Cells {
			for j, cell := range row {
				glog.Infof("(%d, %d): \n %s\n\n", i, j, spew.Sprint(cell))
			}
		}

		glog.Infof("Size: %d %d\n", len(ret.Grid.Cells), len(ret.Grid.Cells[0]))
		assert.Equal(t, limit, len(ret.Grid.Cells))
		assert.Equal(t, limit, len(ret.Grid.Rows))
		assert.Equal(t, 0, len(ret.Grid.Columns))
	}
}

func loadSample(t assert.TestingT, fileName string) *serialize.Sample {
	file, err := os.Open(fileName)
	assert.NoError(t, err)

	sample, err := serialize.DeserializeSample(file)
	assert.NoError(t, err)

	return sample
}
