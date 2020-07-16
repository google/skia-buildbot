// Package bt_tracestore implements a tracestore backed by BigTable
// See BIGTABLE.md for an overview of the schema and design.
package bt_tracestore

import (
	"context"
	"fmt"
	"hash/crc32"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/bigtable"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"

	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/tracestore"
	"go.skia.org/infra/golden/go/types"
)

// InitBT initializes the BT instance for the given configuration. It uses the default way
// to get auth information from the environment and must be called with an account that has
// admin rights.
func InitBT(ctx context.Context, conf BTConfig) error {
	if conf.ProjectID == "" || conf.InstanceID == "" || conf.TableID == "" {
		return skerr.Fmt("invalid config: must specify all parts of BTConfig")
	}
	// Set up admin client, tables, and column families.
	adminClient, err := bigtable.NewAdminClient(ctx, conf.ProjectID, conf.InstanceID)
	if err != nil {
		return skerr.Wrapf(err, "creating admin client for project %s and instance %s", conf.ProjectID, conf.InstanceID)
	}

	err = adminClient.CreateTableFromConf(ctx, &bigtable.TableConf{
		TableID: conf.TableID,
		Families: map[string]bigtable.GCPolicy{
			opsFamily:     bigtable.MaxVersionsPolicy(1),
			optionsFamily: bigtable.MaxVersionsPolicy(1),
			traceFamily:   bigtable.MaxVersionsPolicy(1),
		},
	})

	// Create the table. Ignore error if it already existed.
	err, code := bt.ErrToCode(err)
	if err != nil && code != codes.AlreadyExists {
		return skerr.Wrapf(err, "creating table %s", conf.TableID)
	} else {
		sklog.Infof("Created table %s on %s instance in project %s", conf.TableID, conf.InstanceID, conf.ProjectID)
	}

	return nil
}

// BTConfig contains the configuration information for the BigTable-based implementation of
// TraceStore.
type BTConfig struct {
	ProjectID  string
	InstanceID string
	TableID    string
	VCS        vcsinfo.VCS
}

// BTTraceStore implements the TraceStore interface.
type BTTraceStore struct {
	vcs    vcsinfo.VCS
	client *bigtable.Client
	table  *bigtable.Table

	// if cacheOps is true, then cache the OrderedParamSets between calls
	// where possible.
	cacheOps bool
	// maps rowName (string) -> *OpsCacheEntry
	opsCache sync.Map
}

// New implements the TraceStore interface backed by BigTable. If cache is true,
// the OrderedParamSets will be cached based on the row name.
func New(ctx context.Context, conf BTConfig, cache bool) (*BTTraceStore, error) {
	client, err := bigtable.NewClient(ctx, conf.ProjectID, conf.InstanceID)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not instantiate client")
	}

	ret := &BTTraceStore{
		vcs:      conf.VCS,
		client:   client,
		table:    client.Open(conf.TableID),
		cacheOps: cache,
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

	repoIndex, err := b.vcs.IndexOf(ctx, commitHash)
	if err != nil {
		err = b.vcs.Update(ctx, true, false)
		if err != nil {
			return skerr.Wrapf(err, "could not update VCS to look up %s", commitHash)
		}
		repoIndex, err = b.vcs.IndexOf(ctx, commitHash)
	}
	if err != nil {
		return skerr.Wrapf(err, "could not look up commit index of %s", commitHash)
	}

	commit, err := b.vcs.Details(ctx, commitHash, false)
	if err != nil {
		return skerr.Wrapf(err, "could not look up commit details for %s", commitHash)
	}

	// Find out what tile we need to fetch and what index into that tile we need.
	// Reminder that tileKeys start at 2^32-1 and decrease in value.
	tileKey, commitIndex := GetTileKey(repoIndex)

	// If these entries have any params we haven't seen before, we need to store those in BigTable.
	ops, err := b.updateOrderedParamSet(ctx, tileKey, paramSet)
	if err != nil {
		sklog.Warningf("Bad paramset: %#v", paramSet)
		return skerr.Wrapf(err, "cannot update paramset")
	}

	// These are two parallel arrays. mutations[i] should be applied to rowNames[i] for all i.
	rowNames, mutations, err := b.createPutMutations(entries, tileKey, commitIndex, ops, ts, commit.Timestamp)
	if err != nil {
		return skerr.Wrapf(err, "could not create mutations to put data for tile %d", tileKey)
	}

	// Write the trace data. We pick a batchsize based on the assumption
	// that the whole batch should be 2MB large and each entry is ~200 Bytes of data.
	// 2MB / 200B = 10000. This is extremely conservative but should not be a problem
	// since the batches are written in parallel.
	return b.applyBulkBatched(ctx, rowNames, mutations, 10000)
}

