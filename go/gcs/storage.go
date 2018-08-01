package gcs

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/iterator"
)

// GCSClient is an interface for interacting with Google Cloud Storage (GCS). Introducing
// the interface allows for easier mocking and testing for unit (small) tests. GCSClient
// should have common, general functionality. Users should feel free to create a
// instance-specific GCSClient that creates an abstraction for more instance-specific method calls
// (see fuzzer for an example).
// One intentional thing missing from these method calls is bucket name. The bucket name
// is given at creation time, so as to simplify the method signatures.
// In all methods, context.Background() is a safe value for ctx if you don't want to use
// the context of the web request, for example.
// See also mock_gcs_client.New() for mocking this for unit tests.
type GCSClient interface {
	// FileReader returns an io.ReadCloser pointing to path on GCS, using the provided
	// context. storage.ErrObjectNotExist will be returned if the file is not found.
	// The caller must call Close on the returned Reader when done reading.
	FileReader(ctx context.Context, path string) (io.ReadCloser, error)
	// FileReader returns an io.WriteCloser that writes to the GCS file given by path
	// using the provided context. A new GCS file will be created if it doesn't already exist.
	// Otherwise, the existing file will be overwritten. The caller must call Close on
	// the returned Writer to flush the writes.
	FileWriter(ctx context.Context, path string, opts FileWriteOptions) io.WriteCloser
	// DoesFileExist returns true if the specified path exists and false if it does not.
	// This is a convenience wrapper around
	// https://godoc.org/cloud.google.com/go/storage#ObjectHandle.Attrs
	// If any error, other than storage.ErrObjectNotExist, is encountered then it will be
	// returned.
	DoesFileExist(ctx context.Context, path string) (bool, error)
	// GetFileContents returns the []byte represented by the GCS file at path. This is a
	// convenience wrapper around FileReader. storage.ErrObjectNotExist will be returned
	// if the file is not found.
	GetFileContents(ctx context.Context, path string) ([]byte, error)
	// SetFileContents writes the []byte to the GCS file at path. This is a
	// convenience wrapper around FileWriter. The GCS file will be created if it doesn't exist.
	SetFileContents(ctx context.Context, path string, opts FileWriteOptions, contents []byte) error
	// AllFilesInDirectory executes the callback on all GCS files with the given prefix,
	// i.e. in the directory prefix. It returns an error if it fails to read any of the
	// ObjectAttrs belonging to files.
	AllFilesInDirectory(ctx context.Context, prefix string, callback func(item *storage.ObjectAttrs)) error
	// DeleteFile deletes the given file, returning any error.
	DeleteFile(ctx context.Context, path string) error
	// Bucket() returns the bucket name of this client
	Bucket() string
}

// FileWriteOptions represents the metadata for a GCS file.  See storage.ObjectAttrs
// for a more detailed description of what these are.
type FileWriteOptions struct {
	ContentEncoding    string
	ContentType        string
	ContentLanguage    string
	ContentDisposition string
	Metadata           map[string]string
}

var FILE_WRITE_OPTS_TEXT = FileWriteOptions{ContentEncoding: "text/plain"}

// gcsclient holds the information needed to talk to cloud storage.
type gcsclient struct {
	client *storage.Client
	bucket string
}

// NewGCSClient returns a GCSClient. See the interface for more information.
func NewGCSClient(s *storage.Client, bucket string) GCSClient {
	return &gcsclient{
		client: s,
		bucket: bucket,
	}
}

// See the GCSClient interface for more information about FileReader.
func (g *gcsclient) FileReader(ctx context.Context, path string) (io.ReadCloser, error) {
	return g.client.Bucket(g.bucket).Object(path).NewReader(ctx)
}

// See the GCSClient interface for more information about FileWriter.
func (g *gcsclient) FileWriter(ctx context.Context, path string, opts FileWriteOptions) io.WriteCloser {
	w := g.client.Bucket(g.bucket).Object(path).NewWriter(ctx)
	w.ObjectAttrs.ContentEncoding = opts.ContentEncoding
	w.ObjectAttrs.ContentType = opts.ContentType
	w.ObjectAttrs.ContentLanguage = opts.ContentLanguage
	w.ObjectAttrs.ContentDisposition = opts.ContentDisposition
	w.ObjectAttrs.Metadata = opts.Metadata

	return w
}

// See the GCSClient interface for more information about DoesFileExist.
func (g *gcsclient) DoesFileExist(ctx context.Context, path string) (bool, error) {
	if _, err := g.client.Bucket(g.bucket).Object(path).Attrs(ctx); err != nil {
		if err == storage.ErrObjectNotExist {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// See the GCSClient interface for more information about GetFileContents.
func (g *gcsclient) GetFileContents(ctx context.Context, path string) ([]byte, error) {
	response, err := g.client.Bucket(g.bucket).Object(path).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer util.Close(response)
	return ioutil.ReadAll(response)
}

// See the GCSClient interface for more information about SetFileContents.
func (g *gcsclient) SetFileContents(ctx context.Context, path string, opts FileWriteOptions, contents []byte) error {
	w := g.FileWriter(ctx, path, opts)
	defer util.Close(w)
	if n, err := w.Write(contents); err != nil {
		return fmt.Errorf("There was a problem uploading %s.  Only uploaded %d bytes: %s", path, n, err)
	}
	return nil
}

// See the GCSClient interface for more information about AllFilesInDirectory.
func (g *gcsclient) AllFilesInDirectory(ctx context.Context, prefix string, callback func(item *storage.ObjectAttrs)) error {
	total := 0
	q := &storage.Query{Prefix: prefix, Versions: false}
	it := g.client.Bucket(g.bucket).Objects(ctx, q)
	for obj, err := it.Next(); err != iterator.Done; obj, err = it.Next() {
		if err != nil {
			return fmt.Errorf("Problem reading from Google Storage: %v", err)
		}
		total++
		callback(obj)
	}
	return nil
}

// See the GCSClient interface for more information about DeleteFile.
func (g *gcsclient) DeleteFile(ctx context.Context, path string) error {
	return g.client.Bucket(g.bucket).Object(path).Delete(ctx)
}

// See the GCSClient interface for more information about Bucket.
func (g *gcsclient) Bucket() string {
	return g.bucket
}
