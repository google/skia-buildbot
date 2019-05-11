package search

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs/gcs_testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

func TestTraceViewFn(t *testing.T) {
	unittest.MediumTest(t)

	_, _, tile, _ := getStoragesIndexTile(t, gcs_testutils.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH, false)

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
