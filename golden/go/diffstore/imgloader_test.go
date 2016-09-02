package diffstore

import (
	"os"
	"path/filepath"
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

func TestImageLoader(t *testing.T) {
	testutils.SkipIfShort(t)

	baseDir, workingDir, tile, imageLoader := getImageLoaderAndTile(t)
	defer testutils.RemoveAll(t, baseDir)

	// Iterate over the tile and get all the digests
	digestSet := util.NewStringSet()
	for _, trace := range tile.Traces {
		gt := trace.(*types.GoldenTrace)
		for _, val := range gt.Values {
			if val != types.MISSING_DIGEST {
				digestSet[val] = true
			}
		}
	}

	// Prefetch the images synchronously.
	digests := digestSet.Keys()[:100]
	imageLoader.Warm(1, digests)
	imageLoader.sync()

	// Make sure they are on disk.
	for _, digest := range digests {
		assert.True(t, fileutil.FileExists(fileutil.TwoLevelRadixPath(workingDir, getDigestImageFileName(digest))))
	}

	// Get the images directly from cache.
	ti := timer.New("Fetch images")
	_, err := imageLoader.Get(1, digests)
	assert.NoError(t, err)
	ti.Stop()
}

func getImageLoaderAndTile(t assert.TestingT) (string, string, *tiling.Tile, *ImageLoader) {
	baseDir := TEST_DATA_BASE_DIR + "-imgloader"
	client, tile := getSetupAndTile(t, baseDir)

	workingDir := filepath.Join(baseDir, "images")
	assert.Nil(t, os.Mkdir(workingDir, 0777))

	imgLoader, err := newImgLoader(client, workingDir, TEST_GS_BUCKET_NAME, TEST_GS_IMAGE_DIR)
	assert.NoError(t, err)
	return baseDir, workingDir, tile, imgLoader
}
