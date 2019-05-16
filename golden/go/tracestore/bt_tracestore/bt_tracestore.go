package bt_tracestore

// This implementation of storing tiles in BigTable is very similar to
// that in Perf.  See perf/BIGTABLE.md for an overview.

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
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/tracestore"
	"go.skia.org/infra/golden/go/types"
	"golang.org/x/sync/errgroup"
)

const (
	// Column Families.
	// https://cloud.google.com/bigtable/docs/schema-design#column_families_and_column_qualifiers
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

	// This is the size of the tile in Big Table. That is, how many commits do we store in one tile.
	// We can have up to 2^32 tiles in big table, so this would let us store 1 trillion
	// commits worth of data. This tile size does not  need to be related to the tile size that
	// Gold operates on (although when tuning, it should be greater than, or an even divisor
	// of the Gold tile size).
	DefaultTileSize = 256

	// Default number of shards used, if not shards provided in BTConfig. A shard splits the traces
	// up on a tile. If a tile exists on shard N in tile A, it will be on shard for all tiles.
	// Having traces on shards lets BT split up the work more evenly.
	DefaultShards = 32
)

// Namespace for this package.
const (
	traceStoreNameSpace = "ts"
)

// List of tables we are creating in BT.
var btColumnFamilies = []string{
	cfTraceVals,
	opsFamily,
	cfIDCounter,
	cfDigestMap,
}

// InitBT initializes the BT instance for the given configuration. It uses the default way
// to get auth information from the environment and must be called with an account that has
// admin rights.
func InitBT(conf BTConfig) error {
	return bt.InitBigtable(conf.ProjectID, conf.InstanceID, conf.TableID, btColumnFamilies)
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

// BTTraceStore implements the TraceStore interface.
type BTTraceStore struct {
	vcs    vcsinfo.VCS
	client *bigtable.Client
	table  *bigtable.Table

	tileSize int32
	shards   int32

	// if cacheOps is true, then use opsCache between
	// calls
	cacheOps bool
	// maps rowName (string) -> *OpsCacheEntry
	opsCache sync.Map

	availIDsMutex sync.Mutex
	availIDs      []DigestID
}

// New implements the TraceStore interface backed by BigTable. If cache is true,
// the OrderedParamSets will be cached based on the row name.
func New(ctx context.Context, conf BTConfig, cache bool) (*BTTraceStore, error) {
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
		return nil, skerr.Fmt("could not instantiate client: %s", err)
	}

	ret := &BTTraceStore{
		vcs:      conf.VCS,
		client:   client,
		tileSize: tileSize,
		shards:   shards,
		table:    client.Open(conf.TableID),
		cacheOps: cache,
		availIDs: []DigestID{},
	}
	return ret, nil
}

