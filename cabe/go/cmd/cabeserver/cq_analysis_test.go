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
	t.Run("JetStream2 with redundant sub-metrics uses staircase", func(t *testing.T) {
		results := []*cpb.AnalysisResult{
			{
				ExperimentSpec: &cpb.ExperimentSpec{Analysis: &cpb.AnalysisSpec{Benchmark: []*cpb.Benchmark{{Name: "jetstream2", Workload: []string{"Basic.First"}}}}},
				Statistic:      &cpb.Statistic{PValue: 0.01},
			},
			{
				ExperimentSpec: &cpb.ExperimentSpec{Analysis: &cpb.AnalysisSpec{Benchmark: []*cpb.Benchmark{{Name: "jetstream2", Workload: []string{"Basic.Average"}}}}},
				Statistic:      &cpb.Statistic{PValue: 0.02},
			},
			{
				ExperimentSpec: &cpb.ExperimentSpec{Analysis: &cpb.AnalysisSpec{Benchmark: []*cpb.Benchmark{{Name: "jetstream2", Workload: []string{"Basic.Worst"}}}}},
				Statistic:      &cpb.Statistic{PValue: 0.03},
			},
			{
				ExperimentSpec: &cpb.ExperimentSpec{Analysis: &cpb.AnalysisSpec{Benchmark: []*cpb.Benchmark{{Name: "jetstream2", Workload: []string{"Air.First"}}}}},
				Statistic:      &cpb.Statistic{PValue: 0.04},
			},
		}

		alpha := 0.05
		cv := generateCriticalValues(results, true, alpha)

		assert.Len(t, cv, 4)
		// N=4, m=2 (Basic, Air)
		// Thresholds: 1*0.05/2 = 0.025, 2*0.05/2 = 0.05
		// Ranks: Ceil(1*2/4)=1, Ceil(2*2/4)=1, Ceil(3*2/4)=2, Ceil(4*2/4)=2
		assert.Equal(t, 0.025, cv[0])
		assert.Equal(t, 0.025, cv[1])
		assert.Equal(t, 0.050, cv[2])
		assert.Equal(t, 0.050, cv[3])
	})

	t.Run("Non-JetStream benchmark uses standard BH", func(t *testing.T) {
		results := []*cpb.AnalysisResult{
			{
				ExperimentSpec: &cpb.ExperimentSpec{Analysis: &cpb.AnalysisSpec{Benchmark: []*cpb.Benchmark{{Name: "speedometer3", Workload: []string{"TodoMVC.React"}}}}},
				Statistic:      &cpb.Statistic{PValue: 0.01},
			},
			{
				ExperimentSpec: &cpb.ExperimentSpec{Analysis: &cpb.AnalysisSpec{Benchmark: []*cpb.Benchmark{{Name: "speedometer3", Workload: []string{"TodoMVC.Angular"}}}}},
				Statistic:      &cpb.Statistic{PValue: 0.02},
			},
		}

		alpha := 0.05
		cv := generateCriticalValues(results, true, alpha)

		assert.Len(t, cv, 2)
		// N=2, m=2 (Standard BH because not jetstream)
		// Thresholds: 1*0.05/2 = 0.025, 2*0.05/2 = 0.05
		assert.Equal(t, 0.025, cv[0])
		assert.Equal(t, 0.050, cv[1])
	})

	t.Run("Standard alpha without FDR", func(t *testing.T) {
		results := []*cpb.AnalysisResult{
			{
				ExperimentSpec: &cpb.ExperimentSpec{Analysis: &cpb.AnalysisSpec{Benchmark: []*cpb.Benchmark{{Name: "any", Workload: []string{"any"}}}}},
				Statistic:      &cpb.Statistic{PValue: 0.01},
			},
		}
		cv := generateCriticalValues(results, false, 0.05)
		assert.Equal(t, 0.05, cv[0])
	})

	t.Run("Default FDR alpha with FDR enabled", func(t *testing.T) {
		results := []*cpb.AnalysisResult{
			{
				ExperimentSpec: &cpb.ExperimentSpec{Analysis: &cpb.AnalysisSpec{Benchmark: []*cpb.Benchmark{{Name: "any", Workload: []string{"any"}}}}},
				Statistic:      &cpb.Statistic{PValue: 0.01},
			},
		}
		cv := generateCriticalValues(results, true, defaultFDRAlpha)
		assert.Equal(t, defaultFDRAlpha, cv[0])
	})
}

func TestPickUseFDRControl(t *testing.T) {
	assert.True(t, pickUseFDRControl("true"))
	assert.True(t, pickUseFDRControl("1"))
	assert.False(t, pickUseFDRControl("false"))
	assert.False(t, pickUseFDRControl(""))
	assert.False(t, pickUseFDRControl("not-a-bool"))
}

func TestPickAlpha(t *testing.T) {
	// alpha provided
	assert.Equal(t, 0.1, pickAlpha("0.1", true))
	assert.Equal(t, 0.1, pickAlpha("0.1", false))

	// alpha not provided, use_fdr_control is true
	assert.Equal(t, defaultFDRAlpha, pickAlpha("", true))
	assert.Equal(t, defaultFDRAlpha, pickAlpha("not-a-float", true))

	// alpha not provided, use_fdr_control is false
	assert.Equal(t, defaultAlpha, pickAlpha("", false))
	assert.Equal(t, defaultAlpha, pickAlpha("not-a-float", false))
}
