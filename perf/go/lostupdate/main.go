package main

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"strconv"
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

type TileKey int32

func TileKeyFromOffset(tileOffset int32) TileKey {
	return TileKey(math.MaxInt32 - tileOffset)
}

func (t TileKey) String() string {
	return fmt.Sprintf("%07d", t)
}

func (t TileKey) Offset() int32 {
	return math.MaxInt32 - int32(t)
}

func TileKeyFromString(s string) (TileKey, error) {
	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return TileKey(-1), err
	}
	return TileKey(i), nil
}

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

	tileKey := TileKeyFromOffset(1)

	// Our paramset to encode.
	op := paramtools.NewOrderedParamSet()
	op.Update(paramtools.ParamSet{
		"cpu":    []string{"x86", "arm"},
		"config": []string{"8888", "565"},
	})
	// TODO Move encoding and hash into OrderedParamSet itself.
	buf, err := op.Encode()
	if err != nil {
		sklog.Fatal(err)
	}
	hash := fmt.Sprintf("%x", md5.Sum(buf))
	fmt.Printf("First ops: %x\n", buf)
	fmt.Printf("First hash: %s\n", hash)

	// Write a Tile's ops with a given revision.
	m := bigtable.NewMutation()
	m.Set(OPS_FAMILY, HASH, bigtable.ServerTime, []byte(hash))
	m.Set(OPS_FAMILY, OPS, bigtable.ServerTime, buf)
	if err := ops.Apply(ctx, tileKey.String(), m); err != nil {
		sklog.Fatalf("Failed to apply: %s", err)
	}

	prevHash := hash

	op.Update(paramtools.ParamSet{
		"cpu": []string{"risc-v"},
	})
	buf, err = op.Encode()
	if err != nil {
		sklog.Fatal(err)
	}

	// Hash must be a string because bigtable.ValueFilter() takes a string, and
	// doesn't seem to match if we use the raw []byte from md5.Sum().
	hash = fmt.Sprintf("%x", md5.Sum(buf))
	fmt.Printf("Second ops: %x\n", buf)
	fmt.Printf("Second hash: %s\n", hash)

	// Create an update that avoids the lost update problem.
	cond := bigtable.ChainFilters(
		bigtable.FamilyFilter(OPS_FAMILY),
		bigtable.ColumnFilter(HASH),
		bigtable.ValueFilter(string(prevHash)),
	)
	condTrue := false
	updateMutation := bigtable.NewMutation()
	updateMutation.Set(OPS_FAMILY, HASH, bigtable.ServerTime, []byte(hash))
	updateMutation.Set(OPS_FAMILY, OPS, bigtable.ServerTime, buf)
	// Add a mutation that cleans up old versions.
	before := bigtable.Time(time.Now().Add(-1 * time.Second))
	updateMutation.DeleteTimestampRange(OPS_FAMILY, HASH, 0, before)
	updateMutation.DeleteTimestampRange(OPS_FAMILY, OPS, 0, before)
	condUpdate := bigtable.NewCondMutation(cond, updateMutation, nil)
	if err := ops.Apply(ctx, tileKey.String(), condUpdate, bigtable.GetCondMutationResult(&condTrue)); err != nil {
		sklog.Fatalf("Failed to apply: %s", err)
	}
	// If !condTrue then we need to try again.
	fmt.Printf("Applied: %v\n", condTrue)

	// row, err := ops.ReadRow(ctx, tileKey.String(), bigtable.RowFilter(bigtable.StripValueFilter()))
	row, err := ops.ReadRow(ctx, tileKey.String())
	fmt.Printf("%#v\n", row)
	b, err := json.MarshalIndent(row, "", "  ")
	fmt.Println(string(b))
}
