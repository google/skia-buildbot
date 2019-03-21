/*
See BIGTABLE.md for tiles and traces are stored in BigTable.
*/
package btts

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/bigtable"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/config"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/option"
)

const (
	// BigTable column families.
	VALUES_FAMILY  = "V"
	SOURCES_FAMILY = "S"
	HASHES_FAMILY  = "H"
	OPS_FAMILY     = "D"

	// Columns in the "H" column family.
	HASHES_SOURCE_COLUMN        = "S" // Column
	HASHES_SOURCE_FULL_COL_NAME = HASHES_FAMILY + ":" + HASHES_SOURCE_COLUMN

	// Columns in the "D" column family.
	OPS_HASH_COLUMN    = "H"   // Column
	OPS_OPS_COLUMN     = "OPS" // Column
	HASH_FULL_COL_NAME = OPS_FAMILY + ":" + OPS_HASH_COLUMN
	OPS_FULL_COL_NAME  = OPS_FAMILY + ":" + OPS_OPS_COLUMN

	// MAX_MUTATIONS is the max number of mutations we can send in a single ApplyBulk call. Can be up to 100,000 according to BigTable docs.
	MAX_MUTATIONS = 100000

	TIMEOUT       = 4 * time.Minute
	WRITE_TIMEOUT = 10 * time.Minute
)

// TileKey is the identifier for each tile held in BigTable.
//
// Note that tile keys are in the opposite order of tile offset, that is, the first
// tile for a repo would be 0, and then 1, etc. Those are the offsets, and the most
// recent tile has the largest offset. To make it easy to find the most recent
// tile we calculate tilekey as math.MaxInt32 - tileoffset, so that more recent
// tiles come first in sort order.
type TileKey int32

// BadTileKey is returned in error conditions.
const BadTileKey = TileKey(-1)

// TileKeyFromOffset returns a TileKey from the tile offset.
func TileKeyFromOffset(tileOffset int32) TileKey {
	if tileOffset < 0 {
		return BadTileKey
	}
	return TileKey(math.MaxInt32 - tileOffset)
}

func (t TileKey) PrevTile() TileKey {
	return TileKeyFromOffset(t.Offset() - 1)
}

// OpsRowName returns the name of the BigTable row that the OrderedParamSet for this tile is stored at.
func (t TileKey) OpsRowName() string {
	return fmt.Sprintf("@%07d", t)
}

// TraceRowPrefix returns the prefix of a BigTable row name for any Trace in this tile.
func (t TileKey) TraceRowPrefix(shard int32) string {
	return fmt.Sprintf("%d:%07d:", shard, t)
}

// TraceRowPrefix returns the BigTable row name for the given trace, for the given number of shards.
// TraceRowName(",0=1,", 3) -> 2:2147483647:,0=1,
func (t TileKey) TraceRowName(traceId string, shards int32) string {
	return fmt.Sprintf("%d:%07d:%s", crc32.ChecksumIEEE([]byte(traceId))%uint32(shards), t, traceId)
}

// Offset returns the tile offset, i.e. the not-reversed number.
func (t TileKey) Offset() int32 {
	return math.MaxInt32 - int32(t)
}

func TileKeyFromOpsRowName(s string) (TileKey, error) {
	if s[:1] != "@" {
		return BadTileKey, fmt.Errorf("TileKey strings must beginw with @: Got %q", s)
	}
	i, err := strconv.ParseInt(s[1:], 10, 32)
	if err != nil {
		return BadTileKey, err
	}
	return TileKey(i), nil
}

// When ingesting we keep a cache of the OrderedParamSets we have seen per-tile.
type OpsCacheEntry struct {
	ops  *paramtools.OrderedParamSet
	hash string // md5 has of the serialized ops.
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
	if len(family) != 2 {
		return nil, fmt.Errorf("Didn't get the right number of columns from BT.")
	}
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
			sklog.Infof("Read hash from BT: %q", hash)
		}
	}
	entry, err := opsCacheEntryFromOPS(ops)
	if err != nil {
		return nil, err
	}
	// You might be tempted to check that entry.hash == hash here, but that will fail
	// because GoB encoding of maps is not deterministic.
	if hash == "" {
		return nil, fmt.Errorf("Didn't read hash from BT.")
	}
	entry.hash = hash
	return entry, nil
}

