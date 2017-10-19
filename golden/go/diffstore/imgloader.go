package diffstore

import (
	"bytes"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/rtcache"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
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
	// storageClient is the Google client to local content from GCS.
	storageClient *storage.Client

	// localImgDir is the local directory where images should be written to.
	localImgDir string

	// gsBucketNames is the list of GCS bucket where images are stored.
	gsBucketNames []string

	// gsImageBaseDir is the GCS directory (prefix) where images are stored.
	gsImageBaseDir string

	// imageCache caches and calculates images.
	imageCache rtcache.ReadThroughCache

	// failureStore persists failures in retrieving images.
	failureStore *failureStore

	wg sync.WaitGroup

	// mapper contains various functions for creating image IDs and paths.
	mapper DiffStoreMapper
}

// Creates a new instance of ImageLoader.
func NewImgLoader(client *http.Client, baseDir, imgDir string, gsBucketNames []string, gsImageBaseDir string, maxCacheSize int, mapper DiffStoreMapper) (*ImageLoader, error) {
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	fStore, err := newFailureStore(filepath.Join(baseDir, FAILUREDB_NAME))
	if err != nil {
		return nil, err
	}

	ret := &ImageLoader{
		storageClient:  storageClient,
		localImgDir:    imgDir,
		gsBucketNames:  gsBucketNames,
		gsImageBaseDir: gsImageBaseDir,
		failureStore:   fStore,
		mapper:         mapper,
	}

	// Set up the work queues that balance the load.
	if ret.imageCache, err = rtcache.New(ret.imageLoadWorker, maxCacheSize, N_IMG_WORKERS); err != nil {
		return nil, err
	}
	return ret, nil
}

// Warm makes sure the images are cached. It does not return a result to avoid
// deserialization and unnecessary memory allocation.
// The synchronous flag determines whether the call is blocking or not.
// It workes in sync with Get, any image that is scheduled be retrieved by get
// will not be fetched again.
func (il *ImageLoader) Warm(priority int64, images []string, synchronous bool) {
	var localWg sync.WaitGroup
	il.wg.Add(len(images))
	localWg.Add(len(images))
	for _, id := range images {
		go func(id string) {
			defer il.wg.Done()
			defer localWg.Done()
			if err := il.imageCache.Warm(priority, id); err != nil {
				sklog.Errorf("Unable to retrive image %s. Got error: %s", id, err)
			}
		}(id)
	}

	// Wait for the all images to be loaded.
	if synchronous {
		localWg.Wait()
	}
}

// sync waits until all pending go routines have terminated.
func (il *ImageLoader) Sync() {
	il.wg.Wait()
}

