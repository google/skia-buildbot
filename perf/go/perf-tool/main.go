// Example application using BitTableTraceStore.
package main

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"cloud.google.com/go/bigtable"
	"github.com/jcgregorio/logger"
	"github.com/jcgregorio/slog"
	"github.com/urfave/cli"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/btts"
	"go.skia.org/infra/perf/go/config"
)

var (
	store *btts.BigTableTraceStore
)

// TODO(jcgregorio) Migrate this into its own module that we can use everywhere
// once we're happy with the design.
//
// cloudLoggerImpl implements sklog.CloudLogger.
type cloudLoggerImpl struct {
	stdLog slog.Logger
}

// newLogger creates a new cloudLoggerImpl that either logs to stdout, or does
// no logging, depending upon the value of enable.
func newLogger(enable bool) *cloudLoggerImpl {
	if enable {
		return &cloudLoggerImpl{
			stdLog: logger.NewFromOptions(&logger.Options{SyncWriter: os.Stderr}),
		}
	} else {
		return &cloudLoggerImpl{
			stdLog: logger.NewNopLogger(),
		}
	}
}

func (c *cloudLoggerImpl) CloudLog(reportName string, payload *sklog.LogPayload) {
	switch payload.Severity {
	case sklog.DEBUG:
		c.stdLog.Debug(payload.Payload)
	case sklog.INFO, sklog.NOTICE:
		c.stdLog.Info(payload.Payload)
	case sklog.WARNING:
		c.stdLog.Warning(payload.Payload)
	case sklog.ERROR:
		c.stdLog.Error(payload.Payload)
	case sklog.CRITICAL, sklog.ALERT:
		c.stdLog.Fatal(payload.Payload)
	}
}

func (c *cloudLoggerImpl) BatchCloudLog(reportName string, payloads ...*sklog.LogPayload) {
	for _, payload := range payloads {
		c.CloudLog(reportName, payload)
	}
}

func (c *cloudLoggerImpl) Flush() {
	_ = os.Stdout.Sync()
}

func main() {
	ctx := context.Background()

	app := cli.NewApp()

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "big_table_config",
			Value: "nano",
			Usage: "The name of the config to use when using a BigTable trace store.",
		},
		cli.BoolFlag{
			Name:  "local",
			Usage: "True if running locally.",
		},
		cli.BoolFlag{
			Name:  "logtostderr",
			Usage: "Otherwise logs are not produced.",
		},
	}

	app.Before = func(c *cli.Context) error {
		sklog.SetLogger(newLogger(c.Bool("logtostderr")))

		ts, err := auth.NewDefaultTokenSource(c.Bool("local"), bigtable.Scope)
		if err != nil {
			return fmt.Errorf("Failed to auth: %s", err)
		}

		// Create the store client.
		cfg := config.PERF_BIGTABLE_CONFIGS[c.String("big_table_config")]
		store, err = btts.NewBigTableTraceStoreFromConfig(ctx, cfg, ts, false)
		if err != nil {
			return fmt.Errorf("Failed to create client: %s", err)
		}
		return nil
	}

	app.Version = "0.1.0"
	app.EnableBashCompletion = true

	app.Commands = []cli.Command{

		{
			Name:        "indices",
			Description: "Sub-commands from here are about indices.",
			Subcommands: []cli.Command{
				{
					Name:        "write",
					Usage:       "indices count",
					Description: "Rewrites the indices for the last (most recent) tile, or the tile specified by --tile.",
					Flags: []cli.Flag{
						cli.IntFlag{
							Name:  "tile",
							Value: -1,
							Usage: "The tile to query.",
						},
					},
					Action: indicesWriteAction,
				},
			},
		},

		{
			Name:        "tiles",
			Description: "Sub-commands from here are about tiles.",
			Subcommands: []cli.Command{
				{
					Name:        "last",
					Usage:       "tiles last",
					Description: "Prints the offset of the last (most recent) tile.",
					Action:      tilesLastAction,
				},
			},
		},
		{
			Name:        "traces",
			Usage:       "traces",
			Description: "Sub-commands from here are about traces.",
			Subcommands: []cli.Command{
				{
					Name:        "count",
					Usage:       "traces count",
					Description: "Prints the number of traces in the last (most recent) tile, or the tile specified by the --tile flag.",
					Flags: []cli.Flag{
						cli.IntFlag{
							Name:  "tile",
							Value: -1,
							Usage: "The tile to query.",
						},
						cli.StringFlag{
							Name:  "query",
							Value: "",
							Usage: "The query to run. Defaults to the empty query which matches all traces.",
						},
					},
					Action: tracesCountAction,
				},
				{
					Name:        "list",
					Usage:       "traces list",
					Description: "Prints the IDs of traces in the last (most recent) tile, or the tile specified by the --tile flag, that match --query.",
					Flags: []cli.Flag{
						cli.IntFlag{
							Name:  "tile",
							Value: -1,
							Usage: "The tile to query.",
						},
						cli.StringFlag{
							Name:  "query",
							Value: "",
							Usage: "The query to run. Defaults to the empty query which matches all traces.",
						},
					},
					Action: tracesListAction,
				},
				{
					Name:        "list-by-index",
					Usage:       "traces list-by-index",
					Description: "Prints the IDs of traces in the last (most recent) tile, or the tile specified by the --tile flag, that match --query.",
					Flags: []cli.Flag{
						cli.IntFlag{
							Name:  "tile",
							Value: -1,
							Usage: "The tile to query.",
						},
						cli.StringFlag{
							Name:  "query",
							Value: "",
							Usage: "The query to run. Defaults to the empty query which matches all traces.",
						},
					},
					Action: tracesListByIndexAction,
				},
			},
		},
	}

	app.Run(os.Args)
}

