package sample_area

import (
	"fmt"
	"image"
	"image/draw"
)

// Matcher is a non-exact image matching algorithm.
//
// Instead of comparing pixels one-by-one, areas of pixels are compared together
// and are considered identical if a certain number of pixels in that area are
// identical.
//
// The primary use case for this is large images that are susceptible to random
// noise throughout the entire image. Such actual color difference can be quite
// large and still not be noticeable to a human as long as such differences are
// not closely grouped together.
type Matcher struct {
	// SampleAreaWidth allows values 1+, although 1 is the same as exact matching.
	SampleAreaWidth int
	// MaxDifferentPixelsPerArea allows values 0-SampleAreaWidth^2, although the
	// extremes are equivalent to exact matching and a blanket pass, respectively.
	MaxDifferentPixelsPerArea int
	// SampleAreaChannelDeltaThreshold allows values 0-255, although 255 is
	// equivalent to a blanket pass.
	SampleAreaChannelDeltaThreshold int

	// Debug information about the last comparison.
	failedSampleArea                          image.Rectangle
	numDifferentPixels                        int
	sampleAreaWidthTooSmall                   bool
	sampleAreaWidthTooLarge                   bool
	maxDifferentPixelsPerAreaOutOfRange       bool
	sampleAreaChannelDeltaThresholdOutOfRange bool
}

// Match implements the imgmatching.Matcher interface.
func (m *Matcher) Match(expected, actual image.Image) bool {
	// Expected image will be nil if no recent positive image is found.
	if expected == nil {
		return false
	}

	// Images must be the same size.
	if !expected.Bounds().Eq(actual.Bounds()) {
		return false
	}

	// In practice, erroneous values should be caught at argument parsing time,
	// but validate here as well.
	if m.SampleAreaWidth < 1 {
		m.sampleAreaWidthTooSmall = true
		fmt.Print("Sample area width must be > 0\n")
		return false
	}
	if m.MaxDifferentPixelsPerArea < 0 ||
		m.MaxDifferentPixelsPerArea > m.SampleAreaWidth*m.SampleAreaWidth {

		m.maxDifferentPixelsPerAreaOutOfRange = true
		fmt.Print("Max different pixels per area must be >= 0, <= pixels per sample\n")
		return false
	}
	if m.SampleAreaChannelDeltaThreshold < 0 || m.SampleAreaChannelDeltaThreshold > 255 {
		m.sampleAreaChannelDeltaThresholdOutOfRange = true
		fmt.Print("Sample area tolerance must be >= 0, <= 255\n")
		return false
	}

	// Check that we aren't trying to sample a larger area than the image.
	bounds := expected.Bounds()
	if m.SampleAreaWidth > bounds.Dx() || m.SampleAreaWidth > bounds.Dy() {
		m.sampleAreaWidthTooLarge = true
		fmt.Printf("Given sample area width %d is larger than the %d x %d image\n",
			m.SampleAreaWidth, bounds.Dx(), bounds.Dy())
		return false
	}

	// Convert the images to NRGBA.
	expectedNRGBA := image.NewNRGBA(bounds)
	actualNRGBA := image.NewNRGBA(bounds)
	draw.Draw(expectedNRGBA, bounds, expected, bounds.Min, draw.Src)
	draw.Draw(actualNRGBA, bounds, actual, bounds.Min, draw.Src)

	// Iterate over each sample area and compare.
	for x := bounds.Min.X; x <= bounds.Max.X-m.SampleAreaWidth; x++ {
		for y := bounds.Min.Y; y <= bounds.Max.Y-m.SampleAreaWidth; y++ {
			numDifferentPixels := 0
			for xOffset := 0; xOffset < m.SampleAreaWidth; xOffset++ {
				for yOffset := 0; yOffset < m.SampleAreaWidth; yOffset++ {
					expectedPixel := expectedNRGBA.NRGBAAt(x+xOffset, y+yOffset)
					actualPixel := actualNRGBA.NRGBAAt(x+xOffset, y+yOffset)
					if expectedPixel != actualPixel {
						rDiff := absDiff(expectedPixel.R, actualPixel.R)
						gDiff := absDiff(expectedPixel.G, actualPixel.G)
						bDiff := absDiff(expectedPixel.B, actualPixel.B)
						aDiff := absDiff(expectedPixel.A, actualPixel.A)
						if rDiff > m.SampleAreaChannelDeltaThreshold ||
							gDiff > m.SampleAreaChannelDeltaThreshold ||
							bDiff > m.SampleAreaChannelDeltaThreshold ||
							aDiff > m.SampleAreaChannelDeltaThreshold {

							numDifferentPixels++
						}
					}
				}
			}
			if numDifferentPixels > m.MaxDifferentPixelsPerArea {
				m.failedSampleArea = image.Rect(x, y, x+m.SampleAreaWidth, y+m.SampleAreaWidth)
				m.numDifferentPixels = numDifferentPixels
				return false
			}
		}
	}
	return true
}

// FailedSampleAreaStart returns the point corresponding the the top left corner
// of the first failed sample area.
func (m *Matcher) FailedSampleArea() image.Rectangle { return m.failedSampleArea }

// NumDifferentPixels returns the number of pixels that differed in the failed
// sample area.
func (m *Matcher) NumDifferentPixels() int { return m.numDifferentPixels }

// SampleAreaWidthTooSmall returns whether the comparison failed because the
// provided sample area is too small to be used.
func (m *Matcher) SampleAreaWidthTooSmall() bool { return m.sampleAreaWidthTooSmall }

// SampleAreaWidthTooLarge returns whether the comparison failed because the
// provided sample area is too large to be used.
func (m *Matcher) SampleAreaWidthTooLarge() bool { return m.sampleAreaWidthTooLarge }

// MaxDifferentPixelsPerAreaOutOfRange returns whether the comparison failed
// because the provided max different pixels per area is too large or small to
// be used.
func (m *Matcher) MaxDifferentPixelsPerAreaOutOfRange() bool {
	return m.maxDifferentPixelsPerAreaOutOfRange
}

// SampleAreaChannelDeltaThresholdOutOfRange returns whether the comparison
// failed because the provided pixel delta threshold is too large or small to be
// used.
func (m *Matcher) SampleAreaChannelDeltaThresholdOutOfRange() bool {
	return m.sampleAreaChannelDeltaThresholdOutOfRange
}

// absDiff takes two uint8 values m and n, computes |m - n|, and converts the result into an int
// suitable for addition without the risk of overflowing.
func absDiff(m, n uint8) int {
	if m > n {
		return int(m - n)
	}
	return int(n - m)
}
