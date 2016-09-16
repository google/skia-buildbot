package diff

import (
	"image"
	"math"
)

const (
	METRIC_COMBINED = "combined"
	METRIC_PERCENT  = "percent"
)

// MetricsFn is the signature a custom diff metric has to implmente.
type MetricFn func(*DiffMetrics, *image.NRGBA, *image.NRGBA) float32

// metrics contains the custom diff metrics.
var metrics = map[string]MetricFn{
	METRIC_COMBINED: combinedDiffMetric,
	METRIC_PERCENT:  percentDiffMetric,
}

// diffMetricIds contains the ids of all diff metrics.
var diffMetricIds []string

func init() {
	// Extract the ids of the diffmetrics once.
	diffMetricIds = make([]string, 0, len(metrics))
	for k := range metrics {
		diffMetricIds = append(diffMetricIds, k)
	}
}

// GetDiffMetricIDs returns the ids of the available diff metrics.
func GetDiffMetricIDs() []string {
	return diffMetricIds
}

// CalcDiff calculates the basic difference and then then custom diff metrics.
func CalcDiff(leftImg *image.NRGBA, rightImg *image.NRGBA) (*DiffMetrics, *image.NRGBA) {
	ret, diffImg := Diff(leftImg, rightImg)

	// Calcluate the metrics.
	diffs := make(map[string]float32, len(diffMetricIds))
	for _, id := range diffMetricIds {
		diffs[id] = metrics[id](ret, leftImg, rightImg)
	}
	ret.Diffs = diffs
	return ret, diffImg
}

// combinedDiffMetric returns a value in [0, 1] that represents how large
// the diff is between two images. Implements the MetricFn signature.
func combinedDiffMetric(basic *DiffMetrics, one *image.NRGBA, two *image.NRGBA) float32 {
	//
	// pixelDiffPercent float32, maxRGBA []int) float32 {
	if len(basic.MaxRGBADiffs) == 0 {
		return 1.0
	}
	// Turn maxRGBA into a percent by taking the root mean square difference from
	// [0, 0, 0, 0].
	sum := 0.0
	for _, c := range basic.MaxRGBADiffs {
		sum += float64(c) * float64(c)
	}
	normalizedRGBA := math.Sqrt(sum/float64(len(basic.MaxRGBADiffs))) / 255.0
	// We take the sqrt of (pixelDiffPercent * normalizedRGBA) to straigten out
	// the curve, i.e. think about what a plot of x^2 would look like in the
	// range [0, 1].
	return float32(math.Sqrt(float64(basic.PixelDiffPercent) * normalizedRGBA))
}

// percentDiffMetric returns pixel percent as the metric. Implements the MetricFn signature.
func percentDiffMetric(basic *DiffMetrics, one *image.NRGBA, two *image.NRGBA) float32 {
	return basic.PixelDiffPercent
}
