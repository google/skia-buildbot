package diffstore

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/redisutil"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
)

const (
	// IMG_EXTENSION is the default extension of images.
	IMG_EXTENSION = "png"
)

const (
	// R_FILEINFO_TMPL is the template for the key to a specific file that is
	// temporarily cached in Redis.
	R_FILEINFO_TMPL = "fds:file_info:%s"
)

// fileData stores the name of content of a file. The file type (i.e. image)
// should be clear from the context.
type fileData struct {
	Name    string `redis:"name"`
	Content []byte `redis:"content"`
}

// startFileWriter starts a background process that continously writes files
// from Redis to disk.
func startFileWriter(redisPool *redisutil.RedisPool, listName, targetDir string) {
	go func() {
		ch := redisPool.List(listName)
		for {
			hashKey := <-ch
			go func(hashKey string) {
				var fd fileData
				found, err := redisPool.LoadHashToStruct(hashKey, &fd)
				if err != nil {
					glog.Errorf("Error retrieving file info: %s", err)
					return
				}

				if !found {
					glog.Errorf("Unable to retrieve file info for: %s", hashKey)
					return
				}

				targetPath := filepath.Join(targetDir, fd.Name)
				if err := saveFile(targetPath, bytes.NewBuffer(fd.Content)); err != nil {
					glog.Errorf("Error writing file %s: %s", targetPath, err)
					return
				}

				if err := redisPool.DeleteKey(hashKey); err != nil {
					glog.Errorf("Error deleting key: %s", err)
				}
			}(string(hashKey))
		}
	}()
}

// saveFile writes the given file to disk.
func saveFile(targetPath string, buf *bytes.Buffer) error {
	f, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer util.Close(f)

	_, err = io.Copy(f, buf)
	return err
}

// saveFileAsyncRadixPath saves the given buffer asynchronously in a
// radix path where two directory levels are inserted based on the first four
// characters of the filename. i.e. abcefghijk.png -> ./ab/ce/abcefghijk.png.
func saveFileAsyncRadixPath(baseDir, fileName string, buf *bytes.Buffer) {
	go func() {
		targetPath, err := createRadixPath(baseDir, fileName)
		if err != nil {
			glog.Errorf("Unable to create radix path for %s/%s: %s", baseDir, fileName, err)
			return
		}

		if err := saveFile(targetPath, buf); err != nil {
			glog.Errorf("Unable to save file %s. Got error: %s", targetPath, err)
		}
	}()
}

// loadImg loads an image from disk.
func loadImg(sourcePath string) (*image.NRGBA, error) {
	glog.Infof("Loaded img %s from disk", sourcePath)
	f, err := os.Open(sourcePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return decodeImg(f)
}

// decodeImg decodes an image from the given reader and returns it as a NRGBA image.
func decodeImg(reader io.Reader) (*image.NRGBA, error) {
	im, err := png.Decode(reader)
	if err != nil {
		return nil, err
	}
	return diff.GetNRGBA(im), nil
}

// enqueueFileInfo adds the given file into a Redis queue so that it can
// be written to disk asynchronously.
func enqueueFileInfo(redisPool *redisutil.RedisPool, listKey string, fData *fileData) {
	go func() {
		hashKey := keyFileInfo(fData.Name)
		if err := redisPool.SaveHash(hashKey, fData); err != nil {
			glog.Errorf("Unable to store file info: %s", err)
			return
		}

		if err := redisPool.AppendList(listKey, []byte(hashKey)); err != nil {
			glog.Errorf("Unable add file info hash to list: %s", err)
		}
	}()
}

// keyFileInfo returns the redis key for the given filename.
func keyFileInfo(fname string) string {
	return fmt.Sprintf(R_FILEINFO_TMPL, fname)
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
