/*
Package btts contains the BigTableTraceStore.

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
	multierror "github.com/hashicorp/go-multierror"
	lru "github.com/hashicorp/golang-lru"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/tracestore/btts/engine"
	"go.skia.org/infra/perf/go/types"
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
	INDEX_FAMILY   = "I"

	// Row prefixes.
	INDEX_PREFIX = "j" // An old style of indices uses 'i', so we use 'j' to avoid conflicts.

	// Columns in the "H" column family.
	HASHES_SOURCE_COLUMN        = "S" // Column
	HASHES_SOURCE_FULL_COL_NAME = HASHES_FAMILY + ":" + HASHES_SOURCE_COLUMN

	// Columns in the "D" column family.
	OPS_HASH_COLUMN    = "H"   // Column
	OPS_OPS_COLUMN     = "OPS" // Column
	HASH_FULL_COL_NAME = OPS_FAMILY + ":" + OPS_HASH_COLUMN
	OPS_FULL_COL_NAME  = OPS_FAMILY + ":" + OPS_OPS_COLUMN

	// Columns in the "I" (Index) column family.
	EMPTY_INDEX_COLUMN = "E" // Just contains empty byte slices.

	// MAX_MUTATIONS is the max number of mutations we can send in a single ApplyBulk call. Can be up to 100,000 according to BigTable docs.
	MAX_MUTATIONS = 100000

	// MAX_ROW_BATCH_SIZE is the max number of rowKeys we send in any single
	// ReadRows request. This should keep the size of the serialized request
	// object to under the limit of 1MB and also shard the requests enough to be
	// sufficient.
	MAX_ROW_BATCH_SIZE = 10 * 1000

	// POOL_CHANNEL_SIZE is the channel size that feeds processRows. Adding some
	// buffering seemed to help performance.
	POOL_CHANNEL_SIZE = 1000

	// MAX_INDEX_LRU_CACHE is the size of the lru cache where we remember if
	// we've already stored indices for a rowKey.
	MAX_INDEX_LRU_CACHE = 20 * 1000 * 1000

	// MAX_WRITE_INDICES_BATCH_SIZE is the size of a batch of trace ids to write
	// indices for in WriteIndices. We need to cap the size otherwise we will
	// read all the trace ids into memory before starting to write indices,
	// which uses too much memory at one time.
	MAX_WRITE_INDICES_BATCH_SIZE = 100000

	TIMEOUT       = 4 * time.Minute
	WRITE_TIMEOUT = 10 * time.Minute
)

var (
	EMPTY_VALUE = []byte{}
)

// bttsTileKey is the identifier for each tile held in BigTable.
//
// Note that tile keys are in the opposite order of types.TileNumber, that is,
// the first tile for a repo would be 0, and then 1, etc. Those are the offsets,
// and the most recent tile has the largest offset. To make it easy to find the
// most recent tile we calculate tilekey as math.MaxInt32 - tileoffset, so that
// more recent tiles come first in sort order.
type bttsTileKey int32

// badBttsTileKey is returned in error conditions.
const badBttsTileKey = bttsTileKey(-1)

// TileKeyFromTileNumber returns a TileKey from the tile offset.
func TileKeyFromTileNumber(tileNumber types.TileNumber) bttsTileKey {
	if tileNumber < 0 {
		return badBttsTileKey
	}
	return bttsTileKey(math.MaxInt32 - tileNumber)
}

// Prev returns the TileKey for the previous tile.
func (t bttsTileKey) Prev() bttsTileKey {
	return TileKeyFromTileNumber(t.Offset() - 1)
}

// OpsRowName returns the name of the BigTable row that the OrderedParamSet for this tile is stored at.
func (t bttsTileKey) OpsRowName() string {
	return fmt.Sprintf("@%07d", t)
}

// TraceRowPrefix returns the prefix of a BigTable row name for any Trace in this tile.
func (t bttsTileKey) TraceRowPrefix(shard int32) string {
	return fmt.Sprintf("%d:%07d:", shard, t)
}

// IndexRowPrefix returns the prefix of a BigTable row name for any Indices in this tile.
func (t bttsTileKey) IndexRowPrefix() string {
	return fmt.Sprintf("%s%07d", INDEX_PREFIX, t)
}

// TraceRowName returns the BigTable row name for the given trace, for the given
// number of shards. The returned row name includes the shard.
//
// TraceRowName(",0=1,", 3) -> 2:2147483647:,0=1,
func (t bttsTileKey) TraceRowName(traceId string, shards int32) string {
	ret, _ := t.TraceRowNameAndShard(traceId, shards)
	return ret
}

// TraceRowName returns the BigTable row name for the given trace, for the given
// number of shards. The returned row name includes the shard.
//
// TraceRowName(",0=1,", 3) -> 2:2147483647:,0=1,
func (t bttsTileKey) TraceRowNameAndShard(traceId string, shards int32) (string, uint32) {
	shard := crc32.ChecksumIEEE([]byte(traceId)) % uint32(shards)
	return fmt.Sprintf("%d:%07d:%s", shard, t, traceId), shard
}

// Offset returns the tile number, i.e. the not-reversed number.
func (t bttsTileKey) Offset() types.TileNumber {
	return types.TileNumber(math.MaxInt32 - int32(t))
}

func TileKeyFromOpsRowName(s string) (bttsTileKey, error) {
	if s[:1] != "@" {
		return badBttsTileKey, fmt.Errorf("TileKey strings must begin with @: Got %q", s)
	}
	i, err := strconv.ParseInt(s[1:], 10, 32)
	if err != nil {
		return badBttsTileKey, err
	}
	return bttsTileKey(i), nil
}

// When ingesting we keep a cache of the OrderedParamSets we have seen per-tile.
type opsCacheEntry struct {
	ops  *paramtools.OrderedParamSet
	hash string // md5 has of the serialized ops.
}

func opsCacheEntryFromOPS(ops *paramtools.OrderedParamSet) (*opsCacheEntry, error) {
	buf, err := ops.Encode()
	if err != nil {
		return nil, err
	}
	hash := fmt.Sprintf("%x", md5.Sum(buf))
	return &opsCacheEntry{
		ops:  ops,
		hash: hash,
	}, nil
}

func NewOpsCacheEntry() (*opsCacheEntry, error) {
	return opsCacheEntryFromOPS(paramtools.NewOrderedParamSet())
}

func NewOpsCacheEntryFromRow(row bigtable.Row) (*opsCacheEntry, error) {
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
	tileSize           int32 // How many commits we store per tile.
	shards             int32 // How many shards we break the traces into.
	writesCounter      metrics2.Counter
	indexWritesCounter metrics2.Counter
	table              *bigtable.Table

	// indexed is an LRU cache of all the trace row keys that have been added to
	// the index. Note that the trace row keys include the tile and the shard,
	// so we don't get any duplicates. Since the number of incoming trace ids is
	// unbounded we can only use an LRU cache here.
	indexed *lru.Cache

	// lookup maps column names as strings, "V:2", to the index, 2.
	// Used to speed up reading values out of rows.
	lookup map[string]int

	cacheOps bool // Should we use the opsCache? Only true for ingesters, always false for Perf frontends.

	mutex    sync.RWMutex              // Protects opsCache.
	opsCache map[string]*opsCacheEntry // map[tile] -> ops.
}

// See type.TraceStore.
func (b *BigTableTraceStore) TileSize() int32 {
	return b.tileSize
}

func (b *BigTableTraceStore) getTable() *bigtable.Table {
	return b.table
}

func NewBigTableTraceStoreFromConfig(ctx context.Context, cfg *config.InstanceConfig, ts oauth2.TokenSource, cacheOps bool) (*BigTableTraceStore, error) {
	if cfg.DataStoreConfig.TileSize <= 0 {
		return nil, fmt.Errorf("tileSize must be >0. %d", cfg.DataStoreConfig.TileSize)
	}
	client, err := bigtable.NewClient(ctx, cfg.DataStoreConfig.Project, cfg.DataStoreConfig.Instance, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("Couldn't create client: %s", err)
	}
	lookup := map[string]int{}
	for i := 0; int32(i) < cfg.DataStoreConfig.TileSize; i++ {
		lookup[VALUES_FAMILY+":"+strconv.Itoa(i)] = i
	}
	indexed, err := lru.New(MAX_INDEX_LRU_CACHE)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create index lru cache: %s", err)
	}
	ret := &BigTableTraceStore{
		tileSize:           cfg.DataStoreConfig.TileSize,
		shards:             cfg.DataStoreConfig.Shards,
		table:              client.Open(cfg.DataStoreConfig.Table),
		indexed:            indexed,
		writesCounter:      metrics2.GetCounter("bt_perf_writes", nil),
		indexWritesCounter: metrics2.GetCounter("bt_perf_index_writes", nil),
		lookup:             lookup,
		opsCache:           map[string]*opsCacheEntry{},
		cacheOps:           cacheOps,
	}

	return ret, nil
}

// tileKey returns the tileKey of the tile that would contain that index.
func (b *BigTableTraceStore) tileKey(commitNumber types.CommitNumber) bttsTileKey {
	return TileKeyFromTileNumber(types.TileNumberFromCommitNumber(commitNumber, b.TileSize()))
}

// TileNumber implements the TraceStore interface.
func (b *BigTableTraceStore) TileNumber(commitNumber types.CommitNumber) types.TileNumber {
	return types.TileNumberFromCommitNumber(commitNumber, b.TileSize())
}

// OffsetFromCommitNumber returns the offset within a tile for the given index.
func (b *BigTableTraceStore) OffsetFromCommitNumber(commitNumber types.CommitNumber) int32 {
	return int32(commitNumber) % b.tileSize
}

// CommitNumberOfTileStart implements the tracestore.TraceStore interface.
func (b *BigTableTraceStore) CommitNumberOfTileStart(commitNumber types.CommitNumber) types.CommitNumber {
	return types.CommitNumber(int32(b.tileKey(commitNumber).Offset()) * b.tileSize)
}

// getOps returns the OpsCacheEntry for a given tile.
//
// Note that it will create a new OpsCacheEntry if none exists.
//
// getOps returns true if the returned value came from BT, false if it came
// from the cache.
func (b *BigTableTraceStore) getOPS(tileKey bttsTileKey) (*opsCacheEntry, bool, error) {
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

// GetOrderedParamSet implements the tracestore.TraceStore interface.
func (b *BigTableTraceStore) GetOrderedParamSet(ctx context.Context, tileNumber types.TileNumber, _ time.Time) (*paramtools.OrderedParamSet, error) {
	tileKey := TileKeyFromTileNumber(tileNumber)
	ctx, span := trace.StartSpan(ctx, "BigTableTraceStore.GetOrderedParamSet")
	defer span.End()

	sklog.Infof("OPS for %d", tileKey.Offset())
	entry, _, err := b.getOPS(tileKey)
	if err != nil {
		return nil, err
	}
	return entry.ops, nil
}

// WriteTraces implements the tracestore.TraceStore interface.
func (b *BigTableTraceStore) WriteTraces(commitNumber types.CommitNumber, params []paramtools.Params, values []float32, paramset paramtools.ParamSet, source string, timestamp time.Time) error {
	tileKey := b.tileKey(commitNumber)
	ops, err := b.updateOrderedParamSet(tileKey, paramset)
	if err != nil {
		return fmt.Errorf("Could not write traces, failed to update OPS: %s", err)
	}

	sourceHash := md5.Sum([]byte(source))
	col := strconv.Itoa(int(b.OffsetFromCommitNumber(commitNumber)))
	ts := bigtable.Time(timestamp)

	// Write values as batches of mutations.
	rowKeys := []string{}
	muts := []*bigtable.Mutation{}

	// Write the "source file hash" -> "source file name" row.
	mut := bigtable.NewMutation()
	mut.Set(HASHES_FAMILY, HASHES_SOURCE_COLUMN, ts, []byte(source))
	muts = append(muts, mut)
	rowKeys = append(rowKeys, fmt.Sprintf("&%x", sourceHash))

	// indices maps rowKeys to encoded Params.
	indices := map[string]paramtools.Params{}

	// TODO(jcgregorio) Pass in a context to WriteTraces.
	tctx := context.TODO()
	for i, v := range values {
		mut := bigtable.NewMutation()

		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, math.Float32bits(v))
		mut.Set(VALUES_FAMILY, col, ts, buf)
		mut.Set(SOURCES_FAMILY, col, ts, sourceHash[:])
		muts = append(muts, mut)
		encodedKey, err := ops.EncodeParamsAsString(params[i])
		if err != nil {
			sklog.Warningf("Failed to encode key %q: %s", params[i], err)
			continue
		}
		rowKey := tileKey.TraceRowName(encodedKey, b.shards)
		rowKeys = append(rowKeys, rowKey)
		indices[encodedKey], err = ops.EncodeParams(params[i])
		if err != nil {
			sklog.Warningf("Failed to encode key %q: %s", params[i], err)
			continue
		}
		if len(muts) >= MAX_MUTATIONS {
			if err := b.writeBatchOfTraces(tctx, rowKeys, muts); err != nil {
				return err
			}
			rowKeys = []string{}
			muts = []*bigtable.Mutation{}
		}
	}
	if len(muts) > 0 {
		if err := b.writeBatchOfTraces(tctx, rowKeys, muts); err != nil {
			return err
		}
	}

	return b.writeTraceIndices(tctx, tileKey, indices, ts, false)
}

// writeTraceIndices writes the indices for the given traces.
//
// The 'indices' maps encoded keys to the encoded Params of the key. If
// bypassCache is true then we don't look in the lru cache to see if we've
// already written the indices for a given key.
func (b *BigTableTraceStore) writeTraceIndices(tctx context.Context, tileKey bttsTileKey, indices map[string]paramtools.Params, ts bigtable.Timestamp, bypassCache bool) error {
	// Write indices as batches of mutations.
	indexRowKeys := []string{}
	muts := []*bigtable.Mutation{}

	for rowKey, p := range indices {
		if !bypassCache {
			// Add in the tile
			fullRowKey := tileKey.TraceRowName(rowKey, b.shards)
			if _, ok := b.indexed.Get(fullRowKey); ok {
				continue
			}
		}
		for k, v := range p {
			mut := bigtable.NewMutation()
			// The row keys for index rows contain all the data, but we have to
			// write *some* data into a cell, so we write an empty byte slice.
			mut.Set(INDEX_FAMILY, EMPTY_INDEX_COLUMN, ts, EMPTY_VALUE)
			muts = append(muts, mut)
			indexRowKeys = append(indexRowKeys, fmt.Sprintf("%s:%s:%s:%s", tileKey.IndexRowPrefix(), k, v, rowKey))
			if len(muts) >= MAX_MUTATIONS {
				if err := b.writeBatchOfIndices(tctx, indexRowKeys, muts); err != nil {
					return skerr.Wrap(err)
				}
				indexRowKeys = []string{}
				muts = []*bigtable.Mutation{}
			}
		}
	}
	if len(muts) > 0 {
		if err := b.writeBatchOfIndices(tctx, indexRowKeys, muts); err != nil {
			return skerr.Wrap(err)
		}
	}
	for rowKey := range indices {
		fullRowKey := tileKey.TraceRowName(rowKey, b.shards)
		b.indexed.Add(fullRowKey, "")
	}
	return nil
}

// CountIndices implements the tracestore.TraceStore interface.
func (b *BigTableTraceStore) CountIndices(ctx context.Context, tileNumber types.TileNumber) (int64, error) {
	tileKey := TileKeyFromTileNumber(tileNumber)
	ret := int64(0)
	rowRegex := tileKey.IndexRowPrefix() + ":.*"
	err := b.getTable().ReadRows(ctx, bigtable.PrefixRange(tileKey.IndexRowPrefix()+":"), func(row bigtable.Row) bool {
		ret++
		return true
	}, bigtable.RowFilter(
		bigtable.ChainFilters(
			bigtable.LatestNFilter(1),
			bigtable.RowKeyFilter(rowRegex),
			bigtable.FamilyFilter(INDEX_FAMILY),
			bigtable.CellsPerRowLimitFilter(1),
			bigtable.StripValueFilter(),
		)))
	return ret, err
}

// WriteIndices implements the tracestore.TraceStore interface.
func (b *BigTableTraceStore) WriteIndices(ctx context.Context, tileNumber types.TileNumber) error {
	tileKey := TileKeyFromTileNumber(tileNumber)
	// The full set of trace ids can't fit in memory, do this as an incremental process.
	total := 0
	out, errCh := b.tileKeys(ctx, tileNumber)
	indices := map[string]paramtools.Params{}
	for encodedKey := range out {
		total++
		encodedParams, err := query.ParseKeyFast(encodedKey)
		if err != nil {
			sklog.Warningf("Failed to parse key: %s", err)
			continue
		}
		indices[encodedKey] = encodedParams
		if len(indices) >= MAX_WRITE_INDICES_BATCH_SIZE {
			if err := b.writeTraceIndices(ctx, tileKey, indices, bigtable.Time(time.Now()), true); err != nil {
				return err
			}
			indices = map[string]paramtools.Params{}
			sklog.Infof("Total traces processed: %d", total)
		}
	}
	if len(indices) > 0 {
		if err := b.writeTraceIndices(ctx, tileKey, indices, bigtable.Time(time.Now()), true); err != nil {
			return err
		}
		sklog.Infof("Total traces processed: %d", total)
	}

	// Check for errors in errCh.
	var multipleErrors error
	for err := range errCh {
		multipleErrors = multierror.Append(err, multipleErrors)
	}
	if multipleErrors != nil {
		return multipleErrors
	}
	return nil
}

// writeBatchOfTraces writes the traces to BT.
//
// Note that 'rowKeys' are the keys for 'muts', so they need to be the
// same length and be ordered accordingly.
func (b *BigTableTraceStore) writeBatchOfTraces(ctx context.Context, rowKeys []string, muts []*bigtable.Mutation) error {
	errs, err := b.getTable().ApplyBulk(ctx, rowKeys, muts)
	if err != nil {
		return fmt.Errorf("Failed writing traces: %s", err)
	}
	if errs != nil {
		return fmt.Errorf("Failed writing some traces: %v", errs)
	}
	b.writesCounter.Inc(int64(len(muts)))
	return nil
}

// writeBatchOfIndices writes the indices to BT.
//
// Note that 'indexRowKeys' are the keys for 'muts', so they need to be the
// same length and be ordered accordingly.
func (b *BigTableTraceStore) writeBatchOfIndices(ctx context.Context, indexRowKeys []string, muts []*bigtable.Mutation) error {
	sklog.Infof("writeBatchOfIndices %d", len(indexRowKeys))
	errs, err := b.getTable().ApplyBulk(ctx, indexRowKeys, muts)
	if err != nil {
		return fmt.Errorf("Failed writing indices: %s", err)
	}
	if errs != nil {
		return fmt.Errorf("Failed writing some indices: %v", errs)
	}
	b.indexWritesCounter.Inc(int64(len(muts)))
	return nil
}

// ReadTraces implements the tracestore.TraceStore interface.
func (b *BigTableTraceStore) ReadTraces(tileNumber types.TileNumber, keys []string) (types.TraceSet, error) {
	tileKey := TileKeyFromTileNumber(tileNumber)
	// First encode all the keys by the OrderedParamSet of the given tile.
	ops, err := b.GetOrderedParamSet(context.TODO(), tileNumber, time.Now())
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
	ret := types.TraceSet{}

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

func (b *BigTableTraceStore) regexpFromQuery(ctx context.Context, tileNumber types.TileNumber, q *query.Query) (*regexp.Regexp, error) {
	// Get the OPS, which we need to encode the query, and decode the traceids of the results.
	ops, err := b.GetOrderedParamSet(ctx, tileNumber, time.Now())
	if err != nil {
		return nil, err
	}
	// Convert query to regex.
	r, err := q.Regexp(ops)
	if err != nil {
		return nil, fmt.Errorf("Failed to compile query regex: %s", err)
	}
	if !q.Empty() && r.String() == "" {
		// Not an error, we just won't match anything in this tile. This
		// condition occurs if a new key appears from one tile to the next, in
		// which case Regexp(ops) returns "" for the Tile that's never seen the
		// key.
		return nil, fmt.Errorf("Query matches all traces, which we'll ignore.")
	}
	return r, nil
}

func traceIdFromEncoded(ops *paramtools.OrderedParamSet, encoded string) (string, error) {
	p, err := ops.DecodeParamsFromString(encoded)
	if err != nil {
		return "", fmt.Errorf("Failed to decode key: %s", err)
	}
	return query.MakeKeyFast(p)
}

// queryTraces returns a map of encoded keys to a slice of floats for all
// traces that match the given query.
//
// TODO(jcgregorio) Remove this func.
func (b *BigTableTraceStore) queryTraces(ctx context.Context, tileNumber types.TileNumber, q *query.Query) (types.TraceSet, error) {
	tileKey := TileKeyFromTileNumber(tileNumber)
	ctx, span := trace.StartSpan(ctx, "BigTableTraceStore.QueryTraces")
	defer span.End()

	ops, err := b.GetOrderedParamSet(ctx, tileNumber, time.Now())
	if err != nil {
		return nil, fmt.Errorf("Failed to get OPS: %s", err)
	}

	// Convert query to regex.
	r, err := b.regexpFromQuery(ctx, tileNumber, q)
	if err != nil {
		sklog.Infof("Failed to compile query regex: %s", err)
		// Not an error, we just won't match anything in this tile.
		return nil, nil
	}

	defer timer.New("btts_query_traces").Stop()
	defer metrics2.FuncTimer().Stop()
	var mutex sync.Mutex
	ret := types.TraceSet{}
	var g errgroup.Group
	tctx, cancel := context.WithTimeout(ctx, TIMEOUT)
	defer cancel()
	// Spawn one Go routine for each shard.
	for i := int32(0); i < b.shards; i++ {
		i := i
		g.Go(func() error {
			rowRegex := tileKey.TraceRowPrefix(i) + ".*" + r.String()
			return b.getTable().ReadRows(tctx, bigtable.PrefixRange(tileKey.TraceRowPrefix(i)), func(row bigtable.Row) bool {
				vec := vec32.New(int(b.tileSize))
				for _, col := range row[VALUES_FAMILY] {
					vec[b.lookup[col.Column]] = math.Float32frombits(binary.LittleEndian.Uint32(col.Value))
				}
				parts := strings.Split(row.Key(), ":")
				traceId, err := traceIdFromEncoded(ops, parts[2])
				if err != nil {
					sklog.Infof("Found encoded key %q that can't be decoded: %s", parts[2], err)
					return true
				}
				mutex.Lock()
				ret[traceId] = vec
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

// QueryTracesIDOnlyByIndex implements the tracestore.TraceStore interface.
func (b *BigTableTraceStore) QueryTracesIDOnlyByIndex(ctx context.Context, tileNumber types.TileNumber, q *query.Query) (<-chan paramtools.Params, error) {
	tileKey := TileKeyFromTileNumber(tileNumber)
	ctx, span := trace.StartSpan(ctx, "BigTableTraceStore.QueryTracesIDOnlyByIndex")
	defer span.End()
	outParams := make(chan paramtools.Params, engine.QUERY_ENGINE_CHANNEL_SIZE)

	ops, err := b.GetOrderedParamSet(ctx, tileNumber, time.Now())
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get OPS")
	}

	if q.Empty() {
		return nil, skerr.Fmt("Can't run QueryTracesIDOnlyByIndex for the empty query.")
	}

	plan, err := q.QueryPlan(ops)
	sklog.Infof("Plan %#v", plan)
	if err != nil {
		// Not an error, we just won't match anything in this tile.
		//
		// The plan may be invalid because it is querying with keys or values
		// that don't appear in a tile, which means they query won't work on
		// this tile, but it may still work on other tiles, so we just don't
		// return any results for this tile.
		close(outParams)
		return outParams, nil
	}
	if len(plan) == 0 {
		// We won't match anything in this tile.
		close(outParams)
		return outParams, nil
	}
	encodedPlan, err := ops.EncodeParamSet(plan)
	if err != nil {
		close(outParams)
		// Not an error for the same reasons as above, failure to encode just
		// means the query will never match traces in this tile.
		return outParams, nil
	}

	tctx, cancel := context.WithTimeout(context.Background(), TIMEOUT)

	out, err := ExecutePlan(tctx, encodedPlan, b.getTable(), tileKey, fmt.Sprintf("QueryTracesIDOnlyByIndex: %v %d", plan, tileKey.Offset()))
	if err != nil {
		cancel()
		close(outParams)
		return outParams, skerr.Wrap(err)
	}
	go func() {
		defer cancel()
		defer close(outParams)
		for encodedKey := range out {
			p, err := ops.DecodeParamsFromString(encodedKey)
			if err != nil {
				sklog.Errorf("Failed to decode key %q: %s", encodedKey, err)
			}
			outParams <- p
		}
	}()
	return outParams, nil
}

// QueryTracesByIndex implements the tracestore.TraceStore interface.
func (b *BigTableTraceStore) QueryTracesByIndex(ctx context.Context, tileNumber types.TileNumber, q *query.Query) (types.TraceSet, error) {
	tileKey := TileKeyFromTileNumber(tileNumber)
	ctx, span := trace.StartSpan(ctx, "BigTableTraceStore.QueryTracesByIndex")
	defer span.End()

	ops, err := b.GetOrderedParamSet(ctx, tileNumber, time.Now())
	if err != nil {
		return nil, fmt.Errorf("Failed to get OPS: %s", err)
	}

	// We need to differentiate between two cases:
	//   - The query is empty, which means we want all traces.
	//   - The query plan is empty, which means it won't match any
	//     traces in this tile, so pre-emptively return an empty TraceSet.

	// TODO(jcgregorio) Make this case go away as all the traces may exceed available
	// memory.
	if q.Empty() {
		return b.allTraces(ctx, tileNumber)
	}

	plan, err := q.QueryPlan(ops)
	sklog.Infof("Plan %#v", plan)
	if err != nil {
		// Not an error, we just won't match anything in this tile.
		//
		// The plan may be invalid because it is querying with keys or values
		// that don't appear in a tile, which means they query won't work on
		// this tile, but it may still work on other tiles, so we just don't
		// return any results for this tile.
		return nil, nil
	}
	if len(plan) == 0 {
		// We won't match anything in this tile.
		return nil, nil
	}
	encodedPlan, err := ops.EncodeParamSet(plan)
	if err != nil {
		// Not an error for the same reasons as above, failure to encode just
		// means the query will never match traces in this tile.
		return nil, nil
	}

	defer timer.New("btts_query_traces_by_index").Stop()
	defer metrics2.FuncTimer().Stop()

	// mutex protects ret.
	var mutex sync.Mutex
	ret := types.TraceSet{}
	var g errgroup.Group

	tctx, cancel := context.WithTimeout(context.Background(), TIMEOUT)
	defer cancel()

	processRows := func(rowSetSubset bigtable.RowList) error {
		return b.getTable().ReadRows(tctx, rowSetSubset, func(row bigtable.Row) bool {
			vec := vec32.New(int(b.tileSize))
			for _, col := range row[VALUES_FAMILY] {
				vec[b.lookup[col.Column]] = math.Float32frombits(binary.LittleEndian.Uint32(col.Value))
			}
			parts := strings.Split(row.Key(), ":")
			traceId, err := traceIdFromEncoded(ops, parts[2])
			if err != nil {
				sklog.Infof("Found encoded key %q that can't be decoded: %s", parts[2], err)
				return true
			}
			mutex.Lock()
			defer mutex.Unlock()
			ret[traceId] = vec
			return true
		}, bigtable.RowFilter(
			bigtable.ChainFilters(
				bigtable.LatestNFilter(1),
				bigtable.FamilyFilter(VALUES_FAMILY),
			)))
	}

	// Start a fixed number of goroutines to load trace values, one for each shard.
	poolRequestCh := make([]chan string, b.shards)
	for i := range poolRequestCh {
		poolRequestCh[i] = make(chan string, POOL_CHANNEL_SIZE)
	}
	for i := int32(0); i < b.shards; i++ {
		i := i
		g.Go(func() error {
			rowSetSubset := bigtable.RowList{}
			for fullKey := range poolRequestCh[i] {
				rowSetSubset = append(rowSetSubset, fullKey)
				if len(rowSetSubset) > MAX_ROW_BATCH_SIZE {
					if err := processRows(rowSetSubset); err != nil {
						return skerr.Wrap(err)
					}
					rowSetSubset = bigtable.RowList{}
				}
			}
			// Process any remainders.
			if len(rowSetSubset) > 0 {
				if err := processRows(rowSetSubset); err != nil {
					return skerr.Wrap(err)
				}
			}
			return nil
		})
	}

	out, err := ExecutePlan(tctx, encodedPlan, b.getTable(), tileKey, fmt.Sprintf("QueryTracesByIndex: %v %d", plan, tileKey.Offset()))
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to execute plan.")
	}
	// TODO(jcgregorio) Should we limit the total number of traces?
	for encodedTraceId := range out {
		fullKey, shard := tileKey.TraceRowNameAndShard(encodedTraceId, b.shards)
		poolRequestCh[shard] <- fullKey
	}
	for _, pr := range poolRequestCh {
		close(pr)
	}

	if err := g.Wait(); err != nil {
		return nil, skerr.Wrapf(err, "Failed to query.")
	}

	return ret, nil
}

// allTraces returns a map of encoded keys to a slice of floats for all traces.
func (b *BigTableTraceStore) allTraces(ctx context.Context, tileNumber types.TileNumber) (types.TraceSet, error) {
	tileKey := TileKeyFromTileNumber(tileNumber)
	ctx, span := trace.StartSpan(ctx, "BigTableTraceStore.allTraces")
	defer span.End()

	ops, err := b.GetOrderedParamSet(ctx, tileNumber, time.Now())
	if err != nil {
		return nil, fmt.Errorf("Failed to get OPS: %s", err)
	}

	defer timer.New("btts_all_traces").Stop()
	defer metrics2.FuncTimer().Stop()
	var mutex sync.Mutex
	ret := types.TraceSet{}
	var g errgroup.Group
	tctx, cancel := context.WithTimeout(context.Background(), TIMEOUT)
	defer cancel()
	// Spawn one Go routine for each shard.
	for i := int32(0); i < b.shards; i++ {
		i := i
		g.Go(func() error {
			rowRegex := tileKey.TraceRowPrefix(i) + ".*"
			return b.getTable().ReadRows(tctx, bigtable.PrefixRange(tileKey.TraceRowPrefix(i)), func(row bigtable.Row) bool {
				vec := vec32.New(int(b.tileSize))
				for _, col := range row[VALUES_FAMILY] {
					vec[b.lookup[col.Column]] = math.Float32frombits(binary.LittleEndian.Uint32(col.Value))
				}
				parts := strings.Split(row.Key(), ":")
				traceId, err := traceIdFromEncoded(ops, parts[2])
				if err != nil {
					sklog.Infof("Found encoded key %q that can't be decoded: %s", parts[2], err)
					return true
				}
				mutex.Lock()
				ret[traceId] = vec
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

// TraceCount implements the tracestore.TraceStore interface.
func (b *BigTableTraceStore) TraceCount(ctx context.Context, tileNumber types.TileNumber) (int64, error) {
	tileKey := TileKeyFromTileNumber(tileNumber)
	var g errgroup.Group
	ret := int64(0)

	tctx, cancel := context.WithTimeout(context.Background(), TIMEOUT)
	defer cancel()
	// Spawn one Go routine for each shard.
	for i := int32(0); i < b.shards; i++ {
		i := i
		g.Go(func() error {
			rowRegex := tileKey.TraceRowPrefix(i)
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
		return -1, skerr.Wrapf(err, "Failed to query.")
	}

	return ret, nil
}

// tileKeys returns a channel that streams all the encoded keys for the given
// tile in alphabetical order. The errCh will emit an error if one of the BT
// ReadRows fails, and the channel will be closed when the started go routine
// finishes.
func (b *BigTableTraceStore) tileKeys(ctx context.Context, tileNumber types.TileNumber) (<-chan string, <-chan error) {
	tileKey := TileKeyFromTileNumber(tileNumber)
	out := make(chan string)
	errCh := make(chan error)
	var g errgroup.Group

	tctx := context.Background()

	go func() {
		defer close(out)
		defer close(errCh)
		// Spawn one Go routine for each shard.
		for i := int32(0); i < b.shards; i++ {
			i := i
			g.Go(func() error {
				return b.getTable().ReadRows(tctx, bigtable.PrefixRange(tileKey.TraceRowPrefix(i)), func(row bigtable.Row) bool {
					parts := strings.Split(row.Key(), ":")
					// prevent leaking the row data from the slice.
					// https://go101.org/article/memory-leaking.html
					out <- util.CopyString(parts[2])
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
			errCh <- skerr.Wrapf(err, "Failed to query.")
		}
	}()
	return out, errCh
}

// GetSource implements the tracestore.TraceStore interface.
func (b *BigTableTraceStore) GetSource(ctx context.Context, commitNumber types.CommitNumber, traceId string) (string, error) {
	tileKey := b.tileKey(commitNumber)
	tileNumber := types.TileNumberFromCommitNumber(commitNumber, b.tileSize)
	offset := b.OffsetFromCommitNumber(commitNumber)
	ops, err := b.GetOrderedParamSet(ctx, tileNumber, time.Now())
	if err != nil {
		return "", fmt.Errorf("Failed to load OrderedParamSet for tile: %s", err)
	}
	p, err := query.ParseKey(traceId)
	if err != nil {
		return "", fmt.Errorf("Invalid traceid: %s", err)
	}
	encodedTraceId, err := ops.EncodeParamsAsString(p)
	if err != nil {
		return "", fmt.Errorf("Failed to encode key: %s", err)
	}
	tctx, cancel := context.WithTimeout(context.Background(), TIMEOUT)
	defer cancel()
	row, err := b.getTable().ReadRow(tctx, tileKey.TraceRowName(encodedTraceId, b.shards), bigtable.RowFilter(
		bigtable.ChainFilters(
			bigtable.LatestNFilter(1),
			bigtable.FamilyFilter(SOURCES_FAMILY),
			bigtable.ColumnFilter(fmt.Sprintf("^%d$", offset)),
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

// GetLatestTile implements the tracestore.TraceStore interface.
func (b *BigTableTraceStore) GetLatestTile() (types.TileNumber, error) {
	ret := badBttsTileKey
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
		return badBttsTileKey.Offset(), fmt.Errorf("Failed to scan OPS: %s", err)
	}
	if ret == badBttsTileKey {
		return badBttsTileKey.Offset(), fmt.Errorf("Failed to read any OPS from BigTable: %s", err)
	}
	return ret.Offset(), nil
}

// updateOrderedParamSet will add all params from 'p' to the OrderedParamSet
// for 'tileKey' and write it back to BigTable.
func (b *BigTableTraceStore) updateOrderedParamSet(tileKey bttsTileKey, p paramtools.ParamSet) (*paramtools.OrderedParamSet, error) {
	tctx, cancel := context.WithTimeout(context.Background(), WRITE_TIMEOUT)
	defer cancel()
	var newEntry *opsCacheEntry
	for {
		var err error
		// Get OPS.
		entry, existsInBT, err := b.getOPS(tileKey)
		if err != nil {
			return nil, fmt.Errorf("Failed to get OPS: %s", err)
		}

		// If the OPS contains our paramset then we're done.
		if delta := entry.ops.Delta(p); len(delta) == 0 {
			sklog.Infof("No delta in updateOrderedParamSet for %s. Nothing to do.", tileKey.OpsRowName())
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
				bigtable.LatestNFilter(1),
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

// Confirm that BigTableTraceStore fulfills the tracestore.TraceStore interface.
var _ tracestore.TraceStore = (*BigTableTraceStore)(nil)