// createPutMutations is a helper function that returns two parallel arrays of
// the rows that need updating and the mutations to apply to those rows.
// Specifically, the mutations will add the given entries to BT, clearing out
// anything that was there previously.
func (b *BTTraceStore) createPutMutations(entries []*tracestore.Entry, tk TileKey, commitIndex int, ops *paramtools.OrderedParamSet, digestTS, optionsTS time.Time) ([]string, []*bigtable.Mutation, error) {
	// These mutations...
	mutations := make([]*bigtable.Mutation, 0, len(entries))
	// .. should be applied to these rows.
	rowNames := make([]string, 0, len(entries))

	for _, entry := range entries {
		// To save space, traceID isn't the long form tiling.TraceID
		// (e.g. ,foo=bar,baz=gm,), it's a string of key-value numbers
		// that refer to the params.(e.g. ,0=3,2=18,)
		// See params.paramsEncoder
		sTrace, err := ops.EncodeParamsAsString(entry.Params)
		if err != nil {
			return nil, nil, skerr.Wrapf(err, "invalid params")
		}
		traceID := encodedTraceID(sTrace)

		rowName := b.calcShardedRowName(tk, typeTrace, string(traceID))
		rowNames = append(rowNames, rowName)

		// Create a mutation that puts the given digest at the given row
		// (i.e. the trace combined with the tile), at the given column
		// (i.e. the commit offset into this tile).
		mut := bigtable.NewMutation()
		column := fmt.Sprintf(columnPad, commitIndex)

		mut.Set(traceFamily, column, bigtable.Time(digestTS), toBytes(entry.Digest))
		mutations = append(mutations, mut)

		if len(entry.Options) == 0 {
			continue
		}
		rowName = b.calcShardedRowName(tk, typeOptions, string(traceID))
		rowNames = append(rowNames, rowName)

		pBytes := encodeParams(entry.Options)
		mut.Set(optionsFamily, optionsBytesColumn, bigtable.Time(optionsTS), pBytes)
		mutations = append(mutations, mut)
	}
	return rowNames, mutations, nil
}

// GetTile implements the TraceStore interface.
// Of note, due to this request possibly spanning over multiple tiles, the ParamsSet may have a
// set of params that does not actually correspond to a trace (this shouldn't be a problem, but is
// worth calling out). For example, suppose a trace with param " device=alpha" abruptly ends on
// tile 4, commit 7 (where the device was removed from testing). If we are on tile 5 and need to
// query both tile 4 starting at commit 10 and tile 5 (the whole thing), we'll just merge the
// paramsets from both tiles, which includes the "device=alpha" params, but they don't exist in
// any traces seen in the tile (since it ended prior to our cutoff point).
func (b *BTTraceStore) GetTile(ctx context.Context, nCommits int) (*tiling.Tile, []tiling.Commit, error) {
	defer metrics2.FuncTimer().Stop()
	// Look up the commits we need to query from BT
	idxCommits := b.vcs.LastNIndex(nCommits)
	if len(idxCommits) == 0 {
		return nil, nil, skerr.Fmt("No commits found.")
	}

	// These commits could span across multiple tiles, so derive the tiles we need to query.
	c := idxCommits[0]
	startTileKey, startCommitIndex := GetTileKey(c.Index)

	c = idxCommits[len(idxCommits)-1]
	endTileKey, endCommitIndex := GetTileKey(c.Index)

	var egroup errgroup.Group

	var commits []tiling.Commit
	egroup.Go(func() error {
		hashes := make([]string, 0, len(idxCommits))
		for _, ic := range idxCommits {
			hashes = append(hashes, ic.Hash)
		}
		var err error
		commits, err = b.makeTileCommits(ctx, hashes)
		if err != nil {
			return skerr.Wrapf(err, "could not load tile commits")
		}
		return nil
	})

	var traces traceMap
	var params paramtools.ParamSet
	egroup.Go(func() error {
		var err error
		traces, params, err = b.getTracesInRange(ctx, startTileKey, endTileKey, startCommitIndex, endCommitIndex)

		if err != nil {
			return skerr.Wrapf(err, "could not load tile commits (%d, %d, %d, %d)", startTileKey, endTileKey, startCommitIndex, endCommitIndex)
		}
		return nil
	})

	if err := egroup.Wait(); err != nil {
		return nil, nil, skerr.Wrapf(err, "could not load last %d commits into tile", nCommits)
	}

	ret := &tiling.Tile{
		Traces:   traces,
		ParamSet: params,
		Commits:  commits,
	}

	return ret, commits, nil
}

