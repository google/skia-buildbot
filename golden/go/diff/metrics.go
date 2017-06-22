package diff

import (
	"image"
	"image/draw"
	"math"
	"unsafe"

	"go.skia.org/infra/go/util"
)

const (
	METRIC_COMBINED = "combined"
	METRIC_PERCENT  = "percent"
	METRIC_PIXEL    = "pixel"
)

// MetricsFn is the signature a custom diff metric has to implmente.
type MetricFn func(*DiffMetrics, *image.NRGBA, *image.NRGBA) float32

// metrics contains the custom diff metrics.
var metrics = map[string]MetricFn{
	METRIC_COMBINED: combinedDiffMetric,
	METRIC_PERCENT:  percentDiffMetric,
	METRIC_PIXEL:    pixelDiffMetric,
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
func CalcDiff(leftImg image.Image, rightImg image.Image) (*DiffMetrics, *image.NRGBA) {
	// Convert images to type image.NRGBA
	img1 := GetNRGBA(leftImg)
	img2 := GetNRGBA(rightImg)

	img1Bounds := img1.Bounds()
	img2Bounds := img2.Bounds()

	// Get the bounds we want to compare.
	cmpWidth := util.MinInt(img1Bounds.Dx(), img2Bounds.Dx())
	cmpHeight := util.MinInt(img1Bounds.Dy(), img2Bounds.Dy())

	// Get the bounds of the resulting image. If they dimensions match they
	// will be identical to the result bounds. Fill the image with black pixels.
	resultWidth := util.MaxInt(img1Bounds.Dx(), img2Bounds.Dx())
	resultHeight := util.MaxInt(img1Bounds.Dy(), img2Bounds.Dy())
	resultImg := image.NewNRGBA(image.Rect(0, 0, resultWidth, resultHeight))
	totalPixels := resultWidth * resultHeight

	// Loop through all points and compare. We start assuming all pixels are
	// wrong. This takes care of the case where the images have different sizes
	// and there is an area not inspected by the loop.
	numDiffPixels := totalPixels
	maxRGBADiffs := make([]int, 4)

	// Pix is a []uint8 rotating through R, G, B, A, R, G, B, A, ...
	p1 := GetNRGBA(img1).Pix
	p2 := GetNRGBA(img2).Pix
	// Compare the bounds, if they are the same then use this fast path.
	// We pun to uint64 to compare 2 pixels at a time, so we also require
	// an even number of pixels here.  If that's a big deal, we can easily
	// fix that up, handling the straggler pixel separately at the end.
	if img1Bounds.Eq(img2Bounds) && len(p1)%8 == 0 {
		numDiffPixels = 0
		// Note the += 8.  We're checking two pixels at a time here.
		for i := 0; i < len(p1); i += 8 {
			// Most pixels we compare will be the same, so from here to
			// the 'continue' is the hot path in all this code.
			rgba_2x := (*uint64)(unsafe.Pointer(&p1[i]))
			RGBA_2x := (*uint64)(unsafe.Pointer(&p2[i]))
			if *rgba_2x == *RGBA_2x {
				continue
			}

			// When off == 0, we check the first pixel of the pair; when 4, the second.
			for off := 0; off <= 4; off += 4 {
				r, g, b, a := p1[off+i+0], p1[off+i+1], p1[off+i+2], p1[off+i+3]
				R, G, B, A := p2[off+i+0], p2[off+i+1], p2[off+i+2], p2[off+i+3]
				if r != R || g != G || b != B || a != A {
					numDiffPixels++
					dr := util.AbsInt(int(r) - int(R))
					dg := util.AbsInt(int(g) - int(G))
					db := util.AbsInt(int(b) - int(B))
					da := util.AbsInt(int(a) - int(A))
					maxRGBADiffs[0] = util.MaxInt(dr, maxRGBADiffs[0])
					maxRGBADiffs[1] = util.MaxInt(dg, maxRGBADiffs[1])
					maxRGBADiffs[2] = util.MaxInt(db, maxRGBADiffs[2])
					maxRGBADiffs[3] = util.MaxInt(da, maxRGBADiffs[3])
					if dr+dg+db > 0 {
						copy(resultImg.Pix[off+i:], PixelDiffColor[deltaOffset(dr+dg+db+da)])
					} else {
						copy(resultImg.Pix[off+i:], PixelAlphaDiffColor[deltaOffset(da)])
					}
				}
			}
		}
	} else {
		// Fill the entire image with maximum diff color.
		maxDiffColor := uint8ToColor(PixelDiffColor[deltaOffset(1024)])
		draw.Draw(resultImg, resultImg.Bounds(), &image.Uniform{maxDiffColor}, image.ZP, draw.Src)

		for x := 0; x < cmpWidth; x++ {
			for y := 0; y < cmpHeight; y++ {
				color1 := img1.At(x, y)
				color2 := img2.At(x, y)

				dc := diffColors(color1, color2, maxRGBADiffs)
				if dc == PixelMatchColor {
					numDiffPixels--
				}
				resultImg.Set(x, y, dc)
			}
		}
	}

	ret, diffImg := &DiffMetrics{
		NumDiffPixels:    numDiffPixels,
		PixelDiffPercent: getPixelDiffPercent(numDiffPixels, totalPixels),
		MaxRGBADiffs:     maxRGBADiffs,
		DimDiffer:        (cmpWidth != resultWidth) || (cmpHeight != resultHeight)}, resultImg

	// Calcluate the metrics.
	diffs := make(map[string]float32, len(diffMetricIds))
	for _, id := range diffMetricIds {
		diffs[id] = metrics[id](ret, img1, img2)
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

// pixelDiffMetric returns the number of different pixels as the metric. Implements the MetricFn signature.
func pixelDiffMetric(basic *DiffMetrics, one *image.NRGBA, two *image.NRGBA) float32 {
	return float32(basic.NumDiffPixels)
}
