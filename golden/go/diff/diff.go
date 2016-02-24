package diff

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"unsafe"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/util"
)

var (
	PixelMatchColor = color.Transparent

	// Orange gradient.
	//
	// These are non-premultiplied RGBA values.
	PixelDiffColor = [][]uint8{
		{0xfd, 0xd0, 0xa2, 0xff},
		{0xfd, 0xae, 0x6b, 0xff},
		{0xfd, 0x8d, 0x3c, 0xff},
		{0xf1, 0x69, 0x13, 0xff},
		{0xd9, 0x48, 0x01, 0xff},
		{0xa6, 0x36, 0x03, 0xff},
		{0x7f, 0x27, 0x04, 0xff},
	}

	// Blue gradient.
	//
	// These are non-premultiplied RGBA values.
	PixelAlphaDiffColor = [][]uint8{
		{0xc6, 0xdb, 0xef, 0xff},
		{0x9e, 0xca, 0xe1, 0xff},
		{0x6b, 0xae, 0xd6, 0xff},
		{0x42, 0x92, 0xc6, 0xff},
		{0x21, 0x71, 0xb5, 0xff},
		{0x08, 0x51, 0x9c, 0xff},
		{0x08, 0x30, 0x6b, 0xff},
	}
)

// Returns the offset into the color slices (PixelDiffColor,
// or PixelAlphaDiffColor) based on the delta passed in.
//
// The number passed in is the difference between two colors,
// which can take values in the range 4*[0,65535] == [0,262140].
func deltaOffset(n int) int {
	n = (n + 256) / 257
	// scale down to 4*[0,255] == [0,1020] rounding up, then do the old 8-bit math.
	ret := int(math.Ceil(math.Log(float64(n))/math.Log(3) + 0.5))
	if ret < 1 || ret > 7 {
		glog.Fatalf("Input: %d", n)
	}
	return ret - 1
}

type DiffMetrics struct {
	NumDiffPixels     int
	PixelDiffPercent  float32
	PixelDiffFilePath string
	// Contains the maximum difference between the images for each R/G/B channel.
	MaxRGBADiffs []int
	// True if the dimensions of the compared images are different.
	DimDiffer bool
}

// Diff error to indicate different error conditions during diffing.
type DiffErr string

const (
	// Http related error occured.
	HTTP DiffErr = "http_error"

	// Image is corrupted and cannot be decoded.
	CORRUPTED DiffErr = "corrupted"

	// Arbitrary error.
	OTHER DiffErr = "other"
)

// DigestFailure captures the details of a digest error that occured.
type DigestFailure struct {
	Digest string  `json:"digest"`
	Reason DiffErr `json:"reason"`
	TS     int64   `json:"ts"`
	Error  string  `json:"error"`
}

// Implement sort.Interface for a slice of DigestFailure
type DigestFailureSlice []*DigestFailure

func (d DigestFailureSlice) Len() int           { return len(d) }
func (d DigestFailureSlice) Less(i, j int) bool { return d[i].TS < d[j].TS }
func (d DigestFailureSlice) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }

type DiffStore interface {
	// Get returns the DiffMetrics of the provided dMain digest vs all digests
	// specified in dRest.
	Get(dMain string, dRest []string) (map[string]*DiffMetrics, error)

	// AbsPath returns the paths of the images that correspond to the given
	// image digests.
	AbsPath(digest []string) map[string]string

	// UnavailableDigests returns map[digest]*DigestFailure which can be used
	// to check whether a digest could not be processed and to provide details
	// about failures.
	UnavailableDigests() map[string]*DigestFailure

	// PurgeDigests removes all information related to the indicated digests
	// (image, diffmetric) from local caches. If purgeGS is true it will also
	// purge the digests image from Google storage, forcing that the digest
	// be re-uploaded by the build bots.
	PurgeDigests(digests []string, purgeGS bool) error

	// SetDigestSets sets the sets of digests we want to compare grouped by
	// names (usually test names). This sets the digests we are currently
	// interested in and removes digests (and their diffs) that we are no
	// longer interested in.
	SetDigestSets(namedDigestSets map[string]map[string]bool)
}

// OpenImage is a utility function that opens the specified file and returns an
// image.Image
func OpenImage(filePath string) (image.Image, error) {
	reader, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer util.Close(reader)
	im, err := png.Decode(reader)
	if err != nil {
		return nil, err
	}
	return im, nil
}

func uint8ToColor(c []uint8) color.Color {
	return color.NRGBA{R: c[0], G: c[1], B: c[2], A: c[3]}
}

