package gcs

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

const (
	// GCS bucket where we store test data. Add a folder to this bucket
	// with the tests for a particular component.
	TEST_DATA_BUCKET = "skia-infra-testdata"
)

func getStorangeItem(bucket, gsPath string) (*storage.Reader, error) {
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(httputils.NewTimeoutClient()))
	if err != nil {
		return nil, err
	}

	return storageClient.Bucket(bucket).Object(gsPath).NewReader(context.Background())
}

// DownloadTestDataFile downloads a file with test data from Google Storage.
// The uriPath identifies what to download from the test bucket in GCS.
// The content must be publicly accessible.
// The file will be downloaded and stored at provided target
// path (regardless of what the original name is).
// If the the uri ends with '.gz' it will be transparently unzipped.
func DownloadTestDataFile(t assert.TestingT, bucket, gsPath, targetPath string) error {
	dir, _ := filepath.Split(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	arch, err := getStorangeItem(bucket, gsPath)
	if err != nil {
		return fmt.Errorf("Could not get gs://%s/%s: %s", bucket, gsPath, err)
	}
	defer func() { assert.NoError(t, arch.Close()) }()

	// Open the output
	var r io.ReadCloser = arch
	if strings.HasSuffix(gsPath, ".gz") {
		r, err = gzip.NewReader(r)
		if err != nil {
			return fmt.Errorf("Could not read gzip file: %s", err)
		}
	}

	f, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("Could not create target path: %s", err)
	}
	defer func() { assert.NoError(t, f.Close()) }()
	_, err = io.Copy(f, r)
	return err
}

// DownloadTestDataArchive downloads testfiles that are stored in
// a gz compressed tar archive and decompresses them into the provided
// target directory.
func DownloadTestDataArchive(t assert.TestingT, bucket, gsPath, targetDir string) error {
	if !strings.HasSuffix(gsPath, ".tar.gz") {
		return fmt.Errorf("Expected .tar.gz file. But got:%s", gsPath)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	arch, err := getStorangeItem(bucket, gsPath)
	if err != nil {
		return fmt.Errorf("Could not get gs://%s/%s: %s", bucket, gsPath, err)
	}
	defer func() { assert.NoError(t, arch.Close()) }()

	// Open the output
	r, err := gzip.NewReader(arch)
	if err != nil {
		return fmt.Errorf("Could not read gzip archive: %s", err)
	}
	tarReader := tar.NewReader(r)

	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("Problem reading from tar archive: %s", err)
		}

		targetPath := filepath.Join(targetDir, hdr.Name)

		if hdr.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("Could not make %s: %s", targetPath, err)
			}
		} else {
			f, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("Could not create target file %s: %s", targetPath, err)
			}
			_, err = io.Copy(f, tarReader)
			if err != nil {
				return fmt.Errorf("Problem while copying: %s", err)
			}
			defer func() { assert.NoError(t, f.Close()) }()
		}
	}

	return nil
}

// MemoryGCSClient is a struct used for testing. Instead of writing to GCS, it
// stores data in memory. Not thread-safe.
type MemoryGCSClient struct {
	bucket string
	data   map[string][]byte
	opts   map[string]FileWriteOptions
}

// Return a MemoryGCSClient instance.
func NewMemoryGCSClient(bucket string) *MemoryGCSClient {
	return &MemoryGCSClient{
		bucket: bucket,
		data:   map[string][]byte{},
		opts:   map[string]FileWriteOptions{},
	}
}

// See documentationn for GCSClient interface.
func (c *MemoryGCSClient) FileReader(ctx context.Context, path string) (io.ReadCloser, error) {
	contents, ok := c.data[path]
	if !ok {
		return nil, storage.ErrObjectNotExist
	}
	rv := ioutil.NopCloser(bytes.NewReader(contents))
	// GCS automatically decodes gzip-encoded files. See
	// https://cloud.google.com/storage/docs/transcoding. We do the same here so that tests acurately
	// reflect what will happen when actually using GCS.
	if c.opts[path].ContentEncoding == "gzip" {
		var err error
		rv, err = gzip.NewReader(rv)
		if err != nil {
			return nil, err
		}
	}
	return rv, nil
}

// io.WriteCloser implementation used by MemoryGCSClient.
type memoryWriter struct {
	buf    *bytes.Buffer
	client *MemoryGCSClient
	path   string
}

// See documentation for io.Writer.
func (w *memoryWriter) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}

// See documentation for io.Closer.
func (w *memoryWriter) Close() error {
	w.client.data[w.path] = w.buf.Bytes()
	return nil
}

// See documentation for GCSClient interface.
func (c *MemoryGCSClient) FileWriter(ctx context.Context, path string, opts FileWriteOptions) io.WriteCloser {
	c.opts[path] = opts
	return &memoryWriter{
		buf:    bytes.NewBuffer(nil),
		client: c,
		path:   path,
	}
}

// See documentation for GCSClient interface.
func (c *MemoryGCSClient) DoesFileExist(ctx context.Context, path string) (bool, error) {
	_, err := c.FileReader(ctx, path)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// See documentation for GCSClient interface.
func (c *MemoryGCSClient) GetFileContents(ctx context.Context, path string) ([]byte, error) {
	r, err := c.FileReader(ctx, path)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(r)
}

// See documentation for GCSClient interface.
func (c *MemoryGCSClient) SetFileContents(ctx context.Context, path string, opts FileWriteOptions, contents []byte) error {
	return WithWriteFile(c, ctx, path, opts, func(w io.Writer) error {
		_, err := w.Write(contents)
		return err
	})
}

// See documentation for GCSClient interface.
func (c *MemoryGCSClient) AllFilesInDirectory(ctx context.Context, prefix string, callback func(item *storage.ObjectAttrs)) error {
	for key, data := range c.data {
		if strings.HasPrefix(key, prefix) {
			opts := c.opts[key]
			item := &storage.ObjectAttrs{
				Bucket:             c.bucket,
				Name:               key,
				ContentType:        opts.ContentType,
				ContentLanguage:    opts.ContentLanguage,
				Size:               int64(len(data)),
				ContentEncoding:    opts.ContentEncoding,
				ContentDisposition: opts.ContentDisposition,
				Metadata:           util.CopyStringMap(opts.Metadata),
			}
			callback(item)
		}
	}
	return nil
}

// See documentation for GCSClient interface.
func (c *MemoryGCSClient) DeleteFile(ctx context.Context, path string) error {
	delete(c.data, path)
	delete(c.opts, path)
	return nil
}

// See documentation for GCSClient interface.
func (c *MemoryGCSClient) Bucket() string {
	return c.bucket
}

var _ GCSClient = (*MemoryGCSClient)(nil)
