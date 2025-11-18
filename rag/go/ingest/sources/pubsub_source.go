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
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/rag/go/filereaders/zip"
	"go.skia.org/infra/rag/go/ingest/history"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
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

	sklog.Infof("Ingesting file %s", event.Name)
	objectName := filepath.Base(event.Name)
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
		sklog.Errorf("Error creating temp directory: %v", err)
		return err
	}
	defer func() {
		err := os.RemoveAll(tempDir)
		if err != nil {
			sklog.Errorf("Error removing temp directory %s: %v", tempDir, err)
		}
	}()

	sklog.Infof("Extracting zip file to %s", tempDir)
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
func NewPubSubSource(ctx context.Context, message *pubsub.Message, ingester *history.HistoryIngester) (*PubSubSource, error) {
	ts, err := google.DefaultTokenSource(ctx, storage.ScopeReadOnly, pubsub.ScopePubSub)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	client, err := storage.NewClient(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &PubSubSource{
		message:       message,
		ingester:      ingester,
		storageClient: client,
	}, nil
}
