package storage

import (
	"fmt"

	"cloud.google.com/go/storage"

	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/util"
	"golang.org/x/net/context"
)

type FuzzerGCSClient interface {
	gs.GCSClient
	SetFileContents(path string, opts gs.FileWriteOptions, contents []byte) error
}

type fuzzerclient struct {
	gs.GCSClient
}

func NewFuzzerGCSClient(s *storage.Client, bucket string) FuzzerGCSClient {
	return &fuzzerclient{gs.NewGCSClient(s, bucket)}
}

func (g *fuzzerclient) SetFileContents(path string, opts gs.FileWriteOptions, contents []byte) error {
	w := g.GCSClient.GetFileWriter(context.Background(), path, opts)
	defer util.Close(w)
	if n, err := w.Write(contents); err != nil {
		return fmt.Errorf("There was a problem uploading %s.  Only uploaded %d bytes: %s", path, n, err)
	}
	return nil
}
