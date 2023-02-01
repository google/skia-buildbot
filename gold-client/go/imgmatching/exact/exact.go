package exact

import (
	"image"
	"image/draw"
)

// Matcher is an image matching algorithm.
//
// It implements exact matching. That is, two images match if they are are the same size, and if
// the pixel found at each (x, y) coordinate is identical on both images.
type Matcher struct {
	// Debug information about the last pair of matched images.
	lastDifferentPixelFound *image.Point
}

// Match implements the imgmatching.Matcher interface.
func (m *Matcher) Match(expected, actual image.Image) bool {
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

	// Iterate over and compare all pixels.
	m.lastDifferentPixelFound = nil
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			p1 := expectedNRGBA.NRGBAAt(x, y)
			p2 := actualNRGBA.NRGBAAt(x, y)
			if p1 != p2 {
				m.lastDifferentPixelFound = &image.Point{X: x, Y: y}
				return false
			}
		}
	}

	return true
}

// LastDifferentPixelFound returns the last different pixel found, or nil if the images match.
func (m *Matcher) LastDifferentPixelFound() *image.Point { return m.lastDifferentPixelFound }
