package main

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"sync"
	"time"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
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

	MAX_MUTATIONS = 100000 // Can be up to 100,000 according to BigTable docs.
)

type TileKey int32

const BadTileKey = TileKey(-1)

func TileKeyFromOffset(tileOffset int32) TileKey {
	return TileKey(math.MaxInt32 - tileOffset)
}

func (t TileKey) OpsRowName() string {
	return fmt.Sprintf("@%07d", t)
}

func (t TileKey) TraceRowPrefix() string {
	return fmt.Sprintf("%07d:", t)
}

// TraceRowName(",0=1,") -> 2147483647:,0=1,
func (t TileKey) TraceRowName(traceId string) string {
	return fmt.Sprintf("%07d:%s", t, traceId)
}

func (t TileKey) Offset() int32 {
	return math.MaxInt32 - int32(t)
}

func TileKeyFromString(s string) (TileKey, error) {
	if s[:1] != "@" {
		return BadTileKey, fmt.Errorf("TileKey strings must beginw with @: Got %q", s)
	}
	i, err := strconv.ParseInt(s[1:], 10, 32)
	if err != nil {
		return BadTileKey, err
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
	sklog.Infof("Got entry: %s %#v\n", entry.hash, *entry.ops)
	if entry.hash != hash {
		return nil, fmt.Errorf("Integrity error, hash mismatch, got %q want %q", entry.hash, hash)
	}
	return entry, nil
}

type BigTableTraceStore struct {
	ctx      context.Context
	tileSize int32
	table    *bigtable.Table

	mutex    sync.RWMutex
	opsCache map[string]*OpsCacheEntry // map[tile] -> ops.
}

func NewBigTableTraceStore(ctx context.Context, tileSize int32, table, project, instance string, ts oauth2.TokenSource) (*BigTableTraceStore, error) {
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
		table:    client.Open(table),
		opsCache: map[string]*OpsCacheEntry{},
	}, nil
}

func (b *BigTableTraceStore) TileKey(index int32) TileKey {
	return TileKeyFromOffset(index / b.tileSize)
}

func (b *BigTableTraceStore) clearOPS(tileKey TileKey) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	entry, err := NewOpsCacheEntry()
	if err != nil {
		return err
	}
	encodedOps, err := entry.ops.Encode()
	if err != nil {
		return fmt.Errorf("Failed to encode new ops: %s", err)
	}
	sklog.Infof("Clear sends: %s %#v\n", entry.hash, *entry.ops)
	now := bigtable.Time(time.Now())
	updateMutation := bigtable.NewMutation()
	updateMutation.Set(OPS_FAMILY, HASH, now, []byte(entry.hash))
	updateMutation.Set(OPS_FAMILY, OPS, now, encodedOps)
	err = b.table.Apply(b.ctx, tileKey.OpsRowName(), updateMutation)
	if err == nil {
		delete(b.opsCache, tileKey.OpsRowName())
	}
	return err
}

func (b *BigTableTraceStore) getOPS(tileKey TileKey) (*OpsCacheEntry, error) {
	b.mutex.RLock()
	entry, ok := b.opsCache[tileKey.OpsRowName()]
	b.mutex.RUnlock()
	if ok {
		return entry, nil
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()
	row, err := b.table.ReadRow(b.ctx, tileKey.OpsRowName(), bigtable.RowFilter(bigtable.LatestNFilter(1)))
	if err != nil {
		return nil, fmt.Errorf("Failed to read OPS from BigTable: %s", err)
	}
	// If there is no entry in BigTable then return an empty OPS.
	if len(row) == 0 {
		return NewOpsCacheEntry()
	}

	buf, err := json.MarshalIndent(row, "", "  ")
	sklog.Infof("getOPS: %s\n", string(buf))
	return NewOpsCacheEntryFromRow(row)
}

func (b *BigTableTraceStore) GetOrderedParamSet(tileKey TileKey) (*paramtools.OrderedParamSet, error) {
	entry, err := b.getOPS(tileKey)
	if err != nil {
		return nil, err
	}
	return entry.ops, nil
}

// The keys of 'values' must be the OPS encoded Params of the trace,
// i.e. at this point we know the OPS has been updated.
func (b *BigTableTraceStore) WriteTraces(index int32, values map[string]float32, source string) error {
	tileKey := TileKeyFromOffset(index / b.tileSize)
	col := strconv.Itoa(int(index % b.tileSize))
	now := bigtable.Time(time.Now())
	rowKeys := []string{}
	muts := []*bigtable.Mutation{}
	for k, v := range values {
		mut := bigtable.NewMutation()

		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, math.Float32bits(v))
		mut.Set(VALUES_FAMILY, col, now, buf)
		mut.Set(SOURCES_FAMILY, col, now, []byte(source))
		muts = append(muts, mut)
		rowKeys = append(rowKeys, tileKey.TraceRowName(k))
		if len(muts) >= MAX_MUTATIONS {
			errs, err := b.table.ApplyBulk(b.ctx, rowKeys, muts)
			if err != nil {
				return fmt.Errorf("Failed writing traces: %s", err)
			}
			if errs != nil {
				return fmt.Errorf("Failed writing some traces: %v", errs)
			}
			rowKeys = []string{}
			muts = []*bigtable.Mutation{}
		}
	}
	if len(muts) > 0 {
		errs, err := b.table.ApplyBulk(b.ctx, rowKeys, muts)
		if err != nil {
			return fmt.Errorf("Failed writing traces: %s", err)
		}
		if errs != nil {
			return fmt.Errorf("Failed writing some traces: %v", errs)
		}
	}
	return nil
}

