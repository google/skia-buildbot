package stepfit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/types"
)

func TestEvaluateSimpleRule_ReturnsAnomalyResult(t *testing.T) {
	trace := []float32{1.0, 1.0, 1.0, 5.0, 5.0, 5.0, 5.0}
	stddevThreshold := float32(0.01)

	rule := &alerts.AlgorithmCheck{
		Step:      types.AbsoluteStep,
		Threshold: 2.0,
	}

	isTriggered, results := evaluateSimpleRule(trace, stddevThreshold, rule)
	assert.True(t, isTriggered)
	assert.Len(t, results, 1)
	assert.Equal(t, string(types.AbsoluteStep), results[0].AlgoName)
	assert.Equal(t, float32(4.0), results[0].Value)
	assert.Equal(t, float32(2.0), results[0].Threshold)
	assert.True(t, results[0].IsAnomaly)
}

func TestEvaluateComplexRule_AND_ReturnsFlattenedResults(t *testing.T) {
	trace := []float32{1.0, 1.0, 1.0, 5.0, 5.0, 5.0, 5.0} // A clear step of size 4
	stddevThreshold := float32(0.01)

	// AND rule: Both should trigger
	subrules := []*alerts.AnomalyDetectionRule{
		{
			SimpleRule: &alerts.AlgorithmCheck{
				Step:      types.AbsoluteStep,
				Threshold: 3.0,
			},
		},
		{
			SimpleRule: &alerts.AlgorithmCheck{
				Step:      types.Const,
				Threshold: 1.0,
			},
		},
	}

	complexRule := &alerts.ComplexRule{
		Op:    "AND",
		Rules: subrules,
	}

	rule := &alerts.AnomalyDetectionRule{
		ComplexRule: complexRule,
	}

	stepFit := EvaluateRule(trace, stddevThreshold, rule)
	assert.NotNil(t, stepFit)
	assert.NotEqual(t, UNINTERESTING, stepFit.Status)
	results := stepFit.RuleEvaluations
	assert.Len(t, results, 2)

	var foundConst, foundAbs bool
	for _, res := range results {
		if res.AlgoName == string(types.Const) {
			foundConst = true
		} else if res.AlgoName == string(types.AbsoluteStep) {
			foundAbs = true
		}
	}
	assert.True(t, foundConst)
	assert.True(t, foundAbs)
}

func TestEvaluateComplexRule_OR_ReturnsFlattenedResults(t *testing.T) {
	trace := []float32{1.0, 1.0, 1.0, 5.0, 5.0, 5.0, 5.0} // A clear step of size 4
	stddevThreshold := float32(0.01)

	// OR rule: one triggers, one does not.
	subrules := []*alerts.AnomalyDetectionRule{
		{
			SimpleRule: &alerts.AlgorithmCheck{
				Step:      types.AbsoluteStep,
				Threshold: 10.0, // Should not trigger (value is 4.0)
			},
		},
		{
			SimpleRule: &alerts.AlgorithmCheck{
				Step:      types.OriginalStep,
				Threshold: 0.1, // Should trigger
			},
		},
	}

	complexRule := &alerts.ComplexRule{
		Op:    "OR",
		Rules: subrules,
	}

	rule := &alerts.AnomalyDetectionRule{
		ComplexRule: complexRule,
	}

	stepFit := EvaluateRule(trace, stddevThreshold, rule)
	assert.NotNil(t, stepFit)
	assert.NotEqual(t, UNINTERESTING, stepFit.Status)
	results := stepFit.RuleEvaluations
	assert.Len(t, results, 2)

	// OriginalStep Value is likely high, let's just assert both are present.
	var foundOriginal, foundAbs bool
	for _, res := range results {
		if res.AlgoName == string(types.OriginalStep) {
			foundOriginal = true
			assert.True(t, res.IsAnomaly)
		} else if res.AlgoName == string(types.AbsoluteStep) {
			foundAbs = true
			assert.False(t, res.IsAnomaly)
		}
	}
	assert.True(t, foundOriginal)
	assert.True(t, foundAbs)
}

