package splitter

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/ingest/format"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

// IngestionDataSplitter provides a struct to split large input data files
// into smaller splits and write them to a secondary GCS path.
type IngestionDataSplitter struct {
	// Name of the bucket to write the splits.
	bucket string

	// GCS client to perform the operations.
	gcsClient gcs.GCSClient

	// Max no of results in a single split.
	maxItemsPerSplit int

	// Root directory under which all splits will be written.
	rootDirectory string
}

// NewIngestionDataSplitter returns a new instance of the IngestionDataSplitter.
func NewIngestionDataSplitter(ctx context.Context, maxItemsPerSplit int, secondaryGCSPath string, gcsClient gcs.GCSClient) (*IngestionDataSplitter, error) {
	bucket, rootDir, err := getBucketAndRootDirectory(secondaryGCSPath)

	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// If a gcsClient is not supplied, create a new one.
	if gcsClient == nil {
		ts, err := google.DefaultTokenSource(ctx, storage.ScopeReadWrite)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
		storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		gcsClient = gcsclient.New(storageClient, bucket)
	}

	return &IngestionDataSplitter{
		maxItemsPerSplit: maxItemsPerSplit,
		bucket:           bucket,
		rootDirectory:    rootDir,
		gcsClient:        gcsClient,
	}, nil
}

// SplitAndPublishFormattedData splits the data in f into smaller chunks and writes the individual
// chunks as a separate file in GCS.
func (s IngestionDataSplitter) SplitAndPublishFormattedData(ctx context.Context, f format.Format, filename string) error {

	sklog.Infof("No of results: %d", len(f.Results))

	splitData := []format.Format{}
	for start := 0; start < len(f.Results); start += s.maxItemsPerSplit {
		end := start + s.maxItemsPerSplit
		if end > len(f.Results) {
			end = len(f.Results)
		}
		resultBatch := f.Results[start:end]
		newObj := format.Format{
			Version:  f.Version,
			GitHash:  f.GitHash,
			Issue:    f.Issue,
			Patchset: f.Patchset,
			Key:      f.Key,
			Links:    f.Links,
			Results:  resultBatch,
		}
		splitData = append(splitData, newObj)
	}

	fileExtension := filepath.Ext(filename)
	originalFilenameWithoutExtension := filename[:len(filename)-len(fileExtension)]
	for i := 0; i < len(splitData); i++ {
		splitFilename := fmt.Sprintf("%s/%s_%d%s", s.rootDirectory, originalFilenameWithoutExtension, i, fileExtension)
		w := s.gcsClient.FileWriter(ctx, splitFilename, gcs.FileWriteOptions{
			ContentEncoding: "application/json",
		})
		err := splitData[i].Write(w)
		if err != nil {
			return skerr.Wrap(err)
		}
		sklog.Infof("Successfully uploaded %s", splitFilename)
	}
	return nil
}

// getBucketAndRootDirectory returns the bucket name and the root directory name
// from the supplied gcs path.
func getBucketAndRootDirectory(gcsPath string) (string, string, error) {
	// gcsPath is of the format gs://<bucket>/<root folder>
	if len(gcsPath) < 6 {
		return "", "", skerr.Fmt("Invalid GCS path %s", gcsPath)
	}
	pathWithoutPrefix := gcsPath[5:]
	bucket, directory, _ := strings.Cut(pathWithoutPrefix, "/")
	return bucket, directory, nil
}
