package refiner

import (
	"math"

	"go.skia.org/infra/perf/go/regression"
)

// PeakIndexes holds the indices of a peak in a regression group.
type PeakIndexes struct {
	LeftIndex  int // Start index of the peak (or flat region)
	RightIndex int // End index of the peak (or flat region)
	MaxIndex   int // Center index (or peak index)
}

// findPeaks identifies the "peak" structure of a regression group.
// The input `group` represents a contiguous sequence of regression responses where each regression exceeds a threshold.
// It returns a PeakIndexes struct containing:
//   - LeftIndex: The index of the first (leftmost) local peak found.
//   - RightIndex: The index of the last (rightmost) local peak found.
//   - MaxIndex: The index of the global maximum magnitude.
//
// The area between LeftIndex and RightIndex forms the core minimum regression area.
// Points outside of these two peaks will be evaluated later in the SuperAnomalyRefiner, which may
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

	getMag := func(i int) float64 {
		if i < 0 || i >= n {
			return -1.0 // Implicitly smaller than any valid magnitude (>=0)
		}
		// Validated in validateInput to have Summary/Clusters/StepFit
		return math.Abs(float64(group[i].Summary.Clusters[0].StepFit.Regression))
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