// DEBUG_getTracesInRange exposes getTracesInRange to make it easy to call directly from a helper
// executable such as trace_tool.
func (b *BTTraceStore) DEBUG_getTracesInRange(ctx context.Context, startTileKey, endTileKey TileKey, startCommitIndex, endCommitIndex int) (traceMap, paramtools.ParamSet, error) {
	return b.getTracesInRange(ctx, startTileKey, endTileKey, startCommitIndex, endCommitIndex)
}

// getTracesInRange returns a traceMap with data from the given start and stop points (tile and index).
// It also includes the ParamSet for that range.
func (b *BTTraceStore) getTracesInRange(ctx context.Context, startTileKey, endTileKey TileKey, startCommitIndex, endCommitIndex int) (traceMap, paramtools.ParamSet, error) {
	sklog.Debugf("getTracesInRange(%d, %d, %d, %d)", startTileKey, endTileKey, startCommitIndex, endCommitIndex)
	// Query those tiles.
	nTiles := int(startTileKey - endTileKey + 1)
	nCommits := int(startTileKey-endTileKey)*int(DefaultTileSize) + (endCommitIndex - startCommitIndex) + 1
	encTiles := make([]*encTile, nTiles)
	options := make([]map[encodedTraceID]paramtools.Params, nTiles)
	var egroup errgroup.Group
	tk := startTileKey
	for idx := 0; idx < nTiles; idx++ {
		func(idx int, tk TileKey) {
			egroup.Go(func() error {
				var err error
				encTiles[idx], err = b.loadTile(ctx, tk)
				if err != nil {
					return skerr.Wrapf(err, "could not load tile with key %d to index %d", tk, idx)
				}
				return nil
			})

			egroup.Go(func() error {
				var err error
				options[idx], err = b.loadOptions(ctx, tk)
				if err != nil {
					return skerr.Wrapf(err, "could not load options with key %d to index %d", tk, idx)
				}
				return nil
			})
		}(idx, tk)
		tk--
	}

	if err := egroup.Wait(); err != nil {
		return nil, nil, skerr.Wrapf(err, "could not load %d tiles", nTiles)
	}

	// This is the full tile we are going to return.
	tileTraces := make(traceMap, len(encTiles[0].traces))
	paramSet := paramtools.ParamSet{}

	// traceIdx tracks the index we are writing digests into the trace.
	traceIdx := nCommits
	// We go backwards to make it easier to identify the most recent Options in the map.
	for idx := nTiles - 1; idx >= 0; idx-- {
		encTile := encTiles[idx]
		// Determine the offset within the tile that we should consider.
		startOffset := 0
		if idx == 0 {
			// If we are on the first tile, start at startCommitIndex
			startOffset = startCommitIndex
		}

		endOffset := DefaultTileSize - 1
		if idx == (nTiles - 1) {
			// If we are on the last tile, end at endCommitIndex
			endOffset = endCommitIndex
		}

		segLen := endOffset - startOffset + 1
		traceIdx -= segLen
		if idx == 0 && traceIdx != 0 {
			// Check our math - should never happen.
			return nil, nil, skerr.Fmt("incorrect tile math (tile %d, index %d) => tile(%d, index %d) was not %d commits", startTileKey, startCommitIndex, endTileKey, endCommitIndex, nCommits)
		}

		for _, pair := range encTile.traces {
			// at this point, pair.ID looks like ,0=1,1=3,3=0,
			// See params.paramsEncoder
			params, err := encTile.ops.DecodeParamsFromString(string(pair.ID))
			if err != nil {
				// This can occur because we read the tile's OPS and the tile's
				// traces concurrently. If Put adds a new trace to the tile after
				// we have read the OPS, we may see the new trace, which may contain
				// params that are not in our copy of the OPS. We don't promise that
				// Put is atomic, so it's fine to just skip this trace.
				sklog.Warningf("Unreadable trace key - could not decode %s: %s", pair.ID, err)
				continue
			}

			// Turn the params into the tiling.TraceID we expect elsewhere.
			traceKey := tiling.TraceIDFromParams(params)
			if _, ok := tileTraces[traceKey]; !ok {
				if opts, ok := options[idx][pair.ID]; ok {
					params.Add(opts)
				}
				gt := tiling.NewEmptyTrace(nCommits, params)
				tileTraces[traceKey] = gt

				// Build up the total set of params
				paramSet.AddParams(params)
			}
			trace := tileTraces[traceKey]
			digests := pair.Digests[startOffset : startOffset+segLen]
			copy(trace.Digests[traceIdx:traceIdx+segLen], digests)
		}
	}

	// Sort the params for determinism.
	paramSet.Normalize()

	return tileTraces, paramSet, nil
}

