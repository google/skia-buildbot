package storage

import (
	"fmt"
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/go/gcs"
	"golang.org/x/net/context"
)

// FuzzerGCSClient is the interface for all fuzzer-specific Google Cloud Storage (GCS)
// interactions. It embeds gcs.GCSClient to extend that functionality.
// See also go/fuzzer/tests.NewMockGCSClient() for a mock.
type FuzzerGCSClient interface {
	gcs.GCSClient
	// GetAllFuzzNamesInFolder returns all the fuzz names in a given GCS folder.  It basically
	// returns a list of all files that don't end with a .dump or .err, or error
	// if there was a problem.
	GetAllFuzzNamesInFolder(name string) (hashes []string, err error)
}

type fuzzerclient struct {
	gcs.GCSClient
}

func NewFuzzerGCSClient(s *storage.Client, bucket string) FuzzerGCSClient {
	return &fuzzerclient{gcs.NewGCSClient(s, bucket)}
}

func (g *fuzzerclient) GetAllFuzzNamesInFolder(name string) (hashes []string, err error) {
	filter := func(item *storage.ObjectAttrs) {
		name := item.Name
		fuzzHash := name[strings.LastIndex(name, "/")+1:]
		if !common.IsNameOfFuzz(fuzzHash) {
			return
		}
		hashes = append(hashes, fuzzHash)
	}

	if err = g.AllFilesInDirectory(context.Background(), name, filter); err != nil {
		return hashes, fmt.Errorf("Problem getting fuzzes from folder %s: %s", name, err)
	}
	return hashes, nil
}
