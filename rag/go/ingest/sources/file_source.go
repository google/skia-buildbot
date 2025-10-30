package sources

import (
	"context"
	"os"

	"go.skia.org/infra/rag/go/ingest/history"
)

// FileSource defines a struct to ingest data from files.
type FileSource struct {
	filePath string
	ingester *history.HistoryIngester
}

// Ingest performs the ingestion of the provided file.
func (f *FileSource) Ingest(ctx context.Context) error {
	content, err := os.ReadFile(f.filePath)
	if err != nil {
		return err
	}
	return f.ingester.IngestBlameFileData(ctx, f.filePath, content)
}

// NewFileSource returns a new instance of FileSource.
func NewFileSource(filePath string, ingester *history.HistoryIngester) *FileSource {
	return &FileSource{
		filePath: filePath,
		ingester: ingester,
	}
}
