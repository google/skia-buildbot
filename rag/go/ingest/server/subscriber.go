package main

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/spanner"
	"go.skia.org/infra/go/pubsub/sub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/rag/go/blamestore"
	"go.skia.org/infra/rag/go/config"
	"go.skia.org/infra/rag/go/ingest/history"
	"go.skia.org/infra/rag/go/ingest/sources"
)

// IngestionSubscriber provides a struct to manage ingestion from pubsub notifications.
type IngestionSubscriber struct {
	subscription    *pubsub.Subscription
	historyIngestor *history.HistoryIngester
}

// NewingestionSubscriber returns a new instance of the IngestionSubscriber.
func NewIngestionSubscriber(ctx context.Context, config config.ApiServerConfig) (*IngestionSubscriber, error) {
	// Generate the database identifier string and create the spanner client.
	databaseName := fmt.Sprintf("projects/%s/instances/%s/databases/%s", config.SpannerConfig.ProjectID, config.SpannerConfig.InstanceID, config.SpannerConfig.DatabaseID)
	spannerClient, err := spanner.NewClient(ctx, databaseName)
	if err != nil {
		sklog.Errorf("Error creating a spanner client")
		return nil, err
	}

	sklog.Infof("Creating a new blamestore instance")
	blamestore := blamestore.New(spannerClient)
	sklog.Infof("Creating a new history ingester.")
	ingester := history.New(blamestore)

	sub, err := sub.NewWithSubName(ctx, config.IngestionConfig.Project, config.IngestionConfig.Topic, config.IngestionConfig.Subscription, 1)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &IngestionSubscriber{
		subscription:    sub,
		historyIngestor: ingester,
	}, nil
}

// Start creates a goroutine that listens for incoming pubsub messages to ingest.
func (subscriber *IngestionSubscriber) Start(ctx context.Context) {
	// Process all incoming PubSub requests.
	go func() {
		for {
			// Wait for PubSub events.
			err := subscriber.subscription.Receive(ctx, subscriber.processPubSubMessage)
			if err != nil {
				sklog.Errorf("Failed receiving pubsub message: %s", err)
			}
		}
	}()
}

// processPubSubMessage handles a single pubsub message.
func (s *IngestionSubscriber) processPubSubMessage(ctx context.Context, msg *pubsub.Message) {
	pubsubSource := sources.NewPubSubSource(msg, s.historyIngestor)
	err := pubsubSource.Ingest(ctx)
	if err != nil {
		sklog.Errorf("Error processing file: %v", err)
	}
}
