package trybot

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/testutils"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/goldingestion"
	"go.skia.org/infra/golden/go/types"
)

const (
	TEST_DATA_DIR                = "./testdata"
	TEST_TRACE_DB_FILE           = "./tracedb-data.db"
	TEST_SHAREDB_DIR             = "./sharedb-data"
	TEST_RIETVELD_INGESTION_FILE = TEST_DATA_DIR + "/rietveld-dm.json"
	TEST_GERRIT_INGESTION_FILE   = TEST_DATA_DIR + "/gerrit-dm.json"

	// URL of the rietveld instance.
	TEST_CODE_RIETVELDREVIEW_URL = rietveld.RIETVELD_SKIA_URL

	// URL of the gerrit instance.
	TEST_CODE_GERRITREVIEW_URL = gerrit.GERRIT_SKIA_URL
)

var (
	BEGINNING_OF_TIME = time.Date(2015, time.June, 1, 0, 0, 0, 0, time.UTC)
)

func TestRietveldTrybotResults(t *testing.T) {
	testOneTrybotResults(t, TEST_RIETVELD_INGESTION_FILE)
}

func TestGerritTrybotResults(t *testing.T) {
	testOneTrybotResults(t, TEST_GERRIT_INGESTION_FILE)
}

func testOneTrybotResults(t *testing.T, ingestionFile string) {
	testutils.LargeTest(t)

	rietveldAPI := rietveld.New(TEST_CODE_RIETVELDREVIEW_URL, nil)
	gerritAPI, err := gerrit.NewGerrit(TEST_CODE_GERRITREVIEW_URL, "", nil)
	assert.NoError(t, err)

	server, serverAddress := goldingestion.RunGoldTrybotProcessor(t, TEST_TRACE_DB_FILE, TEST_SHAREDB_DIR, ingestionFile, TEST_DATA_DIR, TEST_CODE_RIETVELDREVIEW_URL, TEST_CODE_GERRITREVIEW_URL)
	defer util.RemoveAll(TEST_SHAREDB_DIR)
	defer testutils.Remove(t, TEST_TRACE_DB_FILE)
	defer server.Stop()

	db, err := tracedb.NewTraceServiceDBFromAddress(serverAddress, types.GoldenTraceBuilder)
	assert.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	ingestionStore, err := goldingestion.NewIngestionStore(serverAddress)
	assert.NoError(t, err)
	defer func() { assert.NoError(t, ingestionStore.Close()) }()

	tileBuilder := tracedb.NewBranchTileBuilder(db, nil, rietveldAPI, gerritAPI, eventbus.New())
	tr := NewTrybotResults(tileBuilder, rietveldAPI, gerritAPI, ingestionStore)
	tr.timeFrame = time.Now().Sub(BEGINNING_OF_TIME)

	issues, total, err := tr.ListTrybotIssues(0, 20)
	assert.NoError(t, err)
	assert.NotNil(t, issues)
	assert.Equal(t, 1, len(issues))
	assert.Equal(t, 1, total)

	// issue, tile, err := tr.GetIssue(issues[0].ID)
	_, tile, err := tr.GetIssue(issues[0].ID, nil)
	assert.NoError(t, err)

	foundDigests := util.NewStringSet()
	for _, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		foundDigests.AddLists(gTrace.Values)
	}

	// // Parse the input file and extract 'by hand'
	fsResult, err := ingestion.FileSystemResult(ingestionFile, "./")
	assert.NoError(t, err)

	r, err := fsResult.Open()
	assert.NoError(t, err)
	dmResults, err := goldingestion.ParseDMResultsFromReader(r, fsResult.Name())
	assert.NoError(t, err)

	expectedDigests := util.NewStringSet()
	for _, result := range dmResults.Results {
		if result.Options["ext"] == "png" {
			expectedDigests[result.Digest] = true
		}
	}
	assert.Equal(t, expectedDigests, foundDigests)
}