// GetDenseTile implements the TraceStore interface. It fetches the most recent tile and sees if
// there is enough non-empty data, then queries the next oldest tile until it has nCommits
// non-empty commits.
func (b *BTTraceStore) GetDenseTile(ctx context.Context, nCommits int) (*tiling.Tile, []tiling.Commit, error) {
	sklog.Debugf("GetDenseTile(%d)", nCommits)
	defer metrics2.FuncTimer().Stop()
	// Figure out what index we are on.
	idxCommits := b.vcs.LastNIndex(1)
	if len(idxCommits) == 0 {
		return nil, nil, skerr.Fmt("No commits found.")
	}

	c := idxCommits[0]
	endKey, endIdx := GetTileKey(c.Index)
	tileStartCommitIdx := c.Index - endIdx

	// Given nCommits and the current index, we can figure out how many tiles to
	// request on the first batch (assuming everything has data).
	n := nCommits - endIdx
	startKey := endKey
	for n > 0 {
		n -= DefaultTileSize
		tileStartCommitIdx -= DefaultTileSize
		startKey++
	}

	// commitsWithData is a slice of indexes of commits that have data. These indexes are
	// relative to the repo itself, with index 0 being the first (oldest) commit in the repo.
	commitsWithData := make([]int, 0, nCommits)
	paramSet := paramtools.ParamSet{}
	allTraces := traceMap{}

	// Start at the most recent tile(s) and step backwards until we have enough commits with data.
	for i := 0; i < maxTilesForDenseTile; i++ {
		commitsToFetch := int(startKey-endKey)*DefaultTileSize + endIdx + 1
		traces, params, err := b.getTracesInRange(ctx, startKey, endKey, 0, endIdx)

		if err != nil {
			return nil, nil, skerr.Wrapf(err, "could not load commits from %d-0 to %d-%d", startKey, endKey, endIdx)
		}

		paramSet.AddParamSet(params)
		// filledCommits are the indexes in the traces that have data.
		// That is, they are the indexes of commits in this tile.
		// It will be sorted from low indexes to high indexes
		filledCommits := traces.CommitIndicesWithData(commitsToFetch)
		sklog.Debugf("Got the following commits with data: %d", filledCommits)

		density := float64(len(filledCommits)) / float64(commitsToFetch)
		metrics2.GetFloat64SummaryMetric("tile_density").Observe(density)

		if len(filledCommits)+len(commitsWithData) > nCommits {
			targetLength := nCommits - len(commitsWithData)
			// trim filledCommits so we get to exactly nCommits
			filledCommits = filledCommits[len(filledCommits)-targetLength:]
		}

		for _, tileIdx := range filledCommits {
			commitsWithData = append(commitsWithData, tileStartCommitIdx+tileIdx)
		}
		cTraces := traces.MakeFromCommitIndexes(filledCommits)
		allTraces.PrependTraces(cTraces)

		if len(commitsWithData) >= nCommits || startKey == tileKeyFromIndex(0) {
			break
		}

		startKey++ // go backwards in time one tile
		endKey = startKey
		endIdx = DefaultTileSize - 1 // fetch the whole previous tile
		tileStartCommitIdx -= DefaultTileSize
	}

	if len(commitsWithData) == 0 {
		return &tiling.Tile{}, nil, nil
	}
	allCommits, denseCommits, err := b.commitsFromVCS(ctx, commitsWithData)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	ret := &tiling.Tile{
		Traces:   allTraces,
		ParamSet: paramSet,
		Commits:  denseCommits,
	}
	sklog.Debugf("GetDenseTile complete")

	return ret, allCommits, nil
}

