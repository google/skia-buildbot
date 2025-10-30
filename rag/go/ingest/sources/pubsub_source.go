package sources

import (
	"context"
	"encoding/json"
	"io"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/rag/go/ingest/history"
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

	// TODO(ashwinpv): We may need to extract the file path from event.Name.
	return source.ingester.IngestBlameFileData(ctx, event.Name, content)
}

// NewPubSubSource returns a new instance of PubSubSource.
func NewPubSubSource(message *pubsub.Message, ingester *history.HistoryIngester) *PubSubSource {
	return &PubSubSource{
		message:  message,
		ingester: ingester,
	}
}
