package dynamicdiff

import (
	"fmt"
	"image"

	// TODO(kjlubick): This package should probably not use path/filepath (which is os dependent)
	// Since the separator is in GCS, it should use something that always uses '/'
	"path/filepath"
	"strings"

	"go.skia.org/infra/ct_pixel_diff/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore"
)

type DynamicDiffMetrics struct {
	// NumDiffPixels is the number of static pixels that are different.
	NumDiffPixels int `json:"numDiffPixels"`

	// PixelDiffPercent is the percentage of static pixels that are different.
	PixelDiffPercent float32 `json:"pixelDiffPercent"`

	// MaxRGBDiffs contains the maximum difference of each channel.
	MaxRGBDiffs []int `json:"maxRGBDiffs"`

	// NumStaticPixels is the total number of static pixels.
	NumStaticPixels int `json:"numStaticPixels"`

	// NumDynamicPixels is the total number of dynamic pixels. Note that
	// NumStaticPixels + NumDynamicPixels = number of total pixels.
	NumDynamicPixels int `json:"numDynamicPixels"`
}

// PixelDiffStoreMapper implements the diffstore.DiffStoreMapper interface.
// It uses instances of
// of an imageID is: runID/{nopatch/withpatch}/rank/URLfilename. A runID has the
// format userID-timeStamp.
type PixelDiffStoreMapper struct {
	util.LRUCodec
}

// NewPixelDiffStoreMapper returns a new instance of PixelDiffStoreMapper with
// a codec that encodes/decodes instance of DynamicDiffMetrics to/from JSON.
func NewPixelDiffStoreMapper(diffInstance interface{}) diffstore.DiffStoreMapper {
	return PixelDiffStoreMapper{LRUCodec: util.JSONCodec(&DynamicDiffMetrics{})}
}

// DiffFn implements the diffstore.DiffStoreMapper interface.
func (g PixelDiffStoreMapper) DiffFn(leftImg *image.NRGBA, rightImg *image.NRGBA) (interface{}, *image.NRGBA) {
	return DynamicContentDiff(leftImg, rightImg)
}

// DiffID implements the diffstore.DiffStoreMapper interface.
func (p PixelDiffStoreMapper) DiffID(leftImgID, rightImgID common.ImageID) string {
	// Return a string containing the common runID, rank and URL of the two image paths.
	path := strings.Split(string(leftImgID), "/")
	return strings.Join([]string{path[0], path[2], path[3]}, ":")
}

// SplitDiffID implements the diffstore.DiffStoreMapper interface.
func (p PixelDiffStoreMapper) SplitDiffID(diffID string) (common.ImageID, common.ImageID) {
	path := strings.Split(diffID, ":")
	return common.ImageID(filepath.Join(path[0], "nopatch", path[1], path[2])),
		common.ImageID(filepath.Join(path[0], "withpatch", path[1], path[2]))
}

// DiffPath implements the diffstore.DiffStoreMapper interface.
func (p PixelDiffStoreMapper) DiffPath(leftImgID, rightImgID common.ImageID) string {
	path := strings.Split(string(leftImgID), "/")
	imageName := path[0] + "/" + path[3]
	return fmt.Sprintf("%s.%s", imageName, diffstore.IMG_EXTENSION)
}

// ImagePaths implements the diffstore.DiffStoreMapper interface.
func (p PixelDiffStoreMapper) ImagePaths(imageID common.ImageID) (string, string, string) {
	localPath := fmt.Sprintf("%s.%s", imageID, diffstore.IMG_EXTENSION)
	path := strings.Split(string(imageID), "/")
	runID := strings.Split(path[0], "-")
	timeStamp := runID[1]
	datePath := filepath.Join(timeStamp[0:4], timeStamp[4:6], timeStamp[6:8], timeStamp[8:10])
	gsPath := filepath.Join(datePath, localPath)
	return localPath, "", gsPath
}

