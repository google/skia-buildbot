package storage

import (
	"fmt"

	"cloud.google.com/go/storage"

	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/util"
)

type FileSetter interface {
	SetFileContents(path string, opts gs.FileWriteOptions, contents []byte) error
}

type FuzzerGCSClient struct {
	gs.GCSClient
}

func NewFuzzerGCSClient(s *storage.Client, bucket string) *FuzzerGCSClient {
	return &FuzzerGCSClient{*gs.NewGCSClient(s, bucket)}
}

func (g *FuzzerGCSClient) SetFileContents(path string, opts gs.FileWriteOptions, contents []byte) error {
	w := g.GetFileWriter(path, opts)
	defer util.Close(w)
	if n, err := w.Write(contents); err != nil {
		return fmt.Errorf("There was a problem uploading %s.  Only uploaded %d bytes: %s", path, n, err)
	}
	return nil
}
