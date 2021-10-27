package storage

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"time"

	gstorage "cloud.google.com/go/storage"
	ttlcache "github.com/patrickmn/go-cache"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
	"google.golang.org/api/option"
)

// GCSClientOptions is used to define input parameters to the GCSClient.
type GCSClientOptions struct {
	// Bucket is the name of the GCS bucket we store to.
	Bucket string
	// KnownHashesGCSPath is the bucket and path for storing the list of known digests.
	KnownHashesGCSPath string

	// If DryRun is true, don't actually write the files (e.g. running locally)
	Dryrun bool
}

// GCSClient provides an abstraction around read/writes to Google storage.
type GCSClient interface {
	// WriteKnownDigests writes the given list of digests to GCS as newline separated strings.
	WriteKnownDigests(ctx context.Context, digests types.DigestSlice) error

	// LoadKnownDigests loads the digests that have previously been written
	// to GS via WriteKnownDigests. The digests should be copied to the
	// provided writer 'w'.
	LoadKnownDigests(ctx context.Context, w io.Writer) error

	// GetImage returns the raw bytes of an image with the corresponding Digest.
	GetImage(ctx context.Context, digest types.Digest) ([]byte, error)

	// Options returns the options that were used to initialize the client
	Options() GCSClientOptions
}

const (
	// Used to cache the digests in a time-to-live (TTL) cache.
	digestsCacheKey = "digestsCacheKey"

	// We re-index the tile (and thus re-compute the known digests) no
	// faster than once per minute.
	digestsCacheFreshness = time.Minute

	// The GCS folder that contains the images, named by their digests.
	imgFolder = "dm-images-v1"
)

// ClientImpl implements the GCSClient interface.
type ClientImpl struct {
	storageClient *gstorage.Client
	options       GCSClientOptions

	digestsCache *ttlcache.Cache
}

// NewGCSClient creates a new instance of ClientImpl. The various
// output paths are set in GCSClientOptions.
func NewGCSClient(ctx context.Context, client *http.Client, options GCSClientOptions) (*ClientImpl, error) {
	storageClient, err := gstorage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	return &ClientImpl{
		storageClient: storageClient,
		options:       options,
		digestsCache:  ttlcache.New(digestsCacheFreshness, digestsCacheFreshness),
	}, nil
}

// Options implements the GCSClient interface.
func (g *ClientImpl) Options() GCSClientOptions {
	return g.options
}

// WriteKnownDigests fulfills the GCSClient interface.
func (g *ClientImpl) WriteKnownDigests(ctx context.Context, digests types.DigestSlice) error {
	if g.options.Dryrun {
		sklog.Infof("dryrun: Writing %d digests", len(digests))
		return nil
	}
	writeFn := func(w *gstorage.Writer) error {
		for _, digest := range digests {
			if _, err := w.Write([]byte(digest + "\n")); err != nil {
				return fmt.Errorf("Error writing digests: %s", err)
			}
		}
		return nil
	}
	// Clear the read cache when the write completes or fails
	defer g.digestsCache.Delete(digestsCacheKey)

	return g.writeToPath(ctx, g.options.KnownHashesGCSPath, "text/plain", writeFn)
}

// LoadKnownDigests fulfills the GCSClient interface.
func (g *ClientImpl) LoadKnownDigests(ctx context.Context, w io.Writer) error {
	if cachedBytes, ok := g.digestsCache.Get(digestsCacheKey); ok {
		b := cachedBytes.([]byte)
		n, err := w.Write(b)
		return skerr.Wrapf(err, "copying %d cached bytes to writer", n)
	}

	bucketName, storagePath := gcs.SplitGSPath(g.options.KnownHashesGCSPath)

	target := g.storageClient.Bucket(bucketName).Object(storagePath)

	// If the item doesn't exist this will return gstorage.ErrObjectNotExist
	_, err := target.Attrs(ctx)
	if err != nil {
		// We simply assume an empty hashes file if the object was not found.
		if err == gstorage.ErrObjectNotExist {
			sklog.Warningf("No known digests found - maybe %s is a wrong path?", g.options.KnownHashesGCSPath)
			return nil
		}
		return err
	}

	// Copy the content to the output writer.
	reader, err := target.NewReader(ctx)
	if err != nil {
		return skerr.Wrapf(err, "opening %s for reading", g.options.KnownHashesGCSPath)
	}
	defer util.Close(reader)

	b, err := ioutil.ReadAll(reader)
	if err != nil {
		return skerr.Wrapf(err, "reading digests from GCS")
	}
	g.digestsCache.SetDefault(digestsCacheKey, b)

	n, err := w.Write(b)
	return skerr.Wrapf(err, "writing %d bytes from cache to writer", n)
}

// removeForTestingOnly removes the given file. Should only be used for testing.
func (g *ClientImpl) removeForTestingOnly(ctx context.Context, targetPath string) error {
	bucketName, storagePath := gcs.SplitGSPath(targetPath)
	target := g.storageClient.Bucket(bucketName).Object(storagePath)
	return target.Delete(ctx)
}

// writeToPath is a generic function that allows to write data to the given
// target path in GCS. The actual writing is done in the passed write function.
func (g *ClientImpl) writeToPath(ctx context.Context, targetPath, contentType string, wrtFn func(w *gstorage.Writer) error) error {
	bucketName, storagePath := gcs.SplitGSPath(targetPath)

	// Only write the known digests if a target path was given.
	if (bucketName == "") || (storagePath == "") {
		return nil
	}

	target := g.storageClient.Bucket(bucketName).Object(storagePath)
	writer := target.NewWriter(ctx)
	writer.ObjectAttrs.ContentType = contentType

	// Write the actual data.
	if err := wrtFn(writer); err != nil {
		return skerr.Wrapf(err, "writing data to %s", targetPath)
	}

	if err := writer.Close(); err != nil {
		return skerr.Wrapf(err, "closing writer for %s", targetPath)
	}

	return nil
}

// GetImage downloads the bytes and returns them for the given image. It returns an error if
// the image is not found.
func (g *ClientImpl) GetImage(ctx context.Context, digest types.Digest) ([]byte, error) {
	// intentionally using path because gcs is forward slashes
	imgPath := path.Join(imgFolder, string(digest)+".png")
	r, err := g.storageClient.Bucket(g.options.Bucket).Object(imgPath).NewReader(ctx)
	if err != nil {
		// If not image not found, this error path will be taken.
		return nil, skerr.Wrap(err)
	}
	defer util.Close(r)
	b, err := ioutil.ReadAll(r)
	return b, skerr.Wrap(err)
}

// Ensure ClientImpl fulfills the GCSClient interface.
var _ GCSClient = (*ClientImpl)(nil)
