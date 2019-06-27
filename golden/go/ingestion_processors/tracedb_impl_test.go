package ingestion_processors

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/go/vcsinfo/mocks"
	trace_utils "go.skia.org/infra/golden/go/testutils"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Same information as testdata/dm.json but uses a secondary repository.
	TEST_SECONDARY_FILE = "testdata/dm-secondary.json"

	// Ingestion file that contains a commit that is neither in the primary nor the secondary repo.
	TEST_SECONDARY_FILE_INVALID = "testdata/dm-secondary-invalid.json"

	// Ingestion file that contains a valid commit but does not have a valid commit in the primary.
	TEST_SECONDARY_FILE_NO_DEPS = "testdata/dm-secondary-missing-deps.json"

	// temporary file used to store traceDB content.
	TRACE_DB_FILENAME = "./test_trace.db"
)

var (
	// trace ids and values that are contained in the test file.
	// These trace ids are the old format, new ones are like:
	//   ,key1=value1,key2=value2
	TEST_ENTRIES = []struct {
		key   tiling.TraceId
		value types.Digest
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
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    "02cb37309c01506e2552e931efa9c04a569ed266",
				Subject: "Really big code change",
			},
			Timestamp: now.Add(-time.Second * 10).Add(-time.Nanosecond * time.Duration(now.Nanosecond())),
			Branches:  map[string]bool{"master": true},
		},
	}

	// Commits in the secondary input files.
	VALID_COMMIT   = "500920e65ced121bf72e690b31316e8bce606b4c"
	INVALID_COMMIT = "789e59b3592bc07288aa13c3dc10422c684a8bd3"
	NO_DEPS_COMMIT = "94252352a0dc5e2fcca754548018029562de0fb1"

	SECONDARY_TEST_COMMITS = []*vcsinfo.LongCommit{
		{ShortCommit: &vcsinfo.ShortCommit{Hash: VALID_COMMIT, Subject: "Really big code change"}},
		{ShortCommit: &vcsinfo.ShortCommit{Hash: NO_DEPS_COMMIT, Subject: "Small code change without DEPS"}},
	}

	SECONDARY_DEPS_FILE_MAP = map[string]string{
		VALID_COMMIT: `
		# the commit queue can handle CLs rolling Skia
		# and whatever else without interference from each other.
		'skia_revision': '02cb37309c01506e2552e931efa9c04a569ed266',
		# Three lines of non-changing comments so that
		# the commit queue can handle CLs rolling Skia
		`,
		NO_DEPS_COMMIT: `
		# and whatever else without interference from each other.
		'skia_revision': '86a1022463a21fc779321c1db029fc3fdb6da2d6',
		# Three lines of non-changing comments so that
		`,
	}
)

// Tests parsing and processing of a single file.
func TestExtractTraceDBEntries(t *testing.T) {
	unittest.SmallTest(t)
	f, err := os.Open(TEST_INGESTION_FILE)
	assert.NoError(t, err)

	dmResults, err := parseDMResultsFromReader(f, TEST_INGESTION_FILE)
	assert.NoError(t, err)

	entries, err := extractTraceDBEntries(dmResults)
	assert.NoError(t, err)
	assert.Equal(t, len(TEST_ENTRIES), len(entries))

	for _, testEntry := range TEST_ENTRIES {
		found, ok := entries[testEntry.key]
		assert.True(t, ok)
		assert.Equal(t, testEntry.value, types.Digest(found.Value))
	}
}

// Tests the processor in conjunction with the vcs.
func TestTraceDBProcessor(t *testing.T) {
	unittest.MediumTest(t)

	// Set up mock VCS and run a server with the given data directory.
	ctx := context.Background()
	vcs := mocks.DeprecatedMockVCS(TEST_COMMITS, nil, nil)
	server, serverAddr := trace_utils.StartTraceDBTestServer(t, TRACE_DB_FILENAME, "")
	defer server.Stop()
	defer testutils.Remove(t, TRACE_DB_FILENAME)

	ingesterConf := &sharedconfig.IngesterConfig{
		ExtraParams: map[string]string{
			tracedbServiceConfig: serverAddr,
		},
	}

	// Set up the processor.
	eventBus := eventbus.New()
	processor, err := newDeprecatedTraceDBProcessor(vcs, ingesterConf, nil, eventBus)
	assert.NoError(t, err)
	defer util.Close(processor.(*traceDBProcessor).traceDB)

	_ = testTraceDBProcessor(t, ctx, processor, TEST_INGESTION_FILE)

	// Fail when there is not secondary repo defined.
	err = testTraceDBProcessor(t, ctx, processor, TEST_SECONDARY_FILE)
	assert.Equal(t, err, ingestion.IgnoreResultsFileErr)

	// Inject a secondary repo and test its use.
	secVCS := mocks.DeprecatedMockVCS(SECONDARY_TEST_COMMITS, SECONDARY_DEPS_FILE_MAP, nil)
	extractor := depot_tools.NewRegExDEPSExtractor(depot_tools.DEPSSkiaVarRegEx)
	vcs.(mocks.MockVCSImpl).SetSecondaryRepo(secVCS, extractor)

	_ = testTraceDBProcessor(t, ctx, processor, TEST_SECONDARY_FILE)
	err = testTraceDBProcessor(t, ctx, processor, TEST_SECONDARY_FILE_INVALID)
	assert.Equal(t, err, ingestion.IgnoreResultsFileErr)
	err = testTraceDBProcessor(t, ctx, processor, TEST_SECONDARY_FILE_NO_DEPS)
	assert.Equal(t, err, ingestion.IgnoreResultsFileErr)
}

func testTraceDBProcessor(t *testing.T, ctx context.Context, processor ingestion.Processor, testFileName string) error {
	// Load the example file and process it.
	fsResult, err := ingestion.FileSystemResult(testFileName, "./")
	assert.NoError(t, err)
	err = processor.Process(ctx, fsResult)
	if err != nil {
		return err
	}
	assert.NoError(t, err)

	// Steal the traceDB used by the processor to verify the results.
	traceDB := processor.(*traceDBProcessor).traceDB

	startTime := time.Now().Add(-time.Hour * 24 * 10)
	commitIDs, err := traceDB.List(startTime, time.Now())
	assert.NoError(t, err)

	assert.Equal(t, 1, len(filterCommitIDs(commitIDs, "master")))
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
		assert.Equal(t, 1, len(goldTrace.Digests))
		assert.Equal(t, testEntry.value, goldTrace.Digests[0])
	}

	assert.Equal(t, "master", commitIDs[0].Source)
	assert.NoError(t, traceDB.Close())
	return nil
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
