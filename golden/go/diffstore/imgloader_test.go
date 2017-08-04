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

	"fmt"
)

const (
	// TEST_IMG_DIGEST needs to be stored in the secondary bucket.
	TEST_IMG_DIGEST = "abc-test-image-digest-xyz"

	// Bucket to test loading images through a GS path.
	TEST_GS_BUCKET = "cluster-telemetry"

	// Directory to test loading images through a GS path.
	TEST_GS_BASE_DIR = "tasks/pixel_diff_runs"

	// Image to test loading images through a GS path.
	TEST_IMG_PATH = "lchoi-20170804012953/nopatch/1/http___www_google_com"
)

func TestImageLoader(t *testing.T) {
	testutils.LargeTest(t)
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
	imageLoader.Warm(1, digests, false)
	imageLoader.sync()

	// Make sure they are on disk.
	for _, digest := range digests {
		assert.True(t, fileutil.FileExists(fileutil.TwoLevelRadixPath(workingDir, getDigestImageFileName(digest))))
	}

	// Get the images directly from cache
	ti := timer.New("Fetch images")
	_, err := imageLoader.Get(1, digests)
	assert.NoError(t, err)
	ti.Stop()

	// Fetch images from the secondary bucket.
	_, err = imageLoader.Get(1, []string{TEST_IMG_DIGEST})
	assert.NoError(t, err)
	_, err = imageLoader.Get(1, []string{"some-image-that-does-not-exist-at-all-in-any-bucket"})
	assert.Error(t, err)
}

// Calls TwoLevelRadixPath to create the local image file path.
func DefaultImagePath(baseDir, imageID string) string {
	imagePath := fmt.Sprintf("%s.%s", imageID, IMG_EXTENSION)
	return fileutil.TwoLevelRadixPath(baseDir, imagePath)
}

func getImageLoaderAndTile(t assert.TestingT) (string, string, *tiling.Tile, *ImageLoader) {
	baseDir := TEST_DATA_BASE_DIR + "-imgloader"
	client, tile := getSetupAndTile(t, baseDir)

	workingDir := filepath.Join(baseDir, "images")
	assert.Nil(t, os.Mkdir(workingDir, 0777))

	imgCacheCount, _ := getCacheCounts(10)
	gsBuckets := []string{TEST_GCS_BUCKET_NAME, TEST_GCS_SECONDARY_BUCKET}
	imgLoader, err := newImgLoader(client, baseDir, workingDir, gsBuckets, TEST_GCS_IMAGE_DIR, imgCacheCount, GoldIDPathMapper{})
	assert.NoError(t, err)
	return baseDir, workingDir, tile, imgLoader
}

// Tests loading GS images that are specified through a path.
func TestImageLoaderGetGSPath(t *testing.T) {
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)

	baseDir := TEST_DATA_BASE_DIR + "-imgloader"
	defer testutils.RemoveAll(t, baseDir)

	client, _ := getSetupAndTile(t, baseDir)

	workingDir := filepath.Join(baseDir, "images")
	assert.Nil(t, os.Mkdir(workingDir, 0777))

	imgCacheCount, _ := getCacheCounts(10)
	gsBuckets := []string{TEST_GS_BUCKET}

	imgLoader, err := newImgLoader(client, baseDir, workingDir, gsBuckets, TEST_GS_BASE_DIR, imgCacheCount, PixelDiffIDPathMapper{})
	assert.NoError(t, err)

	_, err = imgLoader.Get(1, []string{TEST_IMG_PATH})
	imgLoader.sync()
	assert.NoError(t, err)
	assert.True(t, fileutil.FileExists(filepath.Join(workingDir, TEST_IMG_PATH+DOT_EXT)))
}
