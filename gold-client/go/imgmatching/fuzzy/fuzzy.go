package fuzzy

import (
	"image"
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
	// TODO(lovisolo): Implement.
	return false
}
