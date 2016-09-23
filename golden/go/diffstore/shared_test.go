package diffstore

import (
	"net/http"
	"path/filepath"

	"cloud.google.com/go/storage"
	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gs"
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

	// GS locations wher images are stored.
	TEST_GS_BUCKET_NAME      = "chromium-skia-gm"
	TEST_GS_SECONDARY_BUCKET = "skia-infra-testdata"
	TEST_GS_IMAGE_DIR        = "dm-images-v1"
)

func getSetupAndTile(t assert.TestingT, baseDir string) (*http.Client, *tiling.Tile) {
	testDataPath := filepath.Join(baseDir, TEST_DATA_FILE_NAME)
	assert.NoError(t, gs.DownloadTestDataFile(t, TEST_DATA_STORAGE_BUCKET, TEST_DATA_STORAGE_PATH, testDataPath))

	tile := mocks.NewMockTileBuilderFromJson(t, testDataPath).GetTile()

	return getClient(t), tile
}

func getClient(t assert.TestingT) *http.Client {
	// Get the service account client from meta data or a local config file.
	client, err := auth.NewJWTServiceAccountClient("", auth.DEFAULT_JWT_FILENAME, nil, storage.ScopeFullControl)
	assert.NoError(t, err)
	return client
}
