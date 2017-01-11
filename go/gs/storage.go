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

// Context-free calls

type FileGetter interface {
	GetFileContents(path string) ([]byte, error)
}

type FileWriter interface {
	GetFileWriter(path string, options FileWriteOptions) io.WriteCloser
}

type FolderOperator interface {
	ExecuteOnAllFilesInFolder(folder string, callback func(item *storage.ObjectAttrs)) error
}

// Context-dependent calls

type FileGetterCTX interface {
	GetFileContentsCTX(path string, ctx context.Context) ([]byte, error)
}

type FileWriterCTX interface {
	GetFileWriterCTX(path string, options FileWriteOptions, ctx context.Context) io.WriteCloser
}

type FolderOperatorCTX interface {
	ExecuteOnAllFilesInFolderCTX(folder string, callback func(item *storage.ObjectAttrs), ctx context.Context) error
}

// Structs

type FileWriteOptions struct {
	ContentEncoding string
}

type GCSClient struct {
	client *storage.Client
	bucket string
}

func NewGCSClient(s *storage.Client, bucket string) *GCSClient {
	return &GCSClient{
		client: s,
		bucket: bucket,
	}
}

func (g *GCSClient) GetFileContents(path string) ([]byte, error) {
	return g.GetFileContentsCTX(path, context.Background())
}

func (g *GCSClient) GetFileContentsCTX(path string, ctx context.Context) ([]byte, error) {
	response, err := g.client.Bucket(g.bucket).Object(path).NewReader(context.Background())
	if err != nil {
		return nil, err
	}
	defer util.Close(response)
	return ioutil.ReadAll(response)
}

func (g *GCSClient) GetFileWriter(path string, options FileWriteOptions) io.WriteCloser {
	return g.GetFileWriterCTX(path, options, context.Background())

}

func (g *GCSClient) GetFileWriterCTX(path string, options FileWriteOptions, ctx context.Context) io.WriteCloser {
	w := g.client.Bucket(g.bucket).Object(path).NewWriter(ctx)
	if options.ContentEncoding != "" {
		w.ObjectAttrs.ContentEncoding = options.ContentEncoding
	}

	return w
}

func (g *GCSClient) ExecuteOnAllFilesInFolder(folder string, callback func(item *storage.ObjectAttrs)) error {
	return g.ExecuteOnAllFilesInFolderCTX(folder, callback, context.Background())
}

func (g *GCSClient) ExecuteOnAllFilesInFolderCTX(folder string, callback func(item *storage.ObjectAttrs), ctx context.Context) error {
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
