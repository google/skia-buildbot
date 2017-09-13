package diffstore

import (
	"fmt"
	"image"
	"path/filepath"
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

	// PATH_SEG_DELIMITER is the delimiter used to replace the forward slashes
	// in GCS paths to make them URL friendly.
	PATH_SEG_DELIMITER = "_"
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
	return imageIDs[0], imageIDs[1]
}

// SplitDiffID implements the DiffStoreMapper interface.
func (g GoldDiffStoreMapper) DiffPath(leftImgID, rightImgID string) string {
	// Get the diff ID and the left imageID.
	leftImgID, _, diffID := g.getOrderedDiffID(leftImgID, rightImgID)
	imagePath := fmt.Sprintf("%s.%s", diffID, IMG_EXTENSION)

	// For gs images generate a path that corresponds to the GS location.
	if strings.HasPrefix(leftImgID, GS_PREFIX) {
		dir := LocalDirFromGCSImageID(leftImgID)
		return dir + "/" + imagePath
	}
	return fileutil.TwoLevelRadixPath(imagePath)
}

// ImagePaths implements the DiffStoreMapper interface.
func (g GoldDiffStoreMapper) ImagePaths(imageID string) (string, string, string) {
	if strings.HasPrefix(imageID, GS_PREFIX) {
		bucket, gsPath := ImageIDToGCSPath(imageID)
		localPath := GS_PREFIX + "/" + gsPath
		return localPath, bucket, gsPath
	}

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
	if strings.HasPrefix(imgID, GS_PREFIX) {
		return ValidGCSImageID(imgID)
	}
	return validation.IsValidDigest(imgID)
}

func (g GoldDiffStoreMapper) getOrderedDiffID(leftImgID, rightImgID string) (string, string, string) {
	leftGS := strings.HasPrefix(leftImgID, GS_PREFIX)
	rightGS := strings.HasPrefix(rightImgID, GS_PREFIX)

	if leftGS || rightGS {
		// Make sure the first imageID is the smaller GS location.
		if !leftGS || (rightGS && (rightImgID < leftImgID)) {
			leftImgID, rightImgID = rightImgID, leftImgID
		}
	} else if rightImgID < leftImgID {
		// Make sure the smaller digest is left imageID.
		leftImgID, rightImgID = rightImgID, leftImgID
	}
	return leftImgID, rightImgID, leftImgID + DIFF_IMG_SEPARATOR + rightImgID
}

// GCSPathToImageID returns a URL compatible encoding of the given location in
// GCS. It is the inverse of ImageIDToGCSPath.
func GCSPathToImageID(bucket, path string) string {
	path = strings.TrimSuffix(strings.Trim(path, "/"), "."+IMG_EXTENSION)
	return GS_PREFIX + PATH_SEG_DELIMITER + bucket + PATH_SEG_DELIMITER + strings.Replace(path, "/", PATH_SEG_DELIMITER, -1)
}

func ImageIDToGCSPath(imageID string) (string, string) {
	imageID = strings.TrimPrefix(imageID, GS_PREFIX)
	bucketAndPath := strings.SplitN(imageID, "-", 2)
	return bucketAndPath[0], strings.Replace(bucketAndPath[1], "-", "/", -1) + "." + IMG_EXTENSION
}

func ValidGCSImageID(imageID string) bool {
	parts := strings.SplitN(imageID, "-", 3)
	return (len(parts) == 0) &&
		strings.HasPrefix(parts[0], GS_PREFIX) &&
		(len(parts[1]) != 0) &&
		(len(parts[2]) != 0)
}

func LocalDirFromGCSImageID(imageID string) string {
	bucket, path := ImageIDToGCSPath(imageID)
	dir, _ := filepath.Split(path)
	return strings.Join([]string{GS_PREFIX, bucket, dir}, "/")
}
