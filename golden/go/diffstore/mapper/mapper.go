package mapper

import (
	"image"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

// Mapper is the interface to define how the diff metric between two images
// is calculated.
type Mapper interface {
	// LRUCodec defines the Encode and Decode functions to serialize/deserialize
	// instances of the diff metrics returned by the DiffFn function below.
	util.LRUCodec

	// DiffFn computes and returns the diff metrics between two given images.
	// The type underlying interface{} is the input and output of the LRUCodec
	// above. It is also what is returned by the Get(...) function of the
	// DiffStore interface.
	DiffFn(*image.NRGBA, *image.NRGBA) interface{}

	// ImagePaths returns the storage paths for a given image ID. The first return
	// value is the local file path used to store the image on disk and serve it
	// over HTTP. The second return value is the GCS path (not including the bucket).
	// TODO(kjlubick): It might be nice to have Mapper just focus on the
	// diff metric and have a different interface for the disk storing.
	ImagePaths(id types.Digest) (string, string)
}
