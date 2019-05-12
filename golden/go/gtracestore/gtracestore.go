package gtracestore

import (
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/types"
	"golang.org/x/sync/errgroup"
)

const (
	// Column Families.
	cfTraceVals       = "V"               // Holds "0"..."tilesize-1" columns with trace data.
	cfTraceValsPrefix = cfTraceVals + ":" // Prefix to be removed from the columns
	cfIDCounter       = "I"

	// Columns
	colIDCounter = "idc"

	// Define the row types.
	typDigestMap = "d"
	typOPS       = "o"
	typIDCounter = "i"
	typTrace     = "t"

	// Default number of shards used, if not shards provided in BTConfig.
	DefaultShards = 32

	// Default tile size if no tilesize provided in BTConfig.
	DefaultTileSize = 256
)

// Constants copied from btts.go
const (
	OPS_FAMILY = "D"

	// // Columns in the "D" column family.
	OPS_HASH_COLUMN    = "H"   // Column
	OPS_OPS_COLUMN     = "OPS" // Column
	HASH_FULL_COL_NAME = OPS_FAMILY + ":" + OPS_HASH_COLUMN
	OPS_FULL_COL_NAME  = OPS_FAMILY + ":" + OPS_OPS_COLUMN

	// // MAX_MUTATIONS is the max number of mutations we can send in a single ApplyBulk call. Can be up to 100,000 according to BigTable docs.
	// MAX_MUTATIONS = 100000

	TIMEOUT       = 4 * time.Minute
	WRITE_TIMEOUT = 10 * time.Minute
)

// Namespace for this package.
const (
	traceStoreNameSpace = "ts"
)

// List of tables we are creating in BT.
var btColumnFamilies = []string{
	cfTraceVals,
	OPS_FAMILY,
	cfIDCounter,
	cfDigestMap,
}

// InitBT initializes the BT instance for the given configuration. It uses the default way
// to get auth information from the environment and must be called with an account that has
// admin rights.
func InitBT(conf *BTConfig) error {
	return bt.InitBigtable(conf.ProjectID, conf.InstanceID, conf.TableID, btColumnFamilies)
}

// Entry is one digests and related params to be added to the TraceStore.
type Entry struct {
	// Params describe the configuration that produced the digest/image.
	Params map[string]string

	// Digest references the images that were generate by the test.
	Digest types.Digest
}

// TraceStore is the interface to store trace data.
type TraceStore interface {
	// Put writes the given entries to the TraceStore at the given commit hash. The timestamp is
	// assumed to be the time when the entries were generated.
	Put(ctx context.Context, commitHash string, entries []*Entry, ts time.Time) error

	// GetTile reads the last n commits and returns them as a tile. If isSparse is true
	// empty commits are omitted.
	GetTile(ctx context.Context, nCommits int, isSparse bool) (*tiling.Tile, []*tiling.Commit, []int, error)
}

// BTConfig contains the configuration information for the BigTable-based implementation of
// TraceStore.
type BTConfig struct {
	ProjectID  string
	InstanceID string
	TableID    string
	VCS        vcsinfo.VCS
	TileSize   int32
	Shards     int32
}

// btTraceStore implementes the TraceStore interface.
type btTraceStore struct {
	vcs      vcsinfo.VCS
	client   *bigtable.Client
	table    *bigtable.Table
	mutex    sync.RWMutex
	tileSize int32
	shards   int32
	opsCache map[string]*OpsCacheEntry
	cacheOps bool

	availIDsMutex sync.Mutex
	availIDs      []int32
}

// NewBTTraceStore implements the TraceStore interface.
func NewBTTraceStore(ctx context.Context, conf *BTConfig, cache bool) (TraceStore, error) {
	tileSize := conf.TileSize
	if tileSize <= 0 {
		tileSize = DefaultTileSize
	}

	shards := conf.Shards
	if shards <= 0 {
		shards = DefaultShards
	}

	client, err := bigtable.NewClient(ctx, conf.ProjectID, conf.InstanceID)
	if err != nil {
		return nil, err
	}

	ret := &btTraceStore{
		vcs:      conf.VCS,
		client:   client,
		tileSize: tileSize,
		shards:   shards,
		table:    client.Open(conf.TableID),
		opsCache: map[string]*OpsCacheEntry{},
		cacheOps: cache,
		availIDs: []int32{},
	}
	return ret, nil
}

