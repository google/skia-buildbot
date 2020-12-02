// Command-line application for interacting with Perf.
package main

import (
	"context"
	"fmt"
	"os"

	cli "github.com/urfave/cli/v2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sklog/glog_and_cloud"
	"go.skia.org/infra/perf/go/builders"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/perf-tool/app"
	"go.skia.org/infra/perf/go/tracestore"
)

/*
// flags
var (
    tileListNumFlag int32

    indicesTileFlag types.TileNumber
    tracesTileFlag  types.TileNumber

    tracesBeginFlag    types.CommitNumber
    tracesEndFlag      types.CommitNumber
    tracesFilenameFlag string

    tracesQueryFlag string

    ingestStartFlag  string
    ingestEndFlag    string
    ingestDryrunFlag bool
)

const (
    connectionStringFlagName string = "connection_string"
    outputFilenameFlagName   string = "out"
    inputFilenameFlag    string = "in"
    backupToDateFlagName     string = "backup_to_date"
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
*/
const (
	backupToDateFlagName     = "backup_to_date"
	configFilenameFlagName   = "config_filename"
	connectionStringFlagName = "connection_string"
	inputFilenameFlagName    = "in"
	localFlagName            = "local"
	outputFilenameFlagName   = "out"
)

var connectionStringFlag = &cli.StringFlag{
	Name:    connectionStringFlagName,
	Value:   "",
	Usage:   "Override the connection string in the config file.",
	EnvVars: []string{"PERF_CONNECTION_STRING"},
}

var outputFilenameFlag = &cli.StringFlag{
	Name:     outputFilenameFlagName,
	Value:    "",
	Usage:    "The backup is written to this file.",
	Required: true,
}

var inputFilenameFlag = &cli.StringFlag{
	Name:     inputFilenameFlagName,
	Value:    "",
	Usage:    "The backup is restored from this file.",
	Required: true,
}

var backupToDateFlag = &cli.StringFlag{
	Name:  backupToDateFlagName,
	Value: "",
	Usage: "How far back in time to back up Regressions. Defaults to four weeks.",
}

var configFilenameFlag = &cli.StringFlag{
	Name:     configFilenameFlagName,
	Value:    "",
	Usage:    "Load configuration from `FILE`",
	EnvVars:  []string{"PERF_CONFIG_FILENAME"},
	Required: true,
}

var localFlag = &cli.BoolFlag{
	Name:  localFlagName,
	Value: true,
	Usage: "If true then use gcloud credentials.",
}

// instanceConfigFromFlags returns an InstanceConfig based
// on the flags configFilenameFlag and connectionStringFlag.
func instanceConfigFromFlags(c *cli.Context) (*config.InstanceConfig, error) {
	instanceConfig, err := config.InstanceConfigFromFile(configFilenameFlagName)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	override := c.String(connectionStringFlagName)
	if override != "" {
		instanceConfig.DataStoreConfig.ConnectionString = override
	}
	return instanceConfig, nil
}

func getStore(c *cli.Context) (tracestore.TraceStore, error) {
	instanceConfig, err := instanceConfigFromFlags(c)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	local := c.Bool(localFlagName)
	traceStore, err := builders.NewTraceStoreFromConfig(context.Background(), local, instanceConfig)
	if err != nil {
		sklog.Fatalf("Failed to create client: %s", err)
	}
	return traceStore, nil
}

