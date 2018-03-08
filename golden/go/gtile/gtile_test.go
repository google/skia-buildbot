package gtile

import (
	"math/rand"
	"os"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/tiling"
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
	TEST_DATA_PATH = TEST_DATA_DIR + "/10-test-sample-4bytes.tile"

	// Folder in the testdata bucket. See go/testutils for details.
	TEST_DATA_STORAGE_PATH = "gold-testdata/10-test-sample-4bytes.tile"
)

func TestGTile(t *testing.T) {
	_, _, tile, _ := getStoragesIndexTile(t, gcs.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH, false)

	// Reduce the tile to one trace
	for id, trace := range tile.Traces {
		tile.Traces = map[string]tiling.Trace{id: trace}
		tile.ParamSet = paramtools.NewParamSet(trace.Params())
		break
	}

	gTile := FromTile(tile)
	foundTile := gTile.ToTile()
	assert.Equal(t, tile.Commits, foundTile.Commits)
	assert.Equal(t, tile.ParamSet, foundTile.ParamSet)
	assert.Equal(t, tile.Traces, foundTile.Traces)
}

func getStoragesIndexTile(t *testing.T, bucket, storagePath, outputPath string, randomize bool) (*storage.Storage, *indexer.SearchIndex, *tiling.Tile, *indexer.Indexer) {
	err := gcs.DownloadTestDataFile(t, bucket, storagePath, outputPath)
	assert.NoError(t, err, "Unable to download testdata.")
	return getStoragesAndIndexerFromTile(t, outputPath, randomize)
}

func getStoragesAndIndexerFromTile(t assert.TestingT, path string, randomize bool) (*storage.Storage, *indexer.SearchIndex, *tiling.Tile, *indexer.Indexer) {
	sample := loadSample(t, path, randomize)

	tileBuilder := mocks.NewMockTileBuilderFromTile(t, sample.Tile)
	eventBus := eventbus.New()
	expStore := expstorage.NewMemExpectationsStore(eventBus)
	err := expStore.AddChange(sample.Expectations.Tests, "testuser")
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
	return storages, idx, tile, ixr
}

func loadSample(t assert.TestingT, fileName string, randomize bool) *serialize.Sample {
	file, err := os.Open(fileName)
	assert.NoError(t, err)

	sample, err := serialize.DeserializeSample(file)
	assert.NoError(t, err)

	if randomize {
		sample.Tile = randomizeTile(sample.Tile, sample.Expectations)
	}

	return sample
}

func randomizeTile(tile *tiling.Tile, exp *expstorage.Expectations) *tiling.Tile {
	allDigestSet := util.StringSet{}
	for _, digests := range exp.Tests {
		for d := range digests {
			allDigestSet[d] = true
		}
	}
	allDigests := allDigestSet.Keys()

	tileLen := tile.LastCommitIndex() + 1
	ret := tile.Copy()
	for _, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		for i := 0; i < tileLen; i++ {
			gTrace.Values[i] = allDigests[int(rand.Uint32())%len(allDigests)]
		}
	}
	return ret
}
