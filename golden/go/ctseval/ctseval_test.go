package ctseval

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/search"

	assert "github.com/stretchr/testify/require"
)

const (
	TEST_INGESTION_DATA_DIR = "./test_data_ingestion"
	TEST_DATA_DIR           = "./test_data"
	TEST_DIFFSTORE_DATA_DIR = "./test_data_diffstore"

	TEST_INGESTION_BUCKET = "skia-firebase-test-lab"
	TEST_INGESTION_PATH   = "testruns"

	TEST_CTS_DATA_DIR           = "cts-data"
	TEST_CTS_SAMPLED_TILE_FNAME = "cts-eval-tile"
)

func TestCTSEvaluator(t *testing.T) {
	t.SkipNow()

	cloudTilePath := TEST_CTS_DATA_DIR + "/" + TEST_CTS_SAMPLED_TILE_FNAME + ".gz"
	localTilePath := TEST_DATA_DIR + "/" + TEST_CTS_SAMPLED_TILE_FNAME

	if !fileutil.FileExists(localTilePath) {
		assert.NoError(t, gcs.DownloadTestDataFile(t, gcs.TEST_DATA_BUCKET, cloudTilePath, localTilePath))
	}
	defer testutils.RemoveAll(t, TEST_DATA_DIR)

	storages, _, ixr := search.GetStoragesAndIndexerFromTile(t, localTilePath)

	client := mocks.GetHTTPClient(t)

	evaluator, err := NewCTSEvalator(TEST_INGESTION_BUCKET, TEST_INGESTION_PATH, TEST_DATA_DIR, client, time.Minute*60, 30, ixr, storages)
	assert.NoError(t, err)
	assert.NotNil(t, evaluator)

	time.Sleep(time.Second * 60)
	assert.True(t, fileutil.FileExists(filepath.Join(TEST_DATA_DIR, "testrun-1504793111030")))
}

func TestKnowledge(t *testing.T) {

	cloudTilePath := TEST_CTS_DATA_DIR + "/" + TEST_CTS_SAMPLED_TILE_FNAME + ".gz"
	localTilePath := TEST_DATA_DIR + "/" + TEST_CTS_SAMPLED_TILE_FNAME

	if !fileutil.FileExists(localTilePath) {
		assert.NoError(t, gcs.DownloadTestDataFile(t, gcs.TEST_DATA_BUCKET, cloudTilePath, localTilePath))
	}
	defer testutils.RemoveAll(t, TEST_DATA_DIR)

	storages, _, ixr := search.GetStoragesAndIndexerFromTile(t, localTilePath)

	client := mocks.GetHTTPClient(t)
	evaluator, err := NewCTSEvalator(TEST_INGESTION_BUCKET, TEST_INGESTION_PATH, TEST_DATA_DIR, client, 0, 30, ixr, storages)
	assert.NoError(t, err)
	var knowledgeBytes []byte = nil
	for {
		time.Sleep(1 * time.Second)
		knowledgeBytes = evaluator.KnowledgeZip()
		if knowledgeBytes != nil {
			break
		}
	}

	assert.NotNil(t, knowledgeBytes)
	f, err := os.Create("knowledge.zip")
	assert.NoError(t, err)
	defer f.Close()

	_, err = f.Write(knowledgeBytes)
	assert.NoError(t, err)
}