// commitsFromVCS takes the indices of the commits that have data (i.e. what index a commit is
// from the beginning of repo history) and turns those into actual commit objects with data
// from the VCS. It returns all commits that are between the first and last commit index
// (inclusively) as well as just the commits specified. These are "allCommits" and "denseCommits",
// respectively.
func (b *BTTraceStore) commitsFromVCS(ctx context.Context, commitsWithData []int) ([]tiling.Commit, []tiling.Commit, error) {
	// put them in oldest to newest order
	sort.Ints(commitsWithData)

	oldestIdx := commitsWithData[0]
	oldestCommit, err := b.vcs.ByIndex(ctx, oldestIdx)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "invalid oldest index %d", oldestIdx)
	}
	// Timestamps in Git are to the second granularity, so we should step back 1 second to make sure
	// we include oldestCommit in our range.
	hashes := b.vcs.From(oldestCommit.Timestamp.Add(-1 * time.Second))

	// There's no guarantee that hashes[0] == oldestCommit[0] (e.g. two commits at same timestamp)
	// So we trim hashes down if necessary
	for i := 0; i < len(hashes); i++ {
		if hashes[i] == oldestCommit.Hash {
			sklog.Debugf("Trimming first %d commits", i)
			hashes = hashes[i:]
			break
		}
	}

	allCommits, err := b.makeTileCommits(ctx, hashes)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "could not make tile commits")
	}

	denseCommits := make([]tiling.Commit, len(commitsWithData))
	for i, idx := range commitsWithData {
		if idx-oldestIdx >= len(allCommits) {
			// This happened once, causing a nil dereference. If it happens again, this logging may
			// help us figure out what's going wrong (is VCS giving us bad data?).
			sklog.Debugf("Oldest commit with index %d: %#v", oldestIdx, oldestCommit)
			sklog.Debugf("Commits with data: %d", commitsWithData)
			sklog.Debugf("hashes: %q", hashes)
			return nil, nil, skerr.Fmt("could not identify dense commits (%d - %d >= %d)", idx, oldestIdx, len(allCommits))
		}
		denseCommits[i] = allCommits[idx-oldestIdx]
	}
	return allCommits, denseCommits, nil
}

// GetTileKey retrieves the tile key and the index of the commit in the given tile (commitIndex)
// given the index of a commit in the repo (repoIndex).
// commitIndex starts at 0 for the oldest commit in the tile.
func GetTileKey(repoIndex int) (TileKey, int) {
	tileIndex := int32(repoIndex) / DefaultTileSize
	commitIndex := repoIndex % DefaultTileSize
	return tileKeyFromIndex(tileIndex), commitIndex
}

// loadTile returns an *encTile corresponding to the TileKey.
func (b *BTTraceStore) loadTile(ctx context.Context, tileKey TileKey) (*encTile, error) {
	defer metrics2.FuncTimer().Stop()
	var egroup errgroup.Group

	// Load the OrderedParamSet so the caller can decode the data from the tile.
	var ops *paramtools.OrderedParamSet
	egroup.Go(func() error {
		opsEntry, _, err := b.getOPS(ctx, tileKey)
		if err != nil {
			return skerr.Wrapf(err, "could not load OPS")
		}
		ops = opsEntry.ops
		return nil
	})

	var traces []*encodedTracePair
	egroup.Go(func() error {
		var err error
		traces, err = b.loadEncodedTraces(ctx, tileKey)
		if err != nil {
			return skerr.Wrapf(err, "could not load traces")
		}
		return nil
	})

	if err := egroup.Wait(); err != nil {
		return nil, err
	}

	return &encTile{
		ops:    ops,
		traces: traces,
	}, nil
}

