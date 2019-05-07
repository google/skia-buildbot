package diffstore

import (
	"fmt"
	"image"
	"strings"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/validation"
)

const (
	// DIFF_IMG_SEPARATOR is the character that separates two image ids in the
	// resulting diff image.
	DIFF_IMG_SEPARATOR = "-"
)

// GoldIDPathMapper implements the DiffStoreMapper interface. It translates
// between digests (image ids) and storage paths. It uses diff.DiffMetrics
// as the Gold diff metric.
type GoldDiffStoreMapper struct {
	util.LRUCodec
}

// NewGoldDiffStoreMapper returns a new instance of GoldDiffStoreMapper that uses
// a JSON coded to serialize/deserialize instances of diff.DiffMetrics.
func NewGoldDiffStoreMapper(diffInstance interface{}) DiffStoreMapper {
	return GoldDiffStoreMapper{LRUCodec: util.JSONCodec(diffInstance)}
}

// DiffFn implements the DiffStoreMapper interface.
func (g GoldDiffStoreMapper) DiffFn(leftImg *image.NRGBA, rightImg *image.NRGBA) (interface{}, *image.NRGBA) {
	return diff.DefaultDiffFn(leftImg, rightImg)
}

// DiffID implements the DiffStoreMapper interface.
func (g GoldDiffStoreMapper) DiffID(leftImgID, rightImgID string) string {
	_, _, diffID := g.getOrderedDiffID(leftImgID, rightImgID)
	return diffID
}

// SplitDiffID implements the DiffStoreMapper interface.
func (g GoldDiffStoreMapper) SplitDiffID(diffID string) (string, string) {
	imageIDs := strings.Split(diffID, DIFF_IMG_SEPARATOR)

	// TODO(stephana): Remove this legacy handling code as soon as it has converted the
	// database in production.
	if strings.Contains(diffID, ":") {
		imageIDs = strings.Split(diffID, ":")
	}

	return imageIDs[0], imageIDs[1]
}

// SplitDiffID implements the DiffStoreMapper interface.
func (g GoldDiffStoreMapper) DiffPath(leftImgID, rightImgID string) string {
	// Get the diff ID and the left imageID.
	leftImgID, _, diffID := g.getOrderedDiffID(leftImgID, rightImgID)
	imagePath := fmt.Sprintf("%s.%s", diffID, IMG_EXTENSION)

	return fileutil.TwoLevelRadixPath(imagePath)
}

// ImagePaths implements the DiffStoreMapper interface.
func (g GoldDiffStoreMapper) ImagePaths(imageID string) (string, string, string) {
	gsPath := fmt.Sprintf("%s.%s", imageID, IMG_EXTENSION)
	localPath := fileutil.TwoLevelRadixPath(gsPath)
	return localPath, "", gsPath
}

// IsValidDiffImgIDimplements the DiffStoreMapper interface.
func (g GoldDiffStoreMapper) IsValidDiffImgID(diffImgID string) bool {
	imageIDs := strings.Split(diffImgID, DIFF_IMG_SEPARATOR)
	if len(imageIDs) != 2 {
		return false
	}
	return g.IsValidImgID(imageIDs[0]) && g.IsValidImgID(imageIDs[1])
}

// IsValidImgIDimplements the DiffStoreMapper interface.
func (g GoldDiffStoreMapper) IsValidImgID(imgID string) bool {
	return validation.IsValidDigest(imgID)
}

func (g GoldDiffStoreMapper) getOrderedDiffID(leftImgID, rightImgID string) (string, string, string) {
	if rightImgID < leftImgID {
		// Make sure the smaller digest is left imageID.
		leftImgID, rightImgID = rightImgID, leftImgID
	}
	return leftImgID, rightImgID, leftImgID + DIFF_IMG_SEPARATOR + rightImgID
}