type BigTableTraceStore struct {
	tileSize      int32 // How many commits we store per tile.
	shards        int32 // How many shards we break the traces into.
	writesCounter metrics2.Counter
	table         *bigtable.Table

	// lookup maps column names as strings, "V:2", to the index, 2.
	// Used to speed up reading values out of rows.
	lookup map[string]int

	cacheOps bool // Should we use the opsCache? Only true for ingesters, always false for Perf frontends.

	mutex    sync.RWMutex              // Protects opsCache.
	opsCache map[string]*OpsCacheEntry // map[tile] -> ops.
}

func (b *BigTableTraceStore) TileSize() int32 {
	return b.tileSize
}

func (b *BigTableTraceStore) getTable() *bigtable.Table {
	return b.table
}

func NewBigTableTraceStoreFromConfig(ctx context.Context, cfg *config.PerfBigTableConfig, ts oauth2.TokenSource, cacheOps bool) (*BigTableTraceStore, error) {
	if cfg.TileSize <= 0 {
		return nil, fmt.Errorf("tileSize must be >0. %d", cfg.TileSize)
	}
	client, err := bigtable.NewClient(ctx, cfg.Project, cfg.Instance, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("Couldn't create client: %s", err)
	}
	lookup := map[string]int{}
	for i := 0; int32(i) < cfg.TileSize; i++ {
		lookup[VALUES_FAMILY+":"+strconv.Itoa(i)] = i
	}
	ret := &BigTableTraceStore{
		tileSize:      cfg.TileSize,
		shards:        cfg.Shards,
		table:         client.Open(cfg.Table),
		writesCounter: metrics2.GetCounter("bt_perf_writes", nil),
		lookup:        lookup,
		opsCache:      map[string]*OpsCacheEntry{},
		cacheOps:      cacheOps,
	}

	return ret, nil
}

// Given the index return the TileKey of the tile that would contain that column.
func (b *BigTableTraceStore) TileKey(index int32) TileKey {
	return TileKeyFromOffset(index / b.tileSize)
}

// Returns the offset within a tile for the given index.
func (b *BigTableTraceStore) OffsetFromIndex(index int32) int32 {
	return index % b.tileSize
}

// getOps returns the OpsCacheEntry for a given tile.
//
// Note that it will create a new OpsCacheEntry if none exists.
//
// getOps returns true if the returned value came from BT, false if it came
// from the cache.
func (b *BigTableTraceStore) getOPS(tileKey TileKey) (*OpsCacheEntry, bool, error) {
	if b.cacheOps {
		b.mutex.RLock()
		entry, ok := b.opsCache[tileKey.OpsRowName()]
		b.mutex.RUnlock()
		if ok {
			return entry, true, nil
		} else {
			sklog.Infof("OPS Cache is empty.")
		}
	}
	tctx, cancel := context.WithTimeout(context.Background(), TIMEOUT)
	defer cancel()
	row, err := b.getTable().ReadRow(tctx, tileKey.OpsRowName(), bigtable.RowFilter(bigtable.LatestNFilter(1)))
	if err != nil {
		return nil, false, fmt.Errorf("Failed to read OPS from BigTable for %s: %s", tileKey.OpsRowName(), err)
	}
	// If there is no entry in BigTable then return an empty OPS.
	if len(row) == 0 {
		sklog.Warningf("Failed to read OPS from BT for %s.", tileKey.OpsRowName())
		entry, err := NewOpsCacheEntry()
		return entry, false, err
	}
	entry, err := NewOpsCacheEntryFromRow(row)
	if err == nil && b.cacheOps {
		b.mutex.Lock()
		defer b.mutex.Unlock()
		b.opsCache[tileKey.OpsRowName()] = entry
	}
	return entry, true, err
}