func (b *btTraceStore) Put(ctx context.Context, commitHash string, entries []*Entry, ts time.Time) error {
	// if there are no entries this becomes a no-op.
	if len(entries) == 0 {
		return nil
	}

	// Accumulate all parameters into a paramset and collect all the digests.
	paramSet := make(paramtools.ParamSet, len(entries[0].Params))
	digestSet := make(map[types.Digest]bool, len(entries))
	for _, entry := range entries {
		paramSet.AddParams(entry.Params)
		digestSet[entry.Digest] = true
	}

	tileKey, commitOffset, err := b.getTileKey(ctx, commitHash)
	if err != nil {
		return err
	}

	ops, err := b.updateOrderedParamSet(tileKey, paramSet)
	if err != nil {
		return err
	}

	digestMap, err := b.updateDigestMap(ctx, tileKey, digestSet)
	if err != nil {
		return err
	}

	if len(digestMap.Delta(digestSet)) != 0 {
		panic(fmt.Sprintf("Delta should be empty at this point: %v", digestMap.Delta(digestSet)))
	}

	mutations := make([]*bigtable.Mutation, 0, len(entries))
	rowNames := make([]string, 0, len(entries))
	btTS := bigtable.Time(ts)
	before := bigtable.Time(ts.Add(-1 * time.Millisecond))

	for _, entry := range entries {
		traceKey, err := ops.EncodeParamsAsString(entry.Params)
		if err != nil {
			return err
		}

		rowName := b.calcShardedRowName(tileKey, typTrace, traceKey)
		rowNames = append(rowNames, rowName)

		digestID, err := digestMap.ID(entry.Digest)
		if err != nil {
			return err
		}

		mut := bigtable.NewMutation()
		column := strconv.Itoa(commitOffset)
		mut.Set(cfTraceVals, column, btTS, []byte(strconv.Itoa(int(digestID))))
		mut.DeleteTimestampRange(cfTraceVals, column, 0, before)
		mutations = append(mutations, mut)
	}

	// Write the trace data. We pick a batchsize based on the assumption
	// that the whole batch should be 2MB large and each entry is ~200 Bytes of data.
	// 2MB / 200B = 10000. This is extremely conservative but should not be a problem
	// since the batches are written in parallel.
	return b.applyBulkBatched(ctx, rowNames, mutations, 10000)
}

