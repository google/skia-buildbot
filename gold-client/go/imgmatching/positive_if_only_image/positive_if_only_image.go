package positive_if_only_image

import (
	"image"

	"go.skia.org/infra/gold-client/go/imgmatching/exact"
)

// Matcher is an image matching algorithm.
//
// It considers the two images to be equal if they are identical, or if the expected image is nil,
// meaning that there is no existing positive image in the grouping (hence the
// positive_if_only_image name).
type Matcher struct {
	exact.Matcher
}

// Match implements the imgmatching.Matcher interface.
func (m *Matcher) Match(expected, actual image.Image) bool {
	// If there is no existing positive image in the grouping, mark the given image as positive.
	if expected == nil {
		return true
	}

	// But if there is an existing positive image in the grouping, the given image should match it.
	return m.Matcher.Match(expected, actual)
}