func main() {
	glog_and_cloud.SetLogger(glog_and_cloud.NewStdErrCloudLogger(glog_and_cloud.SLogStderr))

	cliApp := &cli.App{
		Name:  "perf-tool",
		Usage: "Command-line tool for working with Perf data.",
		Commands: []*cli.Command{
			{
				Name: "config",
				Subcommands: []*cli.Command{
					{
						Name:  "create-pubsub-topics",
						Usage: "Create PubSub topics for the given big_table_config.",
						Flags: []cli.Flag{
							configFilenameFlag,
							connectionStringFlag,
						},
						Action: func(c *cli.Context) error {
							instanceConfig, err := instanceConfigFromFlags(c)
							if err != nil {
								return skerr.Wrap(err)
							}
							return app.ConfigCreatePubSubTopicsAction(instanceConfig)
						},
					},
				},
			},
			{
				Name: "tiles",
				Subcommands: []*cli.Command{
					{
						Name:  "last",
						Usage: "Prints the index of the last (most recent) tile.",
						Flags: []cli.Flag{
							localFlag,
							configFilenameFlag,
							connectionStringFlag,
						},
						Action: func(c *cli.Context) error {
							store, err := getStore(c)
							if err != nil {
								return skerr.Wrap(err)
							}
							return app.TilesLast(store)
						},
					},
				},
			},
			{
				Name: "database",
				Subcommands: []*cli.Command{
					{
						Name:  "migrate",
						Usage: "Migrate the database to the latest version of the schema.",
						Flags: []cli.Flag{
							configFilenameFlag,
							connectionStringFlag,
						},
						Action: func(c *cli.Context) error {
							instanceConfig, err := instanceConfigFromFlags(c)
							if err != nil {
								return skerr.Wrap(err)
							}
							return app.DatabaseMigrateSubAction(instanceConfig)
						},
					},
					{
						Name: "backup",
						Subcommands: []*cli.Command{
							{
								Name: "alerts",
								Flags: []cli.Flag{
									localFlag,
									configFilenameFlag,
									connectionStringFlag,
									outputFilenameFlag,
								},
								Action: func(c *cli.Context) error {
									instanceConfig, err := instanceConfigFromFlags(c)
									if err != nil {
										return skerr.Wrap(err)
									}
									return app.DatabaseBackupAlerts(c.Bool(localFlagName), instanceConfig, c.String(outputFilenameFlagName))
								},
							},
							{
								Name: "shortcuts",
								Flags: []cli.Flag{
									localFlag,
									configFilenameFlag,
									connectionStringFlag,
									outputFilenameFlag,
								},
								Action: func(c *cli.Context) error {
									instanceConfig, err := instanceConfigFromFlags(c)
									if err != nil {
										return skerr.Wrap(err)
									}

									return app.BackupShortcuts(c.Bool(localFlagName), instanceConfig, c.String(outputFilenameFlagName))
								},
							},
							{
								Name: "regressions",
								Flags: []cli.Flag{
									localFlag,
									configFilenameFlag,
									connectionStringFlag,
									outputFilenameFlag,
									backupToDateFlag,
								},
								Description: `Backups up regressions and any shortcuts they rely on.

When restoring you must restore twice, first:

    'perf-tool database restore regressions'

and then:

    'perf-tool database restore shortcuts'

using the same input file for both restores.
                                 `,
								Action: func(c *cli.Context) error {
									instanceConfig, err := instanceConfigFromFlags(c)
									if err != nil {
										return skerr.Wrap(err)
									}

									return app.DatabaseBackupRegressions(c.Bool(localFlagName), instanceConfig, c.String(outputFilenameFlagName), c.String(backupToDateFlagName))
								},
							},
						},
					},
					{
						Name: "restore",
						Subcommands: []*cli.Command{
							{
								Name: "alerts",
								Flags: []cli.Flag{
									localFlag,
									configFilenameFlag,
									connectionStringFlag,
									inputFilenameFlag,
								},
								Description: "Restores the alerts from the given file.",
								Action: func(c *cli.Context) error {
									instanceConfig, err := instanceConfigFromFlags(c)
									if err != nil {
										return skerr.Wrap(err)
									}
									return app.DatabaseRestoreAlerts(c.Bool(localFlagName), instanceConfig, c.String(outputFilenameFlagName))
								},
							},
							{
								Name: "shortcuts",
								Flags: []cli.Flag{
									localFlag,
									configFilenameFlag,
									connectionStringFlag,
									inputFilenameFlag,
								},
								Description: "Restores the shortcuts from the given file.",
								Action: func(c *cli.Context) error {
									instanceConfig, err := instanceConfigFromFlags(c)
									if err != nil {
										return skerr.Wrap(err)
									}

									return app.DatabaseRestoreShortcuts(c.Bool(localFlagName), instanceConfig, c.String(outputFilenameFlagName))
								},
							},
							{
								Name: "regressions",
								Flags: []cli.Flag{
									localFlag,
									configFilenameFlag,
									connectionStringFlag,
									inputFilenameFlag,
								},
								Description: "Restores from the given backup both the regressions and their associated shortcuts.",
								Action: func(c *cli.Context) error {
									instanceConfig, err := instanceConfigFromFlags(c)
									if err != nil {
										return skerr.Wrap(err)
									}

									return app.DatabaseRestoreRegressions(c.Bool(localFlagName), instanceConfig, c.String(outputFilenameFlagName))
								},
							},
						},
					},
				},
			},
		},
	}
	cliApp.EnableBashCompletion = true

	err := cliApp.Run(os.Args)
	if err != nil {
		fmt.Printf("\nError: %s\n", err.Error())
		os.Exit(2)
	}
	/*





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

	   tracesCmd.PersistentFlags().StringVar(&tracesQueryFlag, "query", "", "The query to run. Defaults to the empty query which matches all traces.")
	   tracesCmd.PersistentFlags().String(connectionStringFlag, "", "Override the connection_string in the config file.")

	   tracesListByIndexCmd := &cobra.Command{
	       Use:   "list",
	       Short: "Prints the IDs of traces in the last (most recent) tile, or the tile specified by the --tile flag, that match --query.",
	       RunE:  tracesListByIndexAction,
	   }
	   tracesListByIndexCmd.PersistentFlags().Int32Var((*int32)(&tracesTileFlag), "tile", -1, "The tile to query")

	   tracesExportCmd := &cobra.Command{
	       Use:   "export",
	       Short: "Writes a JSON files with the traces that match --query for the given range of commits.",
	       RunE:  tracesExportAction,
	   }
	   tracesExportCmd.PersistentFlags().Int32Var((*int32)(&tracesBeginFlag), "begin", -1, "The index of the first commit.")
	   tracesExportCmd.PersistentFlags().Int32Var((*int32)(&tracesEndFlag), "end", -1, "The index of the last commit. If not specified then only the values at --begin are returned.")
	   tracesExportCmd.PersistentFlags().StringVar(&tracesFilenameFlag, "filename", "", "The name of the file to write the results, defaults to stdout if unspecified.")

	   tracesCmd.AddCommand(
	       tracesListByIndexCmd,
	       tracesExportCmd,
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
	       tilesCmd,
	       tracesCmd,
	       ingestCmd,
	   )

	   if err := cmd.Execute(); err != nil {
	       fmt.Println(err)
	       os.Exit(1)
	   }
	*/

}