// GetOrderedParamSet returns the OPS for the given tile.
func (b *BigTableTraceStore) GetOrderedParamSet(ctx context.Context, tileKey TileKey) (*paramtools.OrderedParamSet, error) {
	ctx, span := trace.StartSpan(ctx, "BigTableTraceStore.GetOrderedParamSet")
	defer span.End()

	sklog.Infof("OPS for %d", tileKey.Offset())
	entry, _, err := b.getOPS(tileKey)
	if err != nil {
		return nil, err
	}
	return entry.ops, nil
}

// WriteTraces writes the given values into the store.
//
// The keys of 'values' must be the OPS encoded Params of the trace, i.e. at this point we know the OPS has been updated.
func (b *BigTableTraceStore) WriteTraces(index int32, values map[string]float32, source string, timestamp time.Time) error {
	sourceHash := md5.Sum([]byte(source))
	tileKey := TileKeyFromOffset(index / b.tileSize)
	col := strconv.Itoa(int(index % b.tileSize))
	ts := bigtable.Time(timestamp)

	// Write values as batches of mutations.
	rowKeys := []string{}
	muts := []*bigtable.Mutation{}

	// Write the "source file hash" -> "source file name" row.
	mut := bigtable.NewMutation()
	mut.Set(HASHES_FAMILY, HASHES_SOURCE_COLUMN, ts, []byte(source))
	muts = append(muts, mut)
	rowKeys = append(rowKeys, fmt.Sprintf("&%x", sourceHash))

	tctx, cancel := context.WithTimeout(context.Background(), WRITE_TIMEOUT)
	defer cancel()
	for k, v := range values {
		mut := bigtable.NewMutation()

		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, math.Float32bits(v))
		mut.Set(VALUES_FAMILY, col, ts, buf)
		mut.Set(SOURCES_FAMILY, col, ts, sourceHash[:])
		muts = append(muts, mut)
		rowKeys = append(rowKeys, tileKey.TraceRowName(k, b.shards))
		if len(muts) >= MAX_MUTATIONS {
			b.writesCounter.Inc(int64(len(muts)))
			errs, err := b.getTable().ApplyBulk(tctx, rowKeys, muts)
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
		b.writesCounter.Inc(int64(len(muts)))
		errs, err := b.getTable().ApplyBulk(tctx, rowKeys, muts)
		if err != nil {
			return fmt.Errorf("Failed writing traces: %s", err)
		}
		if errs != nil {
			return fmt.Errorf("Failed writing some traces: %v", errs)
		}
	}

	return nil
}

