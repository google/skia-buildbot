package storage

import (
	"context"
	"fmt"
	"io"
	"path"

	"go.opencensus.io/trace"

	gstorage "cloud.google.com/go/storage"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

// GCSClientOptions is used to define input parameters to the GCSClient.
type GCSClientOptions struct {
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
	// The GCS folder that contains the images, named by their digests.
	imgFolder = "dm-images-v1"
)

// ClientImpl implements the GCSClient interface.
type ClientImpl struct {
	storageClient gcs.GCSClient
	options       GCSClientOptions
}

// NewGCSClient creates a new instance of ClientImpl. The various
// output paths are set in GCSClientOptions.
func NewGCSClient(ctx context.Context, storageClient gcs.GCSClient, options GCSClientOptions) (*ClientImpl, error) {
	return &ClientImpl{
		storageClient: storageClient,
		options:       options,
	}, nil
}

// Options implements the GCSClient interface.
func (g *ClientImpl) Options() GCSClientOptions {
	return g.options
}

// WriteKnownDigests fulfills the GCSClient interface.
func (g *ClientImpl) WriteKnownDigests(ctx context.Context, digests types.DigestSlice) error {
	ctx, span := trace.StartSpan(ctx, "gcsclient_WriteKnownDigests")
	defer span.End()
	if g.options.Dryrun {
		sklog.Infof("dryrun: Writing %d digests", len(digests))
		return nil
	}
	writeFn := func(w io.Writer) error {
		for _, digest := range digests {
			if _, err := w.Write([]byte(digest + "\n")); err != nil {
				return fmt.Errorf("Error writing digests: %s", err)
			}
		}
		return nil
	}
	return g.writeToPath(ctx, g.options.KnownHashesGCSPath, "text/plain", writeFn)
}

// LoadKnownDigests fulfills the GCSClient interface. It does no caching of the result.
func (g *ClientImpl) LoadKnownDigests(ctx context.Context, w io.Writer) error {
	ctx, span := trace.StartSpan(ctx, "gcsclient_LoadKnownDigests")
	defer span.End()
	_, storagePath := gcs.SplitGSPath(g.options.KnownHashesGCSPath)

	// If the item doesn't exist this will return gstorage.ErrObjectNotExist
	_, err := g.storageClient.GetFileObjectAttrs(ctx, storagePath)
	if err != nil {
		// We simply assume an empty hashes file if the object was not found.
		if err == gstorage.ErrObjectNotExist {
			sklog.Warningf("No known digests found - maybe %s is a wrong path?", g.options.KnownHashesGCSPath)
			return nil
		}
		return skerr.Wrap(err)
	}

	// Copy the content to the output writer.
	reader, err := g.storageClient.FileReader(ctx, storagePath)
	if err != nil {
		return skerr.Wrapf(err, "opening %s for reading", g.options.KnownHashesGCSPath)
	}
	defer util.Close(reader)
	n, err := io.Copy(w, reader)
	return skerr.Wrapf(err, "writing %d bytes of digests to writer", n)
}

// removeForTestingOnly removes the given file. Should only be used for testing.
func (g *ClientImpl) removeForTestingOnly(ctx context.Context, targetPath string) error {
	_, storagePath := gcs.SplitGSPath(targetPath)
	return g.storageClient.DeleteFile(ctx, storagePath)
}

// writeToPath is a generic function that allows to write data to the given
// target path in GCS. The actual writing is done in the passed write function.
func (g *ClientImpl) writeToPath(ctx context.Context, targetPath, contentType string, wrtFn func(io.Writer) error) error {
	bucketName, storagePath := gcs.SplitGSPath(targetPath)

	// Only write the known digests if a target path was given.
	if (bucketName == "") || (storagePath == "") {
		return nil
	}

	writer := g.storageClient.FileWriter(ctx, storagePath, gcs.FileWriteOptions{
		ContentType: contentType,
	})

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
	ctx, span := trace.StartSpan(ctx, "gcsclient_GetImage")
	defer span.End()
	// intentionally using path because gcs is forward slashes
	imgPath := path.Join(imgFolder, string(digest)+".png")
	r, err := g.storageClient.FileReader(ctx, imgPath)
	if err != nil {
		// If not image not found, this error path will be taken.
		return nil, skerr.Wrap(err)
	}
	defer util.Close(r)
	b, err := io.ReadAll(r)
	return b, skerr.Wrap(err)
}

// Ensure ClientImpl fulfills the GCSClient interface.
var _ GCSClient = (*ClientImpl)(nil)
