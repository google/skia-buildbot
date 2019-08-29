package search

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gcs/gcs_testutils"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/serialize"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/warmer"
)

func getAPIIndexTile(t *testing.T, bucket, storagePath, outputPath string, randomize bool) (SearchAPI, indexer.IndexSearcher, *tiling.Tile) {
	err := gcs_testutils.DownloadTestDataFile(t, bucket, storagePath, outputPath)
	assert.NoError(t, err, "Unable to download testdata.")
	return getAPIAndIndexerFromTile(t, outputPath, randomize)
}

func getAPIAndIndexerFromTile(t sktest.TestingT, path string, randomize bool) (SearchAPI, indexer.IndexSearcher, *tiling.Tile) {
	sample := loadSample(t, path, randomize)

	mds := &mocks.DiffStore{}
	mes := &mocks.ExpectationsStore{}
	mts := &mocks.TileSource{}

	mes.On("Get").Return(sample.Expectations, nil)

	mds.On("UnavailableDigests").Return(map[types.Digest]*diff.DigestFailure{})
	mds.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(mockDiffStoreGet, nil)

	cpxTile := types.NewComplexTile(sample.Tile)
	mts.On("GetTile").Return(cpxTile, nil)

	eventBus := eventbus.New()

	ic := indexer.IndexerConfig{
		ExpectationsStore: mes,
		TileSource:        mts,
		EventBus:          eventBus,
		DiffStore:         mds,
		Warmer:            warmer.New(),
	}

	// Set this to a long-enough time that the timer won't fire before
	// the test is complete. We'd like to to be non-zero so it goes through
	// at least one execute pipeline.
	ixr, err := indexer.New(ic, 10*time.Minute)
	assert.NoError(t, err)
	idx := ixr.GetIndex()
	tile := idx.Tile().GetTile(types.ExcludeIgnoredTraces)

	api := SearchAPI{
		DiffStore:         mds,
		ExpectationsStore: mes,
		Indexer:           ixr,
	}

	return api, idx, tile
}

// mockDiffStoreGet is a simple implementation of the diff comparison that
// makes some fake data for the given digest and slice of digests to compare to.
func mockDiffStoreGet(priority int64, dMain types.Digest, dRest types.DigestSlice) map[types.Digest]interface{} {
	result := map[types.Digest]interface{}{}
	for _, d := range dRest {
		if dMain != d {
			result[d] = &diff.DiffMetrics{
				NumDiffPixels:    10,
				PixelDiffPercent: 1.0,
				MaxRGBADiffs:     []int{5, 3, 4, 0},
				DimDiffer:        false,
				Diffs: map[string]float32{
					diff.METRIC_COMBINED: rand.Float32(),
					diff.METRIC_PERCENT:  rand.Float32(),
				},
			}
		}
	}
	return result
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

func randomizeTile(tile *tiling.Tile, testExp types.Expectations) *tiling.Tile {
	allDigestSet := types.DigestSet{}
	for _, digests := range testExp {
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
			gTrace.Digests[i] = allDigests[int(rand.Uint32())%len(allDigests)]
		}
	}
	return ret
}
