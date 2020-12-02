package app

import (
	"archive/zip"
	"context"
	"encoding/gob"
	"fmt"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/builders"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/sql/migrations"
	"go.skia.org/infra/perf/go/sql/migrations/cockroachdb"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/types"
)

// regressionBatchSize is the size of batches used when backing up Regressions.
const regressionBatchSize = 1000

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

func ConfigCreatePubSubTopicsAction(instanceConfig *config.InstanceConfig) error {
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, instanceConfig.IngestionConfig.SourceConfig.Project)
	if err != nil {
		return skerr.Wrap(err)
	}
	if err := createPubSubTopic(ctx, client, instanceConfig.IngestionConfig.SourceConfig.Topic); err != nil {
		return skerr.Wrap(err)
	}
	if instanceConfig.IngestionConfig.FileIngestionTopicName != "" {
		if err := createPubSubTopic(ctx, client, instanceConfig.IngestionConfig.FileIngestionTopicName); err != nil {
			return skerr.Wrap(err)
		}
	}

	return nil
}

func DatabaseMigrateSubAction(instanceConfig *config.InstanceConfig) error {

	cockroachdbMigrations, err := cockroachdb.New()
	if err != nil {
		return skerr.Wrapf(err, "failed to load migrations")
	}

	// Modify the connection string so it works with the migration package.
	connectionString := strings.Replace(instanceConfig.DataStoreConfig.ConnectionString, "postgresql://", "cockroachdb://", 1)

	err = migrations.Up(cockroachdbMigrations, connectionString)
	if err != nil {
		return skerr.Wrapf(err, "failed to apply migrations to %q", connectionString)
	}
	fmt.Printf("Successfully applied SQL Schema migrations to %q\n", instanceConfig.DataStoreConfig.ConnectionString)
	return nil
}

// The filenames we use inside the backup .zip files.
const (
	backupFilenameAlerts      = "alerts"
	backupFilenameShortcuts   = "shortcuts"
	backupFilenameRegressions = "regressions"
)

