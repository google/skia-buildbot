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
	local    = flag.Bool("local", false, "True if running locally.")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

func main() {
	common.InitWithMust(
		"lostupdate",
		common.PrometheusOpt(promPort),
	)
	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*local, bigtable.Scope)
	if err != nil {
		sklog.Fatalf("Failed to auth: %s", err)
	}

	cfg := config.PERF_INGESTION_CONFIGS["nano"]
	store, err := btts.NewBigTableTraceStoreFromConfig(ctx, cfg, ts, false)
	if err != nil {
		sklog.Fatalf("Failed to create client: %s", err)
	}

	tileKey, err := store.GetLatestTile()
	if err != nil {
		sklog.Fatal(err)
	}
	tileKey = tileKey.PrevTile()
	op, err := store.GetOrderedParamSet(tileKey)
	if err != nil {
		sklog.Fatal(err)
	}

	q, err := query.New(url.Values{"config": []string{"8888"}, "name": []string{"Chalkboard.svg"}})
	if err != nil {
		sklog.Fatal(err)
	}
	r, err := q.Regexp(op)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Regex: %s", r)
	sklog.Infof("Loading all the data.")
	results, err := store.QueryTraces(tileKey, r)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Results: %d", len(results))
	sklog.Infof("Counting rows.")
	count, err := store.QueryCount(tileKey, r)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Results: %d", count)
}
