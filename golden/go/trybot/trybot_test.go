package trybot

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/testutils"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/goldingestion"
	"go.skia.org/infra/golden/go/types"
)

const (
	TEST_DATA_DIR        = "./testdata"
	TEST_TRACE_DB_FILE   = "./tracedb-data.db"
	TEST_SHAREDB_DIR     = "./sharedb-data"
	TEST_INGESTION_FILE  = TEST_DATA_DIR + "/dm.json"
	TEST_CODE_REVIEW_URL = "https://codereview.chromium.org"
)

var (
	BEGINNING_OF_TIME = time.Date(2015, time.June, 1, 0, 0, 0, 0, time.UTC)
)

func TestTrybotResults(t *testing.T) {
	testutils.SkipIfShort(t)

	rietveldAPI := rietveld.New(TEST_CODE_REVIEW_URL, nil)
	server, serverAddress := goldingestion.RunGoldTrybotProcessor(t, TEST_TRACE_DB_FILE, TEST_SHAREDB_DIR, TEST_INGESTION_FILE, TEST_DATA_DIR, TEST_CODE_REVIEW_URL)
	defer util.RemoveAll(TEST_SHAREDB_DIR)
	defer testutils.Remove(t, TEST_TRACE_DB_FILE)
	defer server.Stop()

	db, err := tracedb.NewTraceServiceDBFromAddress(serverAddress, types.GoldenTraceBuilder)
	assert.Nil(t, err)
	defer util.Close(db)

	ingestionStore, err := goldingestion.NewIngestionStore(serverAddress)
	assert.Nil(t, err)

	tileBuilder := tracedb.NewBranchTileBuilder(db, nil, rietveldAPI, eventbus.New(nil))
	tr := NewTrybotResults(tileBuilder, rietveldAPI, ingestionStore)
	tr.timeFrame = time.Now().Sub(BEGINNING_OF_TIME)

	issues, total, err := tr.ListTrybotIssues(0, 20)
	assert.Nil(t, err)
	assert.NotNil(t, issues)
	assert.Equal(t, 1, len(issues))
	assert.Equal(t, 1, total)

	// issue, tile, err := tr.GetIssue(issues[0].ID)
	_, tile, err := tr.GetIssue(issues[0].ID, nil, false)
	assert.Nil(t, err)

	foundDigests := util.NewStringSet()
	for _, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		foundDigests.AddLists(gTrace.Values)
	}

	// // Parse the input file and extract 'by hand'
	fsResult, err := ingestion.FileSystemResult(TEST_INGESTION_FILE, "./")
	assert.Nil(t, err)

	r, err := fsResult.Open()
	assert.Nil(t, err)
	dmResults, err := goldingestion.ParseDMResultsFromReader(r)
	assert.Nil(t, err)

	expectedDigests := util.NewStringSet()
	for _, result := range dmResults.Results {
		if result.Options["ext"] == "png" {
			expectedDigests[result.Digest] = true
		}
	}
	assert.Equal(t, expectedDigests, foundDigests)

}
