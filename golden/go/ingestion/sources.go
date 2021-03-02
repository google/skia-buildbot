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

// GCSSource represents a bucket and sublocation in Google Cloud Storage.
type GCSSource struct {
	Client *storage.Client `json:"-"`
	Bucket string          `json:"bucket"`
	Prefix string          `json:"dir"`
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
			if strings.HasSuffix(item.Name, ".json") && item.Updated.After(start) {
				files = append(files, item.Name)
			}
		})
		if err != nil {
			sklog.Errorf("Error occurred while retrieving files from %s/%s: %s", s.Bucket, dir, err)
		}
	}
	return files
}

// GetReader returns a ReadCloser with the data from this file or an error.
func (s *GCSSource) GetReader(ctx context.Context, name string) (io.ReadCloser, error) {
	return s.Client.Bucket(s.Bucket).Object(name).NewReader(ctx)
}
