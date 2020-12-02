package app

import (
	"archive/zip"
	"context"
	"encoding/gob"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/builders"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/sql/migrations"
	"go.skia.org/infra/perf/go/sql/migrations/cockroachdb"
	"go.skia.org/infra/perf/go/tracestore"
)

// App is the running state for the perf-tool command-line application.
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
		return skerr.Wrap(err)
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
		return skerr.Wrap(err)
	}
	if err := createPubSubTopic(ctx, client, a.instanceConfig.IngestionConfig.SourceConfig.Topic); err != nil {
		return skerr.Wrap(err)
	}
	if a.instanceConfig.IngestionConfig.FileIngestionTopicName != "" {
		if err := createPubSubTopic(ctx, client, a.instanceConfig.IngestionConfig.FileIngestionTopicName); err != nil {
			return skerr.Wrap(err)
		}
	}

	return nil
}

func (a *App) OverrideConnectionString(connectionStringOverride string) {
	if connectionStringOverride != "" {
		a.instanceConfig.DataStoreConfig.ConnectionString = connectionStringOverride
	}
}

func (a *App) LoadTraceStore() error {
	if a.traceStore != nil {
		return nil
	}
	var err error
	a.traceStore, err = builders.NewTraceStoreFromConfig(context.Background(), a.local, a.instanceConfig)
	return skerr.Wrap(err)
}

func (a *App) DatabaseMigrateSubAction() error {

	cockroachdbMigrations, err := cockroachdb.New()
	if err != nil {
		return skerr.Wrapf(err, "failed to load migrations")
	}

	// Modify the connection string so it works with the migration package.
	connectionString := strings.Replace(a.instanceConfig.DataStoreConfig.ConnectionString, "postgresql://", "cockroachdb://", 1)

	err = migrations.Up(cockroachdbMigrations, connectionString)
	if err != nil {
		return skerr.Wrapf(err, "failed to apply migrations to %q", connectionString)
	}
	fmt.Printf("Successfully applied SQL Schema migrations to %q\n", a.instanceConfig.DataStoreConfig.ConnectionString)
	return nil
}

// The filenames we use inside the backup .zip files.
const (
	backupFilenameAlerts      = "alerts"
	backupFilenameShortcuts   = "shortcuts"
	backupFilenameRegressions = "regressions"
)

func (a *App) DatabaseDatabaseBackupAlertsSubAction(outputFile string) error {
	ctx := context.Background()

	f, err := os.Create(outputFile)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.Close(f)
	z := zip.NewWriter(f)

	alertStore, err := builders.NewAlertStoreFromConfig(ctx, a.local, a.instanceConfig)
	if err != nil {
		return skerr.Wrap(err)
	}
	alerts, err := alertStore.List(ctx, true)
	if err != nil {
		return skerr.Wrap(err)
	}
	alertsZipWriter, err := z.Create(backupFilenameAlerts)
	if err != nil {
		return skerr.Wrap(err)
	}
	encoder := gob.NewEncoder(alertsZipWriter)
	for _, alert := range alerts {
		fmt.Printf("Alert: %q\n", alert.DisplayName)
		if err := encoder.Encode(alert); err != nil {
			return skerr.Wrap(err)
		}
	}
	if err := z.Close(); err != nil {
		return skerr.Wrap(err)
	}

	return nil
}
