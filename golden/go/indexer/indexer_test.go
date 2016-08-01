package indexer

import (
	"math/rand"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Directory with testdata.
	TEST_DATA_DIR = "./testdata"

	// Local file location of the test data.
	TEST_DATA_PATH = TEST_DATA_DIR + "/goldentile.json.zip"

	// Folder in the testdata bucket. See go/testutils for details.
	TEST_DATA_STORAGE_PATH = "gold-testdata/goldentile.json.gz"
)

func TestIndexer(t *testing.T) {
	testutils.SkipIfShort(t)

	err := gs.DownloadTestDataFile(t, gs.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH)
	assert.NoError(t, err, "Unable to download testdata.")
	defer testutils.RemoveAll(t, TEST_DATA_DIR)

	tileBuilder := mocks.NewMockTileBuilderFromJson(t, TEST_DATA_PATH)
	eventBus := eventbus.New(nil)
	expStore := expstorage.NewMemExpectationsStore(eventBus)

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

	ixr, err := New(storages, time.Minute)
	assert.NoError(t, err)

	idxOne := ixr.GetIndex()

	// Change the classifications.
	changes := getChanges(t, idxOne.tilePair.Tile)
	assert.NoError(t, storages.ExpectationsStore.AddChange(changes, ""))

	// Wait for the re-indexing.
	time.Sleep(time.Second)
	idxTwo := ixr.GetIndex()

	assert.NotEqual(t, idxOne, idxTwo)
}

func getChanges(t *testing.T, tile *tiling.Tile) map[string]types.TestClassification {
	ret := map[string]types.TestClassification{}
	labelVals := []types.Label{types.POSITIVE, types.NEGATIVE}
	for _, trace := range tile.Traces {
		if rand.Float32() > 0.5 {
			gTrace := trace.(*types.GoldenTrace)
			for _, digest := range gTrace.Values {
				if digest != types.MISSING_DIGEST {
					testName := gTrace.Params_[types.PRIMARY_KEY_FIELD]
					if found, ok := ret[testName]; ok {
						found[digest] = labelVals[rand.Int()%2]
					} else {
						ret[testName] = types.TestClassification{digest: labelVals[rand.Int()%2]}
					}
				}
			}
		}
	}

	assert.True(t, len(ret) > 0)
	return ret
}
