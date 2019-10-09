package mapper

import (
	"image"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
)

// Mapper is the interface to define how the diff metric between two images
// is calculated.
type Mapper interface {
	// Codec defines the Encode and Decode functions to serialize/deserialize
	// instances of the diff metrics returned by the DiffFn function below.
	util.Codec

	// DiffFn computes and returns the diff metrics between two given images.
	DiffFn(*image.NRGBA, *image.NRGBA) *diff.DiffMetrics
}
