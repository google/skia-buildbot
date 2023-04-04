package fuzzy

import (
	"image"
	"image/draw"
	"math"
)

// Matcher is an image matching algorithm.
//
// It considers two images to be equal if the following conditions are met:
//
//   - Both images are of equal size.
//   - The total number of different pixels is below MaxDifferentPixels.
//   - If PixelDeltaThreshold > 0: There are no pixels such that
//     dR + dG + dB + dA > PixelDeltaThreshold, where d{R,G,B,A} are the per-channel deltas.
//   - Else: There are no pixels such that max(dR, dG, dB, dA) > PixelPerChannelDeltaThreshold,
//     where d{R,G,B,A} are the per-channel deltas.
//   - If IgnoredBorderThickness > 0, then the first/last IgnoredBorderThickness rows/columns will
//     be ignored when performing the above pixel-wise comparisons.
//
// Note that if MaxDifferentPixels = 0 this algorithm will perform an exact image comparison. If
// that is intentional, consider using exact matching instead (e.g. by not specifying the
// image_matching_algorithm optional key).
//
// This algorithm assumes 8-bit channels.
//
// Valid PixelDeltaThreshold values are 0 to 1020 inclusive (0 <= d{R,G,B,A} <= 255, thus
// 0 <= dR + dG + dB + dA <= 255*4 = 1020).
//
// Valid PixelPerChannelDelta values are 0 to 255 inclusive.
type Matcher struct {
	MaxDifferentPixels            int
	PixelDeltaThreshold           int
	PixelPerChannelDeltaThreshold int
	IgnoredBorderThickness        int

	// Debug information about the last pair of matched images.
	actualNumDifferentPixels int
	actualMaxPixelDelta      int
}

// Match implements the imagmatching.Matcher interface.
func (m *Matcher) Match(expected, actual image.Image) bool {
	// Expected image will be nil if no recent positive image is found.
	if expected == nil {
		return false
	}

	// Images must be the same size.
	if !expected.Bounds().Eq(actual.Bounds()) {
		return false
	}

	// Determine which delta threshold we will be using. We assume that at most one of
	// PixelDeltaThreshold and PixelPerChannelDeltaThreshold will be set.
	usePerChannelThreshold := false
	if m.PixelPerChannelDeltaThreshold > 0 {
		usePerChannelThreshold = true
	}

	// Convert both images to NRGBA.
	bounds := expected.Bounds()
	expectedNRGBA := image.NewNRGBA(bounds)
	actualNRGBA := image.NewNRGBA(bounds)
	draw.Draw(expectedNRGBA, bounds, expected, bounds.Min, draw.Src)
	draw.Draw(actualNRGBA, bounds, actual, bounds.Min, draw.Src)

	// Reset counters.
	m.actualNumDifferentPixels = 0
	m.actualMaxPixelDelta = 0

	// Iterate over all pixels, with the exception of the ignored border pixels.
	b := m.IgnoredBorderThickness
	for x := (bounds.Min.X + b); x < (bounds.Max.X - b); x++ {
		for y := (bounds.Min.Y + b); y < (bounds.Max.Y - b); y++ {
			p1 := expectedNRGBA.NRGBAAt(x, y)
			p2 := actualNRGBA.NRGBAAt(x, y)

			// Track number of different pixels.
			if p1 != p2 {
				m.actualNumDifferentPixels++
			}

			// Track maximum pixel-wise difference.
			var pixelDelta int
			if usePerChannelThreshold {
				pixelDelta = absDiff(p1.R, p2.R)
				pixelDelta = int(math.Max(float64(pixelDelta), float64(absDiff(p1.G, p2.G))))
				pixelDelta = int(math.Max(float64(pixelDelta), float64(absDiff(p1.B, p2.B))))
				pixelDelta = int(math.Max(float64(pixelDelta), float64(absDiff(p1.A, p2.A))))
			} else {
				pixelDelta = absDiff(p1.R, p2.R) + absDiff(p1.G, p2.G) + absDiff(p1.B, p2.B) + absDiff(p1.A, p2.A)
			}
			if pixelDelta > m.actualMaxPixelDelta {
				m.actualMaxPixelDelta = pixelDelta
			}
		}
	}

	// Total number of different pixels must be below the given threshold.
	if m.actualNumDifferentPixels > m.MaxDifferentPixels {
		return false
	}

	// Pixel-wise differences must be below the given threshold.
	if usePerChannelThreshold {
		if m.actualMaxPixelDelta > m.PixelPerChannelDeltaThreshold {
			return false
		}
	} else {
		if m.actualMaxPixelDelta > m.PixelDeltaThreshold {
			return false
		}
	}

	return true
}

// NumDifferentPixels returns the number of different pixels between the last two matched images.
func (m *Matcher) NumDifferentPixels() int { return m.actualNumDifferentPixels }

// MaxPixelDelta returns the maximum per-channel delta sum between the last two matched images.
func (m *Matcher) MaxPixelDelta() int { return m.actualMaxPixelDelta }

// PixelComparisonMethod returns whether pixel comparison is being done using
// the sum of per-channel differences or the max per-channel difference.
func (m *Matcher) PixelComparisonMethod() string {
	if m.PixelDeltaThreshold > 0 {
		return "pixel delta threshold"
	}
	return "pixel per-channel delta threshold"
}

// absDiff takes two uint8 values m and n, computes |m - n|, and converts the result into an int
// suitable for addition without the risk of overflowing.
func absDiff(m, n uint8) int {
	if m > n {
		return int(m - n)
	}
	return int(n - m)
}
