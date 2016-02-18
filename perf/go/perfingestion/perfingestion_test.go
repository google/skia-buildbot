package perfingestion

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/testutils"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/types"
)

const (
	// directory were the test data are stored.
	TEST_DATA_DIR = "./testdata"

	// name of the input file containing test data.
	TEST_INGESTION_FILE = "nano.json"

	// temporary file used to store traceDB content.
	TRACE_DB_FILENAME = "./test_trace.db"
)

var (
	// trace ids and values that are contained in the test file.
	TEST_ENTRIES = []struct {
		key       string
		value     float64
		subResult string
	}{
		{key: "x86:GTX660:ShuttleA:Ubuntu12:ChunkAlloc_PushPop_640_480:nonrendering", value: 0.01485466666666667, subResult: "min_ms"},
		{key: "x86:GTX660:ShuttleA:Ubuntu12:ChunkAlloc_Push_640_480:nonrendering", value: 0.009535795081967214, subResult: "min_ms"},
		{key: "x86:GTX660:ShuttleA:Ubuntu12:DeferredSurfaceCopy_discardable_640_480:565", value: 2.215988, subResult: "min_ms"},
		{key: "x86:GTX660:ShuttleA:Ubuntu12:DeferredSurfaceCopy_discardable_640_480:8888", value: 2.223606, subResult: "min_ms"},
		{key: "x86:GTX660:ShuttleA:Ubuntu12:DeferredSurfaceCopy_discardable_640_480:gpu", value: 0.1157132745098039, subResult: "min_ms"},
		{key: "x86:GTX660:ShuttleA:Ubuntu12:Deque_PushAllPopAll_640_480:nonrendering", value: 0.01964637755102041, subResult: "min_ms"},
		{key: "x86:GTX660:ShuttleA:Ubuntu12:memory_usage_0_0:meta:max_rss_mb", value: 858, subResult: "max_rss_mb"},
		{key: "x86:GTX660:ShuttleA:Ubuntu12:src_pipe_global_weak_symbol:memory:bytes", value: 158, subResult: "bytes"},
		{key: "x86:GTX660:ShuttleA:Ubuntu12:DeferredSurfaceCopy_nonDiscardable_640_480:565", value: 2.865907, subResult: "min_ms"},
		{key: "x86:GTX660:ShuttleA:Ubuntu12:DeferredSurfaceCopy_nonDiscardable_640_480:8888:bytes", value: 298888, subResult: "bytes"},
		{key: "x86:GTX660:ShuttleA:Ubuntu12:DeferredSurfaceCopy_nonDiscardable_640_480:8888", value: 2.855735, subResult: "min_ms"},
		{key: "x86:GTX660:ShuttleA:Ubuntu12:DeferredSurfaceCopy_nonDiscardable_640_480:8888:ops", value: 3333, subResult: "ops"},
		{key: "x86:GTX660:ShuttleA:Ubuntu12:DeferredSurfaceCopy_nonDiscardable_640_480:gpu", value: 0.3698998571428572, subResult: "min_ms"},
	}

	// Fix the current point as reference. We remove the nano seconds from
	// now (below) because commits are only precise down to seconds.
	now = time.Now()

	// TEST_COMMITS are the commits we are considering. It needs to contain at
	// least all the commits referenced in the test file.
	TEST_COMMITS = []*vcsinfo.LongCommit{
		&vcsinfo.LongCommit{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    "fe4a4029a080bc955e9588d05a6cd9eb490845d4",
				Subject: "Really big code change",
			},
			Timestamp: now.Add(-time.Second * 10).Add(-time.Nanosecond * time.Duration(now.Nanosecond())),
			Branches:  map[string]bool{"master": true},
		},
	}
)

// Tests parsing and processing of a single file.
func TestBenchData(t *testing.T) {
	// Load the sample data file as BenchData.
	r, err := os.Open(filepath.Join(TEST_DATA_DIR, TEST_INGESTION_FILE))
	assert.Nil(t, err)

	benchData, err := parseBenchDataFromReader(r)
	assert.Nil(t, err)

	assert.Equal(t, "x86:GTX660:ShuttleA:Ubuntu12", benchData.keyPrefix())

	entries := benchData.getTraceDBEntries()
	assert.Equal(t, len(TEST_ENTRIES), len(entries))

	for _, testEntry := range TEST_ENTRIES {
		found, ok := entries[testEntry.key]
		assert.True(t, ok)
		assert.Equal(t, testEntry.value, math.Float64frombits(binary.LittleEndian.Uint64(found.Value)))
		assert.Equal(t, testEntry.subResult, found.Params["sub_result"])
	}
}

// Tests the processor in conjunction with the vcs.
func TestPerfProcessor(t *testing.T) {

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
	processor, err := newPerfProcessor(vcs, ingesterConf, nil)
	assert.Nil(t, err)

	// Load the example file and process it.
	fsResult, err := ingestion.FileSystemResult(filepath.Join(TEST_DATA_DIR, TEST_INGESTION_FILE), TEST_DATA_DIR)
	assert.Nil(t, err)
	err = processor.Process(fsResult)
	assert.Nil(t, err)

	// Steal the traceDB used by the processor to verify the results.
	traceDB := processor.(*perfProcessor).traceDB

	startTime := time.Now().Add(-time.Hour * 24 * 10)
	commitIDs, err := traceDB.List(startTime, time.Now())
	assert.Nil(t, err)

	assert.Equal(t, 1, len(commitIDs))
	assert.Equal(t, &tracedb.CommitID{
		Timestamp: TEST_COMMITS[0].Timestamp.Unix(),
		ID:        TEST_COMMITS[0].Hash,
		Source:    "master",
	}, commitIDs[0])

	// Get a tile and make sure we have the right number of traces.
	tile, _, err := traceDB.TileFromCommits(commitIDs)
	assert.Nil(t, err)

	traces := tile.Traces
	assert.Equal(t, len(TEST_ENTRIES), len(traces))

	for _, testEntry := range TEST_ENTRIES {
		found, ok := traces[testEntry.key]
		assert.True(t, ok)
		perfTrace, ok := found.(*types.PerfTrace)
		assert.True(t, ok)
		assert.Equal(t, 1, len(perfTrace.Values))
		assert.Equal(t, testEntry.value, perfTrace.Values[0])
	}

	assert.Nil(t, traceDB.Close())
}
