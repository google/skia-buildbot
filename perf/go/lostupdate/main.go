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
	traces := client.Open("skia-traces")
	_ = client.Open("skia-ops")
	m := bigtable.NewMutation()
	m.Set(VALUES_FAMILY, "0", bigtable.ServerTime, []byte("1.2"))
	if err := traces.Apply(ctx, "tile0", m); err != nil {
		sklog.Fatalf("Failed to apply: %s", err)
	}

}
