package sources

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/rag/go/filereaders/zip"
	"go.skia.org/infra/rag/go/ingest/history"
)

const (
	embeddingFileName = "embeddings.npy"
	indexFileName     = "index.pkl"
	topicsDirName     = "topics"
)

// PubSubSource provides a struct to ingest pubsub message.
type PubSubSource struct {
	// storageClient is how we talk to Google Cloud Storage.
	storageClient *storage.Client

	message  *pubsub.Message
	ingester *history.HistoryIngester
}

// pubSubEvent is used to deserialize the PubSub data.
//
// The PubSub event data is a JSON serialized storage.ObjectAttrs object.
// See https://cloud.google.com/storage/docs/pubsub-notifications#payload
type pubSubEvent struct {
	Bucket string `json:"bucket"`
	Name   string `json:"name"`
}

// Ingest performs the ingestion of the provided message.
func (source *PubSubSource) Ingest(ctx context.Context) error {
	// Decode the event, which is a GCS event that a file was written.
	var event pubSubEvent
	if err := json.Unmarshal(source.message.Data, &event); err != nil {
		sklog.Error(err)
		return err
	}

	objectName := event.Name
	if !strings.HasSuffix(objectName, ".zip") {
		sklog.Warningf("Invalid object name %s in the pubsub event", objectName)
		return nil
	}
	obj := source.storageClient.Bucket(event.Bucket).Object(event.Name)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		sklog.Error(err)
		return err
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		sklog.Error(err)
		return err
	}

	tempDir, err := os.MkdirTemp("", "index-"+objectName)
	if err != nil {
		return err
	}
	defer func() {
		err := os.RemoveAll(tempDir)
		if err != nil {
			sklog.Errorf("Error removing temp directory %s: %v", tempDir, err)
		}
	}()

	err = zip.ExtractZipData(content, tempDir)
	if err != nil {
		return err
	}
	sklog.Infof("Zip file extracted to %s", tempDir)
	embeddingFilePath := filepath.Join(tempDir, embeddingFileName)
	indexFilePath := filepath.Join(tempDir, indexFileName)
	topicsDirPath := filepath.Join(tempDir, topicsDirName)
	return source.ingester.IngestTopics(ctx, topicsDirPath, embeddingFilePath, indexFilePath)
}

// NewPubSubSource returns a new instance of PubSubSource.
func NewPubSubSource(message *pubsub.Message, ingester *history.HistoryIngester) *PubSubSource {
	return &PubSubSource{
		message:  message,
		ingester: ingester,
	}
}
