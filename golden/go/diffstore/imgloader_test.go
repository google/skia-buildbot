package diffstore

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"testing"

	"cloud.google.com/go/storage"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/diffstore/common"
	"go.skia.org/infra/golden/go/diffstore/mapper"
	"go.skia.org/infra/golden/go/diffstore/mapper/disk_mapper"
	d_utils "go.skia.org/infra/golden/go/diffstore/testutils"
	"go.skia.org/infra/golden/go/types"
	"google.golang.org/api/option"
)

const (
	// TEST_IMG_DIGEST needs to be stored in the secondary bucket.
	TEST_IMG_DIGEST = "abc-test-image-digest-xyz"
)

func TestImageLoader(t *testing.T) {
	unittest.LargeTest(t)

	m := &disk_mapper.DiskMapper{}
	workingDir, tile, imageLoader, cleanup := getImageLoaderAndTile(t, m)
	defer cleanup()

	// Iterate over the tile and get all the digests
	digestSet := types.DigestSet{}
	for _, trace := range tile.Traces {
		gt := trace.(*types.GoldenTrace)
		for _, val := range gt.Digests {
			if val != types.MISSING_DIGEST {
				digestSet[val] = true
			}
		}
	}

	// Prefetch the images synchronously.
	digests := digestSet.Keys()[:100]
	imageLoader.Warm(1, digests, true)

	// Make sure they are all on disk.
	for _, digest := range digests {
		fn := common.GetDigestImageFileName(digest)
		assert.True(t, fileutil.FileExists(fileutil.TwoLevelRadixPath(workingDir, fn)))
	}

	_, _, err := imageLoader.Get(1, types.DigestSlice{"some-image-that-does-not-exist-at-all-in-any-bucket"})
	assert.Error(t, err)
}

func getImageLoaderAndTile(t sktest.TestingT, m mapper.Mapper) (string, *tiling.Tile, *ImageLoader, func()) {
	w, cleanup := testutils.TempDir(t)
	baseDir := path.Join(w, d_utils.TEST_DATA_BASE_DIR+"-imgloader")
	client, tile := d_utils.GetSetupAndTile(t, baseDir)

	workingDir := filepath.Join(baseDir, "images")
	assert.Nil(t, os.Mkdir(workingDir, 0777))

	imgCacheCount, _ := getCacheCounts(10)

	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	assert.NoError(t, err)
	gcsClient := gcsclient.New(storageClient, d_utils.TEST_GCS_BUCKET_NAME)

	imgLoader, err := NewImgLoader(gcsClient, baseDir, workingDir, d_utils.TEST_GCS_IMAGE_DIR, imgCacheCount, m)
	assert.NoError(t, err)
	return workingDir, tile, imgLoader, cleanup
}

func TestImagePaths(t *testing.T) {
	unittest.SmallTest(t)

	digest := types.Digest("098f6bcd4621d373cade4e832627b4f6")
	expectedLocalPath := filepath.Join("09", "8f", string(digest)+".png")
	expectedGSPath := string(digest + ".png")
	localPath, gsPath := ImagePaths(digest)
	assert.Equal(t, expectedLocalPath, localPath)
	assert.Equal(t, expectedGSPath, gsPath)
}
