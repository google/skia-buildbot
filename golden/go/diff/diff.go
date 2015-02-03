package diff

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"reflect"
	"unsafe"

	"github.com/skia-dev/glog"

	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/golden/go/thumb"
)

var (
	PixelMatchColor = color.White
	// Red from the color blind palette.
	PixelDiffColor = color.RGBA{0xE3, 0x1A, 0x1C, 0xFF}
	// Grey from the color blind palette.
	PixelAlphaDiffColor = color.RGBA{0xB3, 0xB3, 0xB3, 0xFF}
)

type DiffMetrics struct {
	NumDiffPixels              int
	PixelDiffPercent           float32
	PixelDiffFilePath          string
	ThumbnailPixelDiffFilePath string
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

	// ThumbAbsPath returns the paths of the thumbnails of the images that
	// correspond to the given image digests.
	ThumbAbsPath(digest []string) map[string]string

	// UnavailableDigests returns the set of digests that cannot be downloaded or
	// processed (e.g. because the PNG is corrupted) and should therefore be
	// be ignored. The return value is considered to be read only.
	UnavailableDigests() map[string]bool
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

func setAllFF(slice []uint8) {
	/*
		for i, _ := range slice {
			slice[i] = 0xFF
		}
	*/
	len64 := len(slice) / 8
	var slice64 []uint64
	h := (*reflect.SliceHeader)(unsafe.Pointer(&slice64))
	h.Data = uintptr(unsafe.Pointer(&slice[0]))
	h.Len = len64
	h.Cap = len64

	for i := 0; i < len64; i++ {
		slice64[i] = 0xFFFFFFFFFFFFFFFF
	}
	for i := len64 * 8; i < len(slice); i++ {
		slice[i] = 0xFF
	}
}

// DiffAndWrite is a utility function that calculates the DiffMetrics for the
// provided images. Intended to be called from the DiffStore implementations.
func DiffAndWrite(img1, img2 image.Image, diffFilePath string) (*DiffMetrics, error) {
	metrics, diff, err := Diff(img1, img2)
	if err != nil {
		return nil, err
	}
	if diffFilePath != "" {
		f, err := os.Create(diffFilePath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		if err := png.Encode(f, diff); err != nil {
			return nil, err
		}
		metrics.PixelDiffFilePath = diffFilePath
		metrics.ThumbnailPixelDiffFilePath = thumb.AbsPath(diffFilePath)
		g, err := os.Create(metrics.ThumbnailPixelDiffFilePath)
		if err != nil {
			return nil, err
		}
		defer g.Close()
		if err := png.Encode(g, thumb.Thumbnail(diff)); err != nil {
			return nil, err
		}
	}
	return metrics, nil
}

// Diff is a utility function that calculates the DiffMetrics and the image of the
// difference for the provided images.
func Diff(img1, img2 image.Image) (*DiffMetrics, *image.NRGBA, error) {

	img1Bounds := img1.Bounds()
	img2Bounds := img2.Bounds()

	// Get the bounds we want to compare.
	cmpWidth := util.MinInt(img1Bounds.Dx(), img2Bounds.Dx())
	cmpHeight := util.MinInt(img1Bounds.Dy(), img2Bounds.Dy())

	// Get the bounds of the resulting image. If they dimensions match they
	// will be identical to the result bounds. Fill the image with black pixels.
	resultWidth := util.MaxInt(img1Bounds.Dx(), img2Bounds.Dx())
	resultHeight := util.MaxInt(img1Bounds.Dy(), img2Bounds.Dy())
	resultImg := image.NewNRGBA(image.Rect(0, 0, resultWidth, resultHeight))
	setAllFF(resultImg.Pix)
	totalPixels := resultWidth * resultHeight

	// Loop through all points and compare. We start assuming all pixels are
	// wrong. This takes care of the case where the images have different sizes
	// and there is an area not inspected by the loop.
	numDiffPixels := resultWidth * resultHeight
	maxRGBADiffs := make([]int, 4)

	// Pix is a []uint8 rotating through R, G, B, A, R, G, B, A, ...
	p1 := getNRGBA(img1).Pix
	p2 := getNRGBA(img2).Pix
	// Compare the bounds, if they are the same then use this fast path.
	// We pun to uint64 to compare 2 pixels at a time, so we also require
	// an even number of pixels here.  If that's a big deal, we can easily
	// fix that up, handling the straggler pixel separately at the end.
	if img1Bounds.Eq(img2Bounds) && len(p1)%8 == 0 {
		numDiffPixels = 0
		// Note the += 8.  We're checking two pixels at a time here.
		for i := 0; i < len(p1); i += 8 {
			// Most pixels we compare will be the same, so from here to
			// the 'continue' is the hot path in all this code.
			rgba_2x := (*uint64)(unsafe.Pointer(&p1[i]))
			RGBA_2x := (*uint64)(unsafe.Pointer(&p2[i]))
			if *rgba_2x == *RGBA_2x {
				continue
			}

			// When off == 0, we check the first pixel of the pair; when 4, the second.
			for off := 0; off <= 4; off += 4 {
				r, g, b, a := p1[off+i+0], p1[off+i+1], p1[off+i+2], p1[off+i+3]
				R, G, B, A := p2[off+i+0], p2[off+i+1], p2[off+i+2], p2[off+i+3]
				if r != R || g != G || b != B || a != A {
					numDiffPixels++
					dr := util.AbsInt(int(r) - int(R))
					dg := util.AbsInt(int(g) - int(G))
					db := util.AbsInt(int(b) - int(B))
					da := util.AbsInt(int(a) - int(A))
					maxRGBADiffs[0] = util.MaxInt(dr, maxRGBADiffs[0])
					maxRGBADiffs[1] = util.MaxInt(dg, maxRGBADiffs[1])
					maxRGBADiffs[2] = util.MaxInt(db, maxRGBADiffs[2])
					maxRGBADiffs[3] = util.MaxInt(da, maxRGBADiffs[3])
					if dr+dg+db > 0 {
						resultImg.Pix[off+i+0] = PixelDiffColor.R
						resultImg.Pix[off+i+1] = PixelDiffColor.G
						resultImg.Pix[off+i+2] = PixelDiffColor.B
						resultImg.Pix[off+i+3] = PixelDiffColor.A
					} else {
						resultImg.Pix[off+i+0] = PixelAlphaDiffColor.R
						resultImg.Pix[off+i+1] = PixelAlphaDiffColor.G
						resultImg.Pix[off+i+2] = PixelAlphaDiffColor.B
						resultImg.Pix[off+i+3] = PixelAlphaDiffColor.A
					}
				}
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

	return &DiffMetrics{
		NumDiffPixels:    numDiffPixels,
		PixelDiffPercent: getPixelDiffPercent(numDiffPixels, totalPixels),
		MaxRGBADiffs:     maxRGBADiffs,
		DimDiffer:        (cmpWidth != resultWidth) || (cmpHeight != resultHeight)}, resultImg, nil
}
