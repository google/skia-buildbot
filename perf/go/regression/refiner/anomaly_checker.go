package refiner

import (
	"math"

	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
)

const (
	defaultCohenDThreshold = 2.0
)

// isAnomaly determines if the given value is an anomaly compared to the baseline.
//
// Note: It is safe to use this only when you want to improve the accuracy of an anomaly range (refinement).
// Otherwise, it is typically not recommended to use this logic, as a single data point is usually not enough
// to make a decision about whether an anomaly happened or not.
func isAnomaly(val float32, baseline []float32, algo string, threshold float32, stdDevThreshold float32) bool {

	// 1. Prepare data
	baseline_mean := vec32.Mean(baseline)
	treatment_mean := val

	baseline_len := len(baseline)

	// Since we are analyzing a single treatment data point, we lack sufficient
	// data to calculate its true distribution statistics. Because some anomaly
	// detection algorithms (like Cohen's d) require the sample size and standard deviation,
	// we simulate them by making the assumption that the median/mean may change,
	// but the other statistical properties of the dataset (size and stddev) remain
	// identical to the baseline data.
	treatment_len := baseline_len

	baseline_stddev := vec32.StdDev(baseline, baseline_mean)
	treatment_stddev := baseline_stddev

	var regression float32

	switch algo {
	case string(types.AbsoluteStep):
		_, regression = stepfit.CalcAbsoluteStep(baseline_mean, treatment_mean)
	case string(types.Const):
		_, regression = stepfit.CalcConstStep(val, threshold)
	case string(types.PercentStep):
		_, regression = stepfit.CalcPercentStep(baseline_mean, treatment_mean)
	case string(types.CohenStep):
		_, regression = stepfit.CalcCohenStep(baseline_mean, treatment_mean, baseline_stddev, treatment_stddev, baseline_len, treatment_len, stdDevThreshold)
	default:
		// If we still end up with an unsupported algorithm here, we use Cohen as a safe default.
		_, regression = stepfit.CalcCohenStep(baseline_mean, treatment_mean, baseline_stddev, treatment_stddev, baseline_len, treatment_len, stdDevThreshold)
		threshold = defaultCohenDThreshold
	}

	// Check against threshold
	return math.Abs(float64(regression)) >= float64(threshold)
}

// evaluateRuleForRefinement evaluates a complex or simple rule for a single point vs baseline.
func evaluateRuleForRefinement(val float32, baseline []float32, rule *alerts.AnomalyDetectionRule, stdDevThreshold float32) bool {
	return stepfit.TraverseRule(rule,
		// 1. How to evaluate a simple rule (leaf node)
		func(check *alerts.AlgorithmCheck) bool {
			return isAnomaly(val, baseline, string(check.Step), check.Threshold, stdDevThreshold)
		},
		// 2. How to combine results (AND/OR)
		func(results []bool, op string) bool {
			return stepfit.CombineBooleans(results, op)
		})
}
