// Command-line application for interacting with BigTable backed Perf storage.
package main

import (
	"archive/zip"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/spf13/cobra"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sklog/glog_and_cloud"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/builders"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/sql/migrations"
	"go.skia.org/infra/perf/go/sql/migrations/cockroachdb"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/types"
	"google.golang.org/api/option"
)

var (
	traceStore     tracestore.TraceStore
	configFilename string
	instanceConfig *config.InstanceConfig
	local          bool
)

// flags
var (
	tileListNumFlag int32

	indicesTileFlag types.TileNumber
	tracesTileFlag  types.TileNumber

	tracesQueryFlag string

	ingestStartFlag  string
	ingestEndFlag    string
	ingestDryrunFlag bool
)

const (
	connectionStringFlag string = "connection_string"
	outputFilenameFlag   string = "out"
	inputFilenameFlag    string = "in"
	backupToDateFlag     string = "backup_to_date"
	databaseLocalFlag    string = "local"

	regressionBatchSize = 1000
)

func mustGetStore() tracestore.TraceStore {
	if traceStore != nil {
		return traceStore
	}
	var err error
	traceStore, err = builders.NewTraceStoreFromConfig(context.Background(), local, instanceConfig)
	if err != nil {
		sklog.Fatalf("Failed to create client: %s", err)
	}
	return traceStore
}

