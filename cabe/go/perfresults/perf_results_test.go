package perfresults

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func loadTestdata(t *testing.T, filename string) PerfResults {
	data, err := os.ReadFile(filename)
	assert.NoError(t, err)

	var pr PerfResults
	err = json.Unmarshal(data, &pr)
	assert.NoError(t, err)
	return pr
}

func Test_LoadValidJSON_ReturnsPerfResult(t *testing.T) {
	histogram := Histogram{
		Name:        "memory:chrome:gpu_process:process_count",
		Unit:        "count_smallerIsBetter",
		Description: "total number of GPU processes in Chrome",
		Diagnostics: map[string]string{
			"benchmarkDescriptions": "b6f9e674-f14d-4dc7-9491-bd9b9186f6b6",
			"benchmarkStart":        "7dcf34a5-76fd-4e45-8c14-3bc7b158dff9",
			"benchmarks":            "8517d434-8402-43e1-89ba-e6de3dea6af7",
			"stories":               "2944381f-f611-4b9a-bd07-779d2533771c",
			"storysetRepeats":       "2dc7ec5f-fdd9-4290-b648-cdda9b072281",
			"traceStart":            "4a1b94fa-951e-4db2-bfcf-7ba272eea572",
			"traceUrls":             "3b0cb6ac-6637-4dee-b959-c7f0dc4a3634",
			"botId":                 "f8101e8a-eff2-4498-b160-18169365fd35",
			"owners":                "13202b76-3b10-40b3-b8eb-91d8ec03e86e",
			"architectures":         "c7397d4d-1651-453c-8621-98c6cc5d2f38",
			"osNames":               "5b76decd-7ee9-4743-b24d-d7cc8eeb9c5f",
			"osVersions":            "4e6632db-47c3-4764-96d7-8b05f64eba78",
			"osDetailedVersions":    "90498cfc-b202-4d2c-8f3f-28e2f3343eb9",
		},
		SampleValues: []float64{1},
	}
	generic_set := GenericSet{
		GUID:   "b4865270-c915-4d2b-a164-793c1514b652",
		Values: []any{"Measures WebXR performance with synthetic sample pages."},
	}

	pr := loadTestdata(t, "testdata/empty.json")
	assert.Empty(t, pr.Histograms)

	pr = loadTestdata(t, "testdata/full.json")
	assert.NotContains(t, pr.GenericSets, GenericSet{})
	assert.Contains(t, pr.GenericSets, generic_set)
	assert.Len(t, pr.Histograms, 11)
	assert.Contains(t, pr.Histograms, histogram.Name)
	assert.Equal(t, pr.Histograms[histogram.Name], histogram)

	pr = loadTestdata(t, "testdata/valid_histograms.json")
	assert.Contains(t, pr.Histograms, histogram.Name)
	assert.Equal(t, pr.Histograms[histogram.Name], histogram)

	pr = loadTestdata(t, "testdata/valid_metadata.json")
	assert.Contains(t, pr.GenericSets, generic_set)
}

func Test_PerfResult_MergeHistogram(t *testing.T) {
	merged := loadTestdata(t, "testdata/merged.json")
	assert.Len(t, merged.Histograms, 1, "two histograms should be merged")
	assert.EqualValues(t, []float64{1, 2}, merged.GetSampleValues("memory:chrome:gpu_process:process_count"))
}