// loadOptions returns the options map corresponding to the given tile.
func (b *BTTraceStore) loadOptions(ctx context.Context, tileKey TileKey) (map[encodedTraceID]paramtools.Params, error) {
	defer metrics2.FuncTimer().Stop()

	var egroup errgroup.Group
	shardResults := make([]map[encodedTraceID]paramtools.Params, DefaultShards)
	traceCount := int64(0)

	// Query all shards in parallel.
	for shard := int32(0); shard < DefaultShards; shard++ {
		func(shard int32) {
			egroup.Go(func() error {
				// Most of the options aren't unique. For example, in a single tile,
				// the same options can be shared across most traces with the same name.
				uniqueParams := make(paramCache, 1000)

				// This prefix will match all traces belonging to the
				// current shard in the current tile.
				prefixRange := bigtable.PrefixRange(shardedRowName(shard, typeOptions, tileKey, ""))
				target := map[encodedTraceID]paramtools.Params{}
				shardResults[shard] = target
				// There should only be one column per row (optionsBytesColumn)
				// The cell in this column should be the most recent based on the timestamp in Put
				err := b.table.ReadRows(ctx, prefixRange, func(row bigtable.Row) bool {
					// The encoded trace id is the "subkey" part of the row name.
					encID := encodedTraceID(extractSubkey(row.Key()))
					atomic.AddInt64(&traceCount, 1)

					var p paramtools.Params
					for _, c := range row[optionsFamily] {
						p = uniqueParams.FromBytesOrCache(c.Value)
						break
					}
					if len(p) == 0 {
						return true
					}

					target[encID] = p
					return true
				}, bigtable.RowFilter(
					bigtable.ChainFilters(
						bigtable.FamilyFilter(optionsFamily),
						bigtable.LatestNFilter(1),
						bigtable.CellsPerRowLimitFilter(1),
					),
				))
				if err != nil {
					return skerr.Wrapf(err, "could not read options on shard %d", shard)
				}
				return nil
			})
		}(shard)
	}

	if err := egroup.Wait(); err != nil {
		return nil, skerr.Wrapf(err, "could not read options")
	}

	// Merge all the results together
	ret := make(map[encodedTraceID]paramtools.Params, traceCount)
	for _, r := range shardResults {
		for encID, opts := range r {
			// different shards should never share results for a tracekey
			// since a trace always maps to the same shard.
			ret[encID] = opts
		}
	}

	return ret, nil
}

// loadEncodedTraces returns all traces belonging to the given TileKey.
// As outlined in BIGTABLE.md, the trace ids and the digest ids they
// map to are in an encoded form and will need to be expanded prior to use.
func (b *BTTraceStore) loadEncodedTraces(ctx context.Context, tileKey TileKey) ([]*encodedTracePair, error) {
	defer metrics2.FuncTimer().Stop()
	var egroup errgroup.Group
	shardResults := [DefaultShards][]*encodedTracePair{}
	traceCount := int64(0)

	// Query all shards in parallel.
	for shard := int32(0); shard < DefaultShards; shard++ {
		func(shard int32) {
			// Most of the strings aren't unique. For example, in a single, stable trace,
			// the same digest may be used for all commits in the row. Thus, we don't want
			// to have to allocate memory on the heap for each of those strings, we can
			// just reuse those (immutable) strings.
			uniqueDigests := make(digestCache, 1000)

			egroup.Go(func() error {
				// This prefix will match all traces belonging to the
				// current shard in the current tile.
				prefixRange := bigtable.PrefixRange(shardedRowName(shard, typeTrace, tileKey, ""))
				var parseErr error
				err := b.table.ReadRows(ctx, prefixRange, func(row bigtable.Row) bool {
					atomic.AddInt64(&traceCount, 1)
					// The encoded trace id is the "subkey" part of the row name.
					traceKey := encodedTraceID(extractSubkey(row.Key()))
					pair := encodedTracePair{
						ID:      traceKey,
						Digests: [DefaultTileSize]types.Digest{},
					}
					// kjlubick@ and benjaminwagner@ are not super sure why, but this
					// helps reduce memory usage by allowing the allocated pair to be
					// cleaned up after GetTile/GetDenseTile completes.
					// Without this initialization step, the Digest arrays seem to
					// persist, causing a 2-3x increase in tile RAM size.
					// It might be due to a bug in golang and/or just that the GC gets a
					// little confused due to the complex procedure of passing things around.
					for i := range pair.Digests {
						pair.Digests[i] = tiling.MissingDigest
					}

					for _, col := range row[traceFamily] {
						// The columns are something like T:35 where the part
						// after the colon is the commitIndex i.e. the index
						// of this commit in the current tile.
						idx, err := strconv.Atoi(strings.TrimPrefix(col.Column, traceFamilyPrefix))
						if err != nil {
							// Should never happen
							parseErr = err
							return false
						}
						if idx < 0 || idx >= DefaultTileSize {
							// This would happen if the tile size changed from a past
							// value. It shouldn't be changed, even if the Gold tile size
							// (n_commits) changes.
							parseErr = skerr.Fmt("got index %d that is outside of the target slice of length %d", idx, DefaultTileSize)
							return false
						}
						d := uniqueDigests.FromBytesOrCache(col.Value)
						pair.Digests[idx] = d
					}
					shardResults[shard] = append(shardResults[shard], &pair)
					return true
				}, bigtable.RowFilter(
					bigtable.ChainFilters(
						bigtable.FamilyFilter(traceFamily),
						// can be used for local testing to keep RAM usage lower
						// bigtable.RowSampleFilter(0.01),
						bigtable.LatestNFilter(1),
						bigtable.CellsPerRowLimitFilter(DefaultTileSize),
					),
				))
				if err != nil {
					return skerr.Wrapf(err, "could not read rows")
				}
				return parseErr
			})
		}(shard)
	}

	if err := egroup.Wait(); err != nil {
		return nil, err
	}

	// Merge all the results together
	ret := make([]*encodedTracePair, 0, traceCount)
	for _, r := range shardResults {
		// different shards should never share results for a tracekey
		// since a trace always maps to the same shard.
		ret = append(ret, r...)
	}

	return ret, nil
}

