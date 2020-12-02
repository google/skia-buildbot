package app

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/builders"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/tracestore"
)

type App struct {
	instanceConfig *config.InstanceConfig
	traceStore     tracestore.TraceStore
	local          bool
}

func (a *App) Init(configFilename string, local bool) error {
	a.local = local

	var err error
	a.instanceConfig, err = config.InstanceConfigFromFile(configFilename)
	if err != nil {
		return skerr.Wrap(err)
	}
	config.Config = a.instanceConfig

	return nil
}

func createPubSubTopic(ctx context.Context, client *pubsub.Client, topicName string) error {
	topic := client.Topic(topicName)
	ok, err := topic.Exists(ctx)
	if err != nil {
		return err
	}
	if ok {
		fmt.Printf("Topic %q already exists\n", topicName)
		return nil
	}

	_, err = client.CreateTopic(ctx, topicName)
	if err != nil {
		return fmt.Errorf("Failed to create topic %q: %s", topicName, err)
	}
	return nil
}

func (a *App) ConfigCreatePubSubTopicsAction() error {
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, a.instanceConfig.IngestionConfig.SourceConfig.Project)
	if err != nil {
		return err
	}
	if err := createPubSubTopic(ctx, client, a.instanceConfig.IngestionConfig.SourceConfig.Topic); err != nil {
		return err
	}
	if a.instanceConfig.IngestionConfig.FileIngestionTopicName != "" {
		if err := createPubSubTopic(ctx, client, a.instanceConfig.IngestionConfig.FileIngestionTopicName); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) LoadTraceStore(connectionStringOverride string) error {
	if a.traceStore != nil {
		return nil
	}
	var err error
	if connectionStringOverride != "" {
		a.instanceConfig.DataStoreConfig.ConnectionString = connectionStringOverride
	}
	a.traceStore, err = builders.NewTraceStoreFromConfig(context.Background(), a.local, a.instanceConfig)
	return err
}
