package main

import (
	"context"
	"fmt"
	"sync"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/spanner"
	"go.skia.org/infra/go/pubsub/sub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/rag/go/blamestore"
	"go.skia.org/infra/rag/go/config"
	"go.skia.org/infra/rag/go/genai"
	"go.skia.org/infra/rag/go/ingest/history"
	"go.skia.org/infra/rag/go/ingest/sources"
	"go.skia.org/infra/rag/go/topicstore"
)

// IngestionSubscriber provides a struct to manage ingestion from pubsub notifications.
type IngestionSubscriber struct {
	subscription        *pubsub.Subscription
	historyIngestor     *history.HistoryIngester
	genAiClient         genai.GenAIClient
	evalSetPath         string
	queryEmbeddingModel string
	dimensionality      int32
	useRepositoryTopics bool
	defaultRepoName     string
}

// NewIngestionSubscriber returns a new instance of the IngestionSubscriber.
func NewIngestionSubscriber(ctx context.Context, config config.ApiServerConfig, genAiClient genai.GenAIClient) (*IngestionSubscriber, error) {
	// Generate the database identifier string and create the spanner client.
	databaseName := fmt.Sprintf("projects/%s/instances/%s/databases/%s", config.SpannerConfig.ProjectID, config.SpannerConfig.InstanceID, config.SpannerConfig.DatabaseID)
	spannerClient, err := spanner.NewClient(ctx, databaseName)
	if err != nil {
		sklog.Errorf("Error creating a spanner client")
		return nil, err
	}

	sklog.Infof("Creating a new blamestore instance")
	blamestore := blamestore.New(spannerClient)
	var topicStore topicstore.TopicStore
	if config.UseRepositoryTopics {
		topicStore = topicstore.NewRepositoryTopicStore(spannerClient)
	} else {
		topicStore = topicstore.New(spannerClient)
	}
	sklog.Infof("Creating a new history ingester.")
	ingester := history.New(blamestore, topicStore, config.OutputDimensionality, config.UseRepositoryTopics, config.DefaultRepoName)

	sub, err := sub.NewWithSubName(ctx, config.IngestionConfig.Project, config.IngestionConfig.Topic, config.IngestionConfig.Subscription, 1)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &IngestionSubscriber{
		subscription:        sub,
		historyIngestor:     ingester,
		genAiClient:         genAiClient,
		evalSetPath:         config.IngestionConfig.EvalSetPath,
		queryEmbeddingModel: config.QueryEmbeddingModel,
		dimensionality:      int32(config.OutputDimensionality),
		useRepositoryTopics: config.UseRepositoryTopics,
		defaultRepoName:     config.DefaultRepoName,
	}, nil
}

// Start creates a goroutine that listens for incoming pubsub messages to ingest.
func (subscriber *IngestionSubscriber) Start(ctx context.Context, wg *sync.WaitGroup) {
	// Process all incoming PubSub requests.
	go func() {
		for {
			// Wait for PubSub events.
			err := subscriber.subscription.Receive(ctx, subscriber.processPubSubMessage)
			if err != nil {
				sklog.Errorf("Failed receiving pubsub message: %s", err)
				wg.Done()
			}
		}
	}()
}

// processPubSubMessage handles a single pubsub message.
func (s *IngestionSubscriber) processPubSubMessage(ctx context.Context, msg *pubsub.Message) {
	sklog.Infof("Received pubsub message: %v", msg)
	pubsubSource, err := sources.NewPubSubSource(ctx, msg, s.historyIngestor, s.genAiClient, s.evalSetPath, s.queryEmbeddingModel, s.dimensionality, s.useRepositoryTopics, s.defaultRepoName)
	if err != nil {
		sklog.Errorf("Error creating pubsub source: %v", err)
	}
	err = pubsubSource.Ingest(ctx)
	if err != nil {
		sklog.Errorf("Error processing file: %v", err)
		msg.Nack()
	} else {
		msg.Ack()
		sklog.Infof("Ack'd message")
	}
}