func main() {
	cmd := cobra.Command{
		Use: "perf-tool [sub]",
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			glog_and_cloud.SetLogger(glog_and_cloud.NewStdErrCloudLogger(glog_and_cloud.SLogStderr))

			if configFilename == "" {
				return skerr.Fmt("The --config_filename flag is required.")
			}
			var err error
			instanceConfig, err = config.InstanceConfigFromFile(configFilename)
			if err != nil {
				return skerr.Wrap(err)
			}
			config.Config = instanceConfig

			return nil
		},
	}
	cmd.PersistentFlags().StringVar(&configFilename, "config_filename", "", "The filename of the config file to use.")
	cmd.PersistentFlags().BoolVar(&local, "local", true, "If true then use glcloud credentials.")

	configCmd := &cobra.Command{
		Use: "config [sub]",
	}
	configPubSubCmd := &cobra.Command{
		Use:   "create-pubsub-topics",
		Short: "Create PubSub topics for the given big_table_config.",
		RunE:  configCreatePubSubTopicsAction,
	}
	configCmd.AddCommand(configPubSubCmd)

	databaseCmd := &cobra.Command{
		Use: "database [sub]",
	}
	databaseCmd.PersistentFlags().String(connectionStringFlag, "", "Override the connection_string in the config file.")

	databaseMigrateSubCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate the database to the latest version of the schema.",
		RunE:  databaseMigrateSubAction,
	}

	databaseBackupSubCmd := &cobra.Command{
		Use: "backup [sub]",
	}
	databaseBackupSubCmd.PersistentFlags().String(outputFilenameFlag, "", "The output filename")

	databaseBackupAlertsSubCmd := &cobra.Command{
		Use:   "alerts",
		Short: "Backup Alerts.",
		RunE:  databaseDatabaseBackupAlertsSubAction,
	}
	databaseBackupSubCmd.AddCommand(databaseBackupAlertsSubCmd)

	databaseBackupShortcutsSubCmd := &cobra.Command{
		Use:   "shortcuts",
		Short: "Backup Shortcuts.",
		RunE:  databaseDatabaseBackupShortcutsSubAction,
	}
	databaseBackupSubCmd.AddCommand(databaseBackupShortcutsSubCmd)

	databaseBackupRegressionsSubCmd := &cobra.Command{
		Use:   "regressions",
		Short: "Backups up regressions and any shortcuts they rely on.",
		Long: `Backups up regressions and any shortcuts they rely on.

When restoring you must restore twice, first

    'perf-tool database restore regressions'

and then

    'perf-tool database restore shortcuts'

using the same input file for both restores.
 `,
		RunE: databaseDatabaseBackupRegressionsSubAction,
	}
	databaseBackupRegressionsSubCmd.Flags().String(backupToDateFlag, "", "How far back in time to back up Regressions. Defaults to four weeks.")
	databaseBackupSubCmd.AddCommand(databaseBackupRegressionsSubCmd)

	databaseRestoreSubCmd := &cobra.Command{
		Use: "restore [sub]",
	}
	databaseRestoreSubCmd.PersistentFlags().String(inputFilenameFlag, "", "The output filename")

	databaseRestoreAlertsSubCmd := &cobra.Command{
		Use:   "alerts",
		Short: "Restores from the given backup.",
		RunE:  databaseDatabaseRestoreAlertsSubAction,
	}
	databaseRestoreSubCmd.AddCommand(databaseRestoreAlertsSubCmd)

	databaseRestoreShortcutsSubCmd := &cobra.Command{
		Use:   "shortcuts",
		Short: "Restores from the given backup.",
		RunE:  databaseDatabaseRestoreShortcutsSubAction,
	}
	databaseRestoreSubCmd.AddCommand(databaseRestoreShortcutsSubCmd)

	databaseRestoreRegressionsSubCmd := &cobra.Command{
		Use:   "regressions",
		Short: "Restores from the given backup both the regressions and their associated shortcuts.",
		RunE:  databaseDatabaseRestoreRegressionsSubAction,
	}
	databaseRestoreSubCmd.AddCommand(databaseRestoreRegressionsSubCmd)

	databaseCmd.AddCommand(
		databaseMigrateSubCmd,
		databaseBackupSubCmd,
		databaseRestoreSubCmd)

	indicesCmd := &cobra.Command{
		Use: "indices [sub]",
	}
	indicesCmd.PersistentFlags().Int32Var((*int32)(&indicesTileFlag), "tile", -1, "The tile to query")
	indicesCountCmd := &cobra.Command{
		Use:   "count",
		Short: "Counts the number of index rows.",
		Long:  "Counts the index rows for the last (most recent) tile, or the tile specified by --tile.",
		RunE:  indicesCountAction,
	}
	indicesWriteCmd := &cobra.Command{
		Use:   "write",
		Short: "Write indices",
		Long:  "Rewrites the indices for the last (most recent) tile, or the tile specified by --tile.",
		RunE:  indicesWriteAction,
	}
	indicesWriteAllCmd := &cobra.Command{
		Use:   "write-all",
		Short: "Write indices for all tiles.",
		Long:  "Rewrites the indices for all tiles, --tiles is ignored. Starts with latest tile and keeps moving to previous tiles until it finds a tile with no traces.",
		RunE:  indicesWriteAllAction,
	}
	indicesWriteCmd.Flags().Int32Var((*int32)(&indicesTileFlag), "tile", -1, "The tile to query")

	indicesCmd.AddCommand(
		indicesCountCmd,
		indicesWriteCmd,
		indicesWriteAllCmd,
	)

	tilesCmd := &cobra.Command{
		Use: "tiles [sub]",
	}
	tilesCmd.PersistentFlags().String(connectionStringFlag, "", "Override the connection_string in the config file.")

	tilesLast := &cobra.Command{
		Use:   "last",
		Short: "Prints the offset of the last (most recent) tile.",
		RunE:  tilesLastAction,
	}
	tilesList := &cobra.Command{
		Use:   "list",
		Short: "Prints the last N tiles and the number of traces they contain.",
		RunE:  tilesListAction,
	}
	tilesList.Flags().Int32Var(&tileListNumFlag, "num", 10, "The number of tiles to display.")

	tilesCmd.AddCommand(
		tilesLast,
		tilesList,
	)

	tracesCmd := &cobra.Command{
		Use: "traces [sub]",
	}
	tracesCmd.PersistentFlags().Int32Var((*int32)(&tracesTileFlag), "tile", -1, "The tile to query")
	tracesCmd.PersistentFlags().StringVar(&tracesQueryFlag, "query", "", "The query to run. Defaults to the empty query which matches all traces.")

	tracesListByIndexCmd := &cobra.Command{
		Use:   "list",
		Short: "Prints the IDs of traces in the last (most recent) tile, or the tile specified by the --tile flag, that match --query.",
		RunE:  tracesListByIndexAction,
	}

	tracesCmd.AddCommand(
		tracesListByIndexCmd,
	)

	ingestCmd := &cobra.Command{
		Use: "ingest [sub]",
	}

	ingestForceReingestCmd := &cobra.Command{
		Use:   "force-reingest",
		Short: "Force re-ingestion of files.",
		RunE:  ingestForceReingestAction,
	}

	ingestForceReingestCmd.Flags().StringVar(&ingestStartFlag, "start", "", "Start the ingestion at this time, of the form: 2006-01-02. Default to one week ago.")
	ingestForceReingestCmd.Flags().StringVar(&ingestEndFlag, "end", "", "Ingest up to this time, of the form: 2006-01-02. Defaults to now.")
	ingestForceReingestCmd.Flags().BoolVar(&ingestDryrunFlag, "dryrun", false, "Just display the list of files to send.")

	ingestCmd.AddCommand(ingestForceReingestCmd)

	cmd.AddCommand(
		configCmd,
		databaseCmd,
		indicesCmd,
		tilesCmd,
		tracesCmd,
		ingestCmd,
	)

	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

}

