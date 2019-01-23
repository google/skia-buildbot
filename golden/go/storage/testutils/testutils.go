package testutils

import (
	"os"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/serialize"
)

func GetTileFromGCS(t assert.TestingT, bucket, storagePath, outputPath string) *tiling.Tile {
	err := gcs.DownloadTestDataFile(t, bucket, storagePath, outputPath)
	assert.NoError(t, err, "Unable to download testdata.")
	return LoadTile(t, outputPath)
}

func LoadTile(t assert.TestingT, path string) *tiling.Tile {
	file, err := os.Open(path)
	assert.NoError(t, err)

	sample, err := serialize.DeserializeSample(file)
	assert.NoError(t, err)

	return sample.Tile
}
