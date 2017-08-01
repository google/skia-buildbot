package dynamicdiff

import (
	"image"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
)

// DynamicContentDiff is a function that calculates the DiffMetrics and diff
// image for the provided images, taking into account that pixels with dynamic
// content are marked cyan and removing such pixels from the calculations. The
// images are assumed to have the same dimensions.
func DynamicContentDiff(left, right image.Image) (*diff.DiffMetrics, *image.NRGBA) {
	bounds := left.Bounds()
	resultImg := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))

	// Pix is a []uint8 of R, G, B, A, R, G, B, A, ... values.
	p1 := diff.GetNRGBA(left).Pix
	p2 := diff.GetNRGBA(right).Pix

	totalPixelsConsidered := 0
	numDiffPixels := 0
	maxRGBDiffs := make([]int, 3)

	// Each pixel consists of 4 values (R, G, B, A).
	for i := 0; i < len(p1); i += 4 {
		r, g, b := p1[i+0], p1[i+1], p1[i+2]
		R, G, B := p2[i+0], p2[i+1], p2[i+2]

		// If either the pixel in the left or right image is cyan, do not consider
		// it, as it contains dynamic content.
		if (r == 0 && g == 255 && b == 255) || (R == 0 && G == 255 && B == 255) {
			continue
		}

		// Increment the count of considered pixels, even if the pixels have the
		// same RGB values.
		totalPixelsConsidered++

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

	return &diff.DiffMetrics{
		NumDiffPixels:    numDiffPixels,
		PixelDiffPercent: diff.GetPixelDiffPercent(numDiffPixels, totalPixelsConsidered),
		MaxRGBADiffs:     maxRGBDiffs,
	}, resultImg
}

// If the pixels don't have the same value, the minimum value that can be passed
// to this function is 1 and the maximum is 255*3 = 765. We must convert the
// range [1, 765] to the range [1, 7] in order to select the correct offset
// into the diff.PixelDiffColor slice. To convert a number n from range [x, y]
// to [a, b], we use the following formula:
//					 (b - a)(n - x)
//		f(n) = -------------- + a
//								y - x
func deltaOffset(n int) int {
	ret := 6*(n-1)/764 + 1
	if ret < 1 || ret > 7 {
		sklog.Fatalf("Input out of range [1, 765]: %d", n)
	}
	return ret - 1
}