// The filenames we use inside the backup .zip files.
const (
	backupFilenameAlerts      = "alerts"
	backupFilenameShortcuts   = "shortcuts"
	backupFilenameRegressions = "regressions"
)

func updateInstanceConfigWithOverride(c *cobra.Command) {
	connectionStringOverride := c.Flag(connectionStringFlag).Value.String()
	if connectionStringOverride != "" {
		instanceConfig.DataStoreConfig.ConnectionString = connectionStringOverride
	}
}

func databaseMigrateSubAction(c *cobra.Command, args []string) error {
	updateInstanceConfigWithOverride(c)

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

func databaseDatabaseBackupAlertsSubAction(c *cobra.Command, args []string) error {
	ctx := context.Background()
	updateInstanceConfigWithOverride(c)
	if c.Flag(outputFilenameFlag).Value.String() == "" {
		return fmt.Errorf("The '-%s' flag is required.", outputFilenameFlag)
	}

	f, err := os.Create(c.Flag(outputFilenameFlag).Value.String())
	if err != nil {
		return err
	}
	defer util.Close(f)
	z := zip.NewWriter(f)

	alertStore, err := builders.NewAlertStoreFromConfig(ctx, local, instanceConfig)
	if err != nil {
		return err
	}
	alerts, err := alertStore.List(ctx, true)
	if err != nil {
		return err
	}
	alertsZipWriter, err := z.Create(backupFilenameAlerts)
	if err != nil {
		return err
	}
	encoder := gob.NewEncoder(alertsZipWriter)
	for _, alert := range alerts {
		fmt.Printf("Alert: %q\n", alert.DisplayName)
		if err := encoder.Encode(alert); err != nil {
			return err
		}
	}
	if err := z.Close(); err != nil {
		return err
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

func databaseDatabaseBackupShortcutsSubAction(c *cobra.Command, args []string) error {
	ctx := context.Background()
	updateInstanceConfigWithOverride(c)
	if c.Flag(outputFilenameFlag).Value.String() == "" {
		return fmt.Errorf("The '-%s' flag is required.", outputFilenameFlag)
	}

	f, err := os.Create(c.Flag(outputFilenameFlag).Value.String())
	if err != nil {
		return err
	}
	defer util.Close(f)
	z := zip.NewWriter(f)

	// Backup Shortcuts.
	shortcutsZipWriter, err := z.Create(backupFilenameShortcuts)
	if err != nil {
		return err
	}
	shortcutsEncoder := gob.NewEncoder(shortcutsZipWriter)

	shortcutStore, err := builders.NewShortcutStoreFromConfig(ctx, local, instanceConfig)
	if err != nil {
		return err
	}
	shortcutCh, err := shortcutStore.GetAll(ctx)
	if err != nil {
		return err
	}

	fmt.Print("Shortcuts: ")
	total := 0
	for s := range shortcutCh {
		if err := shortcutsEncoder.Encode(shortcutWithID{
			ID:       shortcut.IDFromKeys(s),
			Shortcut: s,
		}); err != nil {
			return err
		}
		total++
		if total%100 == 0 {
			fmt.Print(".")
		}
	}
	fmt.Println()

	if err := z.Close(); err != nil {
		return err
	}

	return nil
}

// allRegressionsForCommitWithCommitNumber is the struct we actually write into
// the backup.
type allRegressionsForCommitWithCommitNumber struct {
	CommitNumber            types.CommitNumber
	AllRegressionsForCommit *regression.AllRegressionsForCommit
}

func databaseDatabaseBackupRegressionsSubAction(c *cobra.Command, args []string) error {
	ctx := context.Background()
	updateInstanceConfigWithOverride(c)
	if c.Flag(outputFilenameFlag).Value.String() == "" {
		return fmt.Errorf("The '-%s' flag is required.", outputFilenameFlag)
	}

	var backupToDate time.Time
	if c.Flag(backupToDateFlag).Value.String() == "" {
		backupToDate = time.Now().Add(-time.Hour * 24 * 7 * 4)
	} else {
		var err error
		backupToDate, err = time.Parse("2006-01-02", c.Flag(backupToDateFlag).Value.String())
		if err != nil {
			return err
		}
	}
	fmt.Printf("Backing up from %v", backupToDate)

	f, err := os.Create(c.Flag(outputFilenameFlag).Value.String())
	if err != nil {
		return err
	}
	defer util.Close(f)
	z := zip.NewWriter(f)

	// Backup Regressions.
	regressionsZipWriter, err := z.Create(backupFilenameRegressions)
	if err != nil {
		return err
	}

	regresssionsEncoder := gob.NewEncoder(regressionsZipWriter)
	perfGit, err := builders.NewPerfGitFromConfig(ctx, local, instanceConfig)
	if err != nil {
		return err
	}
	cidl := cid.New(ctx, perfGit, instanceConfig)
	regressionStore, err := builders.NewRegressionStoreFromConfig(ctx, local, cidl, instanceConfig)
	if err != nil {
		return err
	}

	// Get the latest commit.
	end, err := perfGit.CommitNumberFromTime(ctx, time.Time{})
	if err != nil {
		return err
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
			return err
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
				return err
			}
			commitDate := time.Unix(cid.Timestamp, 0)
			if commitDate.Before(backupToDate) {
				fmt.Printf("Finished backup: %v < %v", commitDate, backupToDate)
				goto End
			}
			body := allRegressionsForCommitWithCommitNumber{
				CommitNumber:            commitNumber,
				AllRegressionsForCommit: allRegressionsForCommit,
			}
			if err := regresssionsEncoder.Encode(body); err != nil {
				return err
			}
		}
		end = begin - 1
	}
End:

	// Backup Shortcuts found in Regressions.
	shortcutsZipWriter, err := z.Create(backupFilenameShortcuts)
	if err != nil {
		return err
	}
	shortcutsEncoder := gob.NewEncoder(shortcutsZipWriter)
	shortcutStore, err := builders.NewShortcutStoreFromConfig(ctx, local, instanceConfig)
	if err != nil {
		return err
	}

	fmt.Print("Shortcuts: ")
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
			return err
		}
		total++
		if total%100 == 0 {
			fmt.Print(".")
		}
	}

	if err := z.Close(); err != nil {
		return err
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
		return nil, err
	}
	return alertsZipReader, nil

}

