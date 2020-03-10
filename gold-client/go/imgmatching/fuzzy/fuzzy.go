package fuzzy

import (
	"image"

	"go.skia.org/infra/gold-client/go/imgmatching"
)

// FuzzyMatcher is a non-exact image matching algorithm.
//
// It considers two images to be equal if the following two conditions are met:
//   - Both images are of equal size.
//   - The total number of different pixels is below MaxDifferentPixels.
//   - There are no pixels such that dR + dG + dB + dA > PixelDeltaThreshold, where d{R,G,B,A} are
//     the per-channel deltas.
type FuzzyMatcher struct {
	MaxDifferentPixels  int
	PixelDeltaThreshold int
}

// Match implements the imagmatching.Matcher interface.
func (m *FuzzyMatcher) Match(expected, actual image.Image) bool {
	// TODO(lovisolo): Implement.
	return false
}

// Make sure FuzzyMatcher fulfills the imagmatching.Matcher interface.
var _ imgmatching.Matcher = (*FuzzyMatcher)(nil)
