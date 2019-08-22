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
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/rtcache"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore/common"
	"go.skia.org/infra/golden/go/diffstore/failurestore"
	"go.skia.org/infra/golden/go/diffstore/failurestore/bolt_failurestore"
	"go.skia.org/infra/golden/go/diffstore/mapper"
	"go.skia.org/infra/golden/go/types"
	"google.golang.org/api/option"
)

const (
	// MAX_URI_GET_TRIES is the number of tries we do to load an image.
	MAX_URI_GET_TRIES = 4

	// Number of concurrent workers downloading images.
	N_IMG_WORKERS = 10
)

// ImageLoader facilitates to continuously download images and cache them in RAM.
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
	failureStore failurestore.FailureStore

	// mapper contains various functions for creating image IDs and paths.
	mapper mapper.Mapper
}

// ImagePaths returns the storage paths for a given image ID. The first return
// value is the local file path used to store the image on disk and serve it
// over HTTP. The second return value is the GCS path (not including the bucket).
func ImagePaths(imageID types.Digest) (string, string) {
	gsPath := fmt.Sprintf("%s.%s", imageID, common.IMG_EXTENSION)
	localPath := fileutil.TwoLevelRadixPath(gsPath)
	return localPath, gsPath
}

// Creates a new instance of ImageLoader.
func NewImgLoader(client *http.Client, baseDir, imgDir string, gsBucketNames []string, gsImageBaseDir string, maxCacheSize int, m mapper.Mapper) (*ImageLoader, error) {
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	fStore, err := bolt_failurestore.New(baseDir)
	if err != nil {
		return nil, err
	}
	sklog.Infof("failure store created at %s", baseDir)

	ret := &ImageLoader{
		storageClient:  storageClient,
		localImgDir:    imgDir,
		gsBucketNames:  gsBucketNames,
		gsImageBaseDir: gsImageBaseDir,
		failureStore:   fStore,
		mapper:         m,
	}

	// Set up the work queues that balance the load.
	if ret.imageCache, err = rtcache.New(func(priority int64, digest string) (interface{}, error) {
		return ret.imageLoadWorker(priority, types.Digest(digest))
	}, maxCacheSize, N_IMG_WORKERS); err != nil {
		return nil, err
	}
	return ret, nil
}

// Warm makes sure the images are cached.
// If synchronous is true the call blocks until all fetched images are written to disk.
// It works in sync with Get, any image that is scheduled to be retrieved by Get
// will not be fetched again.
func (il *ImageLoader) Warm(priority int64, images types.DigestSlice, synchronous bool) {
	var pendingWritesWG *sync.WaitGroup
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		_, pendingWritesWG, err = il.Get(priority, images)
		if err != nil {
			sklog.Errorf("Unable to warm images. Got error: %s", err)
		}
	}()

	if synchronous {
		// Wait for the get to finish and for all images to be written to disk.
		wg.Wait()
		pendingWritesWG.Wait()
	}
}

// errResult is a helper type to capture error information in Get(...).
type errResult struct {
	err error
	id  types.Digest
}

// Get returns the images identified by digests and returns them as NRGBA images.
// Priority determines the order in which multiple concurrent calls are processed.
// The returned instance of WaitGroup can be used to wait until all images are
// not just loaded but also written to disk. Calling the Wait() function of the
// WaitGroup is optional and the client should not call any of its other functions.
func (il *ImageLoader) Get(priority int64, images types.DigestSlice) ([]*image.NRGBA, *sync.WaitGroup, error) {
	// Parallel load the requested images.
	result := make([]*image.NRGBA, len(images))
	imgWrappers := make(imgRetSlice, len(images))
	errCh := make(chan errResult, len(images))
	sklog.Debugf("About to Get %d images", len(images))
	var wg sync.WaitGroup
	wg.Add(len(images))
	for idx, id := range images {
		go func(idx int, id types.Digest) {
			defer wg.Done()
			tmp, err := il.imageCache.Get(priority, string(id))
			if err != nil {
				errCh <- errResult{err: err, id: id}
			} else {
				// Extract the image and make sure after the first retrieval the channels
				// are removed as well.
				ret := tmp.(*imgRet)
				result[idx] = ret.img
				imgWrappers[idx] = ret
			}
		}(idx, id)
	}
	wg.Wait()
	sklog.Debugf("Done getting images, might need some cleanup")

	// Cleanup the open channels in the background and extract the wait group to
	// return. Note: This is advised even if there are errors since it deallocates
	// the channels used to signal completion of writes.
	pendingWritesWG := imgWrappers.cleanupPendingOps()

	if len(errCh) > 0 {
		close(errCh)
		var msg bytes.Buffer
		for errRet := range errCh {
			_, _ = msg.WriteString(errRet.err.Error())
			_, _ = msg.WriteString("\n")

			// This captures the edge case when the error is cached in the image loader.
			util.LogErr(il.failureStore.AddDigestFailureIfNew(diff.NewDigestFailure(errRet.id, diff.OTHER)))
		}
		return nil, nil, errors.New(msg.String())
	}

	return result, pendingWritesWG, nil
}