// applyBulkBatched writes the given rowNames/mutation pairs to BigTable in batches that are
// maximally of size 'batchSize'. The batches are written in parallel.
func (b *BTTraceStore) applyBulkBatched(ctx context.Context, rowNames []string, mutations []*bigtable.Mutation, batchSize int) error {
	if len(rowNames) == 0 {
		return nil
	}
	err := util.ChunkIterParallel(ctx, len(rowNames), batchSize, func(ctx context.Context, chunkStart, chunkEnd int) error {
		ctx, cancel := context.WithTimeout(ctx, writeTimeout)
		defer cancel()
		rowNames := rowNames[chunkStart:chunkEnd]
		mutations := mutations[chunkStart:chunkEnd]
		errs, err := b.table.ApplyBulk(ctx, rowNames, mutations)
		if err != nil {
			return skerr.Wrapf(err, "writing batch [%d:%d]", chunkStart, chunkEnd)
		}
		if errs != nil {
			return skerr.Wrapf(err, "writing some portions of batch [%d:%d]", chunkStart, chunkEnd)
		}
		return nil
	})
	if err != nil {
		return skerr.Wrapf(err, "running ChunkIterParallel(%s...%s) batch size %d", rowNames[0], rowNames[len(rowNames)-1], batchSize)
	}
	return nil
}

// calcShardedRowName deterministically assigns a shard for the given subkey (e.g. traceID)
// Once this is done, the shard, rowtype, TileKey and the subkey are combined into a
// single string to be used as a row name in BT.
func (b *BTTraceStore) calcShardedRowName(tileKey TileKey, rowType, subkey string) string {
	shard := int32(crc32.ChecksumIEEE([]byte(subkey)) % uint32(DefaultShards))
	return shardedRowName(shard, rowType, tileKey, subkey)
}

// Copied from btts.go in infra/perf

