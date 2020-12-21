package application

import (
	"archive/zip"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/builders"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/ingest/format"
	"go.skia.org/infra/perf/go/ingest/parser"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/sql/migrations"
	"go.skia.org/infra/perf/go/sql/migrations/cockroachdb"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/trybot/samplesloader/gcssamplesloader"
	"go.skia.org/infra/perf/go/types"
	"google.golang.org/api/option"
)

// Application contains the high level functions needed by perf-tool.
type Application interface {
	ConfigCreatePubSubTopics(instanceConfig *config.InstanceConfig) error
	DatabaseMigrate(instanceConfig *config.InstanceConfig) error
	DatabaseBackupAlerts(local bool, instanceConfig *config.InstanceConfig, outputFile string) error
	DatabaseBackupShortcuts(local bool, instanceConfig *config.InstanceConfig, outputFile string) error
	DatabaseBackupRegressions(local bool, instanceConfig *config.InstanceConfig, outputFile, backupTo string) error
	DatabaseRestoreAlerts(local bool, instanceConfig *config.InstanceConfig, inputFile string) error
	DatabaseRestoreShortcuts(local bool, instanceConfig *config.InstanceConfig, inputFile string) error
	DatabaseRestoreRegressions(local bool, instanceConfig *config.InstanceConfig, inputFile string) error
	TilesLast(store tracestore.TraceStore) error
	TilesList(store tracestore.TraceStore, num int) error
	TracesList(store tracestore.TraceStore, queryString string, tileNumber types.TileNumber) error
	TracesExport(store tracestore.TraceStore, queryString string, begin, end types.CommitNumber, outputFile string) error
	IngestForceReingest(local bool, instanceConfig *config.InstanceConfig, start, stop string, dryrun bool) error
	TrybotReference(local bool, store tracestore.TraceStore, instanceConfig *config.InstanceConfig, trybotFilename string, outputFilename string, numCommits int) error
}

// app implements Application.
type app struct{}

// New return a new instance of App.
func New() Application {
	return app{}
}

// regressionBatchSize is the size of batches used when backing up Regressions.
const regressionBatchSize = 1000

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