// IsOnDisk returns true if the image that corresponds to the given imageID is in the disk cache.
func (il *ImageLoader) IsOnDisk(imageID types.Digest) bool {
	localRelPath, _ := ImagePaths(imageID)
	return fileutil.FileExists(filepath.Join(il.localImgDir, localRelPath))
}

// PurgeImages removes the images that correspond to the given images.
func (il *ImageLoader) PurgeImages(images types.DigestSlice, purgeGCS bool) error {
	for _, id := range images {
		localRelPath, gsRelPath := ImagePaths(id)
		localPath := filepath.Join(il.localImgDir, localRelPath)
		if fileutil.FileExists(localPath) {
			if err := os.Remove(localPath); err != nil {
				sklog.Errorf("Unable to remove image %s. Got error: %s", localPath, err)
			}
		}

		if purgeGCS {
			il.removeImg(gsRelPath)
		}
	}
	return nil
}

// imageLoadWorker implements the rtcache.ReadThroughFunc signature.
// It loads an image file either from disk or from Google storage.
func (il *ImageLoader) imageLoadWorker(priority int64, imageID types.Digest) (interface{}, error) {
	// Check if the image is in the disk cache.
	localRelPath, gsRelPath := ImagePaths(imageID)
	localPath := filepath.Join(il.localImgDir, localRelPath)
	sklog.Debugf("Looking for image with id %s", imageID)
	if fileutil.FileExists(localPath) {
		img, err := common.LoadImg(localPath)
		if err != nil {
			util.LogErr(il.failureStore.AddDigestFailure(diff.NewDigestFailure(imageID, diff.CORRUPTED)))
			return nil, skerr.Fmt("Could not load %s from disk: %s", localPath, err)
		}
		sklog.Debugf("Found it on disk at %s", localPath)
		util.LogErr(il.failureStore.PurgeDigestFailures(types.DigestSlice{imageID}))
		return &imgRet{img: img}, nil
	}

	// Download the image from GCS
	imgBytes, err := il.downloadImg(gsRelPath)
	if err != nil {
		util.LogErr(il.failureStore.AddDigestFailure(diff.NewDigestFailure(imageID, diff.HTTP)))
		return nil, err
	}

	// Decode it and return it.
	img, err := common.DecodeImg(bytes.NewBuffer(imgBytes))
	if err != nil {
		util.LogErr(il.failureStore.AddDigestFailure(diff.NewDigestFailure(imageID, diff.CORRUPTED)))
		return nil, err
	}

	// Save the file to disk.
	writeDoneCh := il.saveImgInfoAsync(imageID, imgBytes)
	return &imgRet{writtenCh: writeDoneCh, img: img}, nil
}

func (il *ImageLoader) saveImgInfoAsync(imageID types.Digest, imgBytes []byte) <-chan bool {
	writeDoneCh := make(chan bool)
	go func() {
		defer close(writeDoneCh)
		localRelPath, _ := ImagePaths(imageID)
		p := filepath.Join(il.localImgDir, localRelPath)
		if err := common.SaveFilePath(p, bytes.NewBuffer(imgBytes)); err != nil {
			sklog.Errorf("Could not write file to disk at %s: %s", p, err)
		}
	}()
	return writeDoneCh
}

