package fuzzy

import (
	"image"
	"image/draw"
)

// FuzzyMatcher is a non-exact image matching algorithm.
//
// It considers two images to be equal if the following two conditions are met:
//   - Both images are of equal size.
//   - The total number of different pixels is below MaxDifferentPixels.
//   - There are no pixels such that dR + dG + dB + dA > PixelDeltaThreshold, where d{R,G,B,A} are
//     the per-channel deltas.
//
// It assumes 8-bit channels.
//
// Valid PixelDeltaThreshold values are 0 to 1020 inclusive (0 <= d{R,G,B,A} <= 255, thus
// 0 <= dR + dG + dB + dA <= 255*4 = 1020).
type FuzzyMatcher struct {
	MaxDifferentPixels  int
	PixelDeltaThreshold int
}

// Match implements the imagmatching.Matcher interface.
func (m *FuzzyMatcher) Match(expected, actual image.Image) bool {
	// Images must be the same size.
	if !expected.Bounds().Eq(actual.Bounds()) {
		return false
	}

	// Convert both images to NRGBA.
	bounds := expected.Bounds()
	expectedNRGBA := image.NewNRGBA(bounds)
	actualNRGBA := image.NewNRGBA(bounds)
	draw.Draw(expectedNRGBA, bounds, expected, bounds.Min, draw.Src)
	draw.Draw(actualNRGBA, bounds, actual, bounds.Min, draw.Src)

	// We'll track the number of different pixels between the two images.
	numDiffPixels := 0

	// Iterate over all pixels.
	for x := bounds.Min.X; x <= bounds.Max.X; x++ {
		for y := bounds.Min.Y; y <= bounds.Max.Y; y++ {
			p1 := expectedNRGBA.NRGBAAt(x, y)
			p2 := actualNRGBA.NRGBAAt(x, y)

			// Total number of different pixels must be below the given threshold.
			if p1 != p2 {
				numDiffPixels++
				if numDiffPixels > m.MaxDifferentPixels {
					return false
				}
			}

			// Pixel-wise differences must be below the given threshold.
			delta := absDiff(p1.R, p2.R) + absDiff(p1.G, p2.G) + absDiff(p1.B, p2.B) + absDiff(p1.A, p2.A)
			if delta > m.PixelDeltaThreshold {
				return false
			}
		}
	}

	return true
}

// absDiff takes two uint8 values m and n, computes |m - n|, and converts the result into an int
// suitable for addition without the risk of overflowing.
func absDiff(m, n uint8) int {
	if m > n {
		return int(m - n)
	}
	return int(n - m)
}
