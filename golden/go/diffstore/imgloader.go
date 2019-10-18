package diffstore

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"sync"
	"time"

	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/rtcache"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore/common"
	"go.skia.org/infra/golden/go/diffstore/failurestore"
	"go.skia.org/infra/golden/go/types"
)

const (
	// maxGetTries is the number of tries we do to load an image.
	maxGetTries = 4

	// numConcurrentDownloads is the maximum number of concurrent workers downloading images.
	numConcurrentDownloads = 10
)

// ImageLoader facilitates to continuously download images and cache them in RAM.
type ImageLoader struct {
	// gsBucketClient targets the specific bucket where the images are stored in GCS.
	gsBucketClient gcs.GCSClient

	// gcsImageBaseDir is the GCS directory (prefix) where images are stored.
	gcsImageBaseDir string

	// imageCache caches and calculates images.
	imageCache rtcache.ReadThroughCache

	// failureStore persists failures in retrieving images.
	failureStore failurestore.FailureStore
}

// getGCSRelPath returns the GCS path for a given image ID (excluding the bucket).
func getGCSRelPath(imageID types.Digest) string {
	return fmt.Sprintf("%s.%s", imageID, common.IMG_EXTENSION)
}

// NewImgLoader creates a new instance of ImageLoader.
func NewImgLoader(client gcs.GCSClient, fStore failurestore.FailureStore, gcsImageBaseDir string, maxCacheSize int) (*ImageLoader, error) {
	ret := &ImageLoader{
		gsBucketClient:  client,
		gcsImageBaseDir: gcsImageBaseDir,
		failureStore:    fStore,
	}

	// Set up the work queues that balance the load.
	rtc, err := rtcache.New(func(ctx context.Context, digest string) (interface{}, error) {
		return ret.imageLoadWorker(ctx, types.Digest(digest))
	}, maxCacheSize, numConcurrentDownloads)
	if err != nil {
		return nil, err
	}
	ret.imageCache = rtc
	return ret, nil
}

// errResult is a helper type to capture error information in Get(...).
type errResult struct {
	err error
	id  types.Digest
}

// Get returns the images identified by digests and returns them as byte slices.
// Priority determines the order in which multiple concurrent calls are processed.
func (il *ImageLoader) Get(ctx context.Context, images types.DigestSlice) ([][]byte, error) {
	// Parallel load the requested images.
	// TODO(lovisolo): Rewrite using errgroup.
	result := make([][]byte, len(images))
	errCh := make(chan errResult, len(images))
	sklog.Debugf("About to Get %d images.", len(images))
	var wg sync.WaitGroup
	wg.Add(len(images))
	for idx, id := range images {
		go func(idx int, id types.Digest) {
			defer wg.Done()
			img, err := il.imageCache.Get(ctx, string(id))
			if err != nil {
				errCh <- errResult{err: err, id: id}
			} else {
				result[idx] = img.([]byte)
			}
		}(idx, id)
	}
	wg.Wait()
	sklog.Debugf("Done getting images.")

	if len(errCh) > 0 {
		close(errCh)
		var msg bytes.Buffer
		for errRet := range errCh {
			_, _ = msg.WriteString(errRet.err.Error())
			_, _ = msg.WriteString("\n")
		}
		return nil, errors.New(msg.String())
	}

	return result, nil
}

// Contains returns true if the image with the given digest is present in the in-memory cache.
func (il *ImageLoader) Contains(image types.Digest) bool {
	return il.imageCache.Contains(string(image))
}

// PurgeImages removes the images that correspond to the given digests.
func (il *ImageLoader) PurgeImages(images types.DigestSlice, purgeGCS bool) error {
	for _, id := range images {
		il.imageCache.Remove([]string{string(id)})
		if purgeGCS {
			gsRelPath := getGCSRelPath(id)
			il.removeImg(gsRelPath)
		}
	}
	return nil
}

// imageLoadWorker implements the rtcache.ReadThroughFunc signature.
// It loads an image file from Google storage.
func (il *ImageLoader) imageLoadWorker(ctx context.Context, imageID types.Digest) (interface{}, error) {
	sklog.Debugf("Downloading (and caching) image with ID %s", imageID)

	// Download the image from GCS.
	gsRelPath := getGCSRelPath(imageID)
	imgBytes, err := il.downloadImg(ctx, gsRelPath)
	if err != nil {
		util.LogErr(il.failureStore.AddDigestFailure(ctx, diff.NewDigestFailure(imageID, diff.HTTP)))
		return nil, err
	}
	return imgBytes, nil
}

// downloadImg retrieves the given image from Google storage.
func (il *ImageLoader) downloadImg(ctx context.Context, gsPath string) ([]byte, error) {
	objLocation := path.Join(il.gcsImageBaseDir, gsPath)

	// Retrieve the attributes.
	attrs, err := il.gsBucketClient.GetFileObjectAttrs(ctx, objLocation)
	if err != nil {
		return nil, skerr.Wrapf(err, "Unable to retrieve attributes for %s/%s.", il.gsBucketClient.Bucket(), objLocation)
	}

	var buf *bytes.Buffer
	for i := 0; i < maxGetTries; i++ {
		if i > 0 {
			sklog.Infof("after error, sleeping 2s before GCS fetch for gs://%s/%s", il.gsBucketClient.Bucket(), objLocation)
			// This is an arbitrary amount of time to wait.
			// TODO(kjlubick): should this be exponential backoff?
			time.Sleep(2 * time.Second)
		}
		err = func() error {
			// Create reader.
			reader, err := il.gsBucketClient.FileReader(ctx, objLocation)
			if err != nil {
				return skerr.Wrapf(err, "New reader failed for %s/%s.", il.gsBucketClient.Bucket(), objLocation)
			}
			defer util.Close(reader)

			// Read file.
			size := attrs.Size
			buf = bytes.NewBuffer(make([]byte, 0, size))
			md5Hash := md5.New()
			multiOut := io.MultiWriter(md5Hash, buf)
			if _, err = io.Copy(multiOut, reader); err != nil {
				return err
			}

			// Check the MD5.
			if hashBytes := md5Hash.Sum(nil); !bytes.Equal(hashBytes, attrs.MD5) {
				return skerr.Fmt("MD5 hash for digest %s incorrect: computed hash is %s.", objLocation, hex.EncodeToString(hashBytes))
			}

			return nil
		}()

		if err == nil {
			break
		}
		sklog.Errorf("Error fetching file for path %s: %s", objLocation, err)
	}

	if err != nil {
		sklog.Errorf("Failed fetching file after %d attempts", maxGetTries)
		return nil, err
	}

	sklog.Infof("Done downloading image for: %s. Length: %d bytes", objLocation, buf.Len())
	return buf.Bytes(), nil
}

// removeImg removes the image that corresponds to the given relative path from GCS.
func (il *ImageLoader) removeImg(gsRelPath string) {
	// If the bucket is not empty then look there otherwise use the default buckets.
	objLocation := path.Join(il.gcsImageBaseDir, gsRelPath)

	ctx := context.Background()
	// Retrieve the attributes to test if the file exists.
	_, err := il.gsBucketClient.GetFileObjectAttrs(ctx, objLocation)
	if err != nil {
		sklog.Errorf("Unable to retrieve attributes for existing object at %s. Got error: %s", objLocation, err)
		return
	}

	// Log an error and continue to the next bucket if we cannot delete the existing file.
	if err := il.gsBucketClient.DeleteFile(ctx, objLocation); err != nil {
		sklog.Errorf("Unable to delete existing object at %s. Got error: %s", objLocation, err)
	}
}