func tilesLastAction(c *cli.Context) error {
	tileKey, err := store.GetLatestTile()
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Printf("Last Tile: %d\n", tileKey.Offset())
	return nil
}

func tracesCountAction(c *cli.Context) error {
	var tileKey btts.TileKey
	if c.Int("tile") == -1 {
		var err error
		tileKey, err = store.GetLatestTile()
		if err != nil {
			sklog.Fatal(err)
		}
	} else {
		tileKey = btts.TileKeyFromOffset(int32(c.Int("tile")))
	}
	values, err := url.ParseQuery(c.String("query"))
	if err != nil {
		sklog.Fatal(err)
	}
	q, err := query.New(values)
	if err != nil {
		sklog.Fatal(err)
	}
	count, err := store.QueryCount(context.Background(), tileKey, q)
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Printf("Tile: %d Num Traces: %d\n", tileKey.Offset(), count)
	return nil
}

func tracesListAction(c *cli.Context) error {
	var tileKey btts.TileKey
	if c.Int("tile") == -1 {
		var err error
		tileKey, err = store.GetLatestTile()
		if err != nil {
			sklog.Fatal(err)
		}
	} else {
		tileKey = btts.TileKeyFromOffset(int32(c.Int("tile")))
	}
	values, err := url.ParseQuery(c.String("query"))
	if err != nil {
		sklog.Fatal(err)
	}
	q, err := query.New(values)
	if err != nil {
		sklog.Fatal(err)
	}
	ts, err := store.QueryTraces(context.Background(), tileKey, q)
	if err != nil {
		sklog.Fatal(err)
	}
	for id, _ := range ts {
		fmt.Println(id)
	}
	return nil
}

func tracesListByIndexAction(c *cli.Context) error {
	var tileKey btts.TileKey
	if c.Int("tile") == -1 {
		var err error
		tileKey, err = store.GetLatestTile()
		if err != nil {
			sklog.Fatal(err)
		}
	} else {
		tileKey = btts.TileKeyFromOffset(int32(c.Int("tile")))
	}
	values, err := url.ParseQuery(c.String("query"))
	if err != nil {
		sklog.Fatal(err)
	}
	q, err := query.New(values)
	if err != nil {
		sklog.Fatal(err)
	}
	ts, err := store.QueryTracesByIndex(context.Background(), tileKey, q)
	if err != nil {
		sklog.Fatal(err)
	}
	for id, _ := range ts {
		fmt.Println(id)
	}
	return nil
}

func indicesWriteAction(c *cli.Context) error {
	var tileKey btts.TileKey
	if c.Int("tile") == -1 {
		var err error
		tileKey, err = store.GetLatestTile()
		if err != nil {
			sklog.Fatal(err)
		}
	} else {
		tileKey = btts.TileKeyFromOffset(int32(c.Int("tile")))
	}
	if err := store.WriteIndices(context.Background(), tileKey); err != nil {
		sklog.Fatal(err)
	}
	return nil
}
