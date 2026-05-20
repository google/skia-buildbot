package refiner

import (
	"math"

	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/types"
)

// PeakIndexes holds the indices of a peak in a regression group.
type PeakIndexes struct {
	LeftIndex  int // Start index of the peak (or flat region)
	RightIndex int // End index of the peak (or flat region)
	MaxIndex   int // Center index (or peak index)
}

// selectAlgorithmForPeakDetection selects the common algorithm to use for comparing signal magnitudes
// across points in a regression area.
//
// Purpose:
// When an anomaly detection rule involves multiple algorithms (e.g., Cohen's D AND Percent Change),
// points in a regression area might have different dominant algorithms. To find a single representative
// "peak" for the area, we need a common metric (common denominator) to compare all points fairly.
//
// Heuristic:
//  1. Count how many times each algorithm triggered across all points in the group.
//  2. If any algorithm triggered for ALL points, we select the one with the highest priority.
//  3. If no algorithm triggered for all points, we fall back to the highest priority algorithm
//     defined in the rule (which is guaranteed to be present in all points since they evaluated the same rule).
func selectAlgorithmForPeakDetection(group []*regression.RegressionDetectionResponse) string {
	n := len(group)
	if n == 0 {
		return ""
	}

	// 1. Count how many times each algorithm triggered.
	algoCounts := make(map[string]int)
	for i := 0; i < n; i++ {
		for _, eval := range group[i].Summary.Clusters[0].StepFit.RuleEvaluations {
			if eval.IsAnomaly {
				algoCounts[eval.AlgoName]++
			}
		}
	}

	// 2. Find the highest priority algorithm that triggered for ALL points.
	selectedAlgo := ""
	maxPriority := -1
	for algoName, count := range algoCounts {
		if count == n {
			priority := types.StepDetection(algoName).Priority()
			if priority > 0 && priority > maxPriority {
				maxPriority = priority
				selectedAlgo = algoName
			}
		}
	}

	// 3. Fallback: just use the highest priority algorithm present in the rule.
	if selectedAlgo == "" {
		sf := group[0].Summary.Clusters[0].StepFit
		for _, eval := range sf.RuleEvaluations {
			priority := types.StepDetection(eval.AlgoName).Priority()
			if priority > 0 && priority > maxPriority {
				maxPriority = priority
				selectedAlgo = eval.AlgoName
			}
		}
	}

	return selectedAlgo
}

// findPeaks identifies the "peak" structure of a regression group.
// The input `group` represents a contiguous sequence of regression responses where each regression exceeds a threshold.
// It returns a PeakIndexes struct containing:
//   - LeftIndex: The index of the first (leftmost) local peak found.
//   - RightIndex: The index of the last (rightmost) local peak found.
//   - MaxIndex: The index of the global maximum magnitude.
//
// The area between LeftIndex and RightIndex forms the core minimum regression area.
// Points outside of these two peaks will be evaluated later in the AnomalyBoundsRefiner, which may
// extend the regression area into non-peak points based on further checks.
//
// The MaxIndex is not used by the regression refiner for boundary expansion, but it is stored as
// metadata for Confirmed Regressions because it represents the most significant regression value.
//
// Behavior details:
//  1. Local Peaks: A point is a local peak if its StepFit regression absolute magnitude
//     is strictly greater than its immediate neighbors (or a flat region of equal magnitudes > neighbors).
//  2. Global Max: The point with the highest StepFit regression magnitude in the group.
//     If there's a flat region of max values, the center index of that flat region is returned.
func findPeaks(group []*regression.RegressionDetectionResponse) PeakIndexes {
	n := len(group)
	if n == 0 {
		return PeakIndexes{}
	}

	selectedAlgo := selectAlgorithmForPeakDetection(group)

	getMag := func(i int) float64 {
		if i < 0 || i >= n {
			return -1.0
		}
		sf := group[i].Summary.Clusters[0].StepFit

		var val float32
		for _, eval := range sf.RuleEvaluations {
			if eval.AlgoName == selectedAlgo {
				val = eval.Value
				break
			}
		}

		mag := math.Abs(float64(val))
		if selectedAlgo == string(types.MannWhitneyU) {
			return 1.0 - mag
		}
		return mag
	}

	localPeaks := findLocalPeaks(n, getMag)
	maxPeak := findGlobalMax(n, getMag, localPeaks)
	return constructResult(localPeaks, maxPeak, n)
}

// peakInfo holds the start and end indices of a peak.
type peakInfo struct {
	start int
	end   int
}

// findLocalPeaks identifies all local peaks in the series.
// A peak is defined as a value (or flat region) strictly greater than its neighbors.
func findLocalPeaks(n int, getMag func(int) float64) []peakInfo {
	var localPeaks []peakInfo

	for i := 0; i < n; {
		val := getMag(i)

		// Find end of the flat region (if values are equal)
		j := i
		for j < n-1 && getMag(j+1) == val {
			j++
		}

		left := getMag(i - 1)
		right := getMag(j + 1)

		// Peak if strictly greater than neighbors (or flat region boundaries)
		if val > left && val > right {
			localPeaks = append(localPeaks, peakInfo{start: i, end: j})
		}

		i = j + 1
	}
	return localPeaks
}

// findGlobalMax identifies the start and end indices of the global maximum value(s).
func findGlobalMax(n int, getMag func(int) float64, localPeaks []peakInfo) peakInfo {
	maxVal := -1.0
	var maxPeak peakInfo
	for _, p := range localPeaks {
		// Check the value of the peak (assuming peak is flat, any index in range works)
		val := getMag(p.start)
		if val > maxVal {
			maxVal = val
			maxPeak = p
		}
	}
	return maxPeak
}

// constructResult creates the final PeakIndexes struct from the findings.
func constructResult(localPeaks []peakInfo, maxPeak peakInfo, n int) PeakIndexes {
	maxIndex := (maxPeak.start + maxPeak.end + 1) / 2
	return PeakIndexes{
		LeftIndex:  localPeaks[0].start,
		RightIndex: localPeaks[len(localPeaks)-1].end,
		MaxIndex:   maxIndex,
	}
}
