package history

import (
	"context"
	"encoding/json"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/rag/go/blamestore"
)

// HistoryIngester provides a struct for performing ingestion for history rag.
type HistoryIngester struct {
	// Store impl for managing blame data.
	blameStore blamestore.BlameStore
}

// New returns a new instance of the history ingester.
func New(blameStore blamestore.BlameStore) *HistoryIngester {
	return &HistoryIngester{
		blameStore: blameStore,
	}
}

// blameData is the structure of the JSON data we receive.
type blameData struct {
	Version  string   `json:"version"`
	FileHash string   `json:"file_hash"`
	Lines    []string `json:"lines"`
}

// Ingests the provided blame file data into the database.
func (ingester *HistoryIngester) IngestBlameFileData(ctx context.Context, filePath string, fileContent []byte) error {
	var data blameData
	if err := json.Unmarshal(fileContent, &data); err != nil {
		return skerr.Wrapf(err, "failed to unmarshal blame data for %s", filePath)
	}

	lineBlames := make([]*blamestore.LineBlame, 0, len(data.Lines))
	for i, commitHash := range data.Lines {
		lineBlames = append(lineBlames, &blamestore.LineBlame{
			LineNumber: int64(i + 1),
			CommitHash: commitHash,
		})
	}

	blame := &blamestore.FileBlame{
		FilePath:   filePath,
		FileHash:   data.FileHash,
		Version:    data.Version,
		LineBlames: lineBlames,
	}

	return ingester.blameStore.WriteBlame(ctx, blame)
}