func DatabaseDatabaseBackupAlerts(local bool, instanceConfig *config.InstanceConfig, outputFile string) error {
	ctx := context.Background()

	f, err := os.Create(outputFile)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.Close(f)
	z := zip.NewWriter(f)

	alertStore, err := builders.NewAlertStoreFromConfig(ctx, local, instanceConfig)
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

// shortcutWithID is the struct we actually serialize into the backup.
//
// This allows checking the created ID and confirm it matches the backed up ID.
type shortcutWithID struct {
	ID       string
	Shortcut *shortcut.Shortcut
}

func DatabaseDatabaseBackupShortcuts(local bool, instanceConfig *config.InstanceConfig, outputFile string) error {
	ctx := context.Background()
	f, err := os.Create(outputFile)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.Close(f)
	z := zip.NewWriter(f)

	// Backup Shortcuts.
	shortcutsZipWriter, err := z.Create(backupFilenameShortcuts)
	if err != nil {
		return skerr.Wrap(err)
	}
	shortcutsEncoder := gob.NewEncoder(shortcutsZipWriter)

	shortcutStore, err := builders.NewShortcutStoreFromConfig(ctx, local, instanceConfig)
	if err != nil {
		return skerr.Wrap(err)
	}
	shortcutCh, err := shortcutStore.GetAll(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}

	fmt.Print("Shortcuts: ")
	total := 0
	for s := range shortcutCh {
		if err := shortcutsEncoder.Encode(shortcutWithID{
			ID:       shortcut.IDFromKeys(s),
			Shortcut: s,
		}); err != nil {
			return skerr.Wrap(err)
		}
		total++
		if total%100 == 0 {
			fmt.Print(".")
		}
	}
	fmt.Println()

	if err := z.Close(); err != nil {
		return skerr.Wrap(err)
	}

	return nil
}

// allRegressionsForCommitWithCommitNumber is the struct we actually write into
// the backup.
type allRegressionsForCommitWithCommitNumber struct {
	CommitNumber            types.CommitNumber
	AllRegressionsForCommit *regression.AllRegressionsForCommit
}

func DatabaseDatabaseBackupRegressions(local bool, instanceConfig *config.InstanceConfig, outputFile, backupTo string) error {
	ctx := context.Background()

	var backupToDate time.Time
	if backupTo == "" {
		backupToDate = time.Now().Add(-time.Hour * 24 * 7 * 4)
	} else {
		var err error
		backupToDate, err = time.Parse("2006-01-02", backupTo)
		if err != nil {
			return skerr.Wrap(err)
		}
	}
	fmt.Printf("Backing up from %v\n", backupToDate)

	f, err := os.Create(outputFile)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.Close(f)
	z := zip.NewWriter(f)

	// Backup Regressions.
	regressionsZipWriter, err := z.Create(backupFilenameRegressions)
	if err != nil {
		return skerr.Wrap(err)
	}

	regresssionsEncoder := gob.NewEncoder(regressionsZipWriter)
	perfGit, err := builders.NewPerfGitFromConfig(ctx, local, instanceConfig)
	if err != nil {
		return skerr.Wrap(err)
	}
	regressionStore, err := builders.NewRegressionStoreFromConfig(ctx, local, instanceConfig)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Get the latest commit.
	end, err := perfGit.CommitNumberFromTime(ctx, time.Time{})
	if err != nil {
		return skerr.Wrap(err)
	}
	if end == types.BadCommitNumber {
		return skerr.Fmt("Got bad commit number.")
	}

	shortcuts := map[string]bool{}
	for {
		if end < 0 {
			fmt.Println("Finished backing up chunks.")
			break
		}

		begin := end - regressionBatchSize + 1
		if begin < 0 {
			begin = 0
		}

		// Read out data in chunks of regressionBatchSize commits to store, going back N commits.
		// Range reads [begin, end], i.e. inclusive of both ends of the interval.
		regressions, err := regressionStore.Range(ctx, begin, end)
		if err != nil {
			return skerr.Wrap(err)
		}
		fmt.Printf("Regressions: [%d, %d]. Total commits: %d\n", begin, end, len(regressions))

		for commitNumber, allRegressionsForCommit := range regressions {
			// Find all the shortcuts in the regressions.
			for _, reg := range allRegressionsForCommit.ByAlertID {
				if reg.High != nil && reg.High.Shortcut != "" {
					shortcuts[reg.High.Shortcut] = true
				}
				if reg.Low != nil && reg.Low.Shortcut != "" {
					shortcuts[reg.Low.Shortcut] = true
				}
			}

			cid, err := perfGit.CommitFromCommitNumber(ctx, commitNumber)
			if err != nil {
				return skerr.Wrap(err)
			}
			commitDate := time.Unix(cid.Timestamp, 0)
			if commitDate.Before(backupToDate) {
				continue
			}
			body := allRegressionsForCommitWithCommitNumber{
				CommitNumber:            commitNumber,
				AllRegressionsForCommit: allRegressionsForCommit,
			}
			if err := regresssionsEncoder.Encode(body); err != nil {
				return skerr.Wrap(err)
			}
		}
		end = begin - 1
	}

	// Backup Shortcuts found in Regressions.
	shortcutsZipWriter, err := z.Create(backupFilenameShortcuts)
	if err != nil {
		return skerr.Wrap(err)
	}
	shortcutsEncoder := gob.NewEncoder(shortcutsZipWriter)
	shortcutStore, err := builders.NewShortcutStoreFromConfig(ctx, local, instanceConfig)
	if err != nil {
		return skerr.Wrap(err)
	}

	fmt.Println("Shortcuts:")
	total := 0
	for shortcutID := range shortcuts {
		shortcut, err := shortcutStore.Get(ctx, shortcutID)
		if err != nil {
			continue
		}
		if err := shortcutsEncoder.Encode(shortcutWithID{
			ID:       shortcutID,
			Shortcut: shortcut,
		}); err != nil {
			return skerr.Wrap(err)
		}
		total++
		if total%100 == 0 {
			fmt.Print(".")
		}
	}

	fmt.Println()
	if err := z.Close(); err != nil {
		return skerr.Wrap(err)
	}

	return nil
}
