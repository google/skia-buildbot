package history

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/rag/go/blamestore"
	"go.skia.org/infra/rag/go/filereaders/npy"
	"go.skia.org/infra/rag/go/filereaders/pickle"
	"go.skia.org/infra/rag/go/topicstore"
)

// HistoryIngester provides a struct for performing ingestion for history rag.
type HistoryIngester struct {
	// Store impl for managing blame data.
	blameStore blamestore.BlameStore

	// Store impl for managing topic data.
	topicStore topicstore.TopicStore
}

// New returns a new instance of the history ingester.
func New(blameStore blamestore.BlameStore, topicStore topicstore.TopicStore) *HistoryIngester {
	return &HistoryIngester{
		blameStore: blameStore,
		topicStore: topicStore,
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

// IngestTopics ingests the topic data from the provided paths to the configured database.
func (ingester *HistoryIngester) IngestTopics(ctx context.Context, topicsDirPath string, embeddingsFilePath string, indexPickleFilePath string) error {
	// 1. Read the embeddings into memory.
	npyReader := npy.NewNpyReader(embeddingsFilePath)
	embeddings, err := npyReader.ReadFloat32()
	if err != nil {
		return err
	}

	// 2. Read the index into memory.
	pickleReader := pickle.NewPickleReader(indexPickleFilePath)
	indexEntries, err := pickleReader.Read()

	if err != nil {
		return err
	}

	sklog.Infof("Embeddings: %d, IndexEntries: %d", len(embeddings), len(indexEntries))

	// Now we read all the topics from the topic files.
	return filepath.WalkDir(topicsDirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var topicJson topicstore.TopicJSON
		if err := json.Unmarshal(content, &topicJson); err != nil {
			return err
		}

		// Create the Topic object using the values from the json, index and embeddings data.
		indexEntry := indexEntries[topicJson.TopicID]
		chunksFromIndex := indexEntry.Chunks

		topic := &topicstore.Topic{
			ID:               topicJson.TopicID,
			Title:            indexEntry.Title,
			TopicGroup:       indexEntry.Group,
			CommitCount:      indexEntry.CommitCount,
			Summary:          topicJson.Summary,
			CodeContext:      topicJson.CodeContext,
			CodeContextLines: indexEntry.CodeContextLines,
		}

		for _, chunkFromIndex := range chunksFromIndex {
			chunk := &topicstore.TopicChunk{
				ID:        chunkFromIndex.ChunkId,
				Chunk:     chunkFromIndex.ChunkContent,
				Embedding: embeddings[chunkFromIndex.EmbeddingIndex],
			}
			topic.Chunks = append(topic.Chunks, chunk)
		}

		return ingester.topicStore.WriteTopic(ctx, topic)
	})
}
