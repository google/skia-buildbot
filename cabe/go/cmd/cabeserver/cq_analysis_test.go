package main

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	cpb "go.skia.org/infra/cabe/go/proto"
)

func TestComputeCQCabeAnalysisResults(t *testing.T) {
	t.Run("T=C should not be a regression", func(t *testing.T) {
		res := []*cpb.AnalysisResult{
			{
				ExperimentSpec: &cpb.ExperimentSpec{
					Analysis: &cpb.AnalysisSpec{
						Benchmark: []*cpb.Benchmark{
							{
								Name:     "jetstream2",
								Workload: []string{"Basic.First"},
							},
						},
					},
				},
				Statistic: &cpb.Statistic{
					ControlMedian:   161.290323,
					TreatmentMedian: 161.290323,
					PValue:          0.029272,
					Lower:           -1.0,
					Upper:           1.0,
				},
			},
		}
		criticalValues := []float64{0.05}

		results := computeCQCabeAnalysisResults(res, criticalValues)

		assert.Empty(t, results.Results, "Should not find any regressions when T=C")
	})

	t.Run("Significant regression should be detected", func(t *testing.T) {
		res := []*cpb.AnalysisResult{
			{
				ExperimentSpec: &cpb.ExperimentSpec{
					Analysis: &cpb.AnalysisSpec{
						Benchmark: []*cpb.Benchmark{
							{
								Name:     "jetstream2",
								Workload: []string{"Basic.First"},
							},
						},
					},
				},
				Statistic: &cpb.Statistic{
					ControlMedian:   100.0,
					TreatmentMedian: 90.0, // Lower is worse for JetStream
					PValue:          0.01,
					Lower:           -15.0,
					Upper:           -5.0,
				},
			},
		}
		criticalValues := []float64{0.05}

		results := computeCQCabeAnalysisResults(res, criticalValues)

		assert.Len(t, results.Results, 1)
		assert.NotNil(t, results.Results["Basic.First"])
	})

	t.Run("NaN PValue with significant CI should be a regression", func(t *testing.T) {
		res := []*cpb.AnalysisResult{
			{
				ExperimentSpec: &cpb.ExperimentSpec{
					Analysis: &cpb.AnalysisSpec{
						Benchmark: []*cpb.Benchmark{
							{
								Name:     "jetstream2",
								Workload: []string{"Basic.First"},
							},
						},
					},
				},
				Statistic: &cpb.Statistic{
					ControlMedian:   100.0,
					TreatmentMedian: 90.0,
					PValue:          math.NaN(),
					Lower:           -15.0,
					Upper:           -5.0,
				},
			},
		}
		criticalValues := []float64{0.05}

		results := computeCQCabeAnalysisResults(res, criticalValues)

		assert.Len(t, results.Results, 1)
	})
}

func TestGenerateCriticalValues(t *testing.T) {
	results := []*cpb.AnalysisResult{
		{Statistic: &cpb.Statistic{PValue: 0.01}},
		{Statistic: &cpb.Statistic{PValue: 0.04}},
		{Statistic: &cpb.Statistic{PValue: 0.03}},
		{Statistic: &cpb.Statistic{PValue: math.NaN()}},
	}

	t.Run("Standard alpha without FDR", func(t *testing.T) {
		alpha := 0.05
		cv := generateCriticalValues(results, false, alpha)
		assert.Len(t, cv, 3) // NaN is excluded from CV count
		for _, v := range cv {
			assert.Equal(t, 0.05, v)
		}
	})

	t.Run("Custom alpha without FDR", func(t *testing.T) {
		alpha := 0.01
		cv := generateCriticalValues(results, false, alpha)
		assert.Len(t, cv, 3)
		for _, v := range cv {
			assert.Equal(t, 0.01, v)
		}
	})

	t.Run("FDR control with standard alpha", func(t *testing.T) {
		alpha := 0.05
		cv := generateCriticalValues(results, true, alpha)
		assert.Len(t, cv, 3)
		// Expected critical values for 3 results:
		// Rank 1: (1 * 0.05) / 3 = 0.01666...
		// Rank 2: (2 * 0.05) / 3 = 0.03333...
		// Rank 3: (3 * 0.05) / 3 = 0.05
		assert.InDelta(t, 0.01666, cv[0], 0.0001)
		assert.InDelta(t, 0.03333, cv[1], 0.0001)
		assert.InDelta(t, 0.05, cv[2], 0.0001)
	})

	t.Run("FDR control with custom alpha", func(t *testing.T) {
		alpha := 0.10
		cv := generateCriticalValues(results, true, alpha)
		assert.Len(t, cv, 3)
		// Expected: 0.1/3, 0.2/3, 0.3/3
		assert.InDelta(t, 0.03333, cv[0], 0.0001)
		assert.InDelta(t, 0.06666, cv[1], 0.0001)
		assert.InDelta(t, 0.1, cv[2], 0.0001)
	})
}
