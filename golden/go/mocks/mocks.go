package mocks

import (
	"os"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/golden/go/types"
)

// NewMockTileStoreFromJson reads a tile that has been serialized to JSON
// and wraps an instance of TileSource around it.
func NewMockTileSourceFromJson(t require.TestingT, fname string) *TileSource {
	f, err := os.Open(fname)
	require.NoError(t, err)

	tile, err := types.TileFromJson(f, &types.GoldenTrace{})
	require.NoError(t, err)

	mts := &TileSource{}
	cpxTile := types.NewComplexTile(tile)
	cpxTile.SetSparse(tile.Commits)
	mts.On("GetTile").Return(cpxTile)
	return mts
}
