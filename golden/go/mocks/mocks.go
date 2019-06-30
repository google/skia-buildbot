package mocks

import (
	"os"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/golden/go/types"
)

// NewMockTileStoreFromJson reads a tile that has been serialized to JSON
// and wraps an instance of TileSource around it.
func NewMockTileSourceFromJson(t assert.TestingT, fname string) *TileSource {
	f, err := os.Open(fname)
	assert.NoError(t, err)

	tile, err := types.TileFromJson(f, &types.GoldenTrace{})
	assert.NoError(t, err)

	mts := &TileSource{}
	cpxTile := types.NewComplexTile(tile)
	cpxTile.SetSparse(tile.Commits)
	mts.On("GetTile").Return(cpxTile, nil)
	return mts
}