// ConfigCreatePubSubTopics creates the PubSub topics for the given config.
func (app) ConfigCreatePubSubTopics(instanceConfig *config.InstanceConfig) error {
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

// DatabaseMigrate applies any migrations to the database.
func (app) DatabaseMigrate(instanceConfig *config.InstanceConfig) error {

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

// DatabaseBackupAlerts backs up alerts from the database.
func (app) DatabaseBackupAlerts(local bool, instanceConfig *config.InstanceConfig, outputFile string) error {
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

// DatabaseBackupShortcuts backs up shortcuts from the database.
func (app) DatabaseBackupShortcuts(local bool, instanceConfig *config.InstanceConfig, outputFile string) error {
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

// DatabaseBackupRegressions backs up Regressions from the database, along with
// all the shortcuts that are mentioned in those backed up regressions.
func (app) DatabaseBackupRegressions(local bool, instanceConfig *config.InstanceConfig, outputFile, backupTo string) error {
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

// findFileInZip finds and opens a file within a .zip archive and returns the
// io.ReadCloser for it.
func findFileInZip(filename string, z *zip.ReadCloser) (io.ReadCloser, error) {
	var zipFile *zip.File
	for _, zipReader := range z.File {
		if zipReader.Name == filename {
			zipFile = zipReader
		}
	}
	if zipFile == nil {
		return nil, skerr.Fmt("Could not find an %q file in the backup", filename)
	}
	alertsZipReader, err := zipFile.Open()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return alertsZipReader, nil

}

// DatabaseRestoreAlerts restores Alerts to the database.
func (app) DatabaseRestoreAlerts(local bool, instanceConfig *config.InstanceConfig, inputFile string) error {
	ctx := context.Background()

	z, err := zip.OpenReader(inputFile)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.Close(z)

	alertStore, err := builders.NewAlertStoreFromConfig(ctx, local, instanceConfig)
	if err != nil {
		return skerr.Wrap(err)
	}
	alertsZipReader, err := findFileInZip(backupFilenameAlerts, z)
	if err != nil {
		return skerr.Wrap(err)
	}

	decoder := gob.NewDecoder(alertsZipReader)
	for {
		var alert alerts.Alert
		err := decoder.Decode(&alert)
		if err == io.EOF {
			break
		}
		if err != nil {
			return skerr.Wrap(err)
		}
		if err := alertStore.Save(ctx, &alert); err != nil {
			return skerr.Wrap(err)
		}
		fmt.Printf("Alerts: %q\n", alert.DisplayName)
	}
	return nil
}

// DatabaseRestoreShortcuts restores shortcuts to the database.
func (app) DatabaseRestoreShortcuts(local bool, instanceConfig *config.InstanceConfig, inputFile string) error {
	ctx := context.Background()

	z, err := zip.OpenReader(inputFile)
	if err != nil {
		return err
	}
	defer util.Close(z)

	// Restore shortcuts.
	shortcutStore, err := builders.NewShortcutStoreFromConfig(ctx, local, instanceConfig)
	if err != nil {
		return err
	}

	shortcutsZipReader, err := findFileInZip(backupFilenameShortcuts, z)
	if err != nil {
		return err
	}

	total := 0
	fmt.Print("Shortcuts: ")
	shortcutDecoder := gob.NewDecoder(shortcutsZipReader)
	for {
		var s shortcutWithID
		err := shortcutDecoder.Decode(&s)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		id, err := shortcutStore.InsertShortcut(ctx, s.Shortcut)
		if err != nil {
			return err
		}
		if id != s.ID {
			fmt.Printf("Failed to get a consistent id: %q != %q: %#v", id, s.ID, s.Shortcut)
		}
		total++
		if total%100 == 0 {
			fmt.Print(".")
		}
	}
	fmt.Println()
	return nil
}

// DatabaseRestoreRegressions restores Regressions to the database.
func (app) DatabaseRestoreRegressions(local bool, instanceConfig *config.InstanceConfig, inputFile string) error {
	ctx := context.Background()
	z, err := zip.OpenReader(inputFile)
	if err != nil {
		return err
	}
	defer util.Close(z)

	// Restore Regressions
	regressionStore, err := builders.NewRegressionStoreFromConfig(ctx, local, instanceConfig)
	if err != nil {
		return err
	}

	// Also re-create the shortcuts in each regression.
	shortcutStore, err := builders.NewShortcutStoreFromConfig(ctx, local, instanceConfig)
	if err != nil {
		return err
	}

	// Find "regressions"
	regressionsZipReader, err := findFileInZip(backupFilenameRegressions, z)
	if err != nil {
		return err
	}

	total := 0
	fmt.Print("Regressions: ")
	regresssionsDecoder := gob.NewDecoder(regressionsZipReader)
	for {
		var a allRegressionsForCommitWithCommitNumber
		err := regresssionsDecoder.Decode(&a)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		err = regressionStore.Write(ctx, map[types.CommitNumber]*regression.AllRegressionsForCommit{
			a.CommitNumber: a.AllRegressionsForCommit,
		})
		if err != nil {
			return err
		}
		total++
		if total%100 == 0 {
			fmt.Print(".")
		}
		// Re-create the shortcuts. Each regression already shorts the shortcut
		// key, so there's no need to modify them and we can ignore the ID
		// returned by InsertShortcut since the ID generation is deterministic
		// and won't change.
		for _, r := range a.AllRegressionsForCommit.ByAlertID {
			if r.High != nil {
				_, err = shortcutStore.InsertShortcut(ctx, &shortcut.Shortcut{
					Keys: r.High.Keys,
				})
				if err != nil {
					sklog.Warningf("Failed to create shortcut: %s", err)
				}
			}
			if r.Low != nil {
				_, err = shortcutStore.InsertShortcut(ctx, &shortcut.Shortcut{
					Keys: r.Low.Keys,
				})
				if err != nil {
					sklog.Warningf("Failed to create shortcut: %s", err)
				}
			}
		}
		if total%100 == 0 {
			fmt.Print("o")
		}
	}
	fmt.Printf("\nRestored: %d", total)

	return nil
}

// TilesLast prints the most recent tile index.
func (app) TilesLast(store tracestore.TraceStore) error {
	tileNumber, err := store.GetLatestTile(context.Background())
	if err != nil {
		return err
	}
	fmt.Println(tileNumber)
	return nil
}

// TilesList prints N tiles and how many traces they each contain.
func (app) TilesList(store tracestore.TraceStore, num int) error {
	ctx := context.Background()

	latestTileNumber, err := store.GetLatestTile(ctx)
	if err != nil {
		return err
	}
	fmt.Println("tile\tnum traces")
	for tileNumber := latestTileNumber; tileNumber > latestTileNumber-types.TileNumber(num); tileNumber-- {
		count, err := store.TraceCount(ctx, tileNumber)
		if err != nil {
			return skerr.Wrapf(err, "failed to count traces for tile %d", tileNumber)
		}
		fmt.Printf("%d\t%d\n", tileNumber, count)
	}

	return nil
}

// TracesList list trace ids that match the given query in the given tile.
func (app) TracesList(store tracestore.TraceStore, queryString string, tileNumber types.TileNumber) error {
	if tileNumber == types.BadTileNumber {
		var err error
		tileNumber, err = store.GetLatestTile(context.Background())
		if err != nil {
			return err
		}
	}
	values, err := url.ParseQuery(queryString)
	if err != nil {
		return err
	}
	q, err := query.New(values)
	if err != nil {
		return err
	}
	ts, err := store.QueryTraces(context.Background(), tileNumber, q)
	if err != nil {
		return err
	}
	for id, trace := range ts {
		fmt.Println(id, trace)
	}
	return nil
}

// TracesExport exports the matching traces and their values as JSON.
func (app) TracesExport(store tracestore.TraceStore, queryString string, begin, end types.CommitNumber, outputFile string) error {
	ctx := context.Background()

	// If --end is unspecified then just return values for the --begin commit.
	if end == types.BadCommitNumber {
		end = begin
	}

	values, err := url.ParseQuery(queryString)
	if err != nil {
		return err
	}
	q, err := query.New(values)
	if err != nil {
		return err
	}

	// First get all the trace names for the given query.
	tileNumber := types.TileNumberFromCommitNumber(begin, store.TileSize())
	ch, err := store.QueryTracesIDOnly(ctx, tileNumber, q)
	if err != nil {
		return err
	}
	traceNames := []string{}
	for p := range ch {
		traceName, err := query.MakeKey(p)
		if err != nil {
			sklog.Warningf("Invalid trace name found in query response: %s", err)
			continue
		}
		traceNames = append(traceNames, traceName)
	}

	// Now read the values for the trace names.
	ts, err := store.ReadTracesForCommitRange(ctx, traceNames, begin, end)
	if err != nil {
		return err
	}

	// Write the JSON results.
	if outputFile != "" {
		return util.WithWriteFile(outputFile, func(w io.Writer) error {
			return json.NewEncoder(w).Encode(ts)
		})
	}
	return json.NewEncoder(os.Stdout).Encode(ts)
}

// IngestForceReingest forces data to be reingested over the given time range.
func (app) IngestForceReingest(local bool, instanceConfig *config.InstanceConfig, start, stop string, dryrun bool) error {
	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(local, storage.ScopeReadOnly)
	if err != nil {
		return skerr.Wrap(err)
	}

	pubSubClient, err := pubsub.NewClient(ctx, instanceConfig.IngestionConfig.SourceConfig.Project, option.WithTokenSource(ts))
	if err != nil {
		return skerr.Wrap(err)
	}
	topic := pubSubClient.Topic(instanceConfig.IngestionConfig.SourceConfig.Topic)

	now := time.Now()
	startTime := now.Add(-7 * 24 * time.Hour)
	if start != "" {
		startTime, err = time.Parse("2006-01-02", start)
		if err != nil {
			return skerr.Wrap(err)
		}
	}
	stopTime := now
	if stop != "" {
		stopTime, err = time.Parse("2006-01-02", stop)
		if err != nil {
			return skerr.Wrap(err)
		}
	}

	client := httputils.DefaultClientConfig().WithTokenSource(ts).WithoutRetries().Client()
	gcsClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		return skerr.Wrap(err)
	}
	for _, prefix := range instanceConfig.IngestionConfig.SourceConfig.Sources {
		u, err := url.Parse(prefix)
		if err != nil {
			return skerr.Wrap(err)
		}

		dirs := fileutil.GetHourlyDirs(u.Path[1:], startTime.Unix(), stopTime.Unix())
		for _, dir := range dirs {
			sklog.Infof("Directory: %q", dir)
			err := gcs.AllFilesInDir(gcsClient, u.Host, dir, func(item *storage.ObjectAttrs) {
				// The PubSub event data is a JSON serialized storage.ObjectAttrs object.
				// See https://cloud.google.com/storage/docs/pubsub-notifications#payload
				sklog.Infof("File: %q", item.Name)
				b, err := json.Marshal(storage.ObjectAttrs{
					Name:   item.Name,
					Bucket: u.Host,
				})
				if err != nil {
					sklog.Errorf("Failed to serialize event: %s", err)
					return
				}
				if dryrun {
					fmt.Println(item.Name, item.Bucket)
					return
				}
				topic.Publish(ctx, &pubsub.Message{
					Data: b,
				})
			})
			if err != nil {
				return skerr.Wrap(err)
			}
		}
	}
	return nil
}

// TrybotReference implements the Application interface.
func (app) TrybotReference(local bool, store tracestore.TraceStore, instanceConfig *config.InstanceConfig, trybotFilename string, outputFilename string, numCommits int) error {
	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(local, storage.ScopeReadOnly, pubsub.ScopePubSub)
	if err != nil {
		return skerr.Wrap(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).WithoutRetries().Client()
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		return skerr.Wrap(err)
	}

	ingestParser := parser.New(instanceConfig)
	u, err := url.Parse(trybotFilename)
	if err != nil {
		return skerr.Wrapf(err, "Trybot filename must be a full GCS url, e.g. gs://...")
	}
	gcsClient := gcsclient.New(storageClient, u.Host)
	fileLoader := gcssamplesloader.New(gcsClient, ingestParser)

	// Load the given file trybot file.
	trybotSampleSet, err := fileLoader.Load(ctx, trybotFilename)
	if err != nil {
		return skerr.Wrapf(err, "Failed to load file.")
	}

	// Get a slice of all the traceIDs in the file.
	traceIDs := make([]string, 0, len(trybotSampleSet))
	for traceID := range trybotSampleSet {
		traceIDs = append(traceIDs, traceID)
	}

	if len(traceIDs) == 0 {
		return skerr.Fmt("No samples found in file: %q", trybotFilename)
	}

	// Sort to make debugging easier.
	sort.Strings(traceIDs)

	// Find that last N ingested files that correspond to the trybot file.
	lastNSources, err := store.GetLastNSources(ctx, traceIDs[0], numCommits)
	if err != nil {
		return skerr.Wrapf(err, "Failed to load sources.")
	}

	// Load each of those source files and add to a common SampleSet.
	sampleSet := parser.SamplesSet{}
	for _, sourceAndCommit := range lastNSources {
		ss, err := fileLoader.Load(ctx, sourceAndCommit.Filename)
		if err != nil {
			return skerr.Wrap(err)
		}
		sampleSet.Add(ss)
	}

	// Create a synthetic nanobench file for the output.
	b := format.BenchData{
		Results: map[string]format.BenchResults{},
	}
	for _, samples := range sampleSet {
		testName := samples.Params["test"]
		configName := samples.Params["config"]
		testResults, ok := b.Results[testName]
		if !ok {
			testResults = format.BenchResults{}
			b.Results[testName] = testResults
		}
		testResults[configName] = map[string]interface{}{
			"samples": samples.Values,
			"options": samples.Params,
		}
	}

	// Write the output file.
	return util.WithWriteFile(outputFilename, func(w io.Writer) error {
		return json.NewEncoder(w).Encode(b)
	})
}

// Confirm app implements App.
var _ Application = app{}