func TestEvaluateComplexRule_CohenAndPercent(t *testing.T) {
	trace := []float32{10.0, 10.1, 9.9, 20.0, 20.1, 19.9, 20.0}
	stddevThreshold := float32(0.01)

	rule := &alerts.AnomalyDetectionRule{
		ComplexRule: &alerts.ComplexRule{
			Op: "AND",
			Rules: []*alerts.AnomalyDetectionRule{
				{
					SimpleRule: &alerts.AlgorithmCheck{
						Step:      types.PercentStep,
						Threshold: 0.5,
					},
				},
				{
					SimpleRule: &alerts.AlgorithmCheck{
						Step:      types.CohenStep,
						Threshold: 2.0,
					},
				},
			},
		},
	}

	stepFit := EvaluateRule(trace, stddevThreshold, rule)
	assert.NotNil(t, stepFit)
	assert.NotEqual(t, UNINTERESTING, stepFit.Status)
	results := stepFit.RuleEvaluations
	assert.Len(t, results, 2)

	var foundPercent, foundCohen bool
	for _, res := range results {
		if res.AlgoName == string(types.PercentStep) {
			foundPercent = true
			assert.True(t, res.IsAnomaly)
		} else if res.AlgoName == string(types.CohenStep) {
			foundCohen = true
			assert.True(t, res.IsAnomaly)
		}
	}
	assert.True(t, foundPercent)
	assert.True(t, foundCohen)
}

func TestEvaluateComplexRule_CohenAndPercent_AND_Fail(t *testing.T) {
	trace := []float32{10.0, 10.1, 9.9, 20.0, 20.1, 19.9, 20.0}
	stddevThreshold := float32(0.01)

	rule := &alerts.AnomalyDetectionRule{
		ComplexRule: &alerts.ComplexRule{
			Op: "AND",
			Rules: []*alerts.AnomalyDetectionRule{
				{
					SimpleRule: &alerts.AlgorithmCheck{
						Step:      types.PercentStep,
						Threshold: 1.5, // Should not trigger (value is ~1.0)
					},
				},
				{
					SimpleRule: &alerts.AlgorithmCheck{
						Step:      types.CohenStep,
						Threshold: 2.0, // Triggers
					},
				},
			},
		},
	}

	stepFit := EvaluateRule(trace, stddevThreshold, rule)
	assert.NotNil(t, stepFit)
	assert.Equal(t, UNINTERESTING, stepFit.Status) // Overall false
	results := stepFit.RuleEvaluations
	assert.Len(t, results, 2)

	var foundPercent, foundCohen bool
	for _, res := range results {
		if res.AlgoName == string(types.PercentStep) {
			foundPercent = true
			assert.False(t, res.IsAnomaly)
		} else if res.AlgoName == string(types.CohenStep) {
			foundCohen = true
			assert.True(t, res.IsAnomaly)
		}
	}
	assert.True(t, foundPercent)
	assert.True(t, foundCohen)
}

func TestEvaluateComplexRule_CohenAndPercent_OR_Success(t *testing.T) {
	trace := []float32{10.0, 10.1, 9.9, 20.0, 20.1, 19.9, 20.0}
	stddevThreshold := float32(0.01)

	rule := &alerts.AnomalyDetectionRule{
		ComplexRule: &alerts.ComplexRule{
			Op: "OR",
			Rules: []*alerts.AnomalyDetectionRule{
				{
					SimpleRule: &alerts.AlgorithmCheck{
						Step:      types.PercentStep,
						Threshold: 1.5, // Should not trigger
					},
				},
				{
					SimpleRule: &alerts.AlgorithmCheck{
						Step:      types.CohenStep,
						Threshold: 2.0, // Triggers
					},
				},
			},
		},
	}

	stepFit := EvaluateRule(trace, stddevThreshold, rule)
	assert.NotNil(t, stepFit)
	assert.NotEqual(t, UNINTERESTING, stepFit.Status) // Overall true
	results := stepFit.RuleEvaluations
	assert.Len(t, results, 2)

	var foundPercent, foundCohen bool
	for _, res := range results {
		if res.AlgoName == string(types.PercentStep) {
			foundPercent = true
			assert.False(t, res.IsAnomaly)
		} else if res.AlgoName == string(types.CohenStep) {
			foundCohen = true
			assert.True(t, res.IsAnomaly)
		}
	}
	assert.True(t, foundPercent)
	assert.True(t, foundCohen)
}

