package ptraceingest

import (
	"os"
	"path/filepath"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/ingestcommon"
)

const (

	// name of the input file containing test data.
	TEST_DURATION_INGESTION_FILE = "task_duration.json"
)

// Tests parsing and processing of a single file.
func TestDurationData(t *testing.T) {
	testutils.SmallTest(t)
	// Load the sample data file as BenchData.
	r, err := os.Open(filepath.Join(TEST_DATA_DIR, TEST_DURATION_INGESTION_FILE))
	assert.NoError(t, err)

	benchData, err := ingestcommon.ParseBenchDataFromReader(r)
	assert.NoError(t, err)

	traceSet := getValueMap(benchData)
	expected := map[string]float32{
		",arch=arm64,compiler=Clang,config=task_duration,configuration=Debug,cpu_or_gpu=GPU,cpu_or_gpu_value=Adreno430,extra_config=Android,model=Nexus6p,os=Android,role=Perf,sub_result=task_ms,test=Perf-Android-Clang-Nexus6p-GPU-Adreno430-arm64-Debug-Android,": 5136.5645,
	}

	assert.Equal(t, expected, traceSet)
}