// ReadTraces loads the traces for the given keys.
//
// Note that the keys are the structured keys, ReadTraces will convert them into OPS encoded keys.
func (b *BigTableTraceStore) ReadTraces(tileKey TileKey, keys []string) (map[string][]float32, error) {
	// First encode all the keys by the OrderedParamSet of the given tile.
	ops, err := b.GetOrderedParamSet(context.TODO(), tileKey)
	if err != nil {
		return nil, fmt.Errorf("Failed to get OPS: %s", err)
	}
	// Also map[encodedKey]originalKey so we can construct our response.
	encodedKeys := map[string]string{}
	rowSet := bigtable.RowList{}
	for _, key := range keys {
		params, err := query.ParseKey(key)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse key %q: %s", key, err)
		}
		// Not all keys may appear in all tiles, that's ok.
		encodedKey, err := ops.EncodeParamsAsString(paramtools.Params(params))
		if err != nil {
			continue
		}
		encodedKeys[encodedKey] = key
		encodedKey = tileKey.TraceRowName(encodedKey, b.shards)
		rowSet = append(rowSet, encodedKey)
	}
	ret := map[string][]float32{}

	tctx, cancel := context.WithTimeout(context.Background(), TIMEOUT)
	defer cancel()
	err = b.getTable().ReadRows(tctx, rowSet, func(row bigtable.Row) bool {
		vec := vec32.New(int(b.tileSize))
		for _, col := range row[VALUES_FAMILY] {
			vec[b.lookup[col.Column]] = math.Float32frombits(binary.LittleEndian.Uint32(col.Value))
		}
		parts := strings.Split(row.Key(), ":")
		ret[encodedKeys[parts[2]]] = vec
		return true
	}, bigtable.RowFilter(
		bigtable.ChainFilters(
			bigtable.LatestNFilter(1),
			bigtable.FamilyFilter(VALUES_FAMILY),
		)))
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// QueryTraces returns a map of encoded keys to a slice of floats for all
// traces that match the given query.
func (b *BigTableTraceStore) QueryTraces(ctx context.Context, tileKey TileKey, q *regexp.Regexp) (map[string][]float32, error) {
	ctx, span := trace.StartSpan(ctx, "BigTableTraceStore.QueryTraces")
	defer span.End()

	defer timer.New("btts_query_traces").Stop()
	defer metrics2.FuncTimer().Stop()
	var mutex sync.Mutex
	ret := map[string][]float32{}
	var g errgroup.Group
	tctx, cancel := context.WithTimeout(ctx, TIMEOUT)
	defer cancel()
	// Spawn one Go routine for each shard.
	for i := int32(0); i < b.shards; i++ {
		i := i
		g.Go(func() error {
			rowRegex := tileKey.TraceRowPrefix(i) + ".*" + q.String()
			return b.getTable().ReadRows(tctx, bigtable.PrefixRange(tileKey.TraceRowPrefix(i)), func(row bigtable.Row) bool {
				vec := vec32.New(int(b.tileSize))
				for _, col := range row[VALUES_FAMILY] {
					vec[b.lookup[col.Column]] = math.Float32frombits(binary.LittleEndian.Uint32(col.Value))
				}
				parts := strings.Split(row.Key(), ":")
				mutex.Lock()
				ret[parts[2]] = vec
				mutex.Unlock()
				return true
			}, bigtable.RowFilter(
				bigtable.ChainFilters(
					bigtable.LatestNFilter(1),
					bigtable.RowKeyFilter(rowRegex),
					bigtable.FamilyFilter(VALUES_FAMILY),
				)))
		})
	}
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("Failed to query: %s", err)
	}

	return ret, nil
}

// QueryCount does the same work as QueryTraces but only returns the number of traces
// that would be returned.
func (b *BigTableTraceStore) QueryCount(tileKey TileKey, q *regexp.Regexp) (int64, error) {
	var g errgroup.Group
	ret := int64(0)
	tctx, cancel := context.WithTimeout(context.Background(), TIMEOUT)
	defer cancel()
	// Spawn one Go routine for each shard.
	for i := int32(0); i < b.shards; i++ {
		i := i
		g.Go(func() error {
			rowRegex := tileKey.TraceRowPrefix(i) + ".*" + q.String()
			return b.getTable().ReadRows(tctx, bigtable.PrefixRange(tileKey.TraceRowPrefix(i)), func(row bigtable.Row) bool {
				_ = atomic.AddInt64(&ret, 1)
				return true
			}, bigtable.RowFilter(
				bigtable.ChainFilters(
					bigtable.LatestNFilter(1),
					bigtable.RowKeyFilter(rowRegex),
					bigtable.FamilyFilter(VALUES_FAMILY),
					bigtable.CellsPerRowLimitFilter(1),
					bigtable.StripValueFilter(),
				)))
		})
	}
	if err := g.Wait(); err != nil {
		return -1, fmt.Errorf("Failed to query: %s", err)
	}

	return ret, nil
}

