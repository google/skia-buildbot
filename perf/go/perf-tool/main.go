// Command-line application for interacting with Perf.
package main

import (
	"context"
	"fmt"
	"os"

	cli "github.com/urfave/cli/v2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog/nooplogging"
	"go.skia.org/infra/go/sklog/sklogimpl"
	"go.skia.org/infra/go/sklog/stdlogging"
	"go.skia.org/infra/go/urfavecli"
	"go.skia.org/infra/perf/go/builders"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/perf-tool/application"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/types"
)

// flag names
const (
	backupToDateFlagName     = "backup_to_date"
	beginCommitFlagName      = "begin"
	configFilenameFlagName   = "config_filename"
	connectionStringFlagName = "connection_string"
	dryrunFlagName           = "dryrun"
	endCommitFlagName        = "end"
	inputFilenameFlagName    = "in"
	localFlagName            = "local"
	loggingFlagName          = "logging"
	numTilesListFlagName     = "num"
	outputFilenameFlagName   = "out"
	queryFlagName            = "query"
	startTimeFlagName        = "start"
	stopTimeFlagName         = "stop"
	tileNumberFlagName       = "tile"
	trybotFilenameFlagName   = "filename"
	trybotNumCommitsFlagName = "num"
	verboseFlagName          = "verbose"
)

// flags
var trybotFilenameFlag = &cli.StringFlag{
	Name:  trybotFilenameFlagName,
	Value: "",
	Usage: "The full URL of a nanobench trybot results files, e.g.: 'gs://skia-perf/...foo.json'",
}

var connectionStringFlag = &cli.StringFlag{
	Name:    connectionStringFlagName,
	Value:   "",
	Usage:   "Override the connection string in the config file.",
	EnvVars: []string{"PERF_CONNECTION_STRING"},
}

var requiredOutputFilenameFlag = &cli.StringFlag{
	Name:     outputFilenameFlagName,
	Value:    "",
	Usage:    "The output filename.",
	Required: true,
}

var optionalOutputFilenameFlag = &cli.StringFlag{
	Name:  outputFilenameFlagName,
	Value: "",
	Usage: "The output filename.",
}

