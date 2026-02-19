package history

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/rag/go/blamestore"
	"go.skia.org/infra/rag/go/filereaders/npy"
	"go.skia.org/infra/rag/go/filereaders/pickle"
	"go.skia.org/infra/rag/go/topicstore"
	"golang.org/x/sync/errgroup"
)

const (
	topicIngestionParallelism = 50
	spannerStringMaxBytes     = 2097152
)

// HistoryIngester provides a struct for performing ingestion for history rag.
type HistoryIngester struct {
	// Store impl for managing blame data.
	blameStore blamestore.BlameStore

	// Store impl for managing topic data.
	topicStore topicstore.TopicStore

	// The output dimensionality for the instance.
	outputDimensionality int

	// Whether to use repository topics.
	useRepositoryTopics bool

	// The default repository name to use.
	defaultRepoName string

	// Counter metric for no of topics ingested.
	topicCounterMetric metrics2.Counter
}

// New returns a new instance of the history ingester.
func New(blameStore blamestore.BlameStore, topicStore topicstore.TopicStore, dimensionality int, useRepositoryTopics bool, defaultRepoName string) *HistoryIngester {
	return &HistoryIngester{
		blameStore:           blameStore,
		topicStore:           topicStore,
		outputDimensionality: dimensionality,
		useRepositoryTopics:  useRepositoryTopics,
		defaultRepoName:      defaultRepoName,

		// Init the metric objects.
		topicCounterMetric: metrics2.GetCounter("historyrag_ingestedTopics_count"),
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
func (ingester *HistoryIngester) IngestTopics(ctx context.Context, topicsDirPath string, embeddingsFilePath string, indexPickleFilePath string, repoName string) error {
	ctx, span := trace.StartSpan(ctx, "historyrag.ingester.IngestTopics")
	defer span.End()

	// 1. Read the embeddings into memory.
	npyReader := npy.NewNpyReader(embeddingsFilePath)
	sklog.Infof("Reading embeddings data from %s.", embeddingsFilePath)
	embeddings, err := npyReader.ReadFloat32()
	if err != nil {
		return err
	}

	// Verify that the embeddings are the length configured.
	for _, embedding := range embeddings {
		if len(embedding) != ingester.outputDimensionality {
			return skerr.Fmt("Invalid embedding length %d. Expected embeddings of length %d", len(embedding), ingester.outputDimensionality)
		}
	}
	// 2. Read the index into memory.
	pickleReader := pickle.NewPickleReader(indexPickleFilePath)
	sklog.Infof("Reading index pickle data from %s.", indexPickleFilePath)
	indexEntries, err := pickleReader.Read()

	if err != nil {
		return err
	}

	sklog.Infof("Embeddings: %d, IndexEntries: %d", len(embeddings), len(indexEntries))

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(topicIngestionParallelism)
	problematicFiles := map[string]error{}

	// Now we read all the topics from the topic files.
	err = filepath.WalkDir(topicsDirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		sklog.Infof("Ingesting file: %s", path)
		eg.Go(func() error {
			if repoName == "" {
				repoName = ingester.defaultRepoName
			}
			err = ingester.ingestTopicFile(ctx, path, repoName, embeddings, indexEntries)
			if err != nil {
				// Let's catch the error but allow processing to continue.
				sklog.Errorf("Error ingesting file %s: %v", path, err)
				problematicFiles[path] = err
			}
			ingester.topicCounterMetric.Inc(1)
			return nil
		})
		return nil
	})
	if err != nil {
		return err
	}
	err = eg.Wait()
	if err != nil {
		return err
	}

	// Log the errors encountered.
	if len(problematicFiles) > 0 {
		sklog.Infof("The following %d files encountered errors during ingestion", len(problematicFiles))
		for file, err := range problematicFiles {
			sklog.Errorf("File: %s, Error: %v", file, err)
		}
	}

	return nil
}

// ingestTopicFile performs topic ingestion for a single file.
func (ingester *HistoryIngester) ingestTopicFile(ctx context.Context, filePath, repoName string, embeddings [][]float32, indexEntries map[int64]pickle.IndexEntry) error {
	ctx, span := trace.StartSpan(ctx, "historyrag.ingester.IngestTopicFile")
	defer span.End()

	content, err := os.ReadFile(filePath)
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

	isTrimmed := trimTopicDataIfNecessary(&topicJson)
	if isTrimmed {
		sklog.Warningf("Trimmed topic data for file %s, TopicID: %d, Title: %s", filepath.Base(filePath), topicJson.TopicID, indexEntry.Title)
	}
	topic := &topicstore.Topic{
		ID:               topicJson.TopicID,
		Repository:       repoName,
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
}

// trimTopicDataIfNecessary ensures that the data we write is within spanner's limits.
func trimTopicDataIfNecessary(topicData *topicstore.TopicJSON) bool {
	if len(topicData.CodeContext) > spannerStringMaxBytes {
		topicData.CodeContext = topicData.CodeContext[:spannerStringMaxBytes]
		return true
	}
	return false
}
