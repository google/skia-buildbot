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
	"golang.org/x/oauth2"
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

	OPS_FAMILY         = "D"
	HASH               = "H"
	OPS                = "OPS"
	HASH_FULL_COL_NAME = OPS_FAMILY + ":" + HASH
	OPS_FULL_COL_NAME  = OPS_FAMILY + ":" + OPS
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

type OpsCacheEntry struct {
	ops  *paramtools.OrderedParamSet
	hash string
}

func opsCacheEntryFromOPS(ops *paramtools.OrderedParamSet) (*OpsCacheEntry, error) {
	buf, err := ops.Encode()
	if err != nil {
		return nil, err
	}
	hash := fmt.Sprintf("%x", md5.Sum(buf))
	return &OpsCacheEntry{
		ops:  ops,
		hash: hash,
	}, nil
}

func NewOpsCacheEntry() (*OpsCacheEntry, error) {
	return opsCacheEntryFromOPS(paramtools.NewOrderedParamSet())
}

func NewOpsCacheEntryFromRow(row bigtable.Row) (*OpsCacheEntry, error) {
	family := row[OPS_FAMILY]
	ops := &paramtools.OrderedParamSet{}
	hash := ""
	for _, col := range family {
		if col.Column == OPS_FULL_COL_NAME {
			var err error
			ops, err = paramtools.NewOrderedParamSetFromBytes(col.Value)
			if err != nil {
				return nil, err
			}
		} else if col.Column == HASH_FULL_COL_NAME {
			hash = string(col.Value)
		}
	}
	entry, err := opsCacheEntryFromOPS(ops)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Got entry: %s %#v\n", entry.hash, *entry.ops)
	if entry.hash != hash {
		return nil, fmt.Errorf("Integrity error, hash mismatch, got %q want %q", entry.hash, hash)
	}
	return entry, nil
}

type BigTableTraceStore struct {
	ctx      context.Context
	tileSize int32
	ops      *bigtable.Table
	traces   *bigtable.Table
	opsCache map[int32]*OpsCacheEntry // map[tile] -> ops.
}

func NewBigTableTraceStore(ctx context.Context, tileSize int32, prefix, project, instance string, ts oauth2.TokenSource) (*BigTableTraceStore, error) {
	if tileSize <= 0 {
		return nil, fmt.Errorf("tileSize must be >0. %d", tileSize)
	}
	client, err := bigtable.NewClient(ctx, project, instance, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("Couldn't create client: %s", err)
	}
	return &BigTableTraceStore{
		ctx:      ctx,
		tileSize: tileSize,
		ops:      client.Open(fmt.Sprintf("%s-ops", prefix)),
		traces:   client.Open(fmt.Sprintf("%s-traces", prefix)),
		opsCache: map[int32]*OpsCacheEntry{},
	}, nil
}

func (b *BigTableTraceStore) TileKey(index int32) TileKey {
	return TileKeyFromOffset(index / b.tileSize)
}

func (b *BigTableTraceStore) clearOpsCacheEntry(tileKey TileKey) error {
	entry, err := NewOpsCacheEntry()
	if err != nil {
		return err
	}
	encodedOps, err := entry.ops.Encode()
	if err != nil {
		return fmt.Errorf("Failed to encode new ops: %s", err)
	}
	fmt.Printf("Clear sends: %s %#v\n", entry.hash, *entry.ops)
	updateMutation := bigtable.NewMutation()
	updateMutation.Set(OPS_FAMILY, HASH, bigtable.ServerTime, []byte(entry.hash))
	updateMutation.Set(OPS_FAMILY, OPS, bigtable.ServerTime, encodedOps)
	err = b.ops.Apply(b.ctx, tileKey.String(), updateMutation)
	if err == nil {
		delete(b.opsCache, tileKey.Offset())
	}
	return err
}

func (b *BigTableTraceStore) getOpsCacheEntry(tileKey TileKey) (*OpsCacheEntry, error) {
	if entry, ok := b.opsCache[tileKey.Offset()]; ok {
		return entry, nil
	}
	row, err := b.ops.ReadRow(b.ctx, tileKey.String(), bigtable.RowFilter(bigtable.LatestNFilter(1)))
	if err != nil {
		return nil, fmt.Errorf("Failed to read OPS from BigTable: %s", err)
	}
	// If there is no entry in BigTable then return an empty OPS.
	if len(row) == 0 {
		return NewOpsCacheEntry()
	}

	buf, err := json.MarshalIndent(row, "", "  ")
	fmt.Printf("getOpsCacheEntry: %s\n", string(buf))
	return NewOpsCacheEntryFromRow(row)
}

