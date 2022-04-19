package mem_gcsclient

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"io/ioutil"
	"strings"
	"sync"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/util"
)

// MemoryGCSClient is a struct used for testing. Instead of writing to GCS, it
// stores data in memory.
type MemoryGCSClient struct {
	bucket string
	data   map[string][]byte
	opts   map[string]gcs.FileWriteOptions
	mtx    sync.RWMutex
}

// Return a MemoryGCSClient instance.
func New(bucket string) *MemoryGCSClient {
	return &MemoryGCSClient{
		bucket: bucket,
		data:   map[string][]byte{},
		opts:   map[string]gcs.FileWriteOptions{},
	}
}

// See documentationn for GCSClient interface.
func (c *MemoryGCSClient) FileReader(ctx context.Context, path string) (io.ReadCloser, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
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
	w.client.mtx.Lock()
	defer w.client.mtx.Unlock()
	w.client.data[w.path] = w.buf.Bytes()
	return nil
}

// See documentation for GCSClient interface.
func (c *MemoryGCSClient) FileWriter(ctx context.Context, path string, opts gcs.FileWriteOptions) io.WriteCloser {
	c.mtx.Lock()
	defer c.mtx.Unlock()
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
func (c *MemoryGCSClient) SetFileContents(ctx context.Context, path string, opts gcs.FileWriteOptions, contents []byte) error {
	return gcs.WithWriteFile(c, ctx, path, opts, func(w io.Writer) error {
		_, err := w.Write(contents)
		return err
	})
}

// See documentation for GCSClient interface.
func (c *MemoryGCSClient) GetFileObjectAttrs(ctx context.Context, path string) (*storage.ObjectAttrs, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	data, ok := c.data[path]
	if !ok {
		return nil, storage.ErrObjectNotExist
	}
	opts, ok := c.opts[path]
	if !ok {
		return nil, storage.ErrObjectNotExist
	}
	return &storage.ObjectAttrs{
		Bucket:             c.bucket,
		Name:               path,
		ContentType:        opts.ContentType,
		ContentLanguage:    opts.ContentLanguage,
		Size:               int64(len(data)),
		ContentEncoding:    opts.ContentEncoding,
		ContentDisposition: opts.ContentDisposition,
		Metadata:           util.CopyStringMap(opts.Metadata),
	}, nil
}

// See documentation for GCSClient interface.
func (c *MemoryGCSClient) AllFilesInDirectory(ctx context.Context, prefix string, callback func(item *storage.ObjectAttrs) error) error {
	items := func() []*storage.ObjectAttrs {
		c.mtx.RLock()
		defer c.mtx.RUnlock()
		var items []*storage.ObjectAttrs
		for key, data := range c.data {
			if strings.HasPrefix(key, prefix) {
				opts := c.opts[key]
				items = append(items, &storage.ObjectAttrs{
					Bucket:             c.bucket,
					Name:               key,
					ContentType:        opts.ContentType,
					ContentLanguage:    opts.ContentLanguage,
					Size:               int64(len(data)),
					ContentEncoding:    opts.ContentEncoding,
					ContentDisposition: opts.ContentDisposition,
					Metadata:           util.CopyStringMap(opts.Metadata),
				})
			}
		}
		return items
	}()
	for _, item := range items {
		if err := callback(item); err != nil {
			return err
		}
	}
	return nil
}

// See documentation for GCSClient interface.
func (c *MemoryGCSClient) DeleteFile(ctx context.Context, path string) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	delete(c.data, path)
	delete(c.opts, path)
	return nil
}

// See documentation for GCSClient interface.
func (c *MemoryGCSClient) Bucket() string {
	return c.bucket
}

// make sure MemoryGCSClient implements the GCSClient interface
var _ gcs.GCSClient = (*MemoryGCSClient)(nil)
