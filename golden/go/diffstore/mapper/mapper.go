package mapper

import (
	"image"
	"strings"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/validation"
)

// Mapper is the interface to define how the diff metric between two images
// is calculated and defines how those diffs are stored on disk.
// TODO(kjlubick): It might be nice to have Mapper just focus on the
// diff metric and have a different interface for the disk storing.
// Or make the interface just serve the bytes (if possible).
type Mapper interface {
	// LRUCodec defines the Encode and Decode functions to serialize/deserialize
	// instances of the diff metrics returned by the DiffFn function below.
	util.LRUCodec

	// DiffFn calculates the different between two given images and returns a
	// difference image. The type underlying interface{} is the input and output
	// of the LRUCodec above. It is also what is returned by the Get(...) function
	// of the DiffStore interface.
	DiffFn(*image.NRGBA, *image.NRGBA) (interface{}, *image.NRGBA)

	// DiffPath returns the local file path for the diff image of two images.
	// This path is used to store the diff image on disk and serve it over HTTP.
	DiffPath(left, right types.Digest) string

	// ImagePaths returns the storage paths for a given image ID. The first return
	// value is the local file path used to store the image on disk and serve it
	// over HTTP. The second return value is the GCS path (not including the bucket).
	ImagePaths(id types.Digest) (string, string)
}

const (
	// DiffImageSeparator is the character that separates two image ids in the
	// resulting diff image.
	DiffImageSeparator = "-"
)

// Takes two image IDs and returns a unique diff ID.
// Note: DiffID(a,b) == DiffID(b, a) holds.
func DiffID(left, right types.Digest) string {
	_, _, diffID := getOrderedDiffID(left, right)
	return diffID
}

// Inverse function of DiffID.
// SplitDiffID(DiffID(a,b)) deterministically returns (a,b) or (b,a).
func SplitDiffID(diffID string) (types.Digest, types.Digest) {
	imageIDs := strings.Split(diffID, DiffImageSeparator)

	return types.Digest(imageIDs[0]), types.Digest(imageIDs[1])
}

// IsValidDiffImgID returns true if the given diffImgID is in the correct format.
func IsValidDiffImgID(diffID string) bool {
	imageIDs := strings.Split(diffID, DiffImageSeparator)
	if len(imageIDs) != 2 {
		return false
	}
	return IsValidImgID(imageIDs[0]) && IsValidImgID(imageIDs[1])
}

// IsValidImgID returns true if the given imgID is in the correct format.
func IsValidImgID(imgID string) bool {
	return validation.IsValidDigest(imgID)
}

func getOrderedDiffID(left, right types.Digest) (types.Digest, types.Digest, string) {
	if right < left {
		// Make sure the smaller digest is left imageID.
		left, right = right, left
	}
	return left, right, string(left) + DiffImageSeparator + string(right)
}