func databaseDatabaseRestoreAlertsSubAction(c *cobra.Command, args []string) error {
	ctx := context.Background()
	updateInstanceConfigWithOverride(c)
	if c.Flag(inputFilenameFlag).Value.String() == "" {
		return fmt.Errorf("The '-%s' flag is required.", inputFilenameFlag)
	}

	z, err := zip.OpenReader(c.Flag(inputFilenameFlag).Value.String())
	if err != nil {
		return err
	}
	defer util.Close(z)

	alertStore, err := builders.NewAlertStoreFromConfig(ctx, local, instanceConfig)
	if err != nil {
		return err
	}
	alertsZipReader, err := findFileInZip(backupFilenameAlerts, z)
	if err != nil {
		return err
	}

	decoder := gob.NewDecoder(alertsZipReader)
	for {
		var alert alerts.Alert
		err := decoder.Decode(&alert)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err := alertStore.Save(ctx, &alert); err != nil {
			return err
		}
		fmt.Printf("Alerts: %q\n", alert.DisplayName)
	}
	return nil
}

func databaseDatabaseRestoreShortcutsSubAction(c *cobra.Command, args []string) error {
	ctx := context.Background()
	updateInstanceConfigWithOverride(c)
	if c.Flag(inputFilenameFlag).Value.String() == "" {
		return fmt.Errorf("The '-%s' flag is required.", inputFilenameFlag)
	}

	z, err := zip.OpenReader(c.Flag(inputFilenameFlag).Value.String())
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

func databaseDatabaseRestoreRegressionsSubAction(c *cobra.Command, args []string) error {
	ctx := context.Background()
	updateInstanceConfigWithOverride(c)
	if c.Flag(inputFilenameFlag).Value.String() == "" {
		return fmt.Errorf("The '-%s' flag is required.", inputFilenameFlag)
	}

	z, err := zip.OpenReader(c.Flag(inputFilenameFlag).Value.String())
	if err != nil {
		return err
	}
	defer util.Close(z)

	// Restore Regressions
	perfGit, err := builders.NewPerfGitFromConfig(ctx, local, instanceConfig)
	if err != nil {
		return err
	}
	cidl := cid.New(ctx, perfGit, instanceConfig)
	regressionStore, err := builders.NewRegressionStoreFromConfig(ctx, local, cidl, instanceConfig)
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

func tilesLastAction(c *cobra.Command, args []string) error {
	updateInstanceConfigWithOverride(c)
	tileNumber, err := mustGetStore().GetLatestTile()
	if err != nil {
		return err
	}
	fmt.Println(tileNumber)
	return nil
}

func tilesListAction(c *cobra.Command, args []string) error {
	ctx := context.Background()
	updateInstanceConfigWithOverride(c)
	store := mustGetStore()

	latestTileNumber, err := store.GetLatestTile()
	if err != nil {
		return err
	}
	fmt.Println("tile\tnum traces")
	for tileNumber := latestTileNumber; tileNumber > latestTileNumber-types.TileNumber(tileListNumFlag); tileNumber-- {
		count, err := store.TraceCount(ctx, tileNumber)
		if err != nil {
			return skerr.Wrapf(err, "failed to count traces for tile %d", tileNumber)
		}
		fmt.Printf("%d\t%d\n", tileNumber, count)
	}

	return nil
}

func tracesListByIndexAction(c *cobra.Command, args []string) error {
	var tileNumber types.TileNumber
	store := mustGetStore()
	if tracesTileFlag == -1 {
		var err error
		tileNumber, err = store.GetLatestTile()
		if err != nil {
			return err
		}
	} else {
		tileNumber = tracesTileFlag
	}
	values, err := url.ParseQuery(tracesQueryFlag)
	if err != nil {
		return err
	}
	q, err := query.New(values)
	if err != nil {
		return err
	}
	ts, err := store.QueryTracesByIndex(context.Background(), tileNumber, q)
	if err != nil {
		return err
	}
	for id, trace := range ts {
		fmt.Println(id, trace)
	}
	return nil
}

func indicesWriteAction(c *cobra.Command, args []string) error {
	store := mustGetStore()
	var tileNumber types.TileNumber
	if indicesTileFlag == -1 {
		var err error
		tileNumber, err = store.GetLatestTile()
		if err != nil {
			return fmt.Errorf("Failed to get latest tile: %s", err)
		}
	} else {
		tileNumber = indicesTileFlag
	}
	return store.WriteIndices(context.Background(), tileNumber)
}

func indicesWriteAllAction(c *cobra.Command, args []string) error {
	store := mustGetStore()
	tileNumber, err := store.GetLatestTile()
	if err != nil {
		return fmt.Errorf("Failed to get latest tile: %s", err)
	}
	for {
		if err := store.WriteIndices(context.Background(), tileNumber); err != nil {
			return err
		}
		sklog.Infof("Wrote index for tile %d", tileNumber)
		tileNumber = tileNumber.Prev()
		count, err := store.TraceCount(context.Background(), tileNumber)
		if err != nil {
			return err
		}
		if count == 0 {
			break
		}
	}
	return nil
}

func indicesCountAction(c *cobra.Command, args []string) error {
	store := mustGetStore()
	var tileNumber types.TileNumber
	if indicesTileFlag == -1 {
		var err error
		tileNumber, err = store.GetLatestTile()
		if err != nil {
			return fmt.Errorf("Failed to get latest tile: %s", err)
		}
	} else {
		tileNumber = indicesTileFlag
	}
	count, err := store.CountIndices(context.Background(), tileNumber)
	if err == nil {
		fmt.Println(count)
	}
	return err
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

func configCreatePubSubTopicsAction(c *cobra.Command, args []string) error {
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, instanceConfig.IngestionConfig.SourceConfig.Project)
	if err != nil {
		return err
	}
	if err := createPubSubTopic(ctx, client, instanceConfig.IngestionConfig.SourceConfig.Topic); err != nil {
		return err
	}
	if instanceConfig.IngestionConfig.FileIngestionTopicName != "" {
		if err := createPubSubTopic(ctx, client, instanceConfig.IngestionConfig.FileIngestionTopicName); err != nil {
			return err
		}
	}

	return nil
}

func ingestForceReingestAction(c *cobra.Command, args []string) error {
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
	if ingestStartFlag != "" {
		startTime, err = time.Parse("2006-01-02", ingestStartFlag)
		if err != nil {
			return skerr.Wrap(err)
		}
	}
	endTime := now
	if ingestEndFlag != "" {
		endTime, err = time.Parse("2006-01-02", ingestEndFlag)
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

		dirs := fileutil.GetHourlyDirs(u.Path[1:], startTime.Unix(), endTime.Unix())
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
				if ingestDryrunFlag {
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
