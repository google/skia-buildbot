package goldingestion

import (
	"context"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/testutils"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/types"
)

const (
	// directory with the test data.
	TEST_DATA_DIR = "./testdata"

	// name of the input file containing test data.
	TRYBOT_INGESTION_FILE = TEST_DATA_DIR + "/trybot-dm.json"

	// temporary file used to store traceDB content.
	TRYBOT_TRACE_DB_FILE = "./trybot_test_trace.db"

	// temporary directory to store sharedb content.
	TRYBOT_SHAREDB_DIR = "./sharedb-testdata"

	// URL of the rietveld instance.
	TEST_CODE_RIETVELDREVIEW_URL = rietveld.RIETVELD_SKIA_URL

	// URL of the gerrit instance.
	TEST_CODE_GERRITREVIEW_URL = gerrit.GERRIT_SKIA_URL
)

var (
	// trace ids and values that are contained in the test file.
	TRYBOT_TEST_ENTRIES = []struct {
		key   string
		value string
	}{
		{key: "x86_64:MSVC:pipe-8888:Debug:CPU:AVX2:ShuttleB:aaclip:Win8:gm", value: "fa3c371d201d6f88f7a47b41862e2e85"},
		{key: "x86_64:MSVC:pipe-8888:Debug:CPU:AVX2:ShuttleB:clipcubic:Win8:gm", value: "64e446d96bebba035887dd7dda6db6c4"},
	}

	TRYBOT_TEST_COMMITS = []*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    "02cb37309c01506e2552e931efa9c04a569ed266",
				Subject: "Really big code change",
			},
			Timestamp: now.Add(-time.Second * 10).Add(-time.Nanosecond * time.Duration(now.Nanosecond())),
			Branches:  map[string]bool{"master": true},
		},
	}
)

// Tests the processor in conjunction with the vcs.
func TestTrybotGoldProcessor(t *testing.T) {
	testutils.LargeTest(t)
	ctx := context.Background()
	server, serverAddress := RunGoldTrybotProcessor(t, ctx, TRYBOT_TRACE_DB_FILE, TRYBOT_SHAREDB_DIR, TRYBOT_INGESTION_FILE, TEST_DATA_DIR, TEST_CODE_RIETVELDREVIEW_URL, TEST_CODE_GERRITREVIEW_URL)
	defer util.RemoveAll(TRYBOT_SHAREDB_DIR)
	defer testutils.Remove(t, TRYBOT_TRACE_DB_FILE)
	defer server.Stop()

	// Steal the traceDB used by the processor to verify the results.
	traceDB, err := tracedb.NewTraceServiceDBFromAddress(serverAddress, types.GoldenTraceBuilder)
	assert.NoError(t, err)
	defer util.Close(traceDB)

	// The timestamp for the issue/patchset in the testfile is 1443718869.
	startTime := time.Unix(1443718868, 0)
	commitIDs, err := traceDB.List(startTime, time.Now())
	assert.NoError(t, err)

	assert.Equal(t, 1, len(filterCommitIDs(commitIDs, TEST_CODE_RIETVELDREVIEW_URL)))
	assert.Equal(t, 0, len(filterCommitIDs(commitIDs, "master")))

	assert.Equal(t, 1, len(commitIDs))
	assert.Equal(t, &tracedb.CommitID{
		Timestamp: 1443718869,
		ID:        "1",
		Source:    TEST_CODE_RIETVELDREVIEW_URL + "/1381483003",
	}, commitIDs[0])

	// Get a tile and make sure we have the right number of traces.
	tile, _, err := traceDB.TileFromCommits(commitIDs)
	assert.NoError(t, err)

	traces := tile.Traces
	assert.Equal(t, len(TEST_ENTRIES), len(traces))

	for _, testEntry := range TEST_ENTRIES {
		found, ok := traces[testEntry.key]
		assert.True(t, ok)
		goldTrace, ok := found.(*types.GoldenTrace)
		assert.True(t, ok)
		assert.Equal(t, 1, len(goldTrace.Values))
		assert.Equal(t, testEntry.value, goldTrace.Values[0])
		assert.Equal(t, "no", goldTrace.Params()["gamma_correct"])
	}

	// Make sure the prefix is stripped correctly.
	rietveldReview := rietveld.New(TEST_CODE_RIETVELDREVIEW_URL, nil)
	gerritReview, err := gerrit.NewGerrit(TEST_CODE_GERRITREVIEW_URL, "", nil)
	assert.NoError(t, err)

	assert.Equal(t, TEST_CODE_RIETVELDREVIEW_URL+"/1381483003", commitIDs[0].Source)
	issueID, patchsetID := ExtractIssueInfo(commitIDs[0], rietveldReview, gerritReview)
	assert.Equal(t, "1", patchsetID)
	assert.Equal(t, "1381483003", issueID)
	issueID, patchsetID = ExtractIssueInfo(&tracedb.CommitID{}, rietveldReview, gerritReview)
	assert.Equal(t, "", issueID)
	assert.Equal(t, "", patchsetID)
	assert.NoError(t, traceDB.Close())
}
