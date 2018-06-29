package ptraceingest

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/ingestcommon"
	"go.skia.org/infra/perf/go/ptracestore"
)

const (
	// directory were the test data are stored.
	TEST_DATA_DIR = "./testdata"

	// name of the input file containing test data.
	TEST_INGESTION_FILE = "nano.json"
)

var (
	// Fix the current point as reference. We remove the nano seconds from
	// now (below) because commits are only precise down to seconds.
	now = time.Now()

	// TEST_COMMITS are the commits we are considering. It needs to contain at
	// least all the commits referenced in the test file.
	TEST_COMMITS = []*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    "fe4a4029a080bc955e9588d05a6cd9eb490845d4",
				Subject: "Really big code change",
			},
			Timestamp: now.Add(-time.Second * 10).Round(time.Second),
			Branches:  map[string]bool{"master": true},
		},
	}
)

// Tests parsing and processing of a single file.
func TestBenchData(t *testing.T) {
	testutils.SmallTest(t)
	// Load the sample data file as BenchData.
	r, err := os.Open(filepath.Join(TEST_DATA_DIR, TEST_INGESTION_FILE))
	assert.NoError(t, err)

	benchData, err := ingestcommon.ParseBenchDataFromReader(r)
	assert.NoError(t, err)

	traceSet := getValueMap(benchData)
	expected := map[string]float32{
		",arch=x86,config=565,gpu=GTX660,model=ShuttleA,os=Ubuntu12,source_type=bench,sub_result=min_ms,system=UNIX,test=DeferredSurfaceCopy_discardable_640_480,":             2.215988,
		",arch=x86,config=gpu,gpu=GTX660,model=ShuttleA,os=Ubuntu12,source_type=bench,sub_result=min_ms,system=UNIX,test=DeferredSurfaceCopy_discardable_640_480,":             0.115713276,
		",arch=x86,config=565,gpu=GTX660,model=ShuttleA,os=Ubuntu12,source_type=bench,sub_result=min_ms,system=UNIX,test=DeferredSurfaceCopy_nonDiscardable_640_480,":          2.865907,
		",arch=x86,config=gpu,gpu=GTX660,model=ShuttleA,os=Ubuntu12,source_type=bench,sub_result=min_ms,system=UNIX,test=DeferredSurfaceCopy_nonDiscardable_640_480,":          0.36989987,
		",arch=x86,config=nonrendering,gpu=GTX660,model=ShuttleA,os=Ubuntu12,source_type=bench,sub_result=min_ms,system=UNIX,test=ChunkAlloc_Push_640_480,":                    0.009535795,
		",arch=x86,config=nonrendering,gpu=GTX660,model=ShuttleA,os=Ubuntu12,source_type=bench,sub_result=min_ms,system=UNIX,test=Deque_PushAllPopAll_640_480,":                0.019646378,
		",arch=x86,config=nonrendering,gpu=GTX660,model=ShuttleA,os=Ubuntu12,source_type=bench,sub_result=min_ms,system=UNIX,test=ChunkAlloc_PushPop_640_480,":                 3.539794921875,
		",arch=x86,config=8888,gpu=GTX660,model=ShuttleA,os=Ubuntu12,source_type=bench,sub_result=min_ms,system=UNIX,test=DeferredSurfaceCopy_discardable_640_480,":            2.223606,
		",arch=x86,config=8888,gpu=GTX660,model=ShuttleA,os=Ubuntu12,source_type=bench,sub_result=min_ms,system=UNIX,test=DeferredSurfaceCopy_nonDiscardable_640_480,":         2.855735,
		",arch=x86,config=8888,gpu=GTX660,model=ShuttleA,os=Ubuntu12,source_type=bench,sub_result=bytes,system=UNIX,test=DeferredSurfaceCopy_nonDiscardable_640_480,":          298888,
		",arch=x86,config=8888,gpu=GTX660,model=ShuttleA,os=Ubuntu12,source_type=bench,sub_result=ops,system=UNIX,test=DeferredSurfaceCopy_nonDiscardable_640_480,":            3333,
		",arch=x86,config=memory,gpu=GTX660,model=ShuttleA,os=Ubuntu12,path=src_pipe,sub_result=bytes,symbol=global_weak_symbol,system=UNIX,test=src_pipe_global_weak_symbol,": 158,
		",arch=x86,config=meta,gpu=GTX660,model=ShuttleA,os=Ubuntu12,sub_result=max_rss_mb,system=UNIX,test=memory_usage_0_0,":                                                 858,
	}

	assert.Equal(t, expected, traceSet)
}

// Tests the processor in conjunction with the vcs.
func TestPerfProcessor(t *testing.T) {
	testutils.MediumTest(t)
	ctx := context.Background()
	orig := ptracestore.Default
	dir, err := ioutil.TempDir("", "ptrace")
	assert.NoError(t, err)
	ptracestore.Default, err = ptracestore.New(dir)
	assert.NoError(t, err)
	defer func() {
		ptracestore.Default = orig
		testutils.RemoveAll(t, dir)
	}()

	vcs := ingestion.MockVCS(TEST_COMMITS, nil, nil)
	ingesterConf := &sharedconfig.IngesterConfig{}

	// Set up the processor.
	processor, err := newPerfProcessor(vcs, ingesterConf, nil, nil)
	assert.NoError(t, err)

	// Load the example file and process it.
	fsResult, err := ingestion.FileSystemResult(filepath.Join(TEST_DATA_DIR, TEST_INGESTION_FILE), TEST_DATA_DIR)
	assert.NoError(t, err)
	err = processor.Process(ctx, fsResult)
	assert.NoError(t, err)

	traceId := ",arch=x86,config=nonrendering,gpu=GTX660,model=ShuttleA,os=Ubuntu12,source_type=bench,sub_result=min_ms,system=UNIX,test=ChunkAlloc_Push_640_480,"
	expectedValue := float32(0.009535795)
	cid := &cid.CommitID{
		Source: "master",
		Offset: 0,
	}
	source, value, err := ptracestore.Default.Details(cid, traceId)
	assert.NoError(t, err)
	assert.Equal(t, expectedValue, value)
	assert.Equal(t, "nano.json", source)
}
