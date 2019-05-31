// Example application using BitTableTraceStore.
package main

import (
	"context"
	"flag"
	"net/url"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/btts"
	"go.skia.org/infra/perf/go/config"
)

// flags
var (
	local = flag.Bool("local", false, "True if running locally.")
)

func main() {
	common.Init()
	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*local, bigtable.Scope)
	if err != nil {
		sklog.Fatalf("Failed to auth: %s", err)
	}

	// Create the store client.
	cfg := config.PERF_BIGTABLE_CONFIGS[config.NANO]
	store, err := btts.NewBigTableTraceStoreFromConfig(ctx, cfg, ts, false)
	if err != nil {
		sklog.Fatalf("Failed to create client: %s", err)
	}

	// Get a tile that should be fully populated, i.e. the second most recent one.
	tileKey, err := store.GetLatestTile()
	if err != nil {
		sklog.Fatal(err)
	}
	tileKey = tileKey.PrevTile()

	// Create a query over the traces.
	q, err := query.New(url.Values{"config": []string{"8888"}, "name": []string{"Chalkboard.svg"}})
	if err != nil {
		sklog.Fatal(err)
	}

	// Time a Query.
	sklog.Infof("Loading all the data.")
	results, err := store.QueryTracesByIndex(ctx, tileKey, q)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Results: %d", len(results))

	// Time a Query that just counts the number of matches.
	sklog.Infof("Counting rows.")
	count, err := store.QueryCount(ctx, tileKey, q)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Results: %d", count)
}
