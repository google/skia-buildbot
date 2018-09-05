package main

import (
	"context"
	"flag"
	"net/url"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/btts"
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
	ts, err := auth.NewDefaultTokenSource(*local, bigtable.AdminScope, bigtable.Scope)
	if err != nil {
		sklog.Fatalf("Failed to auth: %s", err)
	}

	store, err := btts.NewBigTableTraceStore(ctx, 1024, "skia", "skia-public", "perf-bt", ts)
	if err != nil {
		sklog.Fatalf("Failed to create client: %s", err)
	}
	tileKey := btts.TileKeyFromOffset(1)

	if err := store.ClearOPS(tileKey); err != nil {
		sklog.Fatalf("Failed to clear ops: %s", err)
	}

	op, err := store.UpdateOrderedParamSet(tileKey, paramtools.ParamSet{
		"cpu":    []string{"x86", "arm"},
		"config": []string{"8888", "565"},
	})
	if err != nil {
		sklog.Fatal(err)
	}

	op, err = store.UpdateOrderedParamSet(tileKey, paramtools.ParamSet{
		"cpu": []string{"risc-v"},
	})
	if err != nil {
		sklog.Fatal(err)
	}

	// Should just return from cache.
	op, err = store.UpdateOrderedParamSet(tileKey, paramtools.ParamSet{
		"cpu": []string{"risc-v"},
	})
	if err != nil {
		sklog.Fatal(err)
	}

	sklog.Infof("%#v\n", *op)
	// Create OPS for another Tile.

	tileKey = btts.TileKeyFromOffset(2)
	if err := store.ClearOPS(tileKey); err != nil {
		sklog.Fatalf("Failed to clear ops: %s", err)
	}

	latest, err := store.GetLatestTile()
	sklog.Infof("Latest: %d ", latest.Offset())

	values := map[string]float32{}

	key, err := op.EncodeParamsAsString(paramtools.Params{"cpu": "x86", "config": "8888"})
	if err != nil {
		sklog.Fatal(err)
	}
	values[key] = 1.1
	key, err = op.EncodeParamsAsString(paramtools.Params{"cpu": "x86", "config": "565"})
	if err != nil {
		sklog.Fatal(err)
	}
	values[key] = 1.2
	key, err = op.EncodeParamsAsString(paramtools.Params{"cpu": "arm", "config": "8888"})
	if err != nil {
		sklog.Fatal(err)
	}
	values[key] = 1.3
	key, err = op.EncodeParamsAsString(paramtools.Params{"cpu": "arm", "config": "565"})
	if err != nil {
		sklog.Fatal(err)
	}
	values[key] = 1.4
	err = store.WriteTraces(3, values, "gs://test")
	if err != nil {
		sklog.Fatal(err)
	}

	q, err := query.New(url.Values{"config": []string{"8888"}})
	if err != nil {
		sklog.Fatal(err)
	}
	r, err := q.Regexp(op)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Regex: %s", r)

	tileKey = btts.TileKeyFromOffset(0)
	results, err := store.QueryTraces(tileKey, r)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Results: %v", results)

	key, _ = op.EncodeParamsAsString(paramtools.Params{"cpu": "arm", "config": "565"})

	source, err := store.GetSource(3, key)
	sklog.Infof("Source: %q %s", source, err)

	source, err = store.GetSource(4, key)
	sklog.Infof("Source: %q %s", source, err)
}
