package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"

	gstorage "cloud.google.com/go/storage"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
	"google.golang.org/api/option"
)

// GCSClientOptions is used to define input parameters to the GCSClient.
type GCSClientOptions struct {
	// HashesGSPath is the bucket and path for storing the list of known digests.
	HashesGSPath string

	// If DryRun is true, don't actually write the files (e.g. running locally)
	Dryrun bool
}

// GCSClient provides an abstraction around read/writes to Google storage.
// TODO(kjlubick) This interface should take context.Context where appropriate.
type GCSClient interface {
	// WriteKnownDigests writes the given list of digests to GCS as newline separated strings.
	WriteKnownDigests(digests types.DigestSlice) error

	// LoadKnownDigests loads the digests that have previously been written
	// to GS via WriteKnownDigests. The digests should be copied to the
	// provided writer 'w'.
	LoadKnownDigests(w io.Writer) error

	// RemoveForTestingOnly removes the given file. Should only be used for testing.
	RemoveForTestingOnly(targetPath string) error

	// Options returns the options that were used to initialize the client
	Options() GCSClientOptions
}

// ClientImpl implements the GCSClient interface.
type ClientImpl struct {
	storageClient *gstorage.Client
	options       GCSClientOptions
}

// NewGCSClient creates a new instance of ClientImpl. The various
// output paths are set in GCSClientOptions.
func NewGCSClient(client *http.Client, options GCSClientOptions) (*ClientImpl, error) {
	storageClient, err := gstorage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

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
func (g *ClientImpl) WriteKnownDigests(digests types.DigestSlice) error {
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

	return g.writeToPath(g.options.HashesGSPath, "text/plain", writeFn)
}

// LoadKnownDigests fulfills the GCSClient interface.
func (g *ClientImpl) LoadKnownDigests(w io.Writer) error {
	bucketName, storagePath := gcs.SplitGSPath(g.options.HashesGSPath)

	ctx := context.TODO()
	target := g.storageClient.Bucket(bucketName).Object(storagePath)

	// If the item doesn't exist this will return gstorage.ErrObjectNotExist
	_, err := target.Attrs(ctx)
	if err != nil {
		// We simply assume an empty hashes file if the object was not found.
		if err == gstorage.ErrObjectNotExist {
			sklog.Warningf("No known digests found - maybe %s is a wrong path?", g.options.HashesGSPath)
			return nil
		}
		return err
	}

	// Copy the content to the output writer.
	reader, err := target.NewReader(ctx)
	if err != nil {
		return skerr.Wrapf(err, "opening %s for reading", g.options.HashesGSPath)
	}
	defer util.Close(reader)

	n, err := io.Copy(w, reader)
	return skerr.Wrapf(err, "copying %d bytes to writer", n)
}

// RemoveForTestingOnly fulfills the GCSClient interface.
func (g *ClientImpl) RemoveForTestingOnly(targetPath string) error {
	bucketName, storagePath := gcs.SplitGSPath(targetPath)
	target := g.storageClient.Bucket(bucketName).Object(storagePath)
	return target.Delete(context.TODO())
}

// writeToPath is a generic function that allows to write data to the given
// target path in GCS. The actual writing is done in the passed write function.
func (g *ClientImpl) writeToPath(targetPath, contentType string, wrtFn func(w *gstorage.Writer) error) error {
	bucketName, storagePath := gcs.SplitGSPath(targetPath)

	// Only write the known digests if a target path was given.
	if (bucketName == "") || (storagePath == "") {
		return nil
	}

	ctx := context.TODO()
	target := g.storageClient.Bucket(bucketName).Object(storagePath)
	writer := target.NewWriter(ctx)
	writer.ObjectAttrs.ContentType = contentType
	writer.ObjectAttrs.ACL = []gstorage.ACLRule{{Entity: gstorage.AllUsers, Role: gstorage.RoleReader}}

	// Write the actual data.
	if err := wrtFn(writer); err != nil {
		return skerr.Wrapf(err, "writing data to %s", targetPath)
	}

	if err := writer.Close(); err != nil {
		return skerr.Wrapf(err, "closing writer for %s", targetPath)
	}

	return nil
}

// Ensure ClientImpl fulfills the GCSClient interface.
var _ GCSClient = (*ClientImpl)(nil)