/*

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
    fmt.Printf("Backing up from %v\n", backupToDate)

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
    regressionStore, err := builders.NewRegressionStoreFromConfig(ctx, local, instanceConfig)
    if err != nil {
        return err
    }

    // Get the latest commit.
    end, err := perfGit.CommitNumberFromTime(ctx, time.Time{})
    if err != nil {
        return err
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
                continue
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

func tilesLastAction(c *cobra.Command, args []string) error {
    updateInstanceConfigWithOverride(c)
    tileNumber, err := mustGetStore().GetLatestTile(context.Background())
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

    latestTileNumber, err := store.GetLatestTile(ctx)
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
        tileNumber, err = store.GetLatestTile(context.Background())
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
    ts, err := store.QueryTraces(context.Background(), tileNumber, q)
    if err != nil {
        return err
    }
    for id, trace := range ts {
        fmt.Println(id, trace)
    }
    return nil
}

func tracesExportAction(c *cobra.Command, args []string) error {
    ctx := context.Background()

    if tracesBeginFlag == types.BadCommitNumber {
        return fmt.Errorf("The --begin flag is required.")
    }

    // If --end is unspecified then just return values for the --begin commit.
    if tracesEndFlag == types.BadCommitNumber {
        tracesEndFlag = tracesBeginFlag
    }

    updateInstanceConfigWithOverride(c)
    store := mustGetStore()
    values, err := url.ParseQuery(tracesQueryFlag)
    if err != nil {
        return err
    }
    q, err := query.New(values)
    if err != nil {
        return err
    }

    // First get all the trace names for the given query.
    tileNumber := types.TileNumberFromCommitNumber(tracesBeginFlag, store.TileSize())
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
    ts, err := store.ReadTracesForCommitRange(ctx, traceNames, tracesBeginFlag, tracesEndFlag)
    if err != nil {
        return err
    }

    // Write the JSON results.
    if tracesFilenameFlag != "" {
        return util.WithWriteFile(tracesFilenameFlag, func(w io.Writer) error {
            return json.NewEncoder(w).Encode(ts)
        })
    } else {
        return json.NewEncoder(os.Stdout).Encode(ts)
    }
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
*/
