package types

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Prev(t *testing.T) {
	assert.Equal(t, TileNumber(0), TileNumber(1).Prev())
	assert.Equal(t, BadTileNumber, TileNumber(0).Prev())
}

func TestTileNumberFromCommitNumber(t *testing.T) {
	assert.Equal(t, TileNumber(0), TileNumberFromCommitNumber(CommitNumber(0), 256))
	assert.Equal(t, TileNumber(0), TileNumberFromCommitNumber(CommitNumber(255), 256))
	assert.Equal(t, TileNumber(1), TileNumberFromCommitNumber(CommitNumber(256), 256))
	assert.Equal(t, TileNumber(1), TileNumberFromCommitNumber(CommitNumber(257), 256))
}

func TestTileNumberFromCommitNumber_BadTileSize(t *testing.T) {
	assert.Equal(t, BadTileNumber, TileNumberFromCommitNumber(CommitNumber(256), 0))
}

func TestTileCommitRangeForTileNumber(t *testing.T) {
	begin, end := TileCommitRangeForTileNumber(TileNumber(0), 256)
	assert.Equal(t, CommitNumber(0), begin)
	assert.Equal(t, CommitNumber(255), end)

	begin, end = TileCommitRangeForTileNumber(TileNumber(1), 256)
	assert.Equal(t, CommitNumber(256), begin)
	assert.Equal(t, CommitNumber(511), end)
}

func TestCommitNumberSlice_Sort_Success(t *testing.T) {
	toSort := CommitNumberSlice{2, BadCommitNumber, 1}
	sort.Sort(toSort)
	assert.Equal(t, CommitNumberSlice{BadCommitNumber, 1, 2}, toSort)
}

func TestCommitNumber_Add(t *testing.T) {
	assert.Equal(t, CommitNumber(2), CommitNumber(1).Add(1))
	assert.Equal(t, CommitNumber(0), CommitNumber(1).Add(-1))
	assert.Equal(t, BadCommitNumber, CommitNumber(1).Add(-2))
	assert.Equal(t, BadCommitNumber, CommitNumber(1).Add(-100))
}