// downloadImg retrieves the given image from Google storage. If bucket is not empty
// the gsPath is considered absolute within the bucket, otherwise it is relative to gsImageBaseDir.
// We will look in every bucket we are configured for.
func (il *ImageLoader) downloadImg(gsPath string) ([]byte, error) {
	bucketNames := il.gsBucketNames
	objLocation := filepath.Join(il.gsImageBaseDir, gsPath)

	var err error
	var imgData []byte
	for _, bucketName := range bucketNames {
		imgData, err = il.downloadImgFromBucket(objLocation, bucketName)
		if err == nil {
			return imgData, nil
		}
	}
	return nil, skerr.Fmt("Failed finding image %s in buckets %v. Last error: %s", objLocation, bucketNames, err)
}

// downloadImgFromBucket retrieves the given image from the given Google storage bucket.
// It returns storage.ErrObjectNotExist if the given image does not exist in the bucket.
func (il *ImageLoader) downloadImgFromBucket(objLocation, bucketName string) ([]byte, error) {
	ctx := context.Background()

	// Retrieve the attributes.
	attrs, err := il.storageClient.Bucket(bucketName).Object(objLocation).Attrs(ctx)
	if err != nil {
		return nil, skerr.Fmt("Unable to retrieve attributes for %s/%s: %.80s", bucketName, objLocation, err)
	}

	var buf *bytes.Buffer
	for i := 0; i < MAX_URI_GET_TRIES; i++ {
		if i > 0 {
			sklog.Infof("after error, sleeping 2s before GCS fetch for gs://%s/%s", bucketName, objLocation)
			// This is an arbitrary amount of time to wait.
			// TODO(kjlubick): should this be exponential backoff?
			time.Sleep(2 * time.Second)
		}
		err = func() error {
			reader, err := il.storageClient.Bucket(bucketName).Object(objLocation).NewReader(ctx)
			if err != nil {
				return fmt.Errorf("New reader failed for %s/%s: %s", bucketName, objLocation, err)
			}
			defer util.Close(reader)

			size := reader.Attrs.Size
			buf = bytes.NewBuffer(make([]byte, 0, size))
			md5Hash := md5.New()
			multiOut := io.MultiWriter(md5Hash, buf)

			if _, err = io.Copy(multiOut, reader); err != nil {
				return err
			}

			// Check the MD5.
			if !bytes.Equal(md5Hash.Sum(nil), attrs.MD5) {
				return skerr.Fmt("MD5 hash for digest %s incorrect.", objLocation)
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

	sklog.Infof("Done downloading image for: %s. Length: %d bytes", objLocation, buf.Len())
	return buf.Bytes(), err
}

// removeImg removes the image that corresponds to the given relative path from GCS.
func (il *ImageLoader) removeImg(gsRelPath string) {
	// If the bucket is not empty then look there otherwise use the default buckets.
	objLocation := filepath.Join(il.gsImageBaseDir, gsRelPath)
	bucketNames := il.gsBucketNames

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

// imgRet is a container type used to return the loaded image and a channel
// that is closed after the image had been written to disk or nil if the image
// was already on disk and/or RAM.
type imgRet struct {
	writtenCh <-chan bool // will be closed after the image has been written to disk
	img       *image.NRGBA
	mutex     sync.Mutex
}

// waitForDone blocks until the pending write is done and then disposes of writtenCh
// since the value will be in the read-through-cache or on disk.
func (i *imgRet) waitForDone() {
	if i == nil {
		return
	}

	i.mutex.Lock()
	defer i.mutex.Unlock()
	if i.writtenCh == nil {
		return
	}
	<-i.writtenCh
	i.writtenCh = nil
}

// imgRetSlice allows to synchronize many independent operations by using
// closed channels to signal completion. It also converts all the channels into
// a single WaitGroup.
type imgRetSlice []*imgRet

// cleanupPendingOps blocks until the pending operations are finished, i.e. all the underlying
// channels are either nil or closed. It also returns a WaitGroup that allows to
// wait for all operations to be done.
func (i imgRetSlice) cleanupPendingOps() *sync.WaitGroup {
	ret := &sync.WaitGroup{}
	ret.Add(len(i))
	go func() {
		for _, pendingWrite := range i {
			pendingWrite.waitForDone()
			ret.Done()
		}
	}()
	return ret
}
