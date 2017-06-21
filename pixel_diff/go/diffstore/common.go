package diffstore

import (
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/boltdb/bolt"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
)

const (
	// IMG_EXTENSION is the default extension of images.
	IMG_EXTENSION = "png"
)

// saveFile writes the given file to disk.
func saveFile(targetPath string, r io.Reader) error {
	f, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer util.Close(f)

	_, err = io.Copy(f, r)
	return err
}

// saveFileRadixPath saves the given buffer in a
// radix path where two directory levels are inserted based on the first four
// characters of the filename. i.e. abcefghijk.png -> ./ab/ce/abcefghijk.png.
func saveFileRadixPath(baseDir, fileName string, r io.Reader) error {
	targetPath, err := createRadixPath(baseDir, fileName)
	if err != nil {
		return fmt.Errorf("Unable to create radix path for %s/%s: %s", baseDir, fileName, err)
	}

	if err := saveFile(targetPath, r); err != nil {
		return fmt.Errorf("Unable to save file %s. Got error: %s", targetPath, err)
	}
	return nil
}

// loadImg loads an image from disk.
func loadImg(sourcePath string) (*image.NRGBA, error) {
	f, err := os.Open(sourcePath)
	if err != nil {
		return nil, err
	}
	defer util.Close(f)
	return decodeImg(f)
}

// encodeImg encodes the given image as a PNG and writes the result to the
// given writer.
func encodeImg(w io.Writer, img *image.NRGBA) error {
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(w, img); err != nil {
		return err
	}
	return nil
}

// decodeImg decodes an image from the given reader and returns it as a NRGBA image.
func decodeImg(reader io.Reader) (*image.NRGBA, error) {
	im, err := png.Decode(reader)
	if err != nil {
		return nil, err
	}
	return diff.GetNRGBA(im), nil
}

// getDigestImageFileName returns the image name based on the digest.
func getDigestImageFileName(digest string) string {
	return fmt.Sprintf("%s.%s", digest, IMG_EXTENSION)
}

// createRadixPath makes sure radix path exists.
func createRadixPath(baseDir, fileName string) (string, error) {
	targetPath := fileutil.TwoLevelRadixPath(baseDir, fileName)
	radixDir, _ := filepath.Split(targetPath)
	if err := os.MkdirAll(radixDir, 0700); err != nil {
		return "", err
	}

	return targetPath, nil
}

// openBoltDB opens a boltDB in the given given directory with the given name.
func openBoltDB(baseDir, name string) (*bolt.DB, error) {
	baseDir, err := fileutil.EnsureDirExists(baseDir)
	if err != nil {
		return nil, err
	}

	return bolt.Open(path.Join(baseDir, name), 0600, nil)
}
