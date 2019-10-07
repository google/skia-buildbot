package testutils

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"cloud.google.com/go/storage"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gcs/gcs_testutils"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/types"
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

func GetSetupAndTile(t assert.TestingT, baseDir string) (*http.Client, *tiling.Tile) {
	testDataPath := filepath.Join(baseDir, TEST_DATA_FILE_NAME)
	assert.NoError(t, gcs_testutils.DownloadTestDataFile(t, TEST_DATA_STORAGE_BUCKET, TEST_DATA_STORAGE_PATH, testDataPath))

	f, err := os.Open(testDataPath)
	assert.NoError(t, err)

	tile, err := types.TileFromJson(f, &types.GoldenTrace{})
	assert.NoError(t, err)

	return GetHTTPClient(t), tile
}

// GetHTTPClient returns a http client either from locally loading a config file
// or by querying meta data in the cloud.
func GetHTTPClient(t assert.TestingT) *http.Client {
	// Get the service account client from meta data or a local config file.
	ts, err := auth.NewJWTServiceAccountTokenSource("", auth.DEFAULT_JWT_FILENAME, storage.ScopeFullControl)
	if err != nil {
		fmt.Println("If you are running this test locally, be sure you have a service-account.json in the test folder.")
	}
	assert.NoError(t, err)
	return httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
}