func (b *btTraceStore) GetTile(ctx context.Context, nCommits int, isSparse bool) (*tiling.Tile, []*tiling.Commit, []int, error) {
	defer timer.New("BUILDING tile").Stop()

	idxCommits := b.vcs.LastNIndex(nCommits)
	if len(idxCommits) == 0 {
		return nil, nil, nil, skerr.Fmt("No commits found.")
	}

	startTileKey, startCommitOffset, err := b.getTileKey(ctx, idxCommits[0].Hash)
	if err != nil {
		return nil, nil, nil, err
	}

	endTileKey, endCommitOffset, err := b.getTileKey(ctx, idxCommits[len(idxCommits)-1].Hash)
	if err != nil {
		return nil, nil, nil, err
	}
	// sklog.Infof("TTTT:    %d      %d      %d       %d  %d", startTileKey, startCommitOffset, endTileKey, endCommitOffset, len(idxCommits))

	nTiles := int(startTileKey - endTileKey + 1)
	encTiles := make([]*EncTile, nTiles)
	var egroup errgroup.Group
	tileKey := startTileKey
	for idx := 0; idx < nTiles; idx++ {
		func(idx int, tileKey TileKey) {
			egroup.Go(func() error {
				var err error
				encTiles[idx], err = b.loadTile(ctx, tileKey)
				return err
			})
		}(idx, tileKey)
		tileKey--
	}

	if err := egroup.Wait(); err != nil {
		return nil, nil, nil, err
	}

	tileTraces := make(map[tiling.TraceId]tiling.Trace, len(encTiles[0].traces))
	paramSet := paramtools.ParamSet{}
	cardinalities := []int{}

	// Initialize the commits to be empty.
	commits := make([]*tiling.Commit, nCommits, nCommits)
	for i := range commits {
		commits[i] = &tiling.Commit{}
	}

	commitIDX := 0
	for idx, encTile := range encTiles {
		// Determine the offset within the tile that we should consider.
		ecOffset := int(b.tileSize - 1)
		if idx == (len(encTiles) - 1) {
			ecOffset = endCommitOffset
		}
		segLen := ecOffset - startCommitOffset + 1
		// sklog.Infof("\n\n\nSEGLEN: %d\n\n\n", segLen)

		for traceKey, encValues := range encTile.traces {
			if _, ok := tileTraces[traceKey]; !ok {
				tileTraces[traceKey] = types.NewGoldenTraceN(nCommits)
			}
			gt := tileTraces[traceKey].(*types.GoldenTrace)

			// Extract the parameters from the key.
			if gt.Keys, err = encTile.ops.DecodeParamsFromString(string(traceKey)); err != nil {
				return nil, nil, nil, err
			}

			// Convert the digests from integer IDs to strings.
			digestIDs := encValues[startCommitOffset : startCommitOffset+segLen]
			digests, err := encTile.digestMap.DecodeIDs(digestIDs)
			if err != nil {
				return nil, nil, nil, err
			}
			// sklog.Infof("coopppyy: %d   %d    %d   %d     %d", nCommits, len(gt.Digests), commitIDX, segLen, len(digests))
			copy(gt.Digests[commitIDX:commitIDX+segLen], digests)
		}

		hashes := make([]string, 0, segLen)
		for cid := commitIDX; cid < commitIDX+segLen; cid++ {
			hashes = append(hashes, idxCommits[cid].Hash)
		}

		longCommits, err := b.vcs.DetailsMulti(ctx, hashes, false)
		if err != nil {
			return nil, nil, nil, err
		}
		// sklog.Infof("AAA: %d", len(hashes))

		for idx, lc := range longCommits {
			if lc == nil {
				return nil, nil, nil, skerr.Fmt("Commit %s not found", hashes[idx])
			}
			// sklog.Infof("COMM: %s  %20s    %d", lc.Hash, lc.Author, lc.Timestamp.Unix())
			commits[commitIDX+idx].Hash = lc.Hash
			commits[commitIDX+idx].Author = lc.Author
			commits[commitIDX+idx].CommitTime = lc.Timestamp.Unix()
		}

		// After the first tile we always start at the first entry and advance the
		// overall commit index by the segment length.
		commitIDX += segLen
		startCommitOffset = 0
	}

	ret := &tiling.Tile{
		Traces:   tileTraces,
		ParamSet: paramSet,
		Commits:  commits,
	}

	return ret, commits, cardinalities, nil
}

// getTileKey retrieves the tile key for the given commitHash.
func (b *btTraceStore) getTileKey(ctx context.Context, commitHash string) (TileKey, int, error) {
	// Find the index of the commit in the branch of interest.
	commitIndex, err := b.vcs.IndexOf(ctx, commitHash)
	if err != nil {
		return 0, 0, err
	}

	tileOffset := int32(commitIndex) / b.tileSize
	commitOffset := commitIndex % int(b.tileSize)
	return TileKeyFromOffset(tileOffset), commitOffset, nil
}

// EncTile contains an encoded tile.
type EncTile struct {
	traces    map[tiling.TraceId][]int32
	ops       *paramtools.OrderedParamSet
	digestMap *DigestMap
}

func (b *btTraceStore) loadTile(ctx context.Context, tileKey TileKey) (*EncTile, error) {
	var egroup errgroup.Group

	// Load the digestMap and the OPS codes.
	var digestMap *DigestMap
	egroup.Go(func() error {
		var err error
		digestMap, err = b.getDigestMap(ctx, tileKey)
		return err
	})

	var ops *paramtools.OrderedParamSet
	egroup.Go(func() error {
		opsEntry, _, err := b.getOPS(tileKey)
		if err != nil {
			return err
		}
		ops = opsEntry.ops
		return nil
	})

	var traces map[tiling.TraceId][]int32
	egroup.Go(func() error {
		var err error
		traces, err = b.loadTraces(ctx, tileKey)
		return err
	})

	if err := egroup.Wait(); err != nil {
		return nil, err
	}

	return &EncTile{
		digestMap: digestMap,
		ops:       ops,
		traces:    traces,
	}, nil
}