var inputFilenameFlag = &cli.StringFlag{
	Name:     inputFilenameFlagName,
	Value:    "",
	Usage:    "The input filename.",
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

var queryFlag = &cli.StringFlag{
	Name:     queryFlagName,
	Value:    "",
	Usage:    "The query to run.",
	Required: true,
}

var tileNumberFlag = &cli.Int64Flag{
	Name:  tileNumberFlagName,
	Value: int64(types.BadTileNumber),
	Usage: "The tile to query.",
}

var numTilesListFlag = &cli.IntFlag{
	Name:    numTilesListFlagName,
	Value:   10,
	Usage:   "The number of tiles to display.",
	EnvVars: []string{"PERF_CONFIG_FILENAME"},
}

var trybotNumCommitsFlag = &cli.IntFlag{
	Name:  trybotNumCommitsFlagName,
	Value: 5,
	Usage: "The number of ingestion files to load.",
}

var beginCommitFlag = &cli.Int64Flag{
	Name:     beginCommitFlagName,
	Value:    int64(types.BadCommitNumber),
	Usage:    "The commit number to start loading data from. Inclusive.",
	Required: true,
}

var endCommitFlag = &cli.Int64Flag{
	Name:  endCommitFlagName,
	Value: int64(types.BadCommitNumber),
	Usage: "The commit number to load data to.",
}

var startTimeFlag = &cli.StringFlag{
	Name:  startTimeFlagName,
	Value: "",
	Usage: "Start the ingestion at this time, of the form: 2006-01-02. Default to one week ago.",
}

var stopTimeFlag = &cli.StringFlag{
	Name:  stopTimeFlagName,
	Value: "",
	Usage: "Ingest up to this time, of the form: 2006-01-02. Default to now.",
}

var dryrunFlag = &cli.BoolFlag{
	Name:  dryrunFlagName,
	Value: false,
	Usage: "Just display the list of files to send.",
}

var loggingFlag = &cli.BoolFlag{
	Name:  loggingFlagName,
	Value: false,
	Usage: "Turn on logging while running commands.",
}

var verboseFlag = &cli.BoolFlag{
	Name:  verboseFlagName,
	Value: false,
	Usage: "Verbose output.",
}

// instanceConfigFromFlags returns an InstanceConfig based
// on the flags configFilenameFlag and connectionStringFlag.
func instanceConfigFromFlags(c *cli.Context) (*config.InstanceConfig, error) {
	instanceConfig, schemaViolations, err := config.InstanceConfigFromFile(c.String(configFilenameFlagName))
	if err != nil {
		for _, v := range schemaViolations {
			fmt.Println(v)
		}
		return nil, skerr.Wrap(err)
	}

	override := c.String(connectionStringFlagName)
	if override != "" {
		instanceConfig.DataStoreConfig.ConnectionString = override
	}
	return instanceConfig, nil
}

// getStore returns a TraceStore built on the flags configFilenameFlag and
// connectionStringFlag.
func getStore(c *cli.Context) (tracestore.TraceStore, error) {
	instanceConfig, err := instanceConfigFromFlags(c)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	local := c.Bool(localFlagName)
	return builders.NewTraceStoreFromConfig(context.Background(), local, instanceConfig)
}

func main() {
	app := application.New()
	actualMain(app)
}

func actualMain(app application.Application) {
	cli.MarkdownDocTemplate = urfavecli.MarkdownDocTemplate

	cliApp := &cli.App{
		Name:  "perf-tool",
		Usage: "Command-line tool for working with Perf data.",
		Flags: []cli.Flag{
			loggingFlag,
		},
		Before: func(c *cli.Context) error {
			if c.Bool(loggingFlagName) {
				sklogimpl.SetLogger(stdlogging.New(os.Stderr))
			} else {
				sklogimpl.SetLogger(nooplogging.New())
			}
			return nil
		},
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
							return app.ConfigCreatePubSubTopics(instanceConfig)
						},
					},
					{
						Name:  "validate",
						Usage: "Validate the given config",
						Flags: []cli.Flag{
							configFilenameFlag,
						},
						Action: func(c *cli.Context) error {
							_, err := instanceConfigFromFlags(c)
							if err != nil {
								// Unwrap the error since this gets printed as a user facing error message.
								return fmt.Errorf("Validation Failed: %s", skerr.Unwrap(err))
							}
							return nil
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
					{
						Name:  "list",
						Usage: "Prints the last N tiles and the number of traces they contain.",
						Flags: []cli.Flag{
							localFlag,
							configFilenameFlag,
							connectionStringFlag,
							numTilesListFlag,
						},
						Action: func(c *cli.Context) error {
							store, err := getStore(c)
							if err != nil {
								return skerr.Wrap(err)
							}

							return app.TilesList(store, c.Int(numTilesListFlagName))
						},
					},
				},
			},
			{
				Name: "traces",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "Prints the IDs of traces in the last (most recent) tile, or the tile specified by the --tile flag, that match --query.",
						Flags: []cli.Flag{
							localFlag,
							configFilenameFlag,
							connectionStringFlag,
							queryFlag,
							tileNumberFlag,
						},
						Action: func(c *cli.Context) error {
							store, err := getStore(c)
							if err != nil {
								return skerr.Wrap(err)
							}

							return app.TracesList(
								store,
								c.String(queryFlagName),
								types.TileNumber(c.Int64(tileNumberFlagName)))
						},
					},
					{
						Name:  "export",
						Usage: "Writes a JSON files with the traces that match --query for the given range of commits.",
						Flags: []cli.Flag{
							localFlag,
							configFilenameFlag,
							connectionStringFlag,
							queryFlag,
							optionalOutputFilenameFlag,
							beginCommitFlag,
							endCommitFlag,
						},
						Action: func(c *cli.Context) error {
							store, err := getStore(c)
							if err != nil {
								return skerr.Wrap(err)
							}

							return app.TracesExport(
								store,
								c.String(queryFlagName),
								types.CommitNumber(c.Int64(beginCommitFlagName)),
								types.CommitNumber(c.Int64(endCommitFlagName)),
								c.String(outputFilenameFlagName))
						},
					},
				},
			},
			{
				Name: "ingest",
				Subcommands: []*cli.Command{
					{
						Name:        "force-reingest",
						Description: "Force re-ingestion of files.",
						Flags: []cli.Flag{
							localFlag,
							configFilenameFlag,
							startTimeFlag,
							stopTimeFlag,
							dryrunFlag,
						},
						Action: func(c *cli.Context) error {
							instanceConfig, err := instanceConfigFromFlags(c)
							if err != nil {
								return skerr.Wrap(err)
							}

							return app.IngestForceReingest(
								c.Bool(localFlagName),
								instanceConfig,
								c.String(startTimeFlagName),
								c.String(stopTimeFlagName),
								c.Bool(dryrunFlagName))
						},
					},
					{
						Name:        "validate",
						Description: "Validate an ingestion file",
						Flags: []cli.Flag{
							configFilenameFlag,
							inputFilenameFlag,
							verboseFlag,
						},
						Action: func(c *cli.Context) error {
							instanceConfig, err := instanceConfigFromFlags(c)
							if err != nil {
								return skerr.Wrap(err)
							}
							return app.IngestValidate(instanceConfig, c.String(inputFilenameFlag.Name), c.Bool(verboseFlag.Name))
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
							return app.DatabaseMigrate(instanceConfig)
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
									requiredOutputFilenameFlag,
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
									requiredOutputFilenameFlag,
								},
								Action: func(c *cli.Context) error {
									instanceConfig, err := instanceConfigFromFlags(c)
									if err != nil {
										return skerr.Wrap(err)
									}

									return app.DatabaseBackupShortcuts(c.Bool(localFlagName), instanceConfig, c.String(outputFilenameFlagName))
								},
							},
							{
								Name: "regressions",
								Flags: []cli.Flag{
									localFlag,
									configFilenameFlag,
									connectionStringFlag,
									requiredOutputFilenameFlag,
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
									return app.DatabaseRestoreAlerts(c.Bool(localFlagName), instanceConfig, c.String(inputFilenameFlagName))
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

									return app.DatabaseRestoreShortcuts(c.Bool(localFlagName), instanceConfig, c.String(inputFilenameFlagName))
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

									return app.DatabaseRestoreRegressions(c.Bool(localFlagName), instanceConfig, c.String(inputFilenameFlagName))
								},
							},
						},
					},
				},
			},
			{
				Name: "trybot",
				Subcommands: []*cli.Command{
					{
						Name:  "reference",
						Usage: "Generates a reference file to be used by nanostat for the given trybot file.",
						Description: `
This command constructs a synthetic nanobench file filled with 'samples' for
each of the trace ids found in the nanobench file given in --filename.
The samples in the sythentic file are the samples found in the last --num
commits of ingested perf data for all the traces found in --filename.

This allows building a nanobench file with a large number of samples
that can be compared with a nanobench file from a trybot using the nanostat
application.

This is an experimental function that may go away in the future.
`,
						Flags: []cli.Flag{
							localFlag,
							configFilenameFlag,
							connectionStringFlag,
							trybotFilenameFlag,
							trybotNumCommitsFlag,
							requiredOutputFilenameFlag,
						},
						Action: func(c *cli.Context) error {
							instanceConfig, err := instanceConfigFromFlags(c)
							if err != nil {
								return skerr.Wrap(err)
							}
							store, err := getStore(c)
							if err != nil {
								return skerr.Wrap(err)
							}
							return app.TrybotReference(c.Bool(localFlagName), store, instanceConfig, c.String(trybotFilenameFlagName), c.String(outputFilenameFlagName), c.Int(trybotNumCommitsFlagName))
						},
					},
				},
			},

			{
				Name:  "markdown",
				Usage: "Generates markdown help for perf-tool.",
				Action: func(c *cli.Context) error {
					body, err := c.App.ToMarkdown()
					if err != nil {
						return skerr.Wrap(err)
					}
					fmt.Println(body)
					return nil
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
}
