package diffstore

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"path/filepath"
	"sync"

	"cloud.google.com/go/storage"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/rtcache"
	"go.skia.org/infra/go/util"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
)

const (
	// MAX_URI_GET_TRIES is the number of tries we do to load an image.
	MAX_URI_GET_TRIES = 4

	// Number of concurrent workers downloading images.
	N_IMG_WORKERS = 10
)

// ImageLoader facilitates to continously download images and cache them in RAM.
type ImageLoader struct {
	// client is the Google storage client to local content form GS.
	storageClient *storage.Client

	// localImgDir is the local directory where images should be written to.
	localImgDir string

	// gsBucketName is the GS bucket where images are stored.
	gsBucketName string

	// gsImageBaseDir is the GS directory (prefix) where images are stored.
	gsImageBaseDir string

	// imageCache caches and calculates images.
	imageCache rtcache.ReadThroughCache

	// keep ?
	isMaster bool

	wg sync.WaitGroup
}

// Creates a new instance of ImageLoader.
func newImgLoader(client *http.Client, imgDir, gsBucketName, gsImageBaseDir string, maxCacheSize int) (*ImageLoader, error) {
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	ret := &ImageLoader{
		storageClient:  storageClient,
		localImgDir:    imgDir,
		gsBucketName:   gsBucketName,
		gsImageBaseDir: gsImageBaseDir,
	}

	// Set up the work queues that balance the load.
	ret.imageCache = rtcache.New(ret.imageLoadWorker, maxCacheSize, N_IMG_WORKERS)
	return ret, nil
}

// Warm makes sure the images are cached. It does not return a result to avoid
// deserialization and unnecessary memory allocation.
// The synchronous flag determines whether the call is blocking or not.
// It workes in sync with Get, any image that is scheduled be retrieved by get
// will not be fetched again.
func (il *ImageLoader) Warm(priority int64, digests []string) {
	il.wg.Add(len(digests))
	for _, digest := range digests {
		go func(digest string) {
			defer il.wg.Done()
			if err := il.imageCache.Warm(priority, digest); err != nil {
				glog.Errorf("Unable to retrive digest %s. Got error: %s", digest, err)
			}
		}(digest)
	}
}

// sync waits until all pending go routines have terminated.
func (il *ImageLoader) sync() {
	il.wg.Wait()
}

// Get returns the images identified by digests and returns it as an NRGBA image.
// Priority determines the order in which multiple concurrent calls are processed.
// func (il *ImageLoader) Get(priority int64, digests []string) (*image.NRGBA, error) {
// 	// img, err := il.imageCache.Get(priority, digests)
// 	// if err != nil {
// 	// 	return nil, err
// 	// }
// 	// return img.(*image.NRGBA), nil
// 	return nil, nil
// }

func (il *ImageLoader) Get(priority int64, digests []string) ([]*image.NRGBA, error) {
	// Parallel load the requested images.
	result := make([]*image.NRGBA, len(digests))
	errCh := make(chan error, len(digests))
	var wg sync.WaitGroup
	wg.Add(len(digests))
	for idx, digest := range digests {
		go func(idx int, digest string) {
			defer wg.Done()
			img, err := il.imageCache.Get(priority, digest)
			if err != nil {
				errCh <- err
			} else {
				result[idx] = img.(*image.NRGBA)
			}
		}(idx, digest)
	}
	wg.Wait()
	if len(errCh) > 0 {
		close(errCh)
		var msg bytes.Buffer
		for err := range errCh {
			_, _ = msg.WriteString(err.Error())
			_, _ = msg.WriteString("\n")
		}
		return nil, errors.New(msg.String())
	}

	return result, nil
}

// IsOnDisk returns true if the image that corresponds to the given digest is in the disk cache.
func (il *ImageLoader) IsOnDisk(digest string) bool {
	return fileutil.FileExists(fileutil.TwoLevelRadixPath(il.localImgDir, getDigestImageFileName(digest)))
}

// imageLoadWorker implements the rtcache.ReadThroughFunc signature.
// It loads an image file either from disk or from Google storage.
func (il *ImageLoader) imageLoadWorker(priority int64, digest string) (interface{}, error) {
	// Check if the image is in the disk cache.
	imageFileName := getDigestImageFileName(digest)
	imagePath := fileutil.TwoLevelRadixPath(il.localImgDir, imageFileName)
	if fileutil.FileExists(imagePath) {
		img, err := loadImg(imagePath)
		glog.Infof("Loaded img %s from disk", imagePath)
		return img, err
	}

	// Download the image
	imgBytes, err := il.downloadImg(digest)
	if err != nil {
		return nil, err
	}

	// Save the file to disk.
	il.saveImgInfoAsync(imageFileName, imgBytes)

	// Decode it and return it.
	return decodeImg(bytes.NewBuffer(imgBytes))
}

func (il *ImageLoader) saveImgInfoAsync(imageFileName string, imgBytes []byte) {
	il.wg.Add(1)
	go func() {
		defer il.wg.Done()
		if err := saveFileRadixPath(il.localImgDir, imageFileName, bytes.NewBuffer(imgBytes)); err != nil {
			glog.Error(err)
		}
	}()
}

// downloadImg retrieves the given image from Google storage.
func (il *ImageLoader) downloadImg(digest string) ([]byte, error) {
	glog.Infof("Starting download for for: %s", digest)
	objLocation := filepath.Join(il.gsImageBaseDir, getDigestImageFileName(digest))
	ctx := context.Background()

	// Retrieve the attributes.
	attrs, err := il.storageClient.Bucket(il.gsBucketName).Object(objLocation).Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve attributes for %s/%s: %s", il.gsBucketName, objLocation, err)
	}

	var buf *bytes.Buffer
	for i := 0; i < MAX_URI_GET_TRIES; i++ {
		err = func() error {
			reader, err := il.storageClient.Bucket(il.gsBucketName).Object(objLocation).NewReader(ctx)
			if err != nil {
				return fmt.Errorf("New reader failed for %s/%s: %s", il.gsBucketName, objLocation, err)
			}
			defer util.Close(reader)

			size := reader.Size()
			buf = bytes.NewBuffer(make([]byte, 0, size))
			md5Hash := md5.New()
			multiOut := io.MultiWriter(md5Hash, buf)

			if _, err = io.Copy(multiOut, reader); err != nil {
				return err
			}

			// Check the MD5.
			if !bytes.Equal(md5Hash.Sum(nil), attrs.MD5) {
				return fmt.Errorf("MD5 hash for digest %s incorrect.", digest)
			}

			return nil
		}()

		if err == nil {
			break
		}
		glog.Errorf("Error fetching file for digest %s: %s", digest, err)
	}

	if err != nil {
		glog.Errorf("Failed fetching file after %d attempts", MAX_URI_GET_TRIES)
		return nil, err
	}

	glog.Infof("Done downloading image for: %s. Length: %d", digest, buf.Len())
	return buf.Bytes(), err
}