// diffColors compares two color values and returns a color to indicate the difference.
// It also updates maxDiff with the maximum per-channel diffs.
// If the RGB channels are identical but the alpha differs, return PixelAlphaDiffColor.
func diffColors(color1, color2 color.Color, maxDiff []int) color.Color {
	if color1 == color2 {
		return PixelMatchColor
	}

	c1 := color.NRGBA64Model.Convert(color1).(color.NRGBA64)
	c2 := color.NRGBA64Model.Convert(color2).(color.NRGBA64)

	dr := util.AbsInt(int(c1.R) - int(c2.R))
	dg := util.AbsInt(int(c1.G) - int(c2.G))
	db := util.AbsInt(int(c1.B) - int(c2.B))
	da := util.AbsInt(int(c1.A) - int(c2.A))

	maxDiff[0] = util.MaxInt(maxDiff[0], dr)
	maxDiff[1] = util.MaxInt(maxDiff[1], dg)
	maxDiff[2] = util.MaxInt(maxDiff[2], db)
	maxDiff[3] = util.MaxInt(maxDiff[3], da)

	// If the color channels differ we mark with the diff color.
	if dr+dg+db > 0 {
		return uint8ToColor(PixelDiffColor[deltaOffset(dr+dg+db+da)])
	}

	// If only the alpha channel differs we mark it with the alpha diff color.
	if da > 0 {
		return uint8ToColor(PixelAlphaDiffColor[deltaOffset(da)])
	}

	return PixelMatchColor
}

func diff_64_64(img1, img2 *image.NRGBA64, result *image.NRGBA) *DiffMetrics {
	p1 := unsafe.Pointer(&img1.Pix[0])
	p2 := unsafe.Pointer(&img2.Pix[0])

	total := len(img1.Pix) / 8
	diffs := 0
	maxDiff := []int{0, 0, 0, 0}
	for i := 0; i < total; i++ {
		// This awkward-looking unsafe.Pointer construction lets us:
		//   1) do 64-bit reads out of an []uint8
		//   2) bypass bounds checks
		rgba := *(*uint64)(unsafe.Pointer(uintptr(p1) + uintptr(i*8)))
		RGBA := *(*uint64)(unsafe.Pointer(uintptr(p2) + uintptr(i*8)))
		if rgba == RGBA {
			continue
		}
		// Code above this line is the hot path.  Code below here doesn't need to be super fast.
		diffs++
		r := uint16(rgba >> 0)
		g := uint16(rgba >> 16)
		b := uint16(rgba >> 32)
		a := uint16(rgba >> 48)

		R := uint16(RGBA >> 0)
		G := uint16(RGBA >> 16)
		B := uint16(RGBA >> 32)
		A := uint16(RGBA >> 48)

		dr := util.AbsInt(int(r) - int(R))
		dg := util.AbsInt(int(g) - int(G))
		db := util.AbsInt(int(b) - int(B))
		da := util.AbsInt(int(a) - int(A))

		maxDiff[0] = util.MaxInt(maxDiff[0], dr)
		maxDiff[1] = util.MaxInt(maxDiff[1], dg)
		maxDiff[2] = util.MaxInt(maxDiff[2], db)
		maxDiff[3] = util.MaxInt(maxDiff[3], da)

		if dr+dg+db > 0 {
			copy(result.Pix[i*4:], PixelDiffColor[deltaOffset(dr+dg+db+da)])
		} else {
			copy(result.Pix[i*4:], PixelAlphaDiffColor[deltaOffset(da)])
		}
	}
	return &DiffMetrics{
		NumDiffPixels:    diffs,
		PixelDiffPercent: 100 * float32(diffs) / float32(total),
		MaxRGBADiffs:     maxDiff,
		DimDiffer:        false,
	}
}

func diff_32_32(img1, img2 *image.NRGBA, result *image.NRGBA) *DiffMetrics {
	p1 := unsafe.Pointer(&img1.Pix[0])
	p2 := unsafe.Pointer(&img2.Pix[0])

	total := len(img1.Pix) / 4
	diffs := 0
	maxDiff := []int{0, 0, 0, 0}

	for i := 0; i < total; i++ {
		rgba := *(*uint32)(unsafe.Pointer(uintptr(p1) + uintptr(i*4)))
		RGBA := *(*uint32)(unsafe.Pointer(uintptr(p2) + uintptr(i*4)))
		if rgba == RGBA {
			continue
		}
		diffs++
		r := uint8(rgba >> 0)
		g := uint8(rgba >> 8)
		b := uint8(rgba >> 16)
		a := uint8(rgba >> 24)

		R := uint8(RGBA >> 0)
		G := uint8(RGBA >> 8)
		B := uint8(RGBA >> 16)
		A := uint8(RGBA >> 24)

		dr := 257 * util.AbsInt(int(r)-int(R))
		dg := 257 * util.AbsInt(int(g)-int(G))
		db := 257 * util.AbsInt(int(b)-int(B))
		da := 257 * util.AbsInt(int(a)-int(A))

		maxDiff[0] = util.MaxInt(maxDiff[0], dr)
		maxDiff[1] = util.MaxInt(maxDiff[1], dg)
		maxDiff[2] = util.MaxInt(maxDiff[2], db)
		maxDiff[3] = util.MaxInt(maxDiff[3], da)

		if dr+dg+db > 0 {
			copy(result.Pix[i*4:], PixelDiffColor[deltaOffset(dr+dg+db+da)])
		} else {
			copy(result.Pix[i*4:], PixelAlphaDiffColor[deltaOffset(da)])
		}
	}
	return &DiffMetrics{
		NumDiffPixels:    diffs,
		PixelDiffPercent: 100 * float32(diffs) / float32(total),
		MaxRGBADiffs:     maxDiff,
		DimDiffer:        false,
	}
}

