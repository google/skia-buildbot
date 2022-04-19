package gcsclient

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/iterator"
)

// TODO(dogben, kjlubick): This should really have some tests.

// StorageClient holds the information needed to talk to cloud storage
// and fulfill the gcs.GCSClient interface
type StorageClient struct {
	client *storage.Client
	bucket string
}

// New returns a new *StorageClient. See the gcs.GCSClient interface for more information.
func New(s *storage.Client, bucket string) *StorageClient {
	return &StorageClient{
		client: s,
		bucket: bucket,
	}
}

// See the GCSClient interface for more information about FileReader.
func (g *StorageClient) FileReader(ctx context.Context, path string) (io.ReadCloser, error) {
	// TODO(dogben): if reader.Attrs.ContentEncoding == "gzip" then we should use ReadCompressed here
	// to get the compressed content, and wrap the reader in a gzip.Reader. Currently, with NewReader,
	// the content is decompressed on the server side; using ReadCompressed + gzip.Reader would save
	// bandwidth when retrieving while preserving the current behaviour.
	return g.client.Bucket(g.bucket).Object(path).NewReader(ctx)
}

// See the GCSClient interface for more information about FileWriter.
func (g *StorageClient) FileWriter(ctx context.Context, path string, opts gcs.FileWriteOptions) io.WriteCloser {
	w := g.client.Bucket(g.bucket).Object(path).NewWriter(ctx)
	w.ObjectAttrs.ContentEncoding = opts.ContentEncoding
	w.ObjectAttrs.ContentType = opts.ContentType
	w.ObjectAttrs.ContentLanguage = opts.ContentLanguage
	w.ObjectAttrs.ContentDisposition = opts.ContentDisposition
	w.ObjectAttrs.Metadata = opts.Metadata

	return w
}

// See the GCSClient interface for more information about DoesFileExist.
func (g *StorageClient) DoesFileExist(ctx context.Context, path string) (bool, error) {
	if _, err := g.client.Bucket(g.bucket).Object(path).Attrs(ctx); err != nil {
		if err == storage.ErrObjectNotExist {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// See the GCSClient interface for more information about GetFileContents.
func (g *StorageClient) GetFileContents(ctx context.Context, path string) ([]byte, error) {
	response, err := g.client.Bucket(g.bucket).Object(path).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer util.Close(response)
	return ioutil.ReadAll(response)
}

// See the GCSClient interface for more information about SetFileContents.
func (g *StorageClient) SetFileContents(ctx context.Context, path string, opts gcs.FileWriteOptions, contents []byte) (rv error) {
	return gcs.WithWriteFile(g, ctx, path, opts, func(w io.Writer) error {
		_, err := w.Write(contents)
		return err
	})
}

// See the GCSClient interface for more information about GetFileObjectAttrs.
func (g *StorageClient) GetFileObjectAttrs(ctx context.Context, path string) (*storage.ObjectAttrs, error) {
	return g.client.Bucket(g.bucket).Object(path).Attrs(ctx)
}

// See the GCSClient interface for more information about AllFilesInDirectory.
func (g *StorageClient) AllFilesInDirectory(ctx context.Context, prefix string, callback func(item *storage.ObjectAttrs) error) error {
	total := 0
	q := &storage.Query{Prefix: prefix, Versions: false}
	it := g.client.Bucket(g.bucket).Objects(ctx, q)
	for obj, err := it.Next(); err != iterator.Done; obj, err = it.Next() {
		if err != nil {
			return fmt.Errorf("Problem reading from Google Storage: %v", err)
		}
		total++
		if err := callback(obj); err != nil {
			return err
		}
	}
	return nil
}

// See the GCSClient interface for more information about DeleteFile.
func (g *StorageClient) DeleteFile(ctx context.Context, path string) error {
	return g.client.Bucket(g.bucket).Object(path).Delete(ctx)
}

// See the GCSClient interface for more information about Bucket.
func (g *StorageClient) Bucket() string {
	return g.bucket
}

var _ gcs.GCSClient = (*StorageClient)(nil)
