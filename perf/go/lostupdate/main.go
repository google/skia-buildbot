package main

import (
	"context"
	"flag"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/api/option"
)

// flags
var (
	local    = flag.Bool("local", false, "True if running locally.")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

const (
	VALUES_FAMILY  = "V"
	SOURCES_FAMILY = "S"

	OPS_FAMILY = "D"
	REVISION   = "R"
	OPS        = "OPS"
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
	client, err := bigtable.NewClient(ctx, "skia-public", "perf-skia", option.WithTokenSource(ts))
	if err != nil {
		sklog.Fatalf("Failed to create client: %s", err)
	}
	_ = client.Open("skia-traces")
	ops := client.Open("skia-ops")
	// Write a Tile's ops with a given revision.
	m := bigtable.NewMutation()
	m.Set(OPS_FAMILY, REVISION, bigtable.ServerTime, []byte("1"))
	m.Set(OPS_FAMILY, OPS, bigtable.ServerTime, []byte("{\"desc\": \"JSON/GOB Data goes here.\"}"))
	if err := ops.Apply(ctx, "tile0", m); err != nil {
		sklog.Fatalf("Failed to apply: %s", err)
	}

	// Create an update that avoids the lost update problem.
	cond := bigtable.ChainFilters(
		bigtable.FamilyFilter(OPS_FAMILY),
		bigtable.ColumnFilter(REVISION),
		bigtable.ValueFilter("1"),
	)
	updateMutation := bigtable.NewMutation()
	updateMutation.Set(OPS_FAMILY, REVISION, bigtable.ServerTime, []byte("2"))
	updateMutation.Set(OPS_FAMILY, OPS, bigtable.ServerTime, []byte("{\"desc\": \"JSON/GOB Data goes here - 2.\"}"))
	condUpdate := bigtable.NewCondMutation(cond, updateMutation, nil)
	if err := ops.Apply(ctx, "tile0", condUpdate); err != nil {
		sklog.Fatalf("Failed to apply: %s", err)
	}
}
