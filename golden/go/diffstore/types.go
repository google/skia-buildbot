package diffstore

import (
	"image"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

// DiffStoreMapper is the interface to customize the specific behavior of MemDiffStore.
// It defines what diff metric is calculated and how to translate image ids and diff
// ids into paths on the file system and in GCS.
type DiffStoreMapper interface {
	// LRUCodec defines the Encode and Decode functions to serialize/deserialize
	// instances of the diff metrics returned by the DiffFn function below.
	util.LRUCodec

	// DiffFn calculates the different between two given images and returns a
	// difference image. The type underlying interface{} is the input and output
	// of the LRUCodec above. It is also what is returned by the Get(...) function
	// of the DiffStore interface.
	DiffFn(*image.NRGBA, *image.NRGBA) (interface{}, *image.NRGBA)

	// Takes two image IDs and returns a unique diff ID.
	// Note: DiffID(a,b) == DiffID(b, a) should hold.
	DiffID(leftImgID, rightImgID types.Digest) string

	// Inverse function of DiffID.
	// SplitDiffID(DiffID(a,b)) should return (a,b) or (b,a).
	SplitDiffID(diffID string) (types.Digest, types.Digest)

	// DiffPath returns the local file path for the diff image of two images.
	// This path is used to store the diff image on disk and serve it over HTTP.
	DiffPath(leftImgID, rightImgID types.Digest) string

	// ImagePaths returns the storage paths for a given image ID. The first return
	// value is the local file path used to store the image on disk and serve it
	// over HTTP. The second return value is the storage bucket and the third the
	// path within that bucket.
	ImagePaths(imageID types.Digest) (string, string, string)

	// IsValidDiffImgID returns true if the given diffImgID is in the correct format.
	IsValidDiffImgID(diffImgID string) bool

	// IsValidImgID returns true if the given imgID is in the correct format.
	IsValidImgID(imgID string) bool
}
