package gs

import (
	"fmt"
	"io"
	"io/ioutil"

	"google.golang.org/api/iterator"

	"go.skia.org/infra/go/util"

	"cloud.google.com/go/storage"
	"golang.org/x/net/context"
)

type GCSClient interface {
	GetFileContents(ctx context.Context, path string) ([]byte, error)
	GetFileWriter(ctx context.Context, path string, options FileWriteOptions) io.WriteCloser
	ExecuteOnAllFilesInFolder(ctx context.Context, folder string, callback func(item *storage.ObjectAttrs)) error
}

type FileWriteOptions struct {
	ContentEncoding string
}

type gcsclient struct {
	client *storage.Client
	bucket string
}

func NewGCSClient(s *storage.Client, bucket string) GCSClient {
	return &gcsclient{
		client: s,
		bucket: bucket,
	}
}

func (g *gcsclient) GetFileContents(ctx context.Context, path string) ([]byte, error) {
	response, err := g.client.Bucket(g.bucket).Object(path).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer util.Close(response)
	return ioutil.ReadAll(response)
}

func (g *gcsclient) GetFileWriter(ctx context.Context, path string, options FileWriteOptions) io.WriteCloser {
	w := g.client.Bucket(g.bucket).Object(path).NewWriter(ctx)
	if options.ContentEncoding != "" {
		w.ObjectAttrs.ContentEncoding = options.ContentEncoding
	}

	return w
}

func (g *gcsclient) ExecuteOnAllFilesInFolder(ctx context.Context, folder string, callback func(item *storage.ObjectAttrs)) error {
	total := 0
	q := &storage.Query{Prefix: folder, Versions: false}
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
