package diffstore

import (
	"os"
	"path/filepath"
	"testing"

	"cloud.google.com/go/storage"
	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/redisutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/types"
)

const (
	TEST_IMG_WORKING_DIR = "./img-concur-testdata"
	REDIS_SERVER_ADDRESS = "127.0.0.1:6379"
	REDIS_DB             = 0
)

var (
	// Directory with testdata.
	TEST_DATA_DIR = "./testdata"

	// Local file location of the test data.
	TEST_DATA_PATH = TEST_DATA_DIR + "/goldentile.json"

	// Folder in the testdata bucket. See go/testutils for details.
	TEST_DATA_STORAGE_BUCKET = "skia-infra-testdata"
	TEST_DATA_STORAGE_PATH   = "gold-testdata/goldentile.json.gz"

	// Number of images to prefetch.
	TEST_PREFETCH_N_IMAGES = 100
)

func TestImageLoader(t *testing.T) {
	testutils.SkipIfShort(t)

	workingDir, tile, imageLoader := getImageLoaderAndTile(t)
	defer testutils.RemoveAll(t, workingDir)

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
	digests := digestSet.Keys()[:TEST_PREFETCH_N_IMAGES]
	assert.NoError(t, imageLoader.Warm(1, digests, true))

	// Make sure they are on disk.
	for _, digest := range digests {
		assert.True(t, fileutil.FileExists(fileutil.TwoLevelRadixPath(workingDir, getDigestImageFileName(digest))))
	}

	// Get the images directly from cache.
	ti := timer.New("Fetch images")
	for _, d := range digests {
		_, err := imageLoader.Get(1, d)
		assert.NoError(t, err)
	}
	ti.Stop()
}

func getImageLoaderAndTile(t assert.TestingT) (string, *tiling.Tile, *ImageLoader) {
	testDataDir := TEST_DATA_DIR
	testutils.RemoveAll(t, testDataDir)
	assert.NoError(t, gs.DownloadTestDataFile(t, TEST_DATA_STORAGE_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH))
	defer testutils.RemoveAll(t, testDataDir)

	tile := mocks.NewMockTileBuilderFromJson(t, TEST_DATA_PATH).GetTile()

	workingDir := filepath.Join(TEST_IMG_WORKING_DIR)
	testutils.RemoveAll(t, workingDir)
	assert.Nil(t, os.Mkdir(workingDir, 0777))

	// Get the service account client from meta data or a local config file.
	client, err := auth.NewJWTServiceAccountClient("", auth.DEFAULT_JWT_FILENAME, nil, storage.ScopeFullControl)
	assert.NoError(t, err)

	// gsBucketName := "skia-infra-gm"
	gsBucketName := "chromium-skia-gm"
	gsImageDir := "dm-images-v1"

	rp := redisutil.NewRedisPool(REDIS_SERVER_ADDRESS, REDIS_DB)
	assert.NoError(t, rp.FlushDB())

	imgLoader, err := newImgLoader(client, workingDir, gsBucketName, gsImageDir, rp, true)
	assert.NoError(t, err)
	return workingDir, tile, imgLoader
}