func (b *BigTableTraceStore) QueryTraces(tileKey TileKey, q *regexp.Regexp) (map[string][]float32, error) {
	rowRegex := tileKey.TraceRowPrefix() + ".*" + q.String()
	sklog.Infof("rowRegex: %q", rowRegex)
	err := b.table.ReadRows(b.ctx, bigtable.PrefixRange(tileKey.TraceRowPrefix()), func(row bigtable.Row) bool {
		sklog.Infof("%v", row)
		return true
	}, bigtable.RowFilter(bigtable.ChainFilters(bigtable.RowKeyFilter(rowRegex), bigtable.LatestNFilter(1), bigtable.FamilyFilter(VALUES_FAMILY))))
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (b *BigTableTraceStore) GetLatestTile() (TileKey, error) {
	ret := BadTileKey
	err := b.table.ReadRows(b.ctx, bigtable.PrefixRange("@"), func(row bigtable.Row) bool {
		var err error
		ret, err = TileKeyFromString(row.Key())
		if err != nil {
			sklog.Infof("Found invalid value in OPS row: %s %s", row.Key(), err)
		}
		return false
	}, bigtable.LimitRows(1))
	if err != nil {
		return BadTileKey, fmt.Errorf("Failed to scan OPS: %s", err)
	}
	if ret == BadTileKey {
		return BadTileKey, fmt.Errorf("Failed to read any OPS from BigTable: %s", err)
	}
	return ret, nil
}

func (b *BigTableTraceStore) UpdateOrderedParamSet(tileKey TileKey, p paramtools.ParamSet) (*paramtools.OrderedParamSet, error) {
	var newEntry *OpsCacheEntry
	for {
		var err error
		// Get OPS.
		entry, err := b.getOPS(tileKey)
		if err != nil {
			return nil, fmt.Errorf("Failed to get OPS: %s", err)
		}

		// If the OPS contains our paramset then we're done.
		if delta := entry.ops.Delta(p); len(delta) == 0 {
			sklog.Infof("We're done.")
			return entry.ops, nil
		}

		// Create a new updated ops.
		ops := entry.ops.Dup()
		ops.Update(p)
		newEntry, err = opsCacheEntryFromOPS(ops)
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
		now := bigtable.Time(time.Now())
		updateMutation.Set(OPS_FAMILY, HASH, now, []byte(newEntry.hash))
		updateMutation.Set(OPS_FAMILY, OPS, now, encodedOps)

		// Add a mutation that cleans up old versions.
		before := bigtable.Time(time.Now().Add(-1 * time.Second))
		updateMutation.DeleteTimestampRange(OPS_FAMILY, HASH, 0, before)
		updateMutation.DeleteTimestampRange(OPS_FAMILY, OPS, 0, before)
		condUpdate := bigtable.NewCondMutation(cond, updateMutation, nil)
		if err := b.table.Apply(b.ctx, tileKey.OpsRowName(), condUpdate, bigtable.GetCondMutationResult(&condTrue)); err != nil {
			sklog.Infof("Failed to apply: %s", err)
			continue
		}

		// If !condTrue then we need to try again,
		// and clear our local cache.
		if !condTrue {
			b.mutex.Lock()
			delete(b.opsCache, tileKey.OpsRowName())
			b.mutex.Unlock()
			continue
		}
		// Successfully wrote OPS, so update the cache.
		b.mutex.Lock()
		defer b.mutex.Unlock()
		b.opsCache[tileKey.OpsRowName()] = newEntry
		break
	}
	return newEntry.ops, nil
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

	btts, err := NewBigTableTraceStore(ctx, 1024, "skia", "skia-public", "perf-bt", ts)
	if err != nil {
		sklog.Fatalf("Failed to create client: %s", err)
	}
	tileKey := TileKeyFromOffset(1)

	if err := btts.clearOPS(tileKey); err != nil {
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

	// Should just return from cache.
	op, err = btts.UpdateOrderedParamSet(tileKey, paramtools.ParamSet{
		"cpu": []string{"risc-v"},
	})
	if err != nil {
		sklog.Fatal(err)
	}

	sklog.Infof("%#v\n", *op)
	// Create OPS for another Tile.

	tileKey = TileKeyFromOffset(2)
	if err := btts.clearOPS(tileKey); err != nil {
		sklog.Fatalf("Failed to clear ops: %s", err)
	}

	latest, err := btts.GetLatestTile()
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
	err = btts.WriteTraces(3, values, "gs://test")
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

	tileKey = TileKeyFromOffset(0)
	results, err := btts.QueryTraces(tileKey, r)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Results: %v", results)
}
