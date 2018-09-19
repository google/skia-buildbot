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
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/config"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/option"
)

const (
	VALUES_FAMILY  = "V"
	SOURCES_FAMILY = "S"

	HASHES_FAMILY               = "H"
	HASHES_SOURCE               = "S" // Column
	HASHES_SOURCE_FULL_COL_NAME = HASHES_FAMILY + ":" + HASHES_SOURCE

	OPS_FAMILY         = "D"
	HASH               = "H"   // Column
	OPS                = "OPS" // Column
	HASH_FULL_COL_NAME = OPS_FAMILY + ":" + HASH
	OPS_FULL_COL_NAME  = OPS_FAMILY + ":" + OPS

	MAX_MUTATIONS = 100000 // Can be up to 100,000 according to BigTable docs.
)

type TileKey int32

const BadTileKey = TileKey(-1)

func TileKeyFromOffset(tileOffset int32) TileKey {
	if tileOffset < 0 {
		return BadTileKey
	}
	return TileKey(math.MaxInt32 - tileOffset)
}

func (t TileKey) PrevTile() TileKey {
	return TileKeyFromOffset(t.Offset() - 1)
}

func (t TileKey) OpsRowName() string {
	return fmt.Sprintf("@%07d", t)
}

func (t TileKey) TraceRowPrefix(shard int32) string {
	return fmt.Sprintf("%d:%07d:", shard, t)
}

// TraceRowName(",0=1,", 3) -> 2:2147483647:,0=1,
func (t TileKey) TraceRowName(traceId string, shards int32) string {
	return fmt.Sprintf("%d:%07d:%s", crc32.ChecksumIEEE([]byte(traceId))%uint32(shards), t, traceId)
}

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
	ctx           context.Context
	tileSize      int32
	shards        int32
	table         *bigtable.Table
	writesCounter metrics2.Counter

	lookup map[string]int

	mutex    sync.RWMutex
	opsCache map[string]*OpsCacheEntry // map[tile] -> ops.
}

func NewBigTableTraceStoreFromConfig(ctx context.Context, cfg *config.IngesterConfig, ts oauth2.TokenSource) (*BigTableTraceStore, error) {
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
	return &BigTableTraceStore{
		ctx:           ctx,
		tileSize:      cfg.TileSize,
		shards:        cfg.Shards,
		table:         client.Open(cfg.Table),
		writesCounter: metrics2.GetCounter("bt_perf_writes", nil),
		lookup:        lookup,
		opsCache:      map[string]*OpsCacheEntry{},
	}, nil
}

func (b *BigTableTraceStore) TileKey(index int32) TileKey {
	return TileKeyFromOffset(index / b.tileSize)
}