// errResult is a helper type to capture error information in Get(...).
type errResult struct {
	err error
	id  string
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

func (il *ImageLoader) Get(priority int64, images []string) ([]*image.NRGBA, error) {
	// Parallel load the requested images.
	result := make([]*image.NRGBA, len(images))
	errCh := make(chan errResult, len(images))
	var wg sync.WaitGroup
	wg.Add(len(images))
	for idx, id := range images {
		go func(idx int, id string) {
			defer wg.Done()
			img, err := il.imageCache.Get(priority, id)
			if err != nil {
				errCh <- errResult{err: err, id: id}
			} else {
				result[idx] = img.(*image.NRGBA)
			}
		}(idx, id)
	}
	wg.Wait()
	if len(errCh) > 0 {
		close(errCh)
		var msg bytes.Buffer
		for errRet := range errCh {
			_, _ = msg.WriteString(errRet.err.Error())
			_, _ = msg.WriteString("\n")

			// This captures the edge case when the error is cached in the image loader.
			util.LogErr(il.failureStore.addDigestFailureIfNew(diff.NewDigestFailure(errRet.id, diff.OTHER)))
		}
		return nil, errors.New(msg.String())
	}

	return result, nil
}

// IsOnDisk returns true if the image that corresponds to the given imageID is in the disk cache.
func (il *ImageLoader) IsOnDisk(imageID string) bool {
	localRelPath, _, _ := il.mapper.ImagePaths(imageID)
	return fileutil.FileExists(filepath.Join(il.localImgDir, localRelPath))
}

// PurgeImages removes the images that correspond to the given images.
func (il *ImageLoader) PurgeImages(images []string, purgeGCS bool) error {
	for _, id := range images {
		localRelPath, bucket, gsRelPath := il.mapper.ImagePaths(id)
		localPath := filepath.Join(il.localImgDir, localRelPath)
		if fileutil.FileExists(localPath) {
			if err := os.Remove(localPath); err != nil {
				sklog.Errorf("Unable to remove image %s. Got error: %s", localPath, err)
			}
		}

		if purgeGCS {
			il.removeImg(bucket, gsRelPath)
		}
	}
	return nil
}

// imageLoadWorker implements the rtcache.ReadThroughFunc signature.
// It loads an image file either from disk or from Google storage.
func (il *ImageLoader) imageLoadWorker(priority int64, imageID string) (interface{}, error) {
	// Check if the image is in the disk cache.
	localRelPath, bucket, gsRelPath := il.mapper.ImagePaths(imageID)
	localPath := filepath.Join(il.localImgDir, localRelPath)
	if fileutil.FileExists(localPath) {
		img, err := loadImg(localPath)
		if err != nil {
			util.LogErr(il.failureStore.addDigestFailure(diff.NewDigestFailure(imageID, diff.CORRUPTED)))
			return nil, err
		}
		util.LogErr(il.failureStore.purgeDigestFailures([]string{imageID}))
		return img, err
	}

	// Download the image
	imgBytes, err := il.downloadImg(bucket, gsRelPath)
	if err != nil {
		util.LogErr(il.failureStore.addDigestFailure(diff.NewDigestFailure(imageID, diff.HTTP)))
		return nil, err
	}

	// Decode it and return it.
	img, err := decodeImg(bytes.NewBuffer(imgBytes))
	if err != nil {
		util.LogErr(il.failureStore.addDigestFailure(diff.NewDigestFailure(imageID, diff.CORRUPTED)))
		return nil, err
	}

	// Save the file to disk.
	il.saveImgInfoAsync(imageID, imgBytes)
	return img, nil
}

func (il *ImageLoader) saveImgInfoAsync(imageID string, imgBytes []byte) {
	il.wg.Add(1)
	go func() {
		defer il.wg.Done()
		localRelPath, _, _ := il.mapper.ImagePaths(imageID)
		if err := saveFilePath(filepath.Join(il.localImgDir, localRelPath), bytes.NewBuffer(imgBytes)); err != nil {
			sklog.Error(err)
		}
	}()
}

// downloadImg retrieves the given image from Google storage. If bucket is not empty
// the gsPath is considered absolute within the bucket, otherwise it is relative to gsImageBaseDir.
func (il *ImageLoader) downloadImg(bucket, gsPath string) ([]byte, error) {
	var bucketNames []string
	var objLocation string
	if bucket != "" {
		objLocation = gsPath
		bucketNames = []string{bucket}
	} else {
		objLocation = filepath.Join(il.gsImageBaseDir, gsPath)
		bucketNames = il.gsBucketNames
	}

	var err error
	var imgData []byte
	for _, bucketName := range bucketNames {
		imgData, err = il.downloadImgFromBucket(objLocation, bucketName)
		if err == nil {
			return imgData, nil
		}
	}
	return nil, fmt.Errorf("Failed finding image %s in buckets %v. Last error: %s", gsPath, bucketNames, err)
}

// downloadImgFromBucket retrieves the given image from the given Google storage bucket.
// It returns storage.ErrObjectNotExist if the given image does not exist in the bucket.
func (il *ImageLoader) downloadImgFromBucket(objLocation, bucketName string) ([]byte, error) {
	ctx := context.Background()

	// Retrieve the attributes.
	attrs, err := il.storageClient.Bucket(bucketName).Object(objLocation).Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve attributes for %s/%s: %.80s", bucketName, objLocation, err)
	}

	var buf *bytes.Buffer
	for i := 0; i < MAX_URI_GET_TRIES; i++ {
		err = func() error {
			reader, err := il.storageClient.Bucket(bucketName).Object(objLocation).NewReader(ctx)
			if err != nil {
				return fmt.Errorf("New reader failed for %s/%s: %.80s", bucketName, objLocation, err)
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
				return fmt.Errorf("MD5 hash for digest %s incorrect.", objLocation)
			}

			return nil
		}()

		if err == nil {
			break
		}
		sklog.Errorf("Error fetching file for path %s: %s", objLocation, err)
	}

	if err != nil {
		sklog.Errorf("Failed fetching file after %d attempts", MAX_URI_GET_TRIES)
		return nil, err
	}

	sklog.Infof("Done downloading image for: %s. Length: %d", objLocation, buf.Len())
	return buf.Bytes(), err
}

// removeImg removes the image that corresponds to the given relative path from GCS.
func (il *ImageLoader) removeImg(bucket, gsRelPath string) {
	// If the bucket is not empty then look there otherwise use the default buckets.
	objLocation := filepath.Join(il.gsImageBaseDir, gsRelPath)
	bucketNames := il.gsBucketNames
	if bucket != "" {
		bucketNames = []string{bucket}
	}

	ctx := context.Background()
	for _, bucketName := range bucketNames {
		// Retrieve the attributes to test if the file exists.
		_, err := il.storageClient.Bucket(bucketName).Object(objLocation).Attrs(ctx)
		if err != nil {
			// We ignore the error because it most likely indicates that the requested object
			// does not exist. Currently the Attrs(...) call does not return ErrObjectNotExist
			// as documented.
			continue
		}

		// Log an error and continue to the next bucket if we cannot delete the existing file.
		if err := il.storageClient.Bucket(bucketName).Object(objLocation).Delete(ctx); err != nil {
			sklog.Errorf("Unable to delete existing object at %s. Got error: %s", objLocation, err)
			continue
		}
	}
}