// UpdateOrderedParamSet will add all params from 'p' to the OrderedParamSet
// for 'TileKey' and write it back to BigTable.
func (b *BTTraceStore) updateOrderedParamSet(ctx context.Context, tileKey TileKey, p paramtools.ParamSet) (*paramtools.OrderedParamSet, error) {
	defer metrics2.FuncTimer().Stop()

	tctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	var newEntry *opsCacheEntry
	for {
		// Get OPS.
		entry, existsInBT, err := b.getOPS(ctx, tileKey)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to get OPS for tile %d", tileKey)
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
			return nil, skerr.Wrapf(err, "failed to create cache entry")
		}
		encodedOps, err := newEntry.ops.Encode()
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to encode new ops")
		}

		now := bigtable.Time(time.Now())
		condTrue := false
		if existsInBT {
			// Create an update that avoids the lost update problem.
			cond := bigtable.ChainFilters(
				bigtable.LatestNFilter(1),
				bigtable.FamilyFilter(opsFamily),
				bigtable.ColumnFilter(opsHashColumn),
				bigtable.ValueFilter(string(entry.hash)),
			)
			updateMutation := bigtable.NewMutation()
			updateMutation.Set(opsFamily, opsHashColumn, now, []byte(newEntry.hash))
			updateMutation.Set(opsFamily, opsOpsColumn, now, encodedOps)

			// Add a mutation that cleans up old versions.
			before := bigtable.Time(now.Time().Add(-1 * time.Second))
			updateMutation.DeleteTimestampRange(opsFamily, opsHashColumn, 0, before)
			updateMutation.DeleteTimestampRange(opsFamily, opsOpsColumn, 0, before)
			condUpdate := bigtable.NewCondMutation(cond, updateMutation, nil)

			if err := b.table.Apply(tctx, tileKey.OpsRowName(), condUpdate, bigtable.GetCondMutationResult(&condTrue)); err != nil {
				sklog.Warningf("Failed to apply: %s", err)
				return nil, err
			}

			// If !condTrue then we need to try again,
			// and clear our local cache.
			if !condTrue {
				sklog.Warningf("Exists !condTrue - clearing cache and trying again.")
				b.opsCache.Delete(tileKey.OpsRowName())
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
			if err := b.table.Apply(tctx, tileKey.OpsRowName(), condUpdate, bigtable.GetCondMutationResult(&condTrue)); err != nil {
				sklog.Warningf("Failed to apply: %s", err)
				// clear cache and try again
				b.opsCache.Delete(tileKey.OpsRowName())
				continue
			}

			// If condTrue then we need to try again,
			// and clear our local cache.
			if condTrue {
				sklog.Warningf("First Write condTrue - clearing cache and trying again.")
				b.opsCache.Delete(tileKey.OpsRowName())
				continue
			}
		}

		// Successfully wrote OPS, so update the cache.
		if b.cacheOps {
			b.opsCache.Store(tileKey.OpsRowName(), newEntry)
		}
		break
	}
	return newEntry.ops, nil
}

// getOps returns the OpsCacheEntry for a given tile.
//
// Note that it will create a new OpsCacheEntry if none exists.
//
// getOps returns false if the OPS in BT was empty, true otherwise (even if cached).
func (b *BTTraceStore) getOPS(ctx context.Context, tk TileKey) (*opsCacheEntry, bool, error) {
	defer metrics2.FuncTimer().Stop()
	if b.cacheOps {
		entry, ok := b.opsCache.Load(tk.OpsRowName())
		if ok {
			return entry.(*opsCacheEntry), true, nil
		}
	}
	tctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	row, err := b.table.ReadRow(tctx, tk.OpsRowName(), bigtable.RowFilter(
		bigtable.ChainFilters(
			bigtable.LatestNFilter(1),
			bigtable.FamilyFilter(opsFamily),
		),
	))
	if err != nil {
		return nil, false, skerr.Wrapf(err, "failed to read OPS row %s from BigTable", tk.OpsRowName())
	}
	// If there is no entry in BigTable then return an empty OPS.
	if len(row) == 0 {
		sklog.Warningf("Failed to read OPS from BT for %s; the tile could be empty", tk.OpsRowName())
		entry, err := newOpsCacheEntry()
		return entry, false, err
	}
	entry, err := newOpsCacheEntryFromRow(row)
	if err == nil && b.cacheOps {
		b.opsCache.Store(tk.OpsRowName(), entry)
	}
	return entry, true, err
}

// makeTileCommits creates a slice of tiling.Commit from the given git hashes.
// Specifically, we need to look up the details to get the author and subject information.
func (b *BTTraceStore) makeTileCommits(ctx context.Context, hashes []string) ([]tiling.Commit, error) {
	longCommits, err := b.vcs.DetailsMulti(ctx, hashes, false)
	if err != nil {
		// put hashes second in case they get truncated for being quite long.
		return nil, skerr.Wrapf(err, "could not fetch commit data for commits with hashes %q", hashes)
	}

	commits := make([]tiling.Commit, len(hashes))
	for i, lc := range longCommits {
		if lc == nil {
			return nil, skerr.Fmt("commit %s not found from VCS", hashes[i])
		}
		commits[i] = tiling.Commit{
			Hash:       lc.Hash,
			Author:     lc.Author,
			CommitTime: lc.Timestamp,
			Subject:    lc.Subject,
		}
	}
	return commits, nil
}

// Make sure BTTraceStore fulfills the TraceStore Interface
var _ tracestore.TraceStore = (*BTTraceStore)(nil)