// TileKeys returns all the keys for the given tile..
func (b *BigTableTraceStore) TileKeys(tileKey TileKey) ([]string, error) {
	var g errgroup.Group

	// Track the StringSets, one per shard.
	stringSets := []*util.StringSet{}
	tctx, cancel := context.WithTimeout(context.Background(), TIMEOUT)
	defer cancel()

	// Spawn one Go routine for each shard.
	for i := int32(0); i < b.shards; i++ {
		i := i
		ss := util.StringSet{}
		stringSets = append(stringSets, &ss)
		g.Go(func() error {
			return b.getTable().ReadRows(tctx, bigtable.PrefixRange(tileKey.TraceRowPrefix(i)), func(row bigtable.Row) bool {
				parts := strings.Split(row.Key(), ":")
				ss[parts[2]] = true
				return true
			}, bigtable.RowFilter(
				bigtable.ChainFilters(
					bigtable.LatestNFilter(1),
					bigtable.FamilyFilter(VALUES_FAMILY),
					bigtable.CellsPerRowLimitFilter(1),
					bigtable.StripValueFilter(),
				)))
		})
	}
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("Failed to query: %s", err)
	}

	// Now sum up all the StringSets.
	total := util.StringSet{}
	for _, ss := range stringSets {
		for val := range *ss {
			total[val] = true
		}
	}

	return total.Keys(), nil
}

// GetSource returns the full GCS URL of the file that contained the point at 'index' of trace 'traceId'.
//
// The traceId must be an OPS encoded key.
func (b *BigTableTraceStore) GetSource(index int32, traceId string) (string, error) {
	tileKey := TileKeyFromOffset(index / b.tileSize)
	tctx, cancel := context.WithTimeout(context.Background(), TIMEOUT)
	defer cancel()
	row, err := b.getTable().ReadRow(tctx, tileKey.TraceRowName(traceId, b.shards), bigtable.RowFilter(
		bigtable.ChainFilters(
			bigtable.LatestNFilter(1),
			bigtable.FamilyFilter(SOURCES_FAMILY),
			bigtable.ColumnFilter(fmt.Sprintf("^%d$", index%b.tileSize)),
		),
	))
	if err != nil {
		return "", fmt.Errorf("Failed to read source row: %s", err)
	}
	if len(row) == 0 {
		return "", fmt.Errorf("No source found.")
	}
	sourceHash := fmt.Sprintf("&%x", row[SOURCES_FAMILY][0].Value)

	row, err = b.getTable().ReadRow(tctx, sourceHash, bigtable.RowFilter(
		bigtable.ChainFilters(
			bigtable.LatestNFilter(1),
			bigtable.FamilyFilter(HASHES_FAMILY),
		),
	))
	if err != nil {
		return "", fmt.Errorf("Failed to read source row: %s", err)
	}
	if len(row) == 0 {
		return "", fmt.Errorf("No source found.")
	}

	return string(row[HASHES_FAMILY][0].Value), nil
}

