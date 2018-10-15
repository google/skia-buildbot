package diffstore

import (
	"net/http"
	"path/filepath"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/mocks"
)

const (
	TEST_IMG_WORKING_DIR = "./img-concur-testdata"
)

var (
	// Directory with testdata.
	TEST_DATA_BASE_DIR = "./testdata"

	// File name of the test data.
	TEST_DATA_FILE_NAME = "goldentile.json"

	// Folder in the testdata bucket. See go/testutils for details.
	TEST_DATA_STORAGE_BUCKET = "skia-infra-testdata"
	TEST_DATA_STORAGE_PATH   = "gold-testdata/goldentile.json.gz"

	// GCS locations where images are stored.
	TEST_GCS_BUCKET_NAME      = "skia-infra-gm"
	TEST_GCS_SECONDARY_BUCKET = "skia-infra-testdata"
	TEST_GCS_IMAGE_DIR        = "dm-images-v1"

	// Specific path to an image in GCS.
	TEST_PATH_IMG_1 = "gold-testdata/filediffstore-testdata/10552995703607727960.png"
)

func getSetupAndTile(t assert.TestingT, baseDir string) (*http.Client, *tiling.Tile) {
	testDataPath := filepath.Join(baseDir, TEST_DATA_FILE_NAME)
	assert.NoError(t, gcs.DownloadTestDataFile(t, TEST_DATA_STORAGE_BUCKET, TEST_DATA_STORAGE_PATH, testDataPath))

	tile := mocks.NewMockTileBuilderFromJson(t, testDataPath).GetTile()

	return mocks.GetHTTPClient(t), tile
}
