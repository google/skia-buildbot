package search

import (
	"os"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/serialize"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/types"
)

func TestTraceViewFn(t *testing.T) {
	testutils.MediumTest(t)

	_, _, tile, _ := getStoragesIndexTile(t, gcs.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH)

	commits := tile.Commits[0 : tile.LastCommitIndex()+1]
	middle := len(commits) / 2
	beginIdx := middle - 1
	endIdx := middle + 1
	fBegin := commits[beginIdx].Hash
	fEnd := commits[endIdx].Hash

	testTraceView(t, tile, beginIdx, endIdx, fEnd, fBegin, true)
	testTraceView(t, tile, beginIdx, endIdx, fBegin, fEnd, false)
	testTraceView(t, tile, beginIdx, beginIdx, fBegin, fBegin, false)
	testTraceView(t, tile, endIdx, endIdx, fEnd, fEnd, false)
	testTraceView(t, tile, 0, len(commits)-1, "", "", false)
	testTraceView(t, tile, beginIdx, len(commits)-1, fBegin, "", false)
	testTraceView(t, tile, 0, endIdx, "", fEnd, false)
}

func testTraceView(t *testing.T, tile *tiling.Tile, beginIdx, endIdx int, startHash, endHash string, expectErr bool) {
	lastIdxExp := endIdx - beginIdx
	lastIdx, traceViewFn, err := getTraceViewFn(tile, startHash, endHash)
	if expectErr {
		assert.Error(t, err)
		return
	} else {
		assert.NoError(t, err)
	}
	assert.Equal(t, lastIdxExp, lastIdx)

	for _, trace := range tile.Traces {
		tr := trace.(*types.GoldenTrace)
		reducedTr := traceViewFn(tr)
		assert.Equal(t, tr.Values[beginIdx:endIdx+1], reducedTr.Values)
	}
}

func getStoragesIndexTile(t *testing.T, bucket, storagePath, outputPath string) (*storage.Storage, *indexer.SearchIndex, *tiling.Tile, *indexer.Indexer) {
	err := gcs.DownloadTestDataFile(t, bucket, storagePath, outputPath)
	assert.NoError(t, err, "Unable to download testdata.")
	defer testutils.RemoveAll(t, TEST_DATA_DIR)

	sample := loadSample(t, TEST_DATA_PATH)

	tileBuilder := mocks.NewMockTileBuilderFromTile(t, sample.Tile)
	eventBus := eventbus.New()
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
	return storages, idx, tile, ixr
}

func loadSample(t assert.TestingT, fileName string) *serialize.Sample {
	file, err := os.Open(fileName)
	assert.NoError(t, err)

	sample, err := serialize.DeserializeSample(file)
	assert.NoError(t, err)

	return sample
}
