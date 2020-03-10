package imgmatching

import "image"

// Matcher represents a (potentially non-exact) image matching algorithm.
type Matcher interface {
	// Match returns true if the algorithm considers the two images to be equivalent.
	Match(expected, actual image.Image) bool
}
