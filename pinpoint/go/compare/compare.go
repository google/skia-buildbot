// Package compare compares two arrays of data and determines if the two arrays
// are statistically different, the same, or unknown
//
// The method is roughly:
// - Calculate the p-value using the KS test and the MWU test
// - Take the minimum of the two p-values
// - If the p-value < low_threshold (0.01), return different
// - Else if the p-value > high_threshold, return same
// - Else return unknown
//
// See [thresholds] for more context on the thresholds
//
// # Functional bisections vs performance bisections:
//
// Most bisections are performance, meaning they measure performance regressions
// of benchmarks and stories on a device. Functional bisections measure how often
// a test fails during experimentation and if a CL can be attributed to an increase
// in the failure rate. i.e., how functional is the measurement at that build of Chrome.
//
// Functional bisection is not explictly implemented, due to its low demand. However,
// the analysis used in functional bisection is built into performance bisection.
// Sometimes a test will fail and rather than throw out the data, performance
// bisection backdoors into a functional bisection by using the functional
// bisection's thresholds.
//
// Algorithmically, there is no difference between the two, only the mathematical
// parameters used to define differences and their data inputs. Functional bisection
// deals with data in 0s and 1s (fail or success) and performance bisection deals
// with rational nonnegative numbers.

package compare

import (
	"math"
	"sort"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/pinpoint/go/compare/thresholds"
)

// define verdict enums
type verdict int

// These verdicts are the possible results of the statistical analysis.
const (
	// Unknown means that there is not enough evidence to reject
	// either hypothesis. Collect more data before making a final decision.
	Unknown verdict = iota
	// Same means that the sample likely come from the same distribution.
	// Cannot reject the null hypothesis.
	Same
	// Different means that the samples are unlikely to come
	// from the same distribution. Reject the null hypothesis.
	Different
)

type VerdictEnum interface {
	Verdict() verdict
}

func (v verdict) Verdict() verdict {
	return v
}

// Based on https://source.chromium.org/chromium/chromium/src/+/main:third_party/catapult/dashboard/dashboard/pinpoint/models/job_state.py;drc=94f2bff5159bf660910b35c39426102c5982c4a4;l=356
// the default functional analysis error rate expected is 1.0 for all bisections
// pivoting to functional analysis.
const DefaultFunctionalErrRate = 1.0

// CompareResults contains the results of a comparison between two samples.
// TODO(b/299537769): update verdict to use protos
type CompareResults struct {
	// Verdict is the outcome of the statistical analysis which is either
	// Unknown, Same, or Different.
	Verdict VerdictEnum
	// PValue is the consolidated p-value for the statistical tests used.
	PValue float64
	// PValueKS is the p-value estimate from the KS test
	PValueKS float64
	// PValueMWU is the p-value estimate from the MWU test
	PValueMWU float64
	// LowThreshold is `alpha` where if the p-value is lower means we can
	// 										reject the null hypothesis.
	LowThreshold float64
	// 	HighThreshold is the `alpha` where if the p-value is lower means we need
	// 											more information to make a definitive judgement.
	HighThreshold float64
}

// CompareFunctional determines if valuesA and valuesB are statistically different,
// statistically same or unknown from each other using the functional low and high thresholds.
// Functional analysis compares failure rates between A and B.
// The expectedErrRate expresses how much the culprit CL is responsible for flakiness
// in a benchmark measurement. i.e. expectedErrRate = 0.5 means the culprit is
// causing the benchmark to fail 50% of the time more often.
func CompareFunctional(valuesA, valuesB []float64, expectedErrRate float64) (*CompareResults, error) {
	all_values := append(valuesA, valuesB...)
	// avgSampleSize refers to the average number of samples between A and B.
	// The samples may be imbalanced depending on the success of individual runs
	avgSampleSize := len(all_values) / 2

	if expectedErrRate < 0.0 || expectedErrRate > 1.0 {
		sklog.Warning("Magnitude used in functional analysis was outside of the range of 0 and 1. Switching to default magnitude of 1.0")
		expectedErrRate = DefaultFunctionalErrRate
	}

	LowThreshold := thresholds.LowThreshold
	HighThreshold, err := thresholds.HighThresholdFunctional(expectedErrRate, avgSampleSize)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not get functional high threshold")
	}
	return compare(valuesA, valuesB, LowThreshold, HighThreshold)
}

// ComparePerformance determines if valuesA and valuesB are statistically different,
// statistically same or unknown from each other based on the perceived
// rawMagnitude difference between valuesA and valuesB using the performance
// low and high thresholds.
func ComparePerformance(valuesA, valuesB []float64, rawMagnitude float64) (*CompareResults, error) {
	all_values := append(valuesA, valuesB...)
	sort.Float64s(all_values)
	iqr := all_values[len(all_values)*3/4] - all_values[len(all_values)/4]
	normalizedMagnitude := math.Abs(rawMagnitude / iqr)
	// avgSampleSize refers to the average number of samples between A and B.
	// The samples may be imbalanced depending on the success of individual runs
	avgSampleSize := len(all_values) / 2

	LowThreshold := thresholds.LowThreshold
	HighThreshold, err := thresholds.HighThresholdPerformance(normalizedMagnitude, avgSampleSize)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not get high threshold for bisection")
	}

	return compare(valuesA, valuesB, LowThreshold, HighThreshold)
}

// compare decides whether two samples are the same, different, or unknown
// using the KS and MWU tests and compare their p-values against the
// LowThreshold and HighThreshold.
func compare(valuesA, valuesB []float64, LowThreshold, HighThreshold float64) (*CompareResults, error) {
	if len(valuesA) == 0 || len(valuesB) == 0 {
		// A sample has no values in it. Return verdict to measure more data.
		return &CompareResults{Verdict: Unknown}, nil
	}

	// MWU is bad at detecting changes in variance, and K-S is bad with discrete
	// distributions. So use both. We want low p-values for the below examples.
	//        a                     b               MWU(a, b)  KS(a, b)
	// [0]*20            [0]*15+[1]*5                0.0097     0.4973
	// range(10, 30)     range(10)+range(30, 40)     0.4946     0.0082
	PValueKS, err := KolmogorovSmirnov(valuesA, valuesB)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed KS test")
	}
	PValueMWU := MannWhitneyU(valuesA, valuesB)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed MWU test")
	}
	PValue := min(PValueKS, PValueMWU)

	result := &CompareResults{
		PValue:        PValue,
		PValueKS:      PValueKS,
		PValueMWU:     PValueMWU,
		LowThreshold:  LowThreshold,
		HighThreshold: HighThreshold,
	}

	if PValue <= LowThreshold {
		// The p-value is less than the significance level. Reject the null
		// hypothesis.
		result.Verdict = Different
	} else if PValue <= HighThreshold {
		// The p-value is not less than the significance level, but it's small
		// enough to be suspicious. We'd like to investigate more closely.
		result.Verdict = Unknown
	} else {
		// The p-value is quite large. We're not suspicious that the two samples might
		// come from different distributions, and we don't care to investigate more.
		result.Verdict = Same
	}

	return result, nil
}
