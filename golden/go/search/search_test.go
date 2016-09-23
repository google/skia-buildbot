package search

import (
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

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

	testNameSet := util.StringSet{}
	for _, trace := range tile.Traces {
		testNameSet[trace.Params()[types.PRIMARY_KEY_FIELD]] = true
	}

	const DIM = 5
	for testName := range testNameSet {
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
				Limit:          DIM,
			},
			ColumnQuery: &Query{
				Pos:            true,
				Neg:            true,
				Unt:            true,
				Head:           true,
				IncludeIgnores: false,
				Query:          q,
				Limit:          DIM,
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
		assert.Equal(t, DIM, len(ret.Grid.Cells))
		assert.Equal(t, DIM, len(ret.Grid.Rows))
		assert.Equal(t, 0, len(ret.Grid.Columns))

		assert.Nil(t, ret)
		assert.True(t, false)
	}
}

func loadSample(t assert.TestingT, fileName string) *serialize.Sample {
	file, err := os.Open(fileName)
	assert.NoError(t, err)

	sample, err := serialize.DeserializeSample(file)
	assert.NoError(t, err)

	return sample
}
