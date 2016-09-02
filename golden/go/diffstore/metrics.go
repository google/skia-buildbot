package diffstore

import (
	"image"
	"math"

	"go.skia.org/infra/golden/go/diff"
)

const (
	METRIC_COMBINED = "combined"
	METRIC_PERCENT  = "percent"
)

// MetricsFn is the signature a custom diff metric has to implmente.
type MetricFn func(*DiffRecord, *image.NRGBA, *image.NRGBA) float32

// metrics contains the custom diff metrics.
var metrics = map[string]MetricFn{
	METRIC_COMBINED: combinedDiffMetric,
	METRIC_PERCENT:  percentDiffMetric,
}

var diffMetricIds []string

func init() {
	diffMetricIds = make([]string, 0, len(metrics))
	for k := range metrics {
		diffMetricIds = append(diffMetricIds, k)
	}
}

// GetDiffMetricIDs returns the ids of the available diff metrics.
func GetDiffMetricIDs() []string {
	return diffMetricIds
}

// TODO(stephana): Consolidate with diff.DiffMetrics.
type DiffRecord struct {
	NumDiffPixels    int
	PixelDiffPercent float32
	MaxRGBADiffs     []int
	DimDiffer        bool

	// Diffs contains the diff metrics defined in 'metrics'.
	Diffs map[string]float32
}

// CalcDiff calculates the basic difference and then then custom diff metrics.
func CalcDiff(leftImg *image.NRGBA, rightImg *image.NRGBA) (*DiffRecord, *image.NRGBA) {
	basicDiff, diffImg := diff.Diff(leftImg, rightImg)
	ret := &DiffRecord{
		NumDiffPixels:    basicDiff.NumDiffPixels,
		PixelDiffPercent: basicDiff.PixelDiffPercent,
		MaxRGBADiffs:     basicDiff.MaxRGBADiffs,
		DimDiffer:        basicDiff.DimDiffer,
	}

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
func combinedDiffMetric(basic *DiffRecord, one *image.NRGBA, two *image.NRGBA) float32 {
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
func percentDiffMetric(basic *DiffRecord, one *image.NRGBA, two *image.NRGBA) float32 {
	return basic.PixelDiffPercent
}
