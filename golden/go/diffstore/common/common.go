package common

import (
	"image"
	"image/png"
	"io"
	"path"
	"strings"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/types"
)

const (
	// IMG_EXTENSION is the default extension of images.
	IMG_EXTENSION = "png"

	// DiffImageSeparator is the character that separates two image ids in the
	// resulting diff image.
	DiffImageSeparator = "-"
)

// EncodeImg encodes the given image as a PNG and writes the result to the
// given writer.
func EncodeImg(w io.Writer, img *image.NRGBA) error {
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(w, img); err != nil {
		return err
	}
	return nil
}

// DecodeImg decodes an image from the given reader and returns it as a NRGBA image.
func DecodeImg(reader io.Reader) (*image.NRGBA, error) {
	im, err := png.Decode(reader)
	if err != nil {
		return nil, err
	}
	return diff.GetNRGBA(im), nil
}

// OpenBoltDB opens a boltDB in the given given directory with the given name.
func OpenBoltDB(baseDir, name string) (*bolt.DB, error) {
	baseDir, err := fileutil.EnsureDirExists(baseDir)
	if err != nil {
		return nil, err
	}

	return bolt.Open(path.Join(baseDir, name), 0600, nil)
}

func AsStrings(xd types.DigestSlice) []string {
	s := make([]string, 0, len(xd))
	for _, d := range xd {
		s = append(s, string(d))
	}
	return s
}

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

func getOrderedDiffID(left, right types.Digest) (types.Digest, types.Digest, string) {
	if right < left {
		// Make sure the smaller digest is left imageID.
		left, right = right, left
	}
	return left, right, string(left) + DiffImageSeparator + string(right)
}
