package diff

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"math"
	"unsafe"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

var (
	pixelMatchColor = color.Transparent

	// Orange gradient.
	//
	// These are non-premultiplied RGBA values.
	pixelDiffColor = [][]uint8{
		{0xfd, 0xd0, 0xa2, 0xff},
		{0xfd, 0xae, 0x6b, 0xff},
		{0xfd, 0x8d, 0x3c, 0xff},
		{0xf1, 0x69, 0x13, 0xff},
		{0xd9, 0x48, 0x01, 0xff},
		{0xa6, 0x36, 0x03, 0xff},
		{0x7f, 0x27, 0x04, 0xff},
	}

	// Blue gradient.
	//
	// These are non-premultiplied RGBA values.
	pixelAlphaDiffColor = [][]uint8{
		{0xc6, 0xdb, 0xef, 0xff},
		{0x9e, 0xca, 0xe1, 0xff},
		{0x6b, 0xae, 0xd6, 0xff},
		{0x42, 0x92, 0xc6, 0xff},
		{0x21, 0x71, 0xb5, 0xff},
		{0x08, 0x51, 0x9c, 0xff},
		{0x08, 0x30, 0x6b, 0xff},
	}
)

// Returns the offset into the color slices (pixelDiffColor,
// or pixelAlphaDiffColor) based on the delta passed in.
//
// The number passed in is the difference between two colors,
// on a scale from 1 to 1024.
func deltaOffset(n int) int {
	ret := int(math.Ceil(math.Log(float64(n))/math.Log(3) + 0.5))
	if ret < 1 || ret > 7 {
		sklog.Fatalf("Input: %d", n)
	}
	return ret - 1
}

// DiffMetrics contains the diff information between two images.
type DiffMetrics struct {
	// NumDiffPixels is the absolute number of pixels that are different.
	NumDiffPixels int

	// CombinedMetric is a value in [0, 10] that represents how large the diff is between two
	// images. It is based off the MaxRGBADiffs and PixelDiffPercent.
	CombinedMetric float32

	// PixelDiffPercent is the percentage of pixels that are different. The denominator here is
	// (maximum width of the two images) * (maximum height of the two images).
	PixelDiffPercent float32

	// MaxRGBADiffs contains the maximum difference of each channel.
	MaxRGBADiffs [4]int

	// DimDiffer is true if the dimensions between the two images are different.
	DimDiffer bool
}

// ComputeDiffMetrics computes and returns the diff metrics between two given images.
func ComputeDiffMetrics(leftImg *image.NRGBA, rightImg *image.NRGBA) *DiffMetrics {
	defer metrics2.FuncTimer().Stop()
	ret, _ := PixelDiff(leftImg, rightImg)
	ret.CombinedMetric = CombinedDiffMetric(ret.MaxRGBADiffs, ret.PixelDiffPercent)
	return ret
}

// CombinedDiffMetric returns a value in [0, 10] that represents how large
// the diff is between two images. Implements the MetricFn signature.
func CombinedDiffMetric(channelDiffs [4]int, pixelDiffPercent float32) float32 {
	// Turn maxRGBA into a percent by taking the root mean square difference from
	// [0, 0, 0, 0].
	sum := 0.0
	for _, c := range channelDiffs {
		sum += float64(c) * float64(c)
	}
	normalizedRGBA := math.Sqrt(sum/float64(len(channelDiffs))) / 255.0
	// We take the sqrt of (pixelDiffPercent * normalizedRGBA) to straighten out
	// the curve, i.e. think about what a plot of x^2 would look like in the
	// range [0, 1].
	return float32(math.Sqrt(float64(pixelDiffPercent) * normalizedRGBA))
}

// getPixelDiffPercent returns the percentage of pixels that differ, as a float between 0 and 100
// (inclusive).
func getPixelDiffPercent(numDiffPixels, totalPixels int) float32 {
	return (float32(numDiffPixels) * 100) / float32(totalPixels)
}

func uint8ToColor(c []uint8) color.Color {
	return color.NRGBA{R: c[0], G: c[1], B: c[2], A: c[3]}
}

