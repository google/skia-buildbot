package perfresults

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func loadTestdata(t *testing.T, filename string) *PerfResults {
	var r io.Reader
	r, err := os.Open(filename)
	assert.NoError(t, err)

	pr, err := NewResults(r)
	assert.NoError(t, err)
	return pr
}

func Test_LoadValidJSON_ReturnsPerfResult(t *testing.T) {
	traceKey := TraceKey{
		ChartName: "memory:chrome:gpu_process:process_count",
		Unit:      "count_smallerIsBetter",
	}
	histogram := Histogram{
		SampleValues: []float64{1},
	}

	pr := loadTestdata(t, "testdata/empty.json")
	assert.Empty(t, pr.Histograms)

	pr = loadTestdata(t, "testdata/valid_histograms.json")
	assert.Contains(t, pr.Histograms, traceKey)
	assert.EqualValues(t, pr.Histograms[traceKey], histogram)

	assert.NotPanics(t, func() {
		_ = loadTestdata(t, "testdata/valid_metadata.json")
	})
}

func Test_LoadValidFullJSON_ReturnsFullTraceKey(t *testing.T) {
	traceKey := TraceKey{
		ChartName: "memory:chrome:all_processes:reported_by_chrome:v8:heap:code_space:effective_size",
		Unit:      "sizeInBytes_smallerIsBetter",
		Story:     "tests_cube-sea?frameBufferScale_1.4_heavyGpu_1_cubeScale_0.4_workTime_4_halfOnly_1_autorotate_1",
		OSName:    "win",
	}
	histogram := Histogram{
		SampleValues: []float64{524288},
	}
	pr := loadTestdata(t, "testdata/full.json")
	assert.Len(t, pr.Histograms, 11)
	assert.Contains(t, pr.Histograms, traceKey)
	assert.EqualValues(t, pr.Histograms[traceKey], histogram)
	assert.EqualValues(t, pr.GetSampleValues(traceKey.ChartName), histogram.SampleValues)

}

func Test_PerfResult_MergeHistogram(t *testing.T) {
	merged := loadTestdata(t, "testdata/merged.json")
	assert.Len(t, merged.Histograms, 1, "two histograms with same trace key should be merged")
	assert.EqualValues(t, []float64{1, 2}, merged.GetSampleValues("memory:chrome:gpu_process:process_count"))
}

func Test_PerfResult_MergeDiffHistogram(t *testing.T) {
	merged := loadTestdata(t, "testdata/merged_diff.json")
	assert.Len(t, merged.Histograms, 2, "two histograms with diff trace key should not be merged")

	// GetSampleValues get all the samples from different stories
	assert.EqualValues(t, []float64{1, 2}, merged.GetSampleValues("memory:chrome:gpu_process:process_count"))
}