// Put implements the TraceStore interface.
func (b *BTTraceStore) Put(ctx context.Context, commitHash string, entries []*tracestore.Entry, ts time.Time) error {
	defer metrics2.FuncTimer().Stop()
	// if there are no entries this becomes a no-op.
	if len(entries) == 0 {
		return nil
	}

	// Accumulate all parameters into a paramset and collect all the digests.
	paramSet := make(paramtools.ParamSet, len(entries[0].Params))
	digestSet := make(types.DigestSet, len(entries))
	for _, entry := range entries {
		paramSet.AddParams(entry.Params)
		digestSet[entry.Digest] = true
	}

	// Find out what tile we need to fetch and what offset into that tile we need.
	// In normal use, tileKey will usually be the most recent tile, so 2^32-1
	tileKey, commitOffset, err := b.getTileKey(ctx, commitHash)
	if err != nil {
		return skerr.Fmt("cannot derive tilekey/offset: %s", err)
	}

	// If these entries have any params we haven't seen before, we need to store those in BigTable.
	ops, err := b.updateOrderedParamSet(ctx, tileKey, paramSet)
	if err != nil {
		sklog.Warningf("Bad paramset: %#v", paramSet)
		return skerr.Fmt("cannot update paramset: %s", err)
	}

	// Similarly, if we have some new digests (almost certainly), we need to update
	// the digestMap with them in there. Of note, we store this
	// map of string (types.Digest) -> int32(DigestId) in big table, then refer to
	// the DigestID elsewhere in the table. DigestIds are essentially a monotonically
	// increasing arbitrary number.
	digestMap, err := b.updateDigestMap(ctx, tileKey, digestSet)
	if err != nil {
		sklog.Warningf("Bad digestSet: %#v", digestSet)
		return skerr.Fmt("cannot update digest map: %s", err)
	}

	if len(digestMap.Delta(digestSet)) != 0 {
		return fmt.Errorf("Delta should be empty at this point: %v", digestMap.Delta(digestSet))
	}

	// These mutations...
	mutations := make([]*bigtable.Mutation, 0, len(entries))
	// .. should be applied to these rows.
	rowNames := make([]string, 0, len(entries))
	btTS := bigtable.Time(ts)
	before := bigtable.Time(ts.Add(-1 * time.Millisecond))

	for _, entry := range entries {
		// To save space, traceKey isn't the long form EncodedTraceId
		// (e.g. foo:bar:baz:gm), it's a string of key-value numbers
		// that refer to the params. See params.paramsEncoder
		sTrace, err := ops.EncodeParamsAsString(entry.Params)
		if err != nil {
			return skerr.Fmt("invalid params: %s", err)
		}
		traceKey := EncodedTraceId(sTrace)

		rowName := b.calcShardedRowName(tileKey, typTrace, string(traceKey))
		rowNames = append(rowNames, rowName)

		digestID, err := digestMap.ID(entry.Digest)
		if err != nil {
			// this should never happen, given the checks above.
			return skerr.Fmt("Could not fetch id for digest %s: %s", entry.Digest, err)
		}

		// Create a mutation that puts the given digest at the given row
		// (i.e. the trace), at the given column (i.e. the commit).
		mut := bigtable.NewMutation()
		column := strconv.Itoa(commitOffset)
		mut.Set(cfTraceVals, column, btTS, []byte(strconv.Itoa(int(digestID))))
		// Delete anything that existed at this cell before now.
		mut.DeleteTimestampRange(cfTraceVals, column, 0, before)
		mutations = append(mutations, mut)
	}

	// Write the trace data. We pick a batchsize based on the assumption
	// that the whole batch should be 2MB large and each entry is ~200 Bytes of data.
	// 2MB / 200B = 10000. This is extremely conservative but should not be a problem
	// since the batches are written in parallel.
	return b.applyBulkBatched(ctx, rowNames, mutations, 10000)
}

func (b *BTTraceStore) GetTile(ctx context.Context, nCommits int, isSparse bool) (*tiling.Tile, []*tiling.Commit, error) {
	defer metrics2.FuncTimer().Stop()

	idxCommits := b.vcs.LastNIndex(nCommits)
	if len(idxCommits) == 0 {
		return nil, nil, skerr.Fmt("No commits found.")
	}

	startTileKey, startCommitOffset, err := b.getTileKey(ctx, idxCommits[0].Hash)
	if err != nil {
		return nil, nil, err
	}

	endTileKey, endCommitOffset, err := b.getTileKey(ctx, idxCommits[len(idxCommits)-1].Hash)
	if err != nil {
		return nil, nil, err
	}

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
		return nil, nil, err
	}

	// This is the full tile we are going to return.
	tileTraces := make(map[tiling.TraceId]tiling.Trace, len(encTiles[0].traces))
	paramSet := paramtools.ParamSet{}

	// Initialize the commits to be empty.
	commits := make([]*tiling.Commit, nCommits)
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

		for encodedKey, encValues := range encTile.traces {
			// at this point, the encodedKey looks like ,0=1,1=3,3=0,
			// See params.paramsEncoder
			params, err := encTile.ops.DecodeParamsFromString(string(encodedKey))
			if err != nil {
				return nil, nil, skerr.Fmt("corrupted trace key - could not decode %s: %s", encodedKey, err)
			}

			// Turn the params into the EncodedTraceId we expect elsewhere.
			traceKey := tracestore.TraceIDFromParams(params)
			if _, ok := tileTraces[traceKey]; !ok {
				tileTraces[traceKey] = types.NewGoldenTraceN(nCommits)
			}
			gt := tileTraces[traceKey].(*types.GoldenTrace)
			gt.Keys = params
			paramSet.AddParams(params)

			// Convert the digests from integer IDs to strings.
			digestIDs := encValues[startCommitOffset : startCommitOffset+segLen]
			digests, err := encTile.digestMap.DecodeIDs(digestIDs)
			if err != nil {
				return nil, nil, skerr.Fmt("corrupted digest id - could not decode: %s", err)
			}
			copy(gt.Digests[commitIDX:commitIDX+segLen], digests)
		}

		hashes := make([]string, 0, segLen)
		for cid := commitIDX; cid < commitIDX+segLen; cid++ {
			hashes = append(hashes, idxCommits[cid].Hash)
		}

		longCommits, err := b.vcs.DetailsMulti(ctx, hashes, false)
		if err != nil {
			return nil, nil, err
		}

		for idx, lc := range longCommits {
			if lc == nil {
				return nil, nil, skerr.Fmt("Commit %s not found", hashes[idx])
			}
			commits[commitIDX+idx].Hash = lc.Hash
			commits[commitIDX+idx].Author = lc.Author
			commits[commitIDX+idx].CommitTime = lc.Timestamp.Unix()
		}

		// After the first tile we always start at the first entry and advance the
		// overall commit index by the segment length.
		commitIDX += segLen
		startCommitOffset = 0
	}

	paramSet.Normalize()

	ret := &tiling.Tile{
		Traces:   tileTraces,
		ParamSet: paramSet,
		Commits:  commits,
		Scale:    1,
	}

	return ret, commits, nil
}

