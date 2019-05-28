// Example application using BitTableTraceStore.
package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"

	"cloud.google.com/go/bigtable"
	"github.com/urfave/cli"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/perf/go/btts"
	"go.skia.org/infra/perf/go/config"
)

var (
	store *btts.BigTableTraceStore
)

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
	}

	app.Before = func(c *cli.Context) error {
		// TODO(jcgregorio) Needed because sklog presumes sklog which add its own
		// flags and presumes flag.Parse() gets called. We really need to move away
		// from the predefined flags.
		common.Init()

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
			Name:        "tiles",
			Usage:       "tiles",
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
			},
		},
	}

	app.Run(os.Args)
}

func tilesLastAction(c *cli.Context) error {
	tileKey, err := store.GetLatestTile()
	if err != nil {
		log.Fatal(err)
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
			log.Fatal(err)
		}
	} else {
		tileKey = btts.TileKeyFromOffset(int32(c.Int("tile")))
	}
	values, err := url.ParseQuery(c.String("query"))
	if err != nil {
		log.Fatal(err)
	}
	q, err := query.New(values)
	if err != nil {
		log.Fatal(err)
	}
	count, err := store.QueryCount(context.Background(), tileKey, q)
	if err != nil {
		log.Fatal(err)
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
			log.Fatal(err)
		}
	} else {
		tileKey = btts.TileKeyFromOffset(int32(c.Int("tile")))
	}
	values, err := url.ParseQuery(c.String("query"))
	if err != nil {
		log.Fatal(err)
	}
	q, err := query.New(values)
	if err != nil {
		log.Fatal(err)
	}
	count, err := store.QueryCount(context.Background(), tileKey, q)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Tile: %d Num Traces: %d\n", tileKey.Offset(), count)
	return nil
}