func diff_32_64(img1 *image.NRGBA, img2 *image.NRGBA64, result *image.NRGBA) *DiffMetrics {
	p1 := unsafe.Pointer(&img1.Pix[0])
	p2 := unsafe.Pointer(&img2.Pix[0])

	total := len(img1.Pix) / 4
	diffs := 0
	maxDiff := []int{0, 0, 0, 0}
	for i := 0; i < total; i++ {
		rgba := *(*uint32)(unsafe.Pointer(uintptr(p1) + uintptr(i*4)))
		RGBA := *(*uint64)(unsafe.Pointer(uintptr(p2) + uintptr(i*8)))

		r := uint16(uint8(rgba>>0)) * 257
		g := uint16(uint8(rgba>>8)) * 257
		b := uint16(uint8(rgba>>16)) * 257
		a := uint16(uint8(rgba>>24)) * 257

		R := uint16(RGBA >> 0)
		G := uint16(RGBA >> 16)
		B := uint16(RGBA >> 32)
		A := uint16(RGBA >> 48)

		if r == R && g == G && b == B && a == A {
			continue
		}
		diffs++

		dr := util.AbsInt(int(r) - int(R))
		dg := util.AbsInt(int(g) - int(G))
		db := util.AbsInt(int(b) - int(B))
		da := util.AbsInt(int(a) - int(A))

		maxDiff[0] = util.MaxInt(maxDiff[0], dr)
		maxDiff[1] = util.MaxInt(maxDiff[1], dg)
		maxDiff[2] = util.MaxInt(maxDiff[2], db)
		maxDiff[3] = util.MaxInt(maxDiff[3], da)

		if dr+dg+db > 0 {
			copy(result.Pix[i*4:], PixelDiffColor[deltaOffset(dr+dg+db+da)])
		} else {
			copy(result.Pix[i*4:], PixelAlphaDiffColor[deltaOffset(da)])
		}
	}
	return &DiffMetrics{
		NumDiffPixels:    diffs,
		PixelDiffPercent: 100 * float32(diffs) / float32(total),
		MaxRGBADiffs:     maxDiff,
		DimDiffer:        false,
	}
}

// Diff is a utility function that calculates the DiffMetrics and the image of the
// difference for the provided images.
func Diff(img1, img2 image.Image) (*DiffMetrics, *image.NRGBA) {
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
	totalPixels := resultWidth * resultHeight

	if img1Bounds.Eq(img2Bounds) {
		// Fast paths for {NRGBA, NRGBA64}^2, all requiring the two images are the same shape.
		switch i1 := img1.(type) {
		case *image.NRGBA64:
			switch i2 := img2.(type) {
			case *image.NRGBA64:
				return diff_64_64(i1, i2, resultImg), resultImg
			case *image.NRGBA:
				return diff_32_64(i2, i1, resultImg), resultImg
			}
		case *image.NRGBA:
			switch i2 := img2.(type) {
			case *image.NRGBA64:
				return diff_32_64(i1, i2, resultImg), resultImg
			case *image.NRGBA:
				return diff_32_32(i1, i2, resultImg), resultImg
			}
		}
	}

	// This is our very, very slow path.  It's used for images with different
	// dimensions or for images that aren't one of our expected formats (NRGBA 32/64).
	diffs := totalPixels // we'll count down
	maxDiffs := []int{0, 0, 0, 0}

	dimDiffer := !img1Bounds.Eq(img2Bounds)
	if dimDiffer {
		// Start every pixel at max-diff, so the non-overlapping pixels are correct.
		// Per-channel max diffs don't make much sense in this case, so force them:
		maxDiffs = []int{65535, 65535, 65535, 0}
		maxDiffColor := uint8ToColor(PixelDiffColor[deltaOffset(65535*4)])
		draw.Draw(resultImg, resultImg.Bounds(), &image.Uniform{maxDiffColor}, image.ZP, draw.Src)
	}

	for x := 0; x < cmpWidth; x++ {
		for y := 0; y < cmpHeight; y++ {
			dc := diffColors(img1.At(x, y), img2.At(x, y), maxDiffs)
			if dc == PixelMatchColor {
				diffs--
			}
			resultImg.Set(x, y, dc)
		}
	}
	return &DiffMetrics{
		NumDiffPixels:    diffs,
		PixelDiffPercent: 100 * float32(diffs) / float32(totalPixels),
		MaxRGBADiffs:     maxDiffs,
		DimDiffer:        dimDiffer,
	}, resultImg
}
