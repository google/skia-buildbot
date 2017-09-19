package diffstore

import (
	"fmt"
	"image"
	"math/big"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mattheath/base62"
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
		localPath := GS_PREFIX + "/" + bucket + "/" + gsPath
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
	fullPath := bucket + "/" + strings.TrimSuffix(strings.Trim(path, "/"), "."+IMG_EXTENSION)
	return GS_PREFIX + encodeBase62(fullPath)
}

func ImageIDToGCSPath(imageID string) (string, string) {
	imageID = decodeBase62(strings.TrimPrefix(imageID, GS_PREFIX))
	bucketAndPath := strings.SplitN(imageID, "/", 2)
	return bucketAndPath[0], bucketAndPath[1] + "." + IMG_EXTENSION
}

// base62Alphabet is used to verify that a base62 encoded ID only
// contains the valid characters.
var base62Alphabet = regexp.MustCompile("[a-zA-Z0-9]*")

// ValidGCSImageID
func ValidGCSImageID(imageID string) bool {
	if !strings.HasPrefix(imageID, GS_PREFIX) || !base62Alphabet.Match([]byte(imageID[len(GS_PREFIX):])) {
		return false
	}
	// Make sure the bucke and path are non-empty.
	bucket, path := ImageIDToGCSPath(imageID)
	return (bucket != "") && (path != ("." + IMG_EXTENSION))
}

func LocalDirFromGCSImageID(imageID string) string {
	bucket, path := ImageIDToGCSPath(imageID)
	dir, _ := filepath.Split(path)
	return strings.Join([]string{GS_PREFIX, bucket, strings.TrimSuffix(dir, "/")}, "/")
}

//noPad62Encoding allows to base62 encode big.Int's without adding padding.
var noPad62Encoding = base62.NewStdEncoding().Option(base62.Padding(0))

// encodeBase62 encodes the given string to base62 encoding which only contains
// characters and numbers and is URL safe. No padding is added.
func encodeBase62(clearText string) string {
	var bigInt big.Int
	return noPad62Encoding.EncodeBigInt(bigInt.SetBytes([]byte(clearText)))
}

// decodeBase62 decodes the given base62 encoded string without padding.
func decodeBase62(encStr string) string {
	return string(noPad62Encoding.DecodeToBigInt(encStr).Bytes())
}