func (b *BigTableTraceStore) ClearOPS(tileKey TileKey) error {
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

// returns true if the returned value came from BT.
func (b *BigTableTraceStore) getOPS(tileKey TileKey) (*OpsCacheEntry, bool, error) {
	b.mutex.RLock()
	entry, ok := b.opsCache[tileKey.OpsRowName()]
	b.mutex.RUnlock()
	if ok {
		return entry, true, nil
	} else {
		sklog.Infof("OPS Cache is empty.")
	}
	row, err := b.table.ReadRow(b.ctx, tileKey.OpsRowName(), bigtable.RowFilter(bigtable.LatestNFilter(1)))
	if err != nil {
		return nil, false, fmt.Errorf("Failed to read OPS from BigTable: %s", err)
	}
	// If there is no entry in BigTable then return an empty OPS.
	if len(row) == 0 {
		sklog.Warningf("Failed to read OPS from BT.")
		entry, err := NewOpsCacheEntry()
		return entry, false, err
	}
	entry, err = NewOpsCacheEntryFromRow(row)
	if err == nil {
		b.mutex.Lock()
		defer b.mutex.Unlock()
		b.opsCache[tileKey.OpsRowName()] = entry
	}
	return entry, true, err
}

func (b *BigTableTraceStore) GetOrderedParamSet(tileKey TileKey) (*paramtools.OrderedParamSet, error) {
	entry, _, err := b.getOPS(tileKey)
	if err != nil {
		return nil, err
	}
	return entry.ops, nil
}

// The keys of 'values' must be the OPS encoded Params of the trace,
// i.e. at this point we know the OPS has been updated.
func (b *BigTableTraceStore) WriteTraces(index int32, values map[string]float32, source string, timestamp time.Time) error {
	sourceHash := md5.Sum([]byte(source))
	tileKey := TileKeyFromOffset(index / b.tileSize)
	col := strconv.Itoa(int(index % b.tileSize))

	ts := bigtable.Time(timestamp)
	rowKeys := []string{}
	muts := []*bigtable.Mutation{}

	// Write the hash to source file name record.
	mut := bigtable.NewMutation()
	mut.Set(HASHES_FAMILY, HASHES_SOURCE, ts, []byte(source))
	muts = append(muts, mut)
	rowKeys = append(rowKeys, fmt.Sprintf("&%x", sourceHash))

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
		b.writesCounter.Inc(int64(len(muts)))
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

// ReadTraces loads the traces for the given keys (non-OPS translated keys).
func (b *BigTableTraceStore) ReadTraces(tileKey TileKey, keys []string) (map[string][]float32, error) {
	// First encode all the keys by the OrderedParamSet of the given tile.
	ops, err := b.GetOrderedParamSet(tileKey)
	if err != nil {
		return nil, fmt.Errorf("Failed to get OPS: %s", err)
	}
	// Also map[encodedKey]originalKey so we can construct our response.
	encodedKeys := map[string]string{}
	rowSet := bigtable.RowList{}
	for _, key := range keys {
		params, err := query.ParseKey(key)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse key %q: $s", key, err)
		}
		encodedKey, err := ops.EncodeParamsAsString(paramtools.Params(params))
		if err != nil {
			return nil, fmt.Errorf("Failed to encode key %q: $s", key, err)
		}
		encodedKeys[encodedKey] = key
		rowSet = append(rowSet, encodedKey)
	}
	ret := map[string][]float32{}
	b.table.ReadRows(b.ctx, rowSet, func(row bigtable.Row) bool {
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
	return ret, nil
}

// TODO returns map[encodedKey], should probably be unencoded keys.
func (b *BigTableTraceStore) QueryTraces(tileKey TileKey, q *regexp.Regexp) (map[string][]float32, error) {
	mutex := sync.Mutex{}
	ret := map[string][]float32{}
	var g errgroup.Group
	for i := int32(0); i < b.shards; i++ {
		i := i
		g.Go(func() error {
			rowRegex := tileKey.TraceRowPrefix(i) + ".*" + q.String()
			return b.table.ReadRows(b.ctx, bigtable.PrefixRange(tileKey.TraceRowPrefix(i)), func(row bigtable.Row) bool {
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

func (b *BigTableTraceStore) QueryCount(tileKey TileKey, q *regexp.Regexp) (int64, error) {
	var g errgroup.Group
	ret := int64(0)
	for i := int32(0); i < b.shards; i++ {
		i := i
		g.Go(func() error {
			rowRegex := tileKey.TraceRowPrefix(i) + ".*" + q.String()
			return b.table.ReadRows(b.ctx, bigtable.PrefixRange(tileKey.TraceRowPrefix(i)), func(row bigtable.Row) bool {
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

func (b *BigTableTraceStore) GetSource(index int32, traceId string) (string, error) {
	tileKey := TileKeyFromOffset(index / b.tileSize)
	row, err := b.table.ReadRow(b.ctx, tileKey.TraceRowName(traceId, b.shards), bigtable.RowFilter(
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

	row, err = b.table.ReadRow(b.ctx, sourceHash, bigtable.RowFilter(
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

func (b *BigTableTraceStore) GetLatestTile() (TileKey, error) {
	ret := BadTileKey
	err := b.table.ReadRows(b.ctx, bigtable.PrefixRange("@"), func(row bigtable.Row) bool {
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

func (b *BigTableTraceStore) UpdateOrderedParamSet(tileKey TileKey, p paramtools.ParamSet) (*paramtools.OrderedParamSet, error) {
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
			sklog.Infof("Nothing to do.")
			return entry.ops, nil
		}

		// Create a new updated ops.
		ops := entry.ops.Copy()
		ops.Update(p)
		newEntry, err = opsCacheEntryFromOPS(ops)
		encodedOps, err := newEntry.ops.Encode()
		sklog.Infof("Writing hash: new: %q old: %q", newEntry.hash, entry.hash)
		if err != nil {
			return nil, fmt.Errorf("Failed to encode new ops: %s", err)
		}

		now := bigtable.Time(time.Now())
		condTrue := false
		if existsInBT {
			// Create an update that avoids the lost update problem.
			cond := bigtable.ChainFilters(
				bigtable.FamilyFilter(OPS_FAMILY),
				bigtable.ColumnFilter(HASH),
				bigtable.ValueFilter(string(entry.hash)),
			)
			updateMutation := bigtable.NewMutation()
			updateMutation.Set(OPS_FAMILY, HASH, now, []byte(newEntry.hash))
			updateMutation.Set(OPS_FAMILY, OPS, now, encodedOps)

			// Add a mutation that cleans up old versions.
			before := bigtable.Time(time.Now().Add(-1 * time.Second))
			updateMutation.DeleteTimestampRange(OPS_FAMILY, HASH, 0, before)
			updateMutation.DeleteTimestampRange(OPS_FAMILY, OPS, 0, before)
			condUpdate := bigtable.NewCondMutation(cond, updateMutation, nil)
			if err := b.table.Apply(b.ctx, tileKey.OpsRowName(), condUpdate, bigtable.GetCondMutationResult(&condTrue)); err != nil {
				sklog.Warningf("Failed to apply: %s", err)
				continue
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
				bigtable.ColumnFilter(HASH),
			)
			updateMutation := bigtable.NewMutation()
			updateMutation.Set(OPS_FAMILY, HASH, now, []byte(newEntry.hash))
			updateMutation.Set(OPS_FAMILY, OPS, now, encodedOps)

			condUpdate := bigtable.NewCondMutation(cond, nil, updateMutation)
			if err := b.table.Apply(b.ctx, tileKey.OpsRowName(), condUpdate, bigtable.GetCondMutationResult(&condTrue)); err != nil {
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
		b.mutex.Lock()
		defer b.mutex.Unlock()
		b.opsCache[tileKey.OpsRowName()] = newEntry
		break
	}
	return newEntry.ops, nil
}
