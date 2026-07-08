package stepfit

import (
	"sort"

	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/types"
)

// sortAnomalyResults sorts the results by algorithm priority.
func sortAnomalyResults(results []AnomalyResult) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].IsAnomaly != results[j].IsAnomaly {
			return results[i].IsAnomaly
		}

		pi := types.StepDetection(results[i].AlgoName).Priority()
		pj := types.StepDetection(results[j].AlgoName).Priority()
		return pi > pj
	})
}

// EvaluateRule evaluates a complex or simple anomaly detection rule against a trace.
type ruleEvalResult struct {
	isTriggered bool
	results     []AnomalyResult
}

// EvaluateRule evaluates a complex or simple anomaly detection rule against a trace
// and calculates and returns a *StepFit.
//
// stddevThreshold is the minimum standard deviation allowed when normalizing
// traces to a standard deviation of 1 (used only for OriginalStep algorithm).
//
// rule is the complex or simple anomaly detection rule containing the algorithm
// checks and their thresholds.
//
// See StepFit for a description of the values being calculated.
func EvaluateRule(trace []float32, stddevThreshold float32, rule *alerts.AnomalyDetectionRule) *StepFit {
	if rule == nil {
		return nil
	}

	res := TraverseRule(rule,
		func(check *alerts.AlgorithmCheck) ruleEvalResult {
			isTriggered, results := evaluateSimpleRule(trace, stddevThreshold, check)
			return ruleEvalResult{isTriggered, results}
		},
		func(results []ruleEvalResult, op string) ruleEvalResult {
			var allResults []AnomalyResult
			var subResults []bool
			for _, r := range results {
				subResults = append(subResults, r.isTriggered)
				allResults = append(allResults, r.results...)
			}

			finalResult := CombineBooleans(subResults, op)
			return ruleEvalResult{finalResult, allResults}
		})

	results := res.results

	sortAnomalyResults(results)

	stepFit := NewStepFit()
	stepFit.RuleEvaluations = results

	if len(results) > 0 {
		mostSignificant := results[0]
		stepFit.TurningPoint = mostSignificant.TurningPoint
		stepFit.StepSize = mostSignificant.StepSize
		stepFit.LeastSquares = mostSignificant.LeastSquares

		if res.isTriggered {
			stepFit.Status = mostSignificant.Status
			stepFit.Regression = mostSignificant.RawRegression
		} else {
			stepFit.Status = UNINTERESTING
			stepFit.Regression = mostSignificant.RawRegression
		}
	}

	return stepFit
}

// evaluateSimpleRule evaluates a single algorithm check against the threshold.
func evaluateSimpleRule(trace []float32, stddevThreshold float32, simpleRule *alerts.AlgorithmCheck) (bool, []AnomalyResult) {
	if simpleRule == nil {
		return false, nil
	}

	if len(trace) < MinTraceSize {
		return false, nil
	}

	var workTrace []float32
	// Only normalize the trace if doing ORIGINAL_STEP.
	if simpleRule.Step == types.OriginalStep {
		workTrace = vec32.Dup(trace)
		vec32.Norm(workTrace, stddevThreshold)
	} else {
		// For all non-ORIGINAL_STEP regression types we use a symmetric (2*N)
		// trace. If the supplied trace has an odd length, drop the last element;
		// otherwise use trace as-is.
		if len(trace)%2 != 0 {
			workTrace = trace[0 : len(trace)-1]
		} else {
			workTrace = trace
		}
	}

	isValid, anomalyResult := CalculateStepFitValues(workTrace, stddevThreshold, simpleRule)
	if !isValid {
		return false, nil
	}

	return anomalyResult.IsAnomaly, []AnomalyResult{anomalyResult}
}

// TraverseRule traverses the rule tree recursively.
// T is the type of the result (e.g., bool for refinement, *StepFit for detection).
func TraverseRule[T any](rule *alerts.AnomalyDetectionRule,
	evalSimple func(check *alerts.AlgorithmCheck) T,
	combine func(results []T, op string) T) T {

	if rule.ComplexRule != nil {
		var results []T
		for _, r := range rule.ComplexRule.Rules {
			results = append(results, TraverseRule(r, evalSimple, combine))
		}
		return combine(results, rule.ComplexRule.Op)
	}

	if rule.SimpleRule != nil {
		return evalSimple(rule.SimpleRule)
	}

	var zero T
	return zero
}

// CombineBooleans combines a slice of booleans using "AND" or "OR" operator.
func CombineBooleans(results []bool, op string) bool {
	if op == "AND" {
		for _, res := range results {
			if !res {
				return false
			}
		}
		return true
	} else if op == "OR" {
		for _, res := range results {
			if res {
				return true
			}
		}
		return false
	}
	return false
}

// NewSimpleRule creates a simple AnomalyDetectionRule from a step and threshold.
func NewSimpleRule(step types.StepDetection, threshold float32) *alerts.AnomalyDetectionRule {
	return &alerts.AnomalyDetectionRule{
		SimpleRule: &alerts.AlgorithmCheck{
			Step:      step,
			Threshold: threshold,
		},
	}
}

// UsesOriginalStep returns true if step or rule uses OriginalStep algorithm.
func UsesOriginalStep(step types.StepDetection, rule *alerts.AnomalyDetectionRule) bool {
	if step == types.OriginalStep && rule == nil {
		return true
	}
	if rule == nil {
		return false
	}
	return TraverseRule(rule,
		func(check *alerts.AlgorithmCheck) bool {
			return check != nil && check.Step == types.OriginalStep
		},
		func(results []bool, _ string) bool {
			for _, r := range results {
				if r {
					return true
				}
			}
			return false
		})
}

// GetWindowSize returns the working window size for the given radius, step detection, and rule.
// For OriginalStep, it is 2*radius + 1, and for all other algorithms it is 2*radius.
func GetWindowSize(radius int, step types.StepDetection, rule *alerts.AnomalyDetectionRule) int {
	if UsesOriginalStep(step, rule) {
		return 2*radius + 1
	}
	return 2 * radius
}
