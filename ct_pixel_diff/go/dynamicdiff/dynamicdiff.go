package dynamicdiff

import (
	"image"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
)

type DynamicDiffMetrics struct {
	// NumDiffPixels is the number of static pixels that are different.
	NumDiffPixels int `json:"numDiffPixels"`

	// PixelDiffPercent is the percentage of static pixels that are different.
	PixelDiffPercent float32 `json:"pixelDiffPercent"`

	// MaxRGBDiffs contains the maximum difference of each channel.
	MaxRGBDiffs []int `json:"maxRGBDiffs"`

	// NumStaticPixels is the total number of static pixels.
	NumStaticPixels int `json:"numStaticPixels"`

	// NumDynamicPixels is the total number of dynamic pixels. Note that
	// NumStaticPixels + NumDynamicPixels = number of total pixels.
	NumDynamicPixels int `json:"numDynamicPixels"`
}

// DynamicContentDiff is a function that calculates the DiffMetrics and diff
// image for the provided images, taking into account that pixels with dynamic
// content are marked cyan and removing such pixels from the calculations. The
// images are assumed to have the same dimensions.
func DynamicContentDiff(left, right *image.NRGBA) (interface{}, *image.NRGBA) {
	bounds := left.Bounds()
	resultImg := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))

	// Pix is a []uint8 of R, G, B, A, R, G, B, A, ... values.
	p1 := left.Pix
	p2 := right.Pix

	numStaticPixels := 0
	numDynamicPixels := 0
	numDiffPixels := 0
	maxRGBDiffs := make([]int, 3)

	// Each pixel consists of 4 values (R, G, B, A). Alpha is ignored for diff
	// purposes.
	for i := 0; i < len(p1); i += 4 {
		r, g, b := p1[i+0], p1[i+1], p1[i+2]
		R, G, B := p2[i+0], p2[i+1], p2[i+2]

		// Ignore pixels with dynamic content, mark the pixel in the diff image as
		// dynamic, and increment the count of dynamic pixels.
		if isDynamicContentPixel(r, g, b) || isDynamicContentPixel(R, G, B) {
			copy(resultImg.Pix[i:], []uint8{0, 255, 255, 255})
			numDynamicPixels++
			continue
		}

		// Increment the count of static pixels.
		numStaticPixels++

		// If the pixels do not have the same RGB values, update the diff metrics
		// and the diff image.
		if r != R || g != G || b != B {
			numDiffPixels++
			dr := util.AbsInt(int(r) - int(R))
			dg := util.AbsInt(int(g) - int(G))
			db := util.AbsInt(int(b) - int(B))
			maxRGBDiffs[0] = util.MaxInt(dr, maxRGBDiffs[0])
			maxRGBDiffs[1] = util.MaxInt(dg, maxRGBDiffs[1])
			maxRGBDiffs[2] = util.MaxInt(db, maxRGBDiffs[2])
			copy(resultImg.Pix[i:], diff.PixelDiffColor[deltaOffset(dr+dg+db)])
		}
	}

	return &DynamicDiffMetrics{
		NumDiffPixels:    numDiffPixels,
		PixelDiffPercent: diff.GetPixelDiffPercent(numDiffPixels, numStaticPixels),
		MaxRGBDiffs:      maxRGBDiffs,
		NumStaticPixels:  numStaticPixels,
		NumDynamicPixels: numDynamicPixels,
	}, resultImg
}

// If the pixel is cyan, it contains dynamic content. This reflects the current
// behavior of the CT screenshot benchmark when the dynamic content detection
// flag is enabled.
func isDynamicContentPixel(red, green, blue uint8) bool {
	return red == 0 && green == 255 && blue == 255
}

// If the pixels don't have the same value, the minimum value that can be passed
// to this function is 1 and the maximum is 255*3 = 765. We must convert the
// range [1, 765] to the range [1, 7] in order to select the correct offset
// into the diff.PixelDiffColor slice. To convert a number n from range [x, y]
// to [a, b], we use the following formula:
// 			  (b - a)(n - x)
// f(n) = -------------- + a
//						y - x
func deltaOffset(n int) int {
	ret := 6*(n-1)/764 + 1
	if ret < 1 || ret > 7 {
		sklog.Fatalf("Input out of range [1, 765]: %d", n)
	}
	return ret - 1
}