func TestGetWindowSize_And_UsesOriginalStep(t *testing.T) {
	assert.True(t, UsesOriginalStep(types.OriginalStep, nil))
	assert.False(t, UsesOriginalStep(types.PercentStep, nil))
	assert.False(t, UsesOriginalStep(types.CohenStep, nil))

	assert.Equal(t, 7, GetWindowSize(3, types.OriginalStep, nil))
	assert.Equal(t, 6, GetWindowSize(3, types.PercentStep, nil))
	assert.Equal(t, 6, GetWindowSize(3, types.CohenStep, nil))

	complexWithOriginal := &alerts.AnomalyDetectionRule{
		ComplexRule: &alerts.ComplexRule{
			Op: "OR",
			Rules: []*alerts.AnomalyDetectionRule{
				NewSimpleRule(types.CohenStep, 2.0),
				NewSimpleRule(types.OriginalStep, 0.5),
			},
		},
	}
	assert.True(t, UsesOriginalStep(types.CohenStep, complexWithOriginal))
	assert.Equal(t, 7, GetWindowSize(3, types.CohenStep, complexWithOriginal))
}

func TestEvaluateComplexRule_OriginalAndPercent_OR(t *testing.T) {
	// 7 elements: OriginalStep uses all 7; PercentStep drops the 7th and uses first 6.
	trace := []float32{10.0, 10.0, 10.0, 20.0, 20.0, 20.0, 20.0}
	stddevThreshold := float32(0.01)

	rule := &alerts.AnomalyDetectionRule{
		ComplexRule: &alerts.ComplexRule{
			Op: "OR",
			Rules: []*alerts.AnomalyDetectionRule{
				NewSimpleRule(types.OriginalStep, 0.5),
				NewSimpleRule(types.PercentStep, 0.5),
			},
		},
	}

	stepFit := EvaluateRule(trace, stddevThreshold, rule)
	assert.NotNil(t, stepFit)
	assert.NotEqual(t, UNINTERESTING, stepFit.Status)

	results := stepFit.RuleEvaluations
	assert.Len(t, results, 2)

	var foundOriginal, foundPercent bool
	for _, res := range results {
		if res.AlgoName == string(types.OriginalStep) {
			foundOriginal = true
			assert.True(t, res.IsAnomaly)
		} else if res.AlgoName == string(types.PercentStep) {
			foundPercent = true
			assert.True(t, res.IsAnomaly)
		}
	}
	assert.True(t, foundOriginal)
	assert.True(t, foundPercent)
}

func TestMinSizeForAlgorithm(t *testing.T) {
	assert.Equal(t, 3, MinSizeForAlgorithm(types.OriginalStep))
	assert.Equal(t, 4, MinSizeForAlgorithm(types.CohenStep))
	assert.Equal(t, 2, MinSizeForAlgorithm(types.PercentStep))
	assert.Equal(t, 2, MinSizeForAlgorithm(types.PercentMedianStep))
	assert.Equal(t, 2, MinSizeForAlgorithm(types.AbsoluteStep))
	assert.Equal(t, 2, MinSizeForAlgorithm(types.Const))
	assert.Equal(t, 2, MinSizeForAlgorithm(types.Stepiness))
	assert.Equal(t, 2, MinSizeForAlgorithm(types.MannWhitneyU))
}

func TestEvaluateSimpleRule_TooShort_ReturnsFalse(t *testing.T) {
	stddevThreshold := float32(0.01)

	for _, step := range types.AllStepDetections {
		name := string(step)
		if name == "" {
			name = "OriginalStep"
		}
		t.Run(name, func(t *testing.T) {
			minSize := MinSizeForAlgorithm(step)
			tooShortTrace := make([]float32, minSize-1)
			rule := &alerts.AlgorithmCheck{
				Step:      step,
				Threshold: 0.5,
			}
			isTriggered, results := evaluateSimpleRule(tooShortTrace, stddevThreshold, rule)
			assert.False(t, isTriggered)
			assert.Nil(t, results)
		})
	}
}

func TestEvaluateSimpleRule_LongEnough_Evaluates(t *testing.T) {
	stddevThreshold := float32(0.01)

	for _, step := range types.AllStepDetections {
		name := string(step)
		if name == "" {
			name = "OriginalStep"
		}
		t.Run(name, func(t *testing.T) {
			minSize := MinSizeForAlgorithm(step)
			trace := make([]float32, minSize)
			for i := range trace {
				trace[i] = float32(i)
			}
			rule := &alerts.AlgorithmCheck{
				Step:      step,
				Threshold: 0.5,
			}
			_, results := evaluateSimpleRule(trace, stddevThreshold, rule)
			assert.NotNil(t, results, "Should have evaluated for step %s with length %d", step, len(trace))
		})
	}
}