func (b *BigTableTraceStore) GetOrderedParamSet(tileKey TileKey) (*paramtools.OrderedParamSet, error) {
	entry, err := b.getOpsCacheEntry(tileKey)
	if err != nil {
		return nil, err
	}
	return entry.ops, nil
}

func (b *BigTableTraceStore) UpdateOrderedParamSet(tileKey TileKey, p paramtools.ParamSet) (*paramtools.OrderedParamSet, error) {
	// Maybe timeout?
	for {
		entry, err := b.getOpsCacheEntry(tileKey)
		if err != nil {
			return nil, fmt.Errorf("Failed to get OPS: %s", err)
		}
		// Create a new updated ops.
		ops := entry.ops.Dup()
		ops.Update(p)
		newEntry, err := opsCacheEntryFromOPS(ops)
		encodedOps, err := newEntry.ops.Encode()
		if err != nil {
			return nil, fmt.Errorf("Failed to encode new ops: %s", err)
		}

		// Create an update that avoids the lost update problem.
		cond := bigtable.ChainFilters(
			bigtable.FamilyFilter(OPS_FAMILY),
			bigtable.ColumnFilter(HASH),
			bigtable.ValueFilter(string(entry.hash)),
		)
		condTrue := false
		updateMutation := bigtable.NewMutation()
		updateMutation.Set(OPS_FAMILY, HASH, bigtable.ServerTime, []byte(newEntry.hash))
		updateMutation.Set(OPS_FAMILY, OPS, bigtable.ServerTime, encodedOps)
		// Add a mutation that cleans up old versions.
		before := bigtable.Time(time.Now().Add(-1 * time.Second))
		updateMutation.DeleteTimestampRange(OPS_FAMILY, HASH, 0, before)
		updateMutation.DeleteTimestampRange(OPS_FAMILY, OPS, 0, before)
		condUpdate := bigtable.NewCondMutation(cond, updateMutation, nil)
		if err := b.ops.Apply(b.ctx, tileKey.String(), condUpdate, bigtable.GetCondMutationResult(&condTrue)); err != nil {
			sklog.Infof("Failed to apply: %s", err)
			continue
		}
		// If !condTrue then we need to try again,

		// and clear our local cache.
		if !condTrue {
			delete(b.opsCache, tileKey.Offset())
			continue
		}
		b.opsCache[tileKey.Offset()] = newEntry
		break
	}
	return b.opsCache[tileKey.Offset()].ops, nil
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

	btts, err := NewBigTableTraceStore(ctx, 1024, "skia", "skia-public", "perf-skia", ts)
	if err != nil {
		sklog.Fatalf("Failed to create client: %s", err)
	}
	client, err := bigtable.NewClient(ctx, "skia-public", "perf-skia", option.WithTokenSource(ts))
	if err != nil {
		sklog.Fatalf("Couldn't create client: %s", err)
	}
	_ = client.Open("skia-traces")
	ops := client.Open("skia-ops")

	tileKey := TileKeyFromOffset(1)

	if err := btts.clearOpsCacheEntry(tileKey); err != nil {
		sklog.Fatalf("Failed to clear ops: %s", err)
	}

	op, err := btts.UpdateOrderedParamSet(tileKey, paramtools.ParamSet{
		"cpu":    []string{"x86", "arm"},
		"config": []string{"8888", "565"},
	})
	if err != nil {
		sklog.Fatal(err)
	}

	op, err = btts.UpdateOrderedParamSet(tileKey, paramtools.ParamSet{
		"cpu": []string{"risc-v"},
	})
	if err != nil {
		sklog.Fatal(err)
	}

	fmt.Printf("%#v\n", *op)

	// row, err := ops.ReadRow(ctx, tileKey.String(), bigtable.RowFilter(bigtable.StripValueFilter()))
	row, err := ops.ReadRow(ctx, tileKey.String(), bigtable.RowFilter(bigtable.LatestNFilter(1)))
	fmt.Printf("%#v\n", row)
	b, err := json.MarshalIndent(row, "", "  ")
	fmt.Println(string(b))
}
