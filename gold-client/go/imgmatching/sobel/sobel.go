package sobel

import (
	"image"
	"image/color"
	"math"

	"go.skia.org/infra/gold-client/go/imgmatching/fuzzy"
)

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
	EdgeThreshold int
}

// Match implements the imagmatching.Matcher interface.
func (m *Matcher) Match(expected, actual image.Image) bool {
	// TODO(lovisolo): Implement.
	return false
}

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
