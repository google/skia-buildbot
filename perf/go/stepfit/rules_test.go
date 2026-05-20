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
