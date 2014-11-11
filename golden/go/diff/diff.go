package diff

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"

	"skia.googlesource.com/buildbot.git/go/util"
)

type DiffMetrics struct {
	NumDiffPixels     int
	PixelDiffPercent  float32
	PixelDiffFilePath string
	// Contains the maximum difference between the images for each R/G/B channel.
	MaxRGBDiffs []int
	// True if the dimensions of the compared images are different.
	DimDiffer bool
}

type DiffStore interface {
	// Get returns the DiffMetrics of the provided dMain digest vs all digests
	// specified in dRest.
	Get(dMain string, dRest []string) (map[string]*DiffMetrics, error)
	// AbsPath returns the paths of the images that correspond to the given
	// image digests.
	AbsPath(digest []string) map[string]string
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

// Returns the percentage of pixels that differ, as a float between 0 and 100
// (inclusive).
func getPixelDiffPercent(numDiffPixels, totalPixels int) float32 {
	return (float32(numDiffPixels) * 100) / float32(totalPixels)
}

// Finds and stores the max RGB differences between the specified Colors.
func fillMaxRGBDiffs(color1, color2 color.Color, maxRGBDiffs []int) {
	r1, g1, b1, _ := color1.RGBA()
	r2, g2, b2, _ := color2.RGBA()
	rDiff := util.AbsInt(int(r1>>8) - int(r2>>8))
	gDiff := util.AbsInt(int(g1>>8) - int(g2>>8))
	bDiff := util.AbsInt(int(b1>>8) - int(b2>>8))
	maxRGBDiffs[0] = util.MaxInt(maxRGBDiffs[0], rDiff)
	maxRGBDiffs[1] = util.MaxInt(maxRGBDiffs[1], gDiff)
	maxRGBDiffs[2] = util.MaxInt(maxRGBDiffs[2], bDiff)
}

// Diff is a utility function that calculates the DiffMetrics for the provided
// images. Intended to be called from the DiffStore implementations.
func Diff(img1, img2 image.Image, diffFilePath string) (*DiffMetrics, error) {
	img1Bounds := img1.Bounds()
	img2Bounds := img2.Bounds()

	// Get the bounds we want to compare.
	cmpWidth := util.MinInt(img1Bounds.Dx(), img2Bounds.Dx())
	cmpHeight := util.MinInt(img1Bounds.Dy(), img2Bounds.Dy())

	// Get the bounds of the resulting image. If they dimensions match they
	// will be identical to the result bounds. Fill the image with black pixels.
	resultWidth := util.MaxInt(img1Bounds.Dy(), img2Bounds.Dy())
	resultHeight := util.MaxInt(img1Bounds.Dx(), img2Bounds.Dx())
	resultImg := image.NewGray(image.Rect(0, 0, resultWidth, resultHeight))
	draw.Draw(resultImg, image.Rect(0, 0, resultWidth, resultHeight), image.White, image.ZP, draw.Src)

	totalPixels := resultImg.Bounds().Dx() * resultImg.Bounds().Dy()
	// Loop through all points and compare. We start assuming all pixels are
	// wrong. This takes care of the case where the images have different sizes
	// and there is an area not inspected by the loop.
	numDiffPixels := resultWidth * resultHeight
	maxRGBDiffs := make([]int, 3)
	for x := 0; x < cmpWidth; x++ {
		for y := 0; y < cmpHeight; y++ {
			color1 := img1.At(x, y)
			color2 := img2.At(x, y)

			if color1 != color2 {
				fillMaxRGBDiffs(color1, color2, maxRGBDiffs)
			} else {
				numDiffPixels--
				resultImg.Set(x, y, color.Black)
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
		MaxRGBDiffs:       maxRGBDiffs,
		DimDiffer:         (cmpWidth != resultWidth) || (cmpHeight != resultHeight)}, nil
}
