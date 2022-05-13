package sobel

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"go.skia.org/infra/gold-client/go/imgmatching/fuzzy"
)

// testMatcher is an exact copy of the imgmatching.Matcher interface for the sole purpose of
// avoiding an import cycle between packages imgmatching and sobel.
type testMatcher interface {
	Match(expected, actual image.Image) bool
}

// Matcher is a non-exact image matching algorithm.
//
// It extends the fuzzy.Matcher algorithm by performing edge detection using the Sobel operator[1]
// and ignoring any pixels that are part of an edge.
//
// The algorithm performs the following steps:
//   1. It applies the Sobel operator to the expected image, producing a 0 to 255 value per pixel
//      indicating how likely it is to be part of an edge.
//   2. It zeroes-out any (x,y) coordinates on *both* images where the aforementioned value exceeds
//      EdgeThreshold. Note that this assumes both images are of equal size.
//   3. It passes the two resulting images to the fuzzy.Matcher algorithm (using parameters
//      MaxDifferentPixels and PixelDeltaThreshold) and returns its return value.
//
// [1] https://en.wikipedia.org/wiki/Sobel_operator
type Matcher struct {
	fuzzy.Matcher
	EdgeThreshold int // Valid values are 0 to 255 inclusive.

	// If set, fuzzyMatcherForTesting will be used instead of the embedded fuzzy.Matcher.
	fuzzyMatcherForTesting testMatcher

	// Debug information about the last pair of matched images.
	sobelOutput                   *image.Gray
	expectedImageWithEdgesRemoved image.Image
	actualImageWithEdgesRemoved   image.Image
}

// Match implements the imgmatching.Matcher interface.
func (m *Matcher) Match(expected, actual image.Image) bool {
	// Images must be the same size.
	if !expected.Bounds().Eq(actual.Bounds()) {
		return false
	}

	// Extract edges from the expected image.
	m.sobelOutput = sobel(imageToGray(expected))

	// Zero-out edges on *both* the expected and actual images, using the edges from the former in
	// both cases.
	//
	// Note that the [0, 255] value range for EdgeThreshold is enforced by the
	// imgmatching.MakeMatcher() factory function, so it's safe to cast to uint8.
	m.expectedImageWithEdgesRemoved = zeroOutEdges(expected, m.sobelOutput, uint8(m.EdgeThreshold))
	m.actualImageWithEdgesRemoved = zeroOutEdges(actual, m.sobelOutput, uint8(m.EdgeThreshold))

	// Determine whether to use the embedded fuzzy.Matcher or the fuzzyMatcherForTesting.
	fuzzyMatcher := m.fuzzyMatcherForTesting
	if fuzzyMatcher == nil {
		fuzzyMatcher = &m.Matcher
	}

	// Delegate to the fuzzy matcher.
	return fuzzyMatcher.Match(m.expectedImageWithEdgesRemoved, m.actualImageWithEdgesRemoved)
}

// SobelOutput returns an image with the output of applying the Sobel operator to the expected
// image from the last Match method call.
func (m *Matcher) SobelOutput() image.Image { return m.sobelOutput }

// ExpectedImageWithEdgesRemoved returns the left image from the last Match method call with its edges
// removed.
func (m *Matcher) ExpectedImageWithEdgesRemoved() image.Image { return m.expectedImageWithEdgesRemoved }

// ActualImageWithEdgesRemoved returns the right image from the last Match method call with its edges
// removed.
func (m *Matcher) ActualImageWithEdgesRemoved() image.Image { return m.actualImageWithEdgesRemoved }

// sobel returns a grayscale image with the result of applying the Sobel operator[1] to each pixel
// in the input image.
//
// The returned image has the same size as the input image. Border pixels will be black, because
// computing the Sobel operator requires all 8 neighboring pixels (as a consequence, all pixels
// will be black for input images smaller than 3x3). The value of the Sobel operator is clipped at
// 255 before being converted into an 8-bit grayscale pixel.
//
// [1] https://en.wikipedia.org/wiki/Sobel_operator
func sobel(img *image.Gray) *image.Gray {
	kernelX := [3][3]int{
		{1, 0, -1},
		{2, 0, -2},
		{1, 0, -1},
	}

	kernelY := [3][3]int{
		{1, 2, 1},
		{0, 0, 0},
		{-1, -2, -1},
	}

	outputImg := image.NewGray(img.Bounds())

	// Iterate over all pixels except those at the borders of the image, because we need all 8
	// neighboring pixels to be able to apply the convolutions. Border pixels will remain black.
	for y := img.Bounds().Min.Y + 1; y < img.Bounds().Max.Y-1; y++ {
		for x := img.Bounds().Min.X + 1; x < img.Bounds().Max.X-1; x++ {
			// Apply convolutions.
			convolutionX := applyConvolution(img, kernelX, x, y)
			convolutionY := applyConvolution(img, kernelY, x, y)

			// Compute the Sobel operator as the norm of the convolution vector.
			sobelOperator := math.Sqrt(float64(convolutionX*convolutionX + convolutionY*convolutionY))

			// Clip sobelOperator and set output pixel (x,y).
			clippedSobelOperator := uint8(sobelOperator)
			if sobelOperator > float64(math.MaxUint8) {
				clippedSobelOperator = math.MaxUint8
			}
			outputImg.SetGray(x, y, color.Gray{Y: clippedSobelOperator})
		}
	}

	return outputImg
}

// applyConvolution returns the result of applying the given convolution kernel to the pixel at
// (x, y) in the input grayscale image.
func applyConvolution(img *image.Gray, kernel [3][3]int, x, y int) int {
	convolution := 0

	// Iterate over all coordinates of the 3x3 convolution kernel matrix.
	for j := 0; j <= 2; j++ {
		for i := 0; i <= 2; i++ {
			convolution += int(img.GrayAt(x+i-1, y+j-1).Y) * kernel[j][i]
		}
	}

	return convolution
}

// zeroOutEdges returns a copy of the input image in which all pixels above the edge threshold are
// replaced with black pixels. Input and edges images must have the same bounds.
func zeroOutEdges(img image.Image, edges *image.Gray, edgeThreshold uint8) image.Image {
	// Fail loudly if the assumption above isn't met. This indicates a programming error and should
	// never happen in practice.
	if edges.Bounds() != img.Bounds() {
		panic("input and edges images must have the same bounds")
	}

	outputImg := image.NewNRGBA(img.Bounds())

	// Iterate over all pixels.
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			pixel := img.At(x, y)

			// Zero out pixel if it's above the edge threshold.
			if edges.GrayAt(x, y).Y > edgeThreshold {
				pixel = &color.NRGBA{R: 0, G: 0, B: 0, A: 255}
			}

			outputImg.Set(x, y, pixel)
		}
	}

	return outputImg
}

// imageToGray converts the given image to grayscale.
func imageToGray(img image.Image) *image.Gray {
	grayImg := image.NewGray(img.Bounds())
	draw.Draw(grayImg, img.Bounds(), img, img.Bounds().Min, draw.Src)
	return grayImg
}
