package internal

import (
	"context"
	"io"
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

// GCS store info.
type store struct {
	ctx    context.Context
	bucket *storage.BucketHandle
}

func NewStore(ctx context.Context, bucketName string, readOnly bool) (*store, error) {
	var storeOption option.ClientOption
	if readOnly {
		storeOption = option.WithScopes(storage.ScopeReadOnly)
	} else {
		ts, err := google.DefaultTokenSource(ctx, auth.ScopeFullControl)
		if err != nil {
			return nil, skerr.Wrap(err)
		}

		client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
		storeOption = option.WithHTTPClient(client)

	}
	storageClient, err := storage.NewClient(context.Background(), storeOption)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &store{ctx: ctx, bucket: storageClient.Bucket(bucketName)}, nil
}

// Exists returns true if file found.
func (s *store) Exists(storeFilePath string) bool {
	o := s.bucket.Object(storeFilePath)
	_, err := o.Attrs(s.ctx)
	return err == nil
}

// GetFileContent returns the file content in GCS.
func (s *store) GetFileContent(storeFilePath string) ([]byte, error) {
	response, err := s.bucket.Object(storeFilePath).NewReader(s.ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer util.Close(response)
	return io.ReadAll(response)
}

// WriteFile creates or updates a file in GCS.
func (s *store) WriteFile(storeFilePath string, content string) error {
	w := s.bucket.Object(storeFilePath).NewWriter(s.ctx)
	w.ObjectAttrs.ContentEncoding = "text/plain"
	if strings.HasSuffix(storeFilePath, ".json") {
		w.ObjectAttrs.ContentType = "application/json"
	}
	_, err := w.Write([]byte(content))
	if err != nil {
		_ = w.Close()
		return skerr.Wrap(err)
	}
	err = w.Close()
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}
