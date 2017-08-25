package mass_process

/*
   Utilities for processing many objects in Google Storage.
*/

import (
	"cloud.google.com/go/storage"
)

// Transformation is a function applied to an object to produce another object.
type Transformation func([]byte) ([]byte, error)

// Transform a single object. The object is read from Google Storage, the given
// Transformation is applied, and the resulting object is written to Google
// Storage. The path is the full path of the input object. It is expected to
// start with the given inPrefix, which is replaced by outPrefix to determine
// the path of the output object.
func TransformSingle(b *storage.BucketHandle, path, inPrefix, outPrefix string, tf Transformation) error {
	in, err := ReadObj(b.Object(path))
	if err != nil {
		return err
	}
	out, err := tf(in)
	if err != nil {
		return err
	}
	outPath := GetOutputPath(path, inPrefix, outPrefix)
	return WriteObj(b.Object(outPath), out)
}

// Transform some number of objects. The objects are read from Google Storage,
// the given Transformation is applied to each object, and the resulting objects
// are written to Google Storage. All objects in the given bucket with the given
// inPrefix are transformed and written to a path determined by replacing
// inPrefix with outPrefix in the object's path. This preserves sub-paths.
func TransformMany(b *storage.BucketHandle, inPrefix, outPrefix string, tf Transformation, max int) error {
	return ProcessMany(b, inPrefix, func(path string) error {
		return TransformSingle(b, path, inPrefix, outPrefix, tf)
	}, max)
}
