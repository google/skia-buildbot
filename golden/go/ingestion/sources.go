package ingestion

import (
	"context"
	"io"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/sklog"
)

// FileSearcher is an interface around the logic for polling for files that may have been
// missed via the typical event-based ingestion.
type FileSearcher interface {
	// SearchForFiles returns a slice of files that appear in the given time range.
	SearchForFiles(ctx context.Context, start, end time.Time) []string
}

// Source represents a place that an ingester can get a file to process.
type Source interface {
	// GetReader returns a reader to the content. If there is a problem (e.g. file does not exist)
	// an error will be returned.
	GetReader(ctx context.Context, name string) (io.ReadCloser, error)
	// HandlesFile returns true if this file is handled by the given source.
	HandlesFile(name string) bool
}

// GCSSource represents a bucket and sublocation in Google Cloud Storage.
type GCSSource struct {
	Client *storage.Client
	Bucket string
	Prefix string
}

// HandlesFile returns true if this file matches the prefix of the configured GCS source.
func (s *GCSSource) HandlesFile(name string) bool {
	return strings.HasPrefix(name, s.Prefix)
}

// SearchForFiles uses the standard pattern of named, hourly folders to search for all files
// in the given time range.
func (s *GCSSource) SearchForFiles(ctx context.Context, start, end time.Time) []string {
	ctx, span := trace.StartSpan(ctx, "ingestion_SearchForFiles")
	defer span.End()
	dirs := fileutil.GetHourlyDirs(s.Prefix, start, end)

	var files []string
	for _, dir := range dirs {
		err := gcs.AllFilesInDir(s.Client, s.Bucket, dir, func(item *storage.ObjectAttrs) {
			if strings.HasSuffix(item.Name, ".json") {
				files = append(files, item.Name)
			}
		})
		if err != nil {
			sklog.Errorf("Error occurred while retrieving files from %s/%s: %s", s.Bucket, dir, err)
		}
	}
	if len(files) > 0 {
		sklog.Infof("First GCS file in backup range: %s", files[0])
		sklog.Infof("Last GCS file in backup range: %s", files[len(files)-1])
	}
	return files
}

// GetReader returns a ReadCloser with the data from this file or an error.
func (s *GCSSource) GetReader(ctx context.Context, name string) (io.ReadCloser, error) {
	return s.Client.Bucket(s.Bucket).Object(name).NewReader(ctx)
}

func (s *GCSSource) String() string {
	return "gs://" + s.Bucket + "/" + s.Prefix
}

// Validate returns true if all fields are filled in.
func (s *GCSSource) Validate() bool {
	return s.Client != nil && s.Bucket != "" && s.Prefix != ""
}

// Make sure GCSSource implements the Source interface.
var _ Source = (*GCSSource)(nil)