// GetLatestTile returns the latest, i.e. the newest tile.
func (b *BigTableTraceStore) GetLatestTile() (TileKey, error) {
	ret := BadTileKey
	tctx, cancel := context.WithTimeout(context.Background(), TIMEOUT)
	defer cancel()
	err := b.getTable().ReadRows(tctx, bigtable.PrefixRange("@"), func(row bigtable.Row) bool {
		var err error
		ret, err = TileKeyFromOpsRowName(row.Key())
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

// UpdateOrderedParamSet will add all params from 'p' to the OrderedParamSet
// for 'tileKey' and write it back to BigTable.
func (b *BigTableTraceStore) UpdateOrderedParamSet(tileKey TileKey, p paramtools.ParamSet) (*paramtools.OrderedParamSet, error) {
	tctx, cancel := context.WithTimeout(context.Background(), WRITE_TIMEOUT)
	defer cancel()
	var newEntry *OpsCacheEntry
	for {
		var err error
		// Get OPS.
		entry, existsInBT, err := b.getOPS(tileKey)
		if err != nil {
			return nil, fmt.Errorf("Failed to get OPS: %s", err)
		}

		// If the OPS contains our paramset then we're done.
		if delta := entry.ops.Delta(p); len(delta) == 0 {
			sklog.Infof("No delta in UpdateOrderedParamSet for %s. Nothing to do.", tileKey.OpsRowName())
			return entry.ops, nil
		}

		// Create a new updated ops.
		ops := entry.ops.Copy()
		ops.Update(p)
		newEntry, err = opsCacheEntryFromOPS(ops)
		encodedOps, err := newEntry.ops.Encode()
		sklog.Infof("Writing updated OPS for %s. hash: new: %q old: %q", tileKey.OpsRowName(), newEntry.hash, entry.hash)
		if err != nil {
			return nil, fmt.Errorf("Failed to encode new ops: %s", err)
		}

		now := bigtable.Time(time.Now())
		condTrue := false
		if existsInBT {
			// Create an update that avoids the lost update problem.
			cond := bigtable.ChainFilters(
				bigtable.FamilyFilter(OPS_FAMILY),
				bigtable.ColumnFilter(OPS_HASH_COLUMN),
				bigtable.ValueFilter(string(entry.hash)),
			)
			updateMutation := bigtable.NewMutation()
			updateMutation.Set(OPS_FAMILY, OPS_HASH_COLUMN, now, []byte(newEntry.hash))
			updateMutation.Set(OPS_FAMILY, OPS_OPS_COLUMN, now, encodedOps)

			// Add a mutation that cleans up old versions.
			before := bigtable.Time(time.Now().Add(-1 * time.Second))
			updateMutation.DeleteTimestampRange(OPS_FAMILY, OPS_HASH_COLUMN, 0, before)
			updateMutation.DeleteTimestampRange(OPS_FAMILY, OPS_OPS_COLUMN, 0, before)
			condUpdate := bigtable.NewCondMutation(cond, updateMutation, nil)

			if err := b.getTable().Apply(tctx, tileKey.OpsRowName(), condUpdate, bigtable.GetCondMutationResult(&condTrue)); err != nil {
				sklog.Warningf("Failed to apply: %s", err)
				return nil, err
			}

			// If !condTrue then we need to try again,
			// and clear our local cache.
			if !condTrue {
				sklog.Warningf("Exists !condTrue - trying again.")
				b.mutex.Lock()
				delete(b.opsCache, tileKey.OpsRowName())
				b.mutex.Unlock()
				continue
			}
		} else {
			sklog.Infof("FIRST WRITE")
			// Create an update that only works if the ops entry doesn't exist yet.
			// I.e. only apply the mutation if the HASH column doesn't exist for this row.
			cond := bigtable.ChainFilters(
				bigtable.FamilyFilter(OPS_FAMILY),
				bigtable.ColumnFilter(OPS_HASH_COLUMN),
			)
			updateMutation := bigtable.NewMutation()
			updateMutation.Set(OPS_FAMILY, OPS_HASH_COLUMN, now, []byte(newEntry.hash))
			updateMutation.Set(OPS_FAMILY, OPS_OPS_COLUMN, now, encodedOps)

			condUpdate := bigtable.NewCondMutation(cond, nil, updateMutation)
			if err := b.getTable().Apply(tctx, tileKey.OpsRowName(), condUpdate, bigtable.GetCondMutationResult(&condTrue)); err != nil {
				sklog.Warningf("Failed to apply: %s", err)
				continue
			}

			// If condTrue then we need to try again,
			// and clear our local cache.
			if condTrue {
				sklog.Warningf("First Write condTrue - trying again.")
				continue
			}
		}

		// Successfully wrote OPS, so update the cache.
		if b.cacheOps == true {
			b.mutex.Lock()
			defer b.mutex.Unlock()
			b.opsCache[tileKey.OpsRowName()] = newEntry
		}
		break
	}
	return newEntry.ops, nil
}
