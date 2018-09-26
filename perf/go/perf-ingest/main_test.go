package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/ingestcommon"
)

const (
	// TODO(jcgregorio) Move once ptraceingest is deleted.
	TEST_DATA_DIR = "../ptraceingest/testdata"

	TEST_INGESTION_FILE = "nano.json"
)

func TestParamsAndValues(t *testing.T) {
	testutils.SmallTest(t)
	// Load the sample data file as BenchData.
	r, err := os.Open(filepath.Join(TEST_DATA_DIR, TEST_INGESTION_FILE))
	assert.NoError(t, err)

	benchData, err := ingestcommon.ParseBenchDataFromReader(r)
	assert.NoError(t, err)

	params, values, paramSet := getParamSAndValues(benchData)
	assert.Len(t, values, 13)
	assert.Len(t, params, 13)
	expected := paramtools.ParamSet{
		"gpu":         []string{"GTX660"},
		"os":          []string{"Ubuntu12"},
		"arch":        []string{"x86"},
		"config":      []string{"nonrendering", "meta", "8888", "565", "gpu", "memory"},
		"sub_result":  []string{"min_ms", "max_rss_mb", "bytes", "ops"},
		"symbol":      []string{"global_weak_symbol"},
		"test":        []string{"ChunkAlloc_PushPop_640_480", "memory_usage_0_0", "DeferredSurfaceCopy_discardable_640_480", "DeferredSurfaceCopy_nonDiscardable_640_480", "ChunkAlloc_Push_640_480", "Deque_PushAllPopAll_640_480", "src_pipe_global_weak_symbol"},
		"model":       []string{"ShuttleA"},
		"source_type": []string{"bench"},
		"system":      []string{"UNIX"},
		"path":        []string{"src_pipe"},
	}
	expected.Normalize()
	assert.Equal(t, expected, paramSet)
}