// IsValidDiffImgID implements the diffstore.DiffStoreMapper interface.
func (p PixelDiffStoreMapper) IsValidDiffImgID(diffImgID string) bool {
	path := strings.Split(diffImgID, "/")
	return len(path) == 2
}

// IsValidImgID implements the diffstore.DiffStoreMapper interface.
func (p PixelDiffStoreMapper) IsValidImgID(imgID string) bool {
	path := strings.Split(imgID, "/")
	return len(path) == 4
}

// DynamicContentDiff is a function that calculates the DiffMetrics and diff
// image for the provided images, taking into account that pixels with dynamic
// content are marked cyan and removing such pixels from the calculations. The
// images are assumed to have the same dimensions.
func DynamicContentDiff(left, right *image.NRGBA) (*DynamicDiffMetrics, *image.NRGBA) {
	bounds := left.Bounds()
	resultImg := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))

	// Pix is a []uint8 of R, G, B, A, R, G, B, A, ... values.
	p1 := left.Pix
	p2 := right.Pix

	numStaticPixels := 0
	numDynamicPixels := 0
	numDiffPixels := 0
	maxRGBDiffs := make([]int, 3)

	// Each pixel consists of 4 values (R, G, B, A). Alpha is ignored for diff
	// purposes.
	for i := 0; i < len(p1); i += 4 {
		r, g, b := p1[i+0], p1[i+1], p1[i+2]
		R, G, B := p2[i+0], p2[i+1], p2[i+2]

		// Ignore pixels with dynamic content, mark the pixel in the diff image as
		// dynamic, and increment the count of dynamic pixels.
		if isDynamicContentPixel(r, g, b) || isDynamicContentPixel(R, G, B) {
			copy(resultImg.Pix[i:], []uint8{0, 255, 255, 255})
			numDynamicPixels++
			continue
		}

		// Increment the count of static pixels.
		numStaticPixels++

		// If the pixels do not have the same RGB values, update the diff metrics
		// and the diff image.
		if r != R || g != G || b != B {
			numDiffPixels++
			dr := util.AbsInt(int(r) - int(R))
			dg := util.AbsInt(int(g) - int(G))
			db := util.AbsInt(int(b) - int(B))
			maxRGBDiffs[0] = util.MaxInt(dr, maxRGBDiffs[0])
			maxRGBDiffs[1] = util.MaxInt(dg, maxRGBDiffs[1])
			maxRGBDiffs[2] = util.MaxInt(db, maxRGBDiffs[2])
			copy(resultImg.Pix[i:], diff.PixelDiffColor[deltaOffset(dr+dg+db)])
		}
	}

	return &DynamicDiffMetrics{
		NumDiffPixels:    numDiffPixels,
		PixelDiffPercent: diff.GetPixelDiffPercent(numDiffPixels, numStaticPixels),
		MaxRGBDiffs:      maxRGBDiffs,
		NumStaticPixels:  numStaticPixels,
		NumDynamicPixels: numDynamicPixels,
	}, resultImg
}

// If the pixel is cyan, it contains dynamic content. This reflects the current
// behavior of the CT screenshot benchmark when the dynamic content detection
// flag is enabled.
func isDynamicContentPixel(red, green, blue uint8) bool {
	return red == 0 && green == 255 && blue == 255
}

// If the pixels don't have the same value, the minimum value that can be passed
// to this function is 1 and the maximum is 255*3 = 765. We must convert the
// range [1, 765] to the range [1, 7] in order to select the correct offset
// into the diff.PixelDiffColor slice. To convert a number n from range [x, y]
// to [a, b], we use the following formula:
// 			  (b - a)(n - x)
// f(n) = -------------- + a
//						y - x
func deltaOffset(n int) int {
	ret := 6*(n-1)/764 + 1
	if ret < 1 || ret > 7 {
		sklog.Fatalf("Input out of range [1, 765]: %d", n)
	}
	return ret - 1
}
