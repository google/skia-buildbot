package sobel

import (
	"image"

	"go.skia.org/infra/gold-client/go/imgmatching/fuzzy"
)

// SobelFuzzyMatcher is a non-exact image matching algorithm.
//
// It extends the FuzzyMatcher algorithm by performing edge detection using the Sobel operator[1]
// and ignoring any pixels that are part of an edge.
//
// The algorithm performs the following steps:
//   1. It applies the Sobel operator to the expected image, producing a 0 to 255 value per pixel
//      indicating how likely it is to be part of an edge.
//   2. It zeroes-out any (x,y) coordinates on *both* images where the aforementioned value exceeds
//      EdgeThreshold. Note that this assumes both images are of equal size.
//   3. It passes the two resulting images to the FuzzyMatcher algorithm (using parameters
//      MaxDifferentPixels and PixelDeltaThreshold) and returns its return value.
//
// [1] https://en.wikipedia.org/wiki/Sobel_operator
type SobelFuzzyMatcher struct {
	fuzzy.FuzzyMatcher
	EdgeThreshold int
}

// Match implements the imagmatching.Matcher interface.
func (m *SobelFuzzyMatcher) Match(expected, actual image.Image) bool {
	// TODO(lovisolo): Implement.
	return false
}
