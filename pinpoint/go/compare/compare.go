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
	"go.skia.org/infra/go/skerr"
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
// statistically same or unknown from each other based on the perceived
// normalizedMagnitude difference between valuesA and valuesB using the functional
// low and high thresholds.
//
// The normalizedMagnitude is the failure rate,
// a float between 0 and 1. The attemptCount is the average number of
// samples between valuesA and valuesB.
func CompareFunctional(valuesA []float64, valuesB []float64, attemptCount int,
	normalizedMagnitude float64) (*CompareResults, error) {
	LowThreshold := thresholds.LowThreshold
	HighThreshold, err := thresholds.HighThresholdFunctional(normalizedMagnitude, attemptCount)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not get functional high threshold")
	}
	return compare(valuesA, valuesB, LowThreshold, HighThreshold)
}

// ComparePerformance determines if valuesA and valuesB are statistically different,
// statistically same or unknown from each other based on the perceived
// normalizedMagnitude difference between valuesA and valuesB using the performance
// low and high thresholds.
//
// The normalizedMagnitude is the perceived difference
// normalized by the interquartile range (IQR). We need more values to find smaller
// differences. The attemptCount is the average number of samples between valuesA
// and valuesB.
func ComparePerformance(valuesA []float64, valuesB []float64, attemptCount int,
	normalizedMagnitude float64) (*CompareResults, error) {
	LowThreshold := thresholds.LowThreshold
	HighThreshold, err := thresholds.HighThresholdPerformance(normalizedMagnitude, attemptCount)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not get high threshold for bisection")
	}

	return compare(valuesA, valuesB, LowThreshold, HighThreshold)
}

// compare decides whether two samples are the same, different, or unknown
// using the KS and MWU tests and compare their p-values against the
// LowThreshold and HighThreshold.
func compare(valuesA []float64, valuesB []float64, LowThreshold float64, HighThreshold float64) (*CompareResults, error) {
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