// getTileKey retrieves the tile key and commit offset for the given commitHash.
// commitOffset starts at 0 for the most recent commit.
func (b *BTTraceStore) getTileKey(ctx context.Context, commitHash string) (TileKey, int, error) {
	// Find the index of the commit in the branch of interest.
	commitIndex, err := b.vcs.IndexOf(ctx, commitHash)
	if err != nil {
		return 0, 0, skerr.Fmt("could not look up commit %s: %s", commitHash, err)
	}

	tileOffset := int32(commitIndex) / b.tileSize
	commitOffset := commitIndex % int(b.tileSize)
	return TileKeyFromOffset(tileOffset), commitOffset, nil
}

func (b *BTTraceStore) loadTile(ctx context.Context, tileKey TileKey) (*EncTile, error) {
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
		opsEntry, _, err := b.getOPS(ctx, tileKey)
		if err != nil {
			return err
		}
		ops = opsEntry.ops
		return nil
	})

	var traces map[EncodedTraceId][]DigestID
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

func (b *BTTraceStore) loadTraces(ctx context.Context, tileKey TileKey) (map[EncodedTraceId][]DigestID, error) {
	var egroup errgroup.Group
	shardResults := make([]map[EncodedTraceId][]DigestID, b.shards)
	traceCount := int32(0)

	// Query all shards in parallel.
	for shard := int32(0); shard < b.shards; shard++ {
		func(shard int32) {
			egroup.Go(func() error {
				rr := bigtable.PrefixRange(b.shardedRowName(shard, typTrace, tileKey))
				target := map[EncodedTraceId][]DigestID{}
				shardResults[shard] = target
				var parseErr error
				err := b.table.ReadRows(ctx, rr, func(row bigtable.Row) bool {
					traceKey := EncodedTraceId(b.extractKey(row.Key()))
					if _, ok := target[traceKey]; !ok {
						target[traceKey] = make([]DigestID, b.tileSize)
						atomic.AddInt32(&traceCount, 1)
					}

					for _, col := range row[cfTraceVals] {
						idx, err := strconv.Atoi(strings.TrimPrefix(col.Column, cfTraceValsPrefix))
						if err != nil {
							parseErr = err
							return false
						}
						digestID, err := strconv.Atoi(string(col.Value))
						if err != nil {
							parseErr = err
							return false
						}
						if idx < 0 || idx >= int(b.tileSize) {
							parseErr = skerr.Fmt("got index %d that is outside of the target slice of length %d", idx, len(target))
							return false
						}
						target[traceKey][idx] = DigestID(digestID)
					}
					return true
				})
				if err != nil {
					return skerr.Fmt("could not read rows: %s", err)
				}
				return parseErr
			})
		}(shard)
	}

	if err := egroup.Wait(); err != nil {
		return nil, err
	}

	ret := make(map[EncodedTraceId][]DigestID, traceCount)
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
func (b *BTTraceStore) applyBulkBatched(ctx context.Context, rowNames []string, mutations []*bigtable.Mutation, batchSize int) error {
	tctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	var egroup errgroup.Group
	err := util.ChunkIter(len(rowNames), batchSize, func(chunkStart, chunkEnd int) error {
		egroup.Go(func() error {
			rowNames := rowNames[chunkStart:chunkEnd]
			mutations := mutations[chunkStart:chunkEnd]
			errs, err := b.table.ApplyBulk(tctx, rowNames, mutations)
			if err != nil {
				return skerr.Fmt("Error writing batch [%d:%d]: %s", chunkStart, chunkEnd, err)
			}
			if errs != nil {
				return skerr.Fmt("Error writing some portions of batch [%d:%d]: %s", chunkStart, chunkEnd, errs)
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

func (b *BTTraceStore) rowName(rowType string, tileKey TileKey, keys ...string) string {
	if len(keys) > 0 {
		return fmt.Sprintf(":%s:%s:%07d:%s", traceStoreNameSpace, rowType, tileKey, strings.Join(keys, ":"))
	}
	return fmt.Sprintf(":%s:%s:%07d", traceStoreNameSpace, rowType, tileKey)
}

func (b *BTTraceStore) shardedRowName(shard int32, rowType string, tileKey TileKey, keys ...string) string {
	return fmt.Sprintf("%02d%s", shard, b.rowName(rowType, tileKey, keys...))
}

// calcShardedRowName deterministically assigns a given traceID onto one of the shards.
// It attempts to evenly spread traces across shards.
// Once this is done, the shard, rowtype, tileKey and traceId are combined into a single string
// to be used as a row name in BT.
func (b *BTTraceStore) calcShardedRowName(tileKey TileKey, rowType, key string) string {
	shard := int32(crc32.ChecksumIEEE([]byte(key)) % uint32(b.shards))
	return b.shardedRowName(shard, rowType, tileKey, key)
}

func (b *BTTraceStore) rowAndColNameFromDigest(tileKey TileKey, digest string) (string, string) {
	key := digest[:2]
	postfix := digest[2:]
	return b.calcShardedRowName(tileKey, typDigestMap, key), postfix
}

func (b *BTTraceStore) extractKey(rowName string) string {
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

func (b *BTTraceStore) getDigestMap(ctx context.Context, tileKey TileKey) (*DigestMap, error) {
	// Query all shards in parallel.
	var egroup errgroup.Group
	resultCh := make(chan map[types.Digest]DigestID, b.shards)
	total := int64(0)
	for shard := int32(0); shard < b.shards; shard++ {
		func(shard int32) {
			egroup.Go(func() error {
				keyPrefix := b.shardedRowName(shard, typDigestMap, tileKey)
				prefRange := bigtable.PrefixRange(keyPrefix)
				var idx int64
				var parseErr error = nil
				ret := map[types.Digest]DigestID{}
				err := b.table.ReadRows(ctx, prefRange, func(row bigtable.Row) bool {
					digestPrefix := b.extractKey(row.Key())
					for _, col := range row[cfDigestMap] {
						idx, parseErr = strconv.ParseInt(string(col.Value), 10, 64)
						if parseErr != nil {
							return false
						}
						digest := types.Digest(digestPrefix + strings.TrimPrefix(col.Column, colDigestMapPrefix))
						ret[digest] = DigestID(idx)
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

func (b *BTTraceStore) getIDs(ctx context.Context, tileKey TileKey, n int) ([]DigestID, error) {
	availIDs := make([]DigestID, 0, n)
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
	maxID := DigestID(binary.BigEndian.Uint64(encMaxID))
	for len(availIDs) < n {
		availIDs = append(availIDs, maxID)
		maxID--
	}

	return availIDs, nil
}

func (b *BTTraceStore) returnIDs(unusedIDs []DigestID) {
	b.availIDsMutex.Lock()
	defer b.availIDsMutex.Unlock()
	b.availIDs = append(b.availIDs, unusedIDs...)
}

func (b *BTTraceStore) getOrAddDigests(ctx context.Context, tileKey TileKey, digests []types.Digest, digestMap *DigestMap) (*DigestMap, error) {
	availIDs, err := b.getIDs(ctx, tileKey, len(digests))
	if err != nil {
		return nil, err
	}

	newIDMapping := make(map[types.Digest]DigestID, len(digests))
	unusedIDs := make([]DigestID, 0, len(availIDs))
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

func (b *BTTraceStore) updateDigestMap(ctx context.Context, tileKey TileKey, digests types.DigestSet) (*DigestMap, error) {
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
func (b *BTTraceStore) updateOrderedParamSet(ctx context.Context, tileKey TileKey, p paramtools.ParamSet) (*paramtools.OrderedParamSet, error) {
	tctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	var newEntry *OpsCacheEntry
	for {
		// Get OPS.
		entry, existsInBT, err := b.getOPS(ctx, tileKey)
		if err != nil {
			return nil, fmt.Errorf("Failed to get OPS: %s", err)
		}

		// If the OPS contains our paramset then we're done.
		if delta := entry.ops.Delta(p); len(delta) == 0 {
			return entry.ops, nil
		}

		// Create a new updated ops.
		ops := entry.ops.Copy()
		ops.Update(p)
		newEntry, err = opsCacheEntryFromOPS(ops)
		if err != nil {
			return nil, skerr.Fmt("Failed to fetch ops from cache: %s", err)
		}
		encodedOps, err := newEntry.ops.Encode()
		if err != nil {
			return nil, skerr.Fmt("Failed to encode new ops: %s", err)
		}

		now := bigtable.Time(time.Now())
		condTrue := false
		if existsInBT {
			// Create an update that avoids the lost update problem.
			cond := bigtable.ChainFilters(
				bigtable.FamilyFilter(opsFamily),
				bigtable.ColumnFilter(opsHashColumn),
				bigtable.ValueFilter(string(entry.hash)),
			)
			updateMutation := bigtable.NewMutation()
			updateMutation.Set(opsFamily, opsHashColumn, now, []byte(newEntry.hash))
			updateMutation.Set(opsFamily, opsOpsColumn, now, encodedOps)

			// Add a mutation that cleans up old versions.
			before := bigtable.Time(time.Now().Add(-1 * time.Second))
			updateMutation.DeleteTimestampRange(opsFamily, opsHashColumn, 0, before)
			updateMutation.DeleteTimestampRange(opsFamily, opsOpsColumn, 0, before)
			condUpdate := bigtable.NewCondMutation(cond, updateMutation, nil)

			if err := b.getTable().Apply(tctx, tileKey.OpsRowName(b), condUpdate, bigtable.GetCondMutationResult(&condTrue)); err != nil {
				sklog.Warningf("Failed to apply: %s", err)
				return nil, err
			}

			// If !condTrue then we need to try again,
			// and clear our local cache.
			if !condTrue {
				sklog.Warningf("Exists !condTrue - clearing cache and trying again.")
				b.opsCache.Delete(tileKey.OpsRowName(b))
				continue
			}
		} else {
			// Create an update that only works if the ops entry doesn't exist yet.
			// I.e. only apply the mutation if the HASH column doesn't exist for this row.
			cond := bigtable.ChainFilters(
				bigtable.FamilyFilter(opsFamily),
				bigtable.ColumnFilter(opsHashColumn),
			)
			updateMutation := bigtable.NewMutation()
			updateMutation.Set(opsFamily, opsHashColumn, now, []byte(newEntry.hash))
			updateMutation.Set(opsFamily, opsOpsColumn, now, encodedOps)

			condUpdate := bigtable.NewCondMutation(cond, nil, updateMutation)
			if err := b.getTable().Apply(tctx, tileKey.OpsRowName(b), condUpdate, bigtable.GetCondMutationResult(&condTrue)); err != nil {
				sklog.Warningf("Failed to apply: %s", err)
				// clear cache and try again
				b.opsCache.Delete(tileKey.OpsRowName(b))
				continue
			}

			// If condTrue then we need to try again,
			// and clear our local cache.
			if condTrue {
				sklog.Warningf("First Write condTrue - clearing cache and trying again.")
				b.opsCache.Delete(tileKey.OpsRowName(b))
				continue
			}
		}

		// Successfully wrote OPS, so update the cache.
		if b.cacheOps {
			b.opsCache.Store(tileKey.OpsRowName(b), newEntry)
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
func (b *BTTraceStore) getOPS(ctx context.Context, tileKey TileKey) (*OpsCacheEntry, bool, error) {
	if b.cacheOps {
		entry, ok := b.opsCache.Load(tileKey.OpsRowName(b))
		if ok {
			return entry.(*OpsCacheEntry), false, nil
		}
	}
	tctx, cancel := context.WithTimeout(ctx, readTimeout)
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
		b.opsCache.Store(tileKey.OpsRowName(b), entry)
	}
	return entry, true, err
}

func (b *BTTraceStore) getTable() *bigtable.Table {
	return b.table
}

// Make sure BTTraceStore fulfills the TraceStore Interface
var _ tracestore.TraceStore = (*BTTraceStore)(nil)
