package gcssources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/types"
)

func TestIter_FullRangeOK_Success(t *testing.T) {
	unittest.SmallTest(t)

	results := [][2]types.CommitNumber{}
	it, err := newIter(500, 10, 5)
	require.NoError(t, err)
	for it.Next() {
		begin, end := it.Range()
		results = append(results, [2]types.CommitNumber{begin, end})
	}
	require.Equal(t, [][2]types.CommitNumber{{491, 500}, {471, 490}, {431, 470}, {351, 430}, {191, 350}}, results)
}

func TestIter_FullRangeWouldGoBelowZero_ReturnsRangesToExaclyZeroAndStops(t *testing.T) {
	unittest.SmallTest(t)

	results := [][2]types.CommitNumber{}
	it, err := newIter(50, 10, 5)
	require.NoError(t, err)
	for it.Next() {
		begin, end := it.Range()
		results = append(results, [2]types.CommitNumber{begin, end})
	}
	require.Equal(t, [][2]types.CommitNumber{{41, 50}, {21, 40}, {0, 20}}, results)
}

func TestIter_InvalidParams_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	_, err := newIter(types.BadCommitNumber, 10, 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "end is invalid")
	_, err = newIter(500, -10, 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tileSize is invalid")
	_, err = newIter(500, 10, -1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max is invalid")
}
