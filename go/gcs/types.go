package gcs

import (
	"context"
	"io"

	"cloud.google.com/go/storage"
)

// GCSClient is an interface for interacting with Google Cloud Storage (GCS). Introducing
// the interface allows for easier mocking and testing for unit (small) tests. GCSClient
// should have common, general functionality. Users should feel free to create a
// instance-specific GCSClient that creates an abstraction for more instance-specific method calls.
// One intentional thing missing from these method calls is bucket name. The bucket name
// is given at creation time, so as to simplify the method signatures.
// In all methods, context.Background() is a safe value for ctx if you don't want to use
// the context of the web request, for example.
// See also test_gcsclient.NewMockClient() for mocking this for unit tests.
// See also mem_gcsclient.New() for an alternative.
type GCSClient interface {
	// FileReader returns an io.ReadCloser pointing to path on GCS, using the provided
	// context. storage.ErrObjectNotExist will be returned if the file is not found.
	// The caller must call Close on the returned Reader when done reading.
	// Note that per https://cloud.google.com/storage/docs/transcoding, a file that is gzip encoded
	// will be automatically uncompressed.
	FileReader(ctx context.Context, path string) (io.ReadCloser, error)
	// FileWriter returns an io.WriteCloser that writes to the GCS file given by path
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
	// Note that per https://cloud.google.com/storage/docs/transcoding, a file that is gzip encoded
	// will be automatically uncompressed.
	GetFileContents(ctx context.Context, path string) ([]byte, error)
	// SetFileContents writes the []byte to the GCS file at path. This is a
	// convenience wrapper around FileWriter. The GCS file will be created if it doesn't exist.
	SetFileContents(ctx context.Context, path string, opts FileWriteOptions, contents []byte) error
	// GetFileObjectAttrs returns the storage.ObjectAttrs associated with the given
	// path.
	GetFileObjectAttrs(ctx context.Context, path string) (*storage.ObjectAttrs, error)
	// AllFilesInDirectory executes the callback on all GCS files with the given prefix,
	// i.e. in the directory prefix. It returns an error if it fails to read any of the
	// ObjectAttrs belonging to files. If the callback returns an error, iteration stops
	// and the error is returned without modification.
	AllFilesInDirectory(ctx context.Context, prefix string, callback func(item *storage.ObjectAttrs) error) error
	// DeleteFile deletes the given file, returning any error.
	DeleteFile(ctx context.Context, path string) error
	// Bucket returns the bucket name of this client
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

// FILE_WRITE_OPTS_TEXT are default options for writing a text file.
var FILE_WRITE_OPTS_TEXT = FileWriteOptions{ContentType: "text/plain"}