func (b *btTraceStore) loadTraces(ctx context.Context, tileKey TileKey) (map[tiling.TraceId][]int32, error) {
	var egroup errgroup.Group
	shardResults := make([]map[tiling.TraceId][]int32, b.shards)
	traceCount := int32(0)

	// Query all shards in parallel.
	for shard := int32(0); shard < b.shards; shard++ {
		func(shard int32) {
			egroup.Go(func() error {
				rr := bigtable.PrefixRange(b.shardedRowName(shard, typTrace, tileKey))
				target := map[tiling.TraceId][]int32{}
				shardResults[shard] = target
				var parseErr error
				err := b.table.ReadRows(ctx, rr, func(row bigtable.Row) bool {
					traceKey := tiling.TraceId(b.extractKey(row.Key()))
					if _, ok := target[traceKey]; !ok {
						target[traceKey] = make([]int32, b.tileSize)
						atomic.AddInt32(&traceCount, 1)
					}

					for _, col := range row[cfTraceVals] {
						idx, parseErr := strconv.Atoi(strings.TrimPrefix(col.Column, cfTraceValsPrefix))
						if parseErr != nil {
							return false
						}

						digestID, parseErr := strconv.Atoi(string(col.Value))
						if parseErr != nil {
							return false
						}
						if idx < 0 || idx >= int(b.tileSize) {
							parseErr = skerr.Fmt("Got index %d that is outside of the target slice of length %d", idx, len(target))
							return false
						}
						target[traceKey][idx] = int32(digestID)
					}
					return true
				})
				if err != nil {
					return err
				}
				return parseErr
			})
		}(shard)
	}

	if err := egroup.Wait(); err != nil {
		return nil, err
	}

	ret := make(map[tiling.TraceId][]int32, traceCount)
	for _, r := range shardResults {
		for traceKey, digestIDs := range r {
			if found, ok := ret[traceKey]; ok {
				for idx, digestID := range digestIDs {
					if digestID != 0 {
						found[idx] = digestID
					}
				}
			} else {
				ret[traceKey] = digestIDs
			}
		}
	}

	return ret, nil
}

// applyBulkBatched writes the given rowNames/mutation pairs to bigtable in batches that are
// maximally of size 'batchSize'. The batches are written in parallel.
func (b *btTraceStore) applyBulkBatched(ctx context.Context, rowNames []string, mutations []*bigtable.Mutation, batchSize int) error {
	var egroup errgroup.Group
	err := util.ChunkIter(len(rowNames), batchSize, func(chunkStart, chunkEnd int) error {
		egroup.Go(func() error {
			rowNames := rowNames[chunkStart:chunkEnd]
			mutations := mutations[chunkStart:chunkEnd]
			errs, err := b.table.ApplyBulk(context.TODO(), rowNames, mutations)
			if err != nil {
				return skerr.Fmt("Error writing batch: %s", err)
			}
			if errs != nil {
				return skerr.Fmt("Error writing some portions of batch: %s", errs)
			}
			return nil
		})
		return nil
	})
	if err != nil {
		return skerr.Fmt("Error running ChunkIter: %s", err)
	}
	return egroup.Wait()
}

func (b *btTraceStore) rowName(rowType string, tileKey TileKey, keys ...string) string {
	if len(keys) > 0 {
		return fmt.Sprintf(":%s:%s:%07d:%s", traceStoreNameSpace, rowType, tileKey, strings.Join(keys, ":"))
	}
	return fmt.Sprintf(":%s:%s:%07d", traceStoreNameSpace, rowType, tileKey)
}

func (b *btTraceStore) shardedRowName(shard int32, rowType string, tileKey TileKey, keys ...string) string {
	return fmt.Sprintf("%02d%s", shard, b.rowName(rowType, tileKey, keys...))
}

func (b *btTraceStore) calcShardedRowName(tileKey TileKey, rowType, key string) string {
	shard := int32(crc32.ChecksumIEEE([]byte(key)) % uint32(b.shards))
	return b.shardedRowName(shard, rowType, tileKey, key)
}

func (b *btTraceStore) rowAndColNameFromDigest(tileKey TileKey, digest string) (string, string) {
	key := digest[:2]
	postfix := digest[2:]
	return b.calcShardedRowName(tileKey, typDigestMap, key), postfix
}

