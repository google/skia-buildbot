package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/gob"
	"flag"
	"fmt"
	"time"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/paramtools"
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
	HASH       = "H"
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

	// TODO Switch from revision to hash.

	// Our paramset to encode.
	op := paramtools.NewOrderedParamSet()
	op.Update(paramtools.ParamSet{
		"cpu":    []string{"x86", "arm"},
		"config": []string{"8888", "565"},
	})
	// TODO Move encoding and hash into OrderedParamSet itself.
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(op); err != nil {
		sklog.Fatal(err)
	}
	hash := fmt.Sprintf("%x", md5.Sum(buf.Bytes()))
	fmt.Printf("First gob: %x\n", buf.Bytes())
	fmt.Printf("First hash: %s\n", hash)

	// Write a Tile's ops with a given revision.
	m := bigtable.NewMutation()
	m.Set(OPS_FAMILY, HASH, bigtable.ServerTime, []byte(hash))
	m.Set(OPS_FAMILY, OPS, bigtable.ServerTime, buf.Bytes())
	if err := ops.Apply(ctx, "tile0", m); err != nil {
		sklog.Fatalf("Failed to apply: %s", err)
	}

	prevHash := hash

	op.Update(paramtools.ParamSet{
		"cpu": []string{"risc-v"},
	})
	buf.Reset()
	if err := gob.NewEncoder(&buf).Encode(op); err != nil {
		sklog.Fatal(err)
	}
	hash = fmt.Sprintf("%x", md5.Sum(buf.Bytes()))
	fmt.Printf("Second gob: %x\n", buf.Bytes())
	fmt.Printf("Second hash: %s\n", hash)

	// Create an update that avoids the lost update problem.
	cond := bigtable.ChainFilters(
		bigtable.FamilyFilter(OPS_FAMILY),
		bigtable.ColumnFilter(HASH),
		bigtable.ValueFilter(prevHash),
	)
	condTrue := false
	updateMutation := bigtable.NewMutation()
	updateMutation.Set(OPS_FAMILY, HASH, bigtable.ServerTime, []byte(hash))
	updateMutation.Set(OPS_FAMILY, OPS, bigtable.ServerTime, buf.Bytes())
	before := bigtable.Time(time.Now().Add(-1 * time.Second))
	updateMutation.DeleteTimestampRange(OPS_FAMILY, HASH, 0, before)
	updateMutation.DeleteTimestampRange(OPS_FAMILY, OPS, 0, before)
	// Can we add a mutation that cleans up old version?
	condUpdate := bigtable.NewCondMutation(cond, updateMutation, nil)
	if err := ops.Apply(ctx, "tile0", condUpdate, bigtable.GetCondMutationResult(&condTrue)); err != nil {
		sklog.Fatalf("Failed to apply: %s", err)
	}
	// If !condTrue then we need to try again.
	fmt.Printf("Applied: %v\n", condTrue)

	// row, err := ops.ReadRow(ctx, "tile0", bigtable.RowFilter(bigtable.FamilyFilter("foo")))
	row, err := ops.ReadRow(ctx, "tile0")
	fmt.Printf("%#v\n", row)
}
