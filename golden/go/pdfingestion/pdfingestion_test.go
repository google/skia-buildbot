package pdfingestion

import (
	"net/http"
	"path/filepath"
	"testing"

	"cloud.google.com/go/storage"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/goldingestion"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
)

const (
	// name of the input file containing test data.
	TEST_INGESTION_FILE = "testdata/dm.json"

	// bucket where the results are written.
	TEST_BUCKET = "skia-infra-testdata"

	// dirctories with the input and output.
	IMAGES_IN_DIR  = "pdfingestion/dm-images-v1"
	IMAGES_OUT_DIR = "pdfingestion/output/images"
	JSON_OUT_DIR   = "pdfingestion/output/json"
	CACHE_DIR      = "./pdfcache"
)

func TestPDFProcessor(t *testing.T) {
	testutils.LargeTest(t)
	testutils.SkipIfShort(t)

	// Get the service account client from meta data or a local config file.
	client, err := auth.NewJWTServiceAccountClient("", auth.DEFAULT_JWT_FILENAME, nil, storage.ScopeFullControl)
	assert.NoError(t, err)

	cacheDir, err := fileutil.EnsureDirExists(CACHE_DIR)
	assert.NoError(t, err)

	// Clean up after the test.
	defer func() {
		defer util.RemoveAll(cacheDir)
		deleteFolderContent(t, TEST_BUCKET, IMAGES_OUT_DIR, client)
		deleteFolderContent(t, TEST_BUCKET, JSON_OUT_DIR, client)
	}()

	// Configure the processor.
	ingesterConf := &sharedconfig.IngesterConfig{
		ExtraParams: map[string]string{
			CONFIG_INPUT_IMAGES_BUCKET:  TEST_BUCKET,
			CONFIG_INPUT_IMAGES_DIR:     IMAGES_IN_DIR,
			CONFIG_OUTPUT_JSON_BUCKET:   TEST_BUCKET,
			CONFIG_OUTPUT_JSON_DIR:      JSON_OUT_DIR,
			CONFIG_OUTPUT_IMAGES_BUCKET: TEST_BUCKET,
			CONFIG_OUTPUT_IMAGES_DIR:    IMAGES_OUT_DIR,
			CONFIG_PDF_CACHEDIR:         cacheDir,
		},
	}
	processor, err := newPDFProcessor(nil, ingesterConf, client)
	assert.NoError(t, err)

	// Load the example file and process it.
	fsResult, err := ingestion.FileSystemResult(TEST_INGESTION_FILE, "./")
	assert.NoError(t, err)

	err = processor.Process(fsResult)
	assert.NoError(t, err)

	// Fetch the json output and parse it.
	pProcessor := processor.(*pdfProcessor)

	// download the result.
	resultFileName := filepath.Join(CACHE_DIR, "result-file.json")
	assert.NoError(t, pProcessor.download(TEST_BUCKET, JSON_OUT_DIR, fsResult.Name(), resultFileName))

	// Make sure we get the expected result.
	fsResult, err = ingestion.FileSystemResult(TEST_INGESTION_FILE, "./")
	assert.NoError(t, err)
	r, err := fsResult.Open()
	assert.NoError(t, err)
	fsDMResults, err := goldingestion.ParseDMResultsFromReader(r, TEST_INGESTION_FILE)
	assert.NoError(t, err)

	foundResult, err := ingestion.FileSystemResult(resultFileName, "./")
	assert.NoError(t, err)
	r, err = foundResult.Open()
	assert.NoError(t, err)
	foundDMResults, err := goldingestion.ParseDMResultsFromReader(r, TEST_INGESTION_FILE)
	assert.NoError(t, err)

	dmResult1 := *fsDMResults
	dmResult2 := *foundDMResults
	dmResult1.Results = nil
	dmResult2.Results = nil
	assert.Equal(t, dmResult1, dmResult2)

	foundIdx := 0
	srcResults := fsDMResults.Results
	tgtResults := foundDMResults.Results
	for _, result := range srcResults {
		assert.True(t, foundIdx < len(tgtResults))
		if result.Options["ext"] == "pdf" {
			for ; (foundIdx < len(tgtResults)) && (result.Key["name"] == tgtResults[foundIdx].Key["name"]); foundIdx++ {
				assert.True(t, tgtResults[foundIdx].Key["rasterizer"] != "")
				delete(tgtResults[foundIdx].Key, "rasterizer")
				assert.Equal(t, result.Key, tgtResults[foundIdx].Key)
				assert.Equal(t, "png", tgtResults[foundIdx].Options["ext"])
			}
		}
	}
	assert.Equal(t, len(foundDMResults.Results), foundIdx)
}

// deleteFolderContent removes all content ing the given GCS bucket/foldername.
func deleteFolderContent(t *testing.T, bucket, folderName string, client *http.Client) {
	ctx := context.Background()
	cStorage, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	assert.NoError(t, err)

	assert.NoError(t, gcs.DeleteAllFilesInDir(cStorage, bucket, folderName, 1))
}
