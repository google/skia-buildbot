package imgmatching

import (
	"image"

	"go.skia.org/infra/gold-client/go/imgmatching/exact"
	"go.skia.org/infra/gold-client/go/imgmatching/fuzzy"
	"go.skia.org/infra/gold-client/go/imgmatching/positive_if_only_image"
	"go.skia.org/infra/gold-client/go/imgmatching/sample_area"
	"go.skia.org/infra/gold-client/go/imgmatching/sobel"
)

// Matcher represents a generic image matching algorithm.
type Matcher interface {
	// Match returns true if the algorithm considers the two images to be equivalent.
	Match(expected, actual image.Image) bool
}

// Make sure the matchers implement the imgmatching.Matcher interface.
// Note: this is done here instead of in their respective packages to prevent import cycles.
var _ Matcher = (*exact.Matcher)(nil)
var _ Matcher = (*fuzzy.Matcher)(nil)
var _ Matcher = (*positive_if_only_image.Matcher)(nil)
var _ Matcher = (*sample_area.Matcher)(nil)
var _ Matcher = (*sobel.Matcher)(nil)
