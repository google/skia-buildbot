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
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/serialize"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/warmer"
)

const (
	// Directory with testdata.
	TEST_DATA_DIR = "./testdata"

	// Local file location of the test data.
	TEST_DATA_PATH = TEST_DATA_DIR + "/10-test-sample-4bytes.tile"

	// Folder in the testdata bucket. See go/testutils for details.
	TEST_DATA_STORAGE_PATH = "gold-testdata/10-test-sample-4bytes.tile"
)

func TestTraceViewFn(t *testing.T) {
	unittest.MediumTest(t)

	_, _, tile := getAPIIndexTile(t, gcs_testutils.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH, false)

	commits := tile.Commits[0 : tile.LastCommitIndex()+1]
	middle := len(commits) / 2
	beginIdx := middle - 1
	endIdx := middle + 1
	fBegin := commits[beginIdx].Hash
	fEnd := commits[endIdx].Hash

	// Make sure we get an error when the beginning commit comes before the ending commit.
	testTraceView(t, tile, beginIdx, endIdx, fEnd, fBegin, true)

	// Check various valid commit ranges that should all be valid.
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
		assert.Equal(t, tr.Digests[beginIdx:endIdx+1], reducedTr.Digests)
	}
}

var (
	testOne     = types.TestName("test-1")
	testTwo     = types.TestName("test-2")
	digestOne   = types.Digest("abcefgh")
	paramSetOne = paramtools.ParamSet{
		"param-01": {"val-01"},
		"param-02": {"val-02"},
	}

	paramsTwo = paramtools.Params{
		"param-01": "gato",
		"param-03": "robato",
	}

	goldTrace = types.GoldenTrace{
		Keys: map[string]string{"param-01": "dog"},
	}
)

// TestIntermediate adds a few entries to the intermediate
// representation and makes sure that the data properly reflects it.
func TestIntermediate(t *testing.T) {
	unittest.SmallTest(t)

	srMap := srInterMap{}
	srMap.Add(testOne, digestOne, "", nil, paramSetOne)
	srMap.AddTestParams(testOne, digestOne, paramsTwo)
	srMap.AddTestParams(testTwo, digestOne, paramsTwo)
	srMap.Add(testTwo, digestOne, "mytrace", &goldTrace, paramSetOne)

	assert.Equal(t, srInterMap{
		testOne: map[types.Digest]*srIntermediate{
			digestOne: {
				test:   testOne,
				digest: digestOne,
				params: paramtools.ParamSet{
					"param-01": {"val-01", "gato"},
					"param-02": {"val-02"},
					"param-03": {"robato"},
				},
				traces: map[tiling.TraceId]*types.GoldenTrace{},
			},
		},
		testTwo: map[types.Digest]*srIntermediate{
			digestOne: {
				test:   testTwo,
				digest: digestOne,
				params: paramtools.ParamSet{
					"param-01": {"gato", "dog"},
					"param-03": {"robato"},
				},
				traces: map[tiling.TraceId]*types.GoldenTrace{
					"mytrace": &goldTrace,
				},
			},
		},
	}, srMap)

}

func getAPIIndexTile(t *testing.T, bucket, storagePath, outputPath string, randomize bool) (SearchImpl, indexer.IndexSearcher, *tiling.Tile) {
	err := gcs_testutils.DownloadTestDataFile(t, bucket, storagePath, outputPath)
	assert.NoError(t, err, "Unable to download testdata.")
	return getAPIAndIndexerFromTile(t, outputPath, randomize)
}

func getAPIAndIndexerFromTile(t sktest.TestingT, path string, randomize bool) (SearchImpl, indexer.IndexSearcher, *tiling.Tile) {
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

	api := SearchImpl{
		diffStore:         mds,
		expectationsStore: mes,
		indexSource:       ixr,
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
