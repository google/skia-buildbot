package diff

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

type DiffMetrics struct {
	NumDiffPixels     int
	PixelDiffPercent  float32
	PixelDiffFilePath string
	// Contains the maximum difference between the images for each R/G/B channel.
	MaxRGBDiffs []int
}

type DiffStore interface {
	Get(d1, d2 string) (*DiffMetrics, error)
}

// OpenImage is a utility function that opens the specified file and returns an
// image.Image
func OpenImage(filePath string) (image.Image, error) {
	reader, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	im, err := png.Decode(reader)
	if err != nil {
		return nil, err
	}
	return im, nil
}

// TODO(rmistry): Move the below functions to a 'util' or 'math' package.
func maxInt(a, b int) int {
	if a < b {
		return b
	}
	return a
}

func absInt(a int) int {
	if a < 0 {
		return a * -1
	}
	return a
}

// Returns the percentage of pixels that differ, as a float between 0 and 100
// (inclusive).
func getPixelDiffPercent(numDiffPixels, totalPixels int) float32 {
	return (float32(numDiffPixels) * 100) / float32(totalPixels)
}

// Finds and stores the max RGB differences between the specified Colors.
func fillMaxRGBDiffs(color1, color2 color.Color, maxRGBDiffs []int) {
	r1, g1, b1, _ := color1.RGBA()
	r2, g2, b2, _ := color2.RGBA()
	rDiff := absInt(int(r1>>8) - int(r2>>8))
	gDiff := absInt(int(g1>>8) - int(g2>>8))
	bDiff := absInt(int(b1>>8) - int(b2>>8))
	maxRGBDiffs[0] = maxInt(maxRGBDiffs[0], rDiff)
	maxRGBDiffs[1] = maxInt(maxRGBDiffs[1], gDiff)
	maxRGBDiffs[2] = maxInt(maxRGBDiffs[2], bDiff)
}

// Diff is a utility function that calculates the DiffMetrics for the provided
// images. Intended to be called from the DiffStore implementations.
func Diff(img1, img2 image.Image, diffFilePath string) (*DiffMetrics, error) {
	img1Bounds := img1.Bounds()
	img2Bounds := img2.Bounds()
	resultImg := image.NewGray(
		image.Rect(0, 0, maxInt(img1Bounds.Dx(), img2Bounds.Dx()), maxInt(img1Bounds.Dy(), img2Bounds.Dy())))

	totalPixels := resultImg.Bounds().Dx() * resultImg.Bounds().Dy()
	// Loop through all points and compare.
	numDiffPixels := 0
	maxRGBDiffs := make([]int, 3)
	for x := 0; x <= resultImg.Bounds().Dx(); x++ {
		for y := 0; y <= resultImg.Bounds().Dy(); y++ {
			color1 := img1.At(x, y)
			color2 := img2.At(x, y)

			if color1 != color2 {
				fillMaxRGBDiffs(color1, color2, maxRGBDiffs)
				numDiffPixels++
				// Display differing pixels in white.
				resultImg.Set(x, y, color.White)
			}
		}
	}
	f, err := os.Create(diffFilePath)
	if err != nil {
		return nil, err
	}
	if err := png.Encode(f, resultImg); err != nil {
		return nil, err
	}

	return &DiffMetrics{
		NumDiffPixels:     numDiffPixels,
		PixelDiffPercent:  getPixelDiffPercent(numDiffPixels, totalPixels),
		PixelDiffFilePath: diffFilePath,
		MaxRGBDiffs:       maxRGBDiffs}, nil
}
