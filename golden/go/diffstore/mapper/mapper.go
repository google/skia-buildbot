package mapper

import (
	"image"

	"go.skia.org/infra/go/util"
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
}
