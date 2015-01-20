package diff

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"

	"github.com/skia-dev/glog"

	"skia.googlesource.com/buildbot.git/go/util"
)

var (
	PixelMatchColor = color.White
	// Red from the color blind palette.
	PixelDiffColor = color.RGBA{0xE3, 0x1A, 0x1C, 0xFF}
	// Grey from the color blind palette.
	PixelAlphaDiffColor = color.RGBA{0xB3, 0xB3, 0xB3, 0xFF}
)

type DiffMetrics struct {
	NumDiffPixels     int
	PixelDiffPercent  float32
	PixelDiffFilePath string
	// Contains the maximum difference between the images for each R/G/B channel.
	MaxRGBADiffs []int
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

// diffColors compares two color values and returns a color to indicate the
// difference. If the colors differ it updates maxRGBADiffs to contain the
// maximum difference over multiple calls.
// If the RGB channels are identical, but the alpha differ then
// PixelAlphaDiffColor is returned. This allows to distinguish pixels that
// render the same, but have different alpha values.
func diffColors(color1, color2 color.Color, maxRGBADiffs []int) color.Color {
	// We compare them before normalizing to non-premultiplied. If one of the
	// original images did not have an alpha channel (but the other did) the
	// equality will be false.
	if color1 == color2 {
		return PixelMatchColor
	}

	// Treat all colors as non-premultiplied.
	c1 := color.NRGBAModel.Convert(color1).(color.NRGBA)
	c2 := color.NRGBAModel.Convert(color2).(color.NRGBA)

	rDiff := util.AbsInt(int(c1.R) - int(c2.R))
	gDiff := util.AbsInt(int(c1.G) - int(c2.G))
	bDiff := util.AbsInt(int(c1.B) - int(c2.B))
	aDiff := util.AbsInt(int(c1.A) - int(c2.A))
	maxRGBADiffs[0] = util.MaxInt(maxRGBADiffs[0], rDiff)
	maxRGBADiffs[1] = util.MaxInt(maxRGBADiffs[1], gDiff)
	maxRGBADiffs[2] = util.MaxInt(maxRGBADiffs[2], bDiff)
	maxRGBADiffs[3] = util.MaxInt(maxRGBADiffs[3], aDiff)

	// If the color channels differ we mark with the diff color.
	if (c1.R != c2.R) || (c1.G != c2.G) || (c1.B != c2.B) {
		return PixelDiffColor
	}

	// If only the alpha channel differs we mark it with the alpha diff color.
	return PixelAlphaDiffColor
}

// recode creates a new NRGBA image from the given image.
func recode(img image.Image) *image.NRGBA {
	ret := image.NewNRGBA(img.Bounds())
	draw.Draw(ret, img.Bounds(), img, image.Pt(0, 0), draw.Src)
	return ret
}

// getNRGBA converts the image to an *image.NRGBA in an efficent manner.
func getNRGBA(img image.Image) *image.NRGBA {
	switch t := img.(type) {
	case *image.NRGBA:
		return t
	case *image.RGBA:
		for i := 0; i < len(t.Pix); i += 4 {
			if t.Pix[i+3] != 0xff {
				glog.Warning("Unexpected premultiplied image!")
				return recode(img)
			}
		}
		// If every alpha is 0xff then t.Pix is already in NRGBA format, simply
		// share Pix between the RGBA and NRGBA structs.
		return &image.NRGBA{
			Pix:    t.Pix,
			Stride: t.Stride,
			Rect:   t.Rect,
		}
	default:
		// TODO(mtklein): does it make sense we're getting other types, or a DM bug?
		return recode(img)
	}
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
	resultWidth := util.MaxInt(img1Bounds.Dx(), img2Bounds.Dx())
	resultHeight := util.MaxInt(img1Bounds.Dy(), img2Bounds.Dy())
	resultImg := image.NewRGBA(image.Rect(0, 0, resultWidth, resultHeight))
	for i, _ := range resultImg.Pix {
		resultImg.Pix[i] = 0xff
	}
	totalPixels := resultWidth * resultHeight

	// Loop through all points and compare. We start assuming all pixels are
	// wrong. This takes care of the case where the images have different sizes
	// and there is an area not inspected by the loop.
	numDiffPixels := resultWidth * resultHeight
	maxRGBADiffs := make([]int, 4)

	n1 := getNRGBA(img1)
	n2 := getNRGBA(img2)
	// Compare the bounds, if they are the same then use the fast path.
	if img1Bounds.Eq(img2Bounds) {
		// Fastpath
		// Pix is a []uint8 rotating through R, G, B, A, R, G, B, A, ...
		for i := 0; i < len(n1.Pix); i += 4 {
			dr := util.AbsInt(int(n1.Pix[i+0]) - int(n2.Pix[i+0]))
			dg := util.AbsInt(int(n1.Pix[i+1]) - int(n2.Pix[i+1]))
			db := util.AbsInt(int(n1.Pix[i+2]) - int(n2.Pix[i+2]))
			da := util.AbsInt(int(n1.Pix[i+3]) - int(n2.Pix[i+3]))
			maxRGBADiffs[0] = util.MaxInt(dr, maxRGBADiffs[0])
			maxRGBADiffs[1] = util.MaxInt(dg, maxRGBADiffs[1])
			maxRGBADiffs[2] = util.MaxInt(db, maxRGBADiffs[2])
			maxRGBADiffs[3] = util.MaxInt(da, maxRGBADiffs[3])
			rgbDelta := dr + dg + db
			if rgbDelta > 0 {
				resultImg.Pix[i+0] = 0xE3
				resultImg.Pix[i+1] = 0x1A
				resultImg.Pix[i+2] = 0x1C
				resultImg.Pix[i+3] = 0xFF
			} else if da > 0 {
				resultImg.Pix[i+0] = 0xB3
				resultImg.Pix[i+1] = 0xB3
				resultImg.Pix[i+2] = 0xB3
				resultImg.Pix[i+3] = 0xFF
			} else {
				numDiffPixels--
			}
		}
	} else {
		for x := 0; x < cmpWidth; x++ {
			for y := 0; y < cmpHeight; y++ {
				color1 := img1.At(x, y)
				color2 := img2.At(x, y)

				dc := diffColors(color1, color2, maxRGBADiffs)
				if dc == PixelMatchColor {
					numDiffPixels--
				}
				resultImg.Set(x, y, dc)
			}
		}
	}
	if diffFilePath != "" {
		f, err := os.Create(diffFilePath)
		if err != nil {
			return nil, err
		}
		if err := png.Encode(f, resultImg); err != nil {
			return nil, err
		}
	}

	return &DiffMetrics{
		NumDiffPixels:     numDiffPixels,
		PixelDiffPercent:  getPixelDiffPercent(numDiffPixels, totalPixels),
		PixelDiffFilePath: diffFilePath,
		MaxRGBADiffs:      maxRGBADiffs,
		DimDiffer:         (cmpWidth != resultWidth) || (cmpHeight != resultHeight)}, nil
}