// diffColors compares two color values and returns a color to indicate the
// difference. If the colors differ it updates maxRGBADiffs to contain the
// maximum difference over multiple calls.
// If the RGB channels are identical, but the alpha differ then
// pixelAlphaDiffColor is returned. This allows to distinguish pixels that
// render the same, but have different alpha values.
func diffColors(color1, color2 color.Color, maxRGBADiffs *[4]int) color.Color {
	// We compare them before normalizing to non-premultiplied. If one of the
	// original images did not have an alpha channel (but the other did) the
	// equality will be false.
	if color1 == color2 {
		return pixelMatchColor
	}

	// Treat all colors as non-premultiplied.
	c1 := color.NRGBAModel.Convert(color1).(color.NRGBA)
	c2 := color.NRGBAModel.Convert(color2).(color.NRGBA)

	rDiff := util.AbsInt(int(c1.R) - int(c2.R))
	gDiff := util.AbsInt(int(c1.G) - int(c2.G))
	bDiff := util.AbsInt(int(c1.B) - int(c2.B))
	aDiff := util.AbsInt(int(c1.A) - int(c2.A))
	maxRGBADiffs[0] = util.MaxInt(maxRGBADiffs[0], rDiff)
	maxRGBADiffs[1] = util.MaxInt(maxRGBADiffs[1], gDiff)
	maxRGBADiffs[2] = util.MaxInt(maxRGBADiffs[2], bDiff)
	maxRGBADiffs[3] = util.MaxInt(maxRGBADiffs[3], aDiff)

	// If the color channels differ we mark with the diff color.
	if (c1.R != c2.R) || (c1.G != c2.G) || (c1.B != c2.B) {
		// We use the Manhattan metric for color difference.
		return uint8ToColor(pixelDiffColor[deltaOffset(rDiff+gDiff+bDiff+aDiff)])
	}

	// If only the alpha channel differs we mark it with the alpha diff color.
	//
	if aDiff > 0 {
		return uint8ToColor(pixelAlphaDiffColor[deltaOffset(aDiff)])
	}

	return pixelMatchColor
}

// recode creates a new NRGBA image from the given image.
func recode(img image.Image) *image.NRGBA {
	ret := image.NewNRGBA(img.Bounds())
	draw.Draw(ret, img.Bounds(), img, image.Pt(0, 0), draw.Src)
	return ret
}

// GetNRGBA converts the image to an *image.NRGBA in an efficient manner.
func GetNRGBA(img image.Image) *image.NRGBA {
	switch t := img.(type) {
	case *image.NRGBA:
		return t
	case *image.RGBA:
		for i := 0; i < len(t.Pix); i += 4 {
			if t.Pix[i+3] != 0xff {
				sklog.Warning("Unexpected premultiplied image!")
				return recode(img)
			}
		}
		// If every alpha is 0xff then t.Pix is already in NRGBA format, simply
		// share Pix between the RGBA and NRGBA structs.
		return &image.NRGBA{
			Pix:    t.Pix,
			Stride: t.Stride,
			Rect:   t.Rect,
		}
	default:
		// Some of our tests produce more than 8 bit per channel color.
		return recode(img)
	}
}

// PixelDiff is a utility function that calculates the DiffMetrics and the image of the
// difference for the provided images.
func PixelDiff(img1, img2 image.Image) (*DiffMetrics, *image.NRGBA) {
	defer metrics2.FuncTimer().Stop()
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
	maxRGBADiffs := [4]int{0, 0, 0, 0}

	// Pix is a []uint8 rotating through R, G, B, A, R, G, B, A, ...
	p1 := GetNRGBA(img1).Pix
	p2 := GetNRGBA(img2).Pix
	// Compare the bounds, if they are the same then use this fast path.
	// We cast to uint64 to compare 2 pixels at a time, so we also require
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
						copy(resultImg.Pix[off+i:], pixelDiffColor[deltaOffset(dr+dg+db+da)])
					} else {
						copy(resultImg.Pix[off+i:], pixelAlphaDiffColor[deltaOffset(da)])
					}
				}
			}
		}
	} else {
		// Set pixels outside of the comparison area with the maximum diff color.
		maxDiffColor := uint8ToColor(pixelDiffColor[deltaOffset(1024)])
		for x := 0; x < resultWidth; x++ {
			for y := 0; y < resultHeight; y++ {
				if x < cmpWidth && y < cmpHeight {
					color1 := img1.At(x, y)
					color2 := img2.At(x, y)

					dc := diffColors(color1, color2, &maxRGBADiffs)
					if dc == pixelMatchColor {
						numDiffPixels--
					}
					resultImg.Set(x, y, dc)
				} else {
					resultImg.Set(x, y, maxDiffColor)
				}
			}
		}
	}

	return &DiffMetrics{
		NumDiffPixels:    numDiffPixels,
		PixelDiffPercent: getPixelDiffPercent(numDiffPixels, totalPixels),
		MaxRGBADiffs:     maxRGBADiffs,
		DimDiffer:        (cmpWidth != resultWidth) || (cmpHeight != resultHeight)}, resultImg
}

type Calculator interface {
	// CalculateDiffs recomputes all diffs for the current grouping, including any digests provided.
	CalculateDiffs(ctx context.Context, grouping paramtools.Params, additional []types.Digest) error
}
