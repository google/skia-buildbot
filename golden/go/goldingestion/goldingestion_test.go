package goldingestion

import (
	"os"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/testutils"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/types"
)

const (
	// name of the input file containing test data.
	TEST_INGESTION_FILE = "testdata/dm.json"

	// temporary file used to store traceDB content.
	TRACE_DB_FILENAME = "./test_trace.db"
)

var (
	// trace ids and values that are contained in the test file.
	TEST_ENTRIES = []struct {
		key   string
		value string
	}{
		{key: "x86_64:MSVC:pipe-8888:Debug:CPU:AVX2:ShuttleB:aaclip:Win8:gm", value: "fa3c371d201d6f88f7a47b41862e2e85"},
		{key: "x86_64:MSVC:pipe-8888:Debug:CPU:AVX2:ShuttleB:clipcubic:Win8:gm", value: "64e446d96bebba035887dd7dda6db6c4"},
	}

	// Fix the current point as reference. We remove the nano seconds from
	// now (below) because commits are only precise down to seconds.
	now = time.Now()

	// TEST_COMMITS are the commits we are considering. It needs to contain at
	// least all the commits referenced in the test file.
	TEST_COMMITS = []*vcsinfo.LongCommit{
		&vcsinfo.LongCommit{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    "02cb37309c01506e2552e931efa9c04a569ed266",
				Subject: "Really big code change",
			},
			Timestamp: now.Add(-time.Second * 10).Add(-time.Nanosecond * time.Duration(now.Nanosecond())),
			Branches:  map[string]bool{"master": true},
		},
	}
)

// Tests parsing and processing of a single file.
func TestDMResults(t *testing.T) {
	testutils.SmallTest(t)
	f, err := os.Open(TEST_INGESTION_FILE)
	assert.NoError(t, err)

	dmResults, err := ParseDMResultsFromReader(f)
	assert.NoError(t, err)

	entries := dmResults.getTraceDBEntries()
	assert.Equal(t, len(TEST_ENTRIES), len(entries))

	for _, testEntry := range TEST_ENTRIES {
		found, ok := entries[testEntry.key]
		assert.True(t, ok)
		assert.Equal(t, testEntry.value, string(found.Value))
	}
}

// Tests the processor in conjunction with the vcs.
func TestGoldProcessor(t *testing.T) {
	testutils.SmallTest(t)

	// Set up mock VCS and run a servcer with the given data directory.
	vcs := ingestion.MockVCS(TEST_COMMITS)
	server, serverAddr := ingestion.StartTraceDBTestServer(t, TRACE_DB_FILENAME, "")
	defer server.Stop()
	defer testutils.Remove(t, TRACE_DB_FILENAME)

	ingesterConf := &sharedconfig.IngesterConfig{
		ExtraParams: map[string]string{
			CONFIG_TRACESERVICE: serverAddr,
		},
	}

	// Set up the processor.
	processor, err := newGoldProcessor(vcs, ingesterConf, nil)
	assert.NoError(t, err)
	defer util.Close(processor.(*goldProcessor).traceDB)

	// Load the example file and process it.
	fsResult, err := ingestion.FileSystemResult(TEST_INGESTION_FILE, "./")
	assert.NoError(t, err)
	err = processor.Process(fsResult)
	assert.NoError(t, err)

	// Steal the traceDB used by the processor to verify the results.
	traceDB := processor.(*goldProcessor).traceDB

	startTime := time.Now().Add(-time.Hour * 24 * 10)
	commitIDs, err := traceDB.List(startTime, time.Now())
	assert.NoError(t, err)

	assert.Equal(t, 1, len(filterCommitIDs(commitIDs, "master")))
	assert.Equal(t, 0, len(filterCommitIDs(commitIDs, TEST_CODE_RIETVELDREVIEW_URL)))

	assert.Equal(t, 1, len(commitIDs))
	assert.Equal(t, &tracedb.CommitID{
		Timestamp: TEST_COMMITS[0].Timestamp.Unix(),
		ID:        TEST_COMMITS[0].Hash,
		Source:    "master",
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
	}

	assert.Equal(t, "master", commitIDs[0].Source)
	assert.NoError(t, traceDB.Close())
}

// filterCommitIDs returns all commitIDs that have the given prefix. If the
// prefix is an empty string it will return the input slice.
func filterCommitIDs(commitIDs []*tracedb.CommitID, prefix string) []*tracedb.CommitID {
	if prefix == "" {
		return commitIDs
	}

	ret := make([]*tracedb.CommitID, 0, len(commitIDs))
	for _, cid := range commitIDs {
		if strings.HasPrefix(cid.Source, prefix) {
			ret = append(ret, cid)
		}
	}
	return ret
}