func (b *btTraceStore) extractKey(rowName string) string {
	parts := strings.Split(rowName, ":")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

const (
	cfDigestMap        = "D"
	colDigestMapPrefix = cfDigestMap + ":"
)

func (b *btTraceStore) getDigestMap(ctx context.Context, tileKey TileKey) (*DigestMap, error) {
	// defer timer.New("Fetching digest map").Stop()

	// Query all shards in parallel.
	var egroup errgroup.Group
	resultCh := make(chan map[types.Digest]int32, b.shards)
	total := int64(0)
	for shard := int32(0); shard < b.shards; shard++ {
		func(shard int32) {
			egroup.Go(func() error {
				keyPrefix := b.shardedRowName(shard, typDigestMap, tileKey)
				prefRange := bigtable.PrefixRange(keyPrefix)
				var idx int64
				var parseErr error = nil
				ret := map[types.Digest]int32{}
				err := b.table.ReadRows(ctx, prefRange, func(row bigtable.Row) bool {
					digestPrefix := b.extractKey(row.Key())
					for _, col := range row[cfDigestMap] {
						idx, parseErr = strconv.ParseInt(string(col.Value), 10, 64)
						if parseErr != nil {
							return false
						}
						digest := types.Digest(digestPrefix + strings.TrimPrefix(col.Column, colDigestMapPrefix))
						ret[digest] = int32(idx)
					}
					return true
				})

				if err != nil {
					return err
				}
				if parseErr != nil {
					return parseErr
				}

				resultCh <- ret
				atomic.AddInt64(&total, 1)
				return nil
			})
		}(shard)
	}
	if err := egroup.Wait(); err != nil {
		return nil, err
	}
	close(resultCh)

	ret := NewDigestMap(int(total))
	for dm := range resultCh {
		if err := ret.Add(dm); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func (b *btTraceStore) getIDs(ctx context.Context, tileKey TileKey, n int) ([]int32, error) {
	availIDs := make([]int32, 0, n)
	b.availIDsMutex.Lock()
	startIdx := len(b.availIDs) - util.MinInt(len(b.availIDs), n)
	availIDs = append(availIDs, b.availIDs[startIdx:]...)
	b.availIDs = b.availIDs[:startIdx]
	b.availIDsMutex.Unlock()

	missing := int64(n - len(availIDs))
	if missing == 0 {
		return availIDs, nil
	}

	// Reserve new IDs via the ID counter
	rowName := b.rowName(typIDCounter, tileKey)
	rmw := bigtable.NewReadModifyWrite()
	rmw.Increment(cfIDCounter, colIDCounter, missing)
	row, err := b.table.ApplyReadModifyWrite(ctx, rowName, rmw)
	if err != nil {
		return nil, err
	}

	encMaxID := []byte(rowMap(row).GetStr(cfIDCounter, colIDCounter))
	maxID := int32(binary.BigEndian.Uint64(encMaxID))
	for len(availIDs) < n {
		availIDs = append(availIDs, maxID)
		maxID--
	}

	return availIDs, nil
}

func (b *btTraceStore) returnIDs(unusedIDs []int32) {
	b.availIDsMutex.Lock()
	defer b.availIDsMutex.Unlock()
	b.availIDs = append(b.availIDs, unusedIDs...)
}

func (b *btTraceStore) getOrAddDigests(ctx context.Context, tileKey TileKey, digests []types.Digest, digestMap *DigestMap) (*DigestMap, error) {
	availIDs, err := b.getIDs(ctx, tileKey, len(digests))
	if err != nil {
		return nil, err
	}

	newIDMapping := make(map[types.Digest]int32, len(digests))
	unusedIDs := make([]int32, 0, len(availIDs))
	for idx, digest := range digests {
		rowName, colName := b.rowAndColNameFromDigest(tileKey, string(digest))
		addMut := bigtable.NewMutation()
		idVal := availIDs[idx]
		addMut.Set(cfDigestMap, colName, bigtable.ServerTime, []byte(strconv.Itoa(int(idVal))))
		filter := bigtable.ColumnFilter(colName)
		condMut := bigtable.NewCondMutation(filter, nil, addMut)
		var condMatched bool
		if err := b.table.Apply(ctx, rowName, condMut, bigtable.GetCondMutationResult(&condMatched)); err != nil {
			return nil, err
		}

		// We didn't need this ID so let's re-use it later.
		if condMatched {
			unusedIDs = append(unusedIDs, idVal)
		} else {
			newIDMapping[digest] = idVal
		}
	}

	// If all Ids were added to BT then add them to given DigestMap and return it.
	if len(unusedIDs) == 0 {
		if err := digestMap.Add(newIDMapping); err != nil {
			return nil, err
		}
		return digestMap, nil
	}

	// Return the unused IDs for laster use and reload the entire DigestMap.
	b.returnIDs(unusedIDs)
	return b.getDigestMap(ctx, tileKey)
}

func (b *btTraceStore) updateDigestMap(ctx context.Context, tileKey TileKey, digests map[types.Digest]bool) (*DigestMap, error) {
	// Load the digest map from BT.
	digestMap, err := b.getDigestMap(ctx, tileKey)
	if err != nil {
		return nil, err
	}

	delta := digestMap.Delta(digests)
	if len(delta) == 0 {
		return digestMap, nil
	}

	return b.getOrAddDigests(ctx, tileKey, delta, digestMap)
}

// Copied from btts.go in infra/perf

// UpdateOrderedParamSet will add all params from 'p' to the OrderedParamSet
// for 'tileKey' and write it back to BigTable.
func (b *btTraceStore) updateOrderedParamSet(tileKey TileKey, p paramtools.ParamSet) (*paramtools.OrderedParamSet, error) {
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
			// sklog.Infof("No delta in UpdateOrderedParamSet for %s. Nothing to do.", tileKey.OpsRowName(b))
			return entry.ops, nil
		}

		// Create a new updated ops.
		ops := entry.ops.Copy()
		ops.Update(p)
		newEntry, err = opsCacheEntryFromOPS(ops)
		encodedOps, err := newEntry.ops.Encode()
		// sklog.Infof("Writing updated OPS for %s. hash: new: %q old: %q", tileKey.OpsRowName(b), newEntry.hash, entry.hash)
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

			if err := b.getTable().Apply(tctx, tileKey.OpsRowName(b), condUpdate, bigtable.GetCondMutationResult(&condTrue)); err != nil {
				sklog.Warningf("Failed to apply: %s", err)
				return nil, err
			}

			// If !condTrue then we need to try again,
			// and clear our local cache.
			if !condTrue {
				sklog.Warningf("Exists !condTrue - trying again.")
				b.mutex.Lock()
				delete(b.opsCache, tileKey.OpsRowName(b))
				b.mutex.Unlock()
				continue
			}
		} else {
			// sklog.Infof("FIRST WRITE")
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
			if err := b.getTable().Apply(tctx, tileKey.OpsRowName(b), condUpdate, bigtable.GetCondMutationResult(&condTrue)); err != nil {
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
			b.opsCache[tileKey.OpsRowName(b)] = newEntry
		}
		break
	}
	return newEntry.ops, nil
}

// getOps returns the OpsCacheEntry for a given tile.
//
// Note that it will create a new OpsCacheEntry if none exists.
//
// getOps returns true if the returned value came from BT, false if it came
// from the cache.
func (b *btTraceStore) getOPS(tileKey TileKey) (*OpsCacheEntry, bool, error) {
	if b.cacheOps {
		b.mutex.RLock()
		entry, ok := b.opsCache[tileKey.OpsRowName(b)]
		b.mutex.RUnlock()
		if ok {
			return entry, true, nil
		} else {
			// sklog.Infof("OPS Cache is empty.")
		}
	}
	tctx, cancel := context.WithTimeout(context.Background(), TIMEOUT)
	defer cancel()
	row, err := b.getTable().ReadRow(tctx, tileKey.OpsRowName(b), bigtable.RowFilter(bigtable.LatestNFilter(1)))
	if err != nil {
		return nil, false, fmt.Errorf("Failed to read OPS from BigTable for %s: %s", tileKey.OpsRowName(b), err)
	}
	// If there is no entry in BigTable then return an empty OPS.
	if len(row) == 0 {
		sklog.Warningf("Failed to read OPS from BT for %s.", tileKey.OpsRowName(b))
		entry, err := NewOpsCacheEntry()
		return entry, false, err
	}
	entry, err := NewOpsCacheEntryFromRow(row)
	if err == nil && b.cacheOps {
		b.mutex.Lock()
		defer b.mutex.Unlock()
		b.opsCache[tileKey.OpsRowName(b)] = entry
	}
	return entry, true, err
}

func (b *btTraceStore) getTable() *bigtable.Table {
	return b.table
}
