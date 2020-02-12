package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func Test_Prev(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, TileNumber(0), TileNumber(1).Prev())
	assert.Equal(t, BadTileNumber, TileNumber(0).Prev())
}

func TestTileNumberFromCommitNumber(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, TileNumber(0), TileNumberFromCommitNumber(CommitNumber(0), 256))
	assert.Equal(t, TileNumber(0), TileNumberFromCommitNumber(CommitNumber(255), 256))
	assert.Equal(t, TileNumber(1), TileNumberFromCommitNumber(CommitNumber(256), 256))
	assert.Equal(t, TileNumber(1), TileNumberFromCommitNumber(CommitNumber(257), 256))
}

func TestTileNumberFromCommitNumber_BadTileSize(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, BadTileNumber, TileNumberFromCommitNumber(CommitNumber(256), 0))
}
