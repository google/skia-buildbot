package bt_tracestore

// This implementation of storing tiles in BigTable is very similar to
// that in Perf.  See perf/BIGTABLE.md for an overview.

import (
	"context"
	"encoding/binary"
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

	// if cacheOps is true, then cache the OrderedParamSets between calls
	// where possible.
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
	// map of string (types.Digest) -> int64(DigestId) in big table, then refer to
	// the DigestID elsewhere in the table. DigestIds are essentially a monotonically
	// increasing arbitrary number.
	digestMap, err := b.updateDigestMap(ctx, digestSet)
	if err != nil {
		sklog.Warningf("Bad digestSet: %#v", digestSet)
		return skerr.Fmt("cannot update digest map: %s", err)
	}

	metrics2.GetInt64Metric("gold_digest_map_size").Update(int64(digestMap.Len()))

	if len(digestMap.Delta(digestSet)) != 0 {
		// Should never happen
		return skerr.Fmt("delta should be empty at this point: %v", digestMap.Delta(digestSet))
	}

	// These are two parallel arrays. mutations[i] should be applied to rowNames[i] for all i.
	rowNames, mutations, err := b.createPutMutations(entries, ts, tileKey, commitOffset, ops, digestMap)
	if err != nil {
		return skerr.Fmt("could not create mutations to put data: %s", err)
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
func (b *BTTraceStore) createPutMutations(entries []*tracestore.Entry, ts time.Time, tk TileKey, commitOffset int, ops *paramtools.OrderedParamSet, dm *DigestMap) ([]string, []*bigtable.Mutation, error) {
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
			return nil, nil, skerr.Fmt("invalid params: %s", err)
		}
		traceKey := EncodedTraceId(sTrace)

		rowName := b.calcShardedRowName(tk, typeTrace, string(traceKey))
		rowNames = append(rowNames, rowName)

		digestID, err := dm.ID(entry.Digest)
		if err != nil {
			// this should never happen, the digest map should know about every digest already.
			return nil, nil, skerr.Fmt("could not fetch id for digest %s: %s", entry.Digest, err)
		}

		// Create a mutation that puts the given digest at the given row
		// (i.e. the trace combined with the tile), at the given column
		// (i.e. the commit offset into this tile).
		mut := bigtable.NewMutation()
		column := strconv.Itoa(commitOffset)
		mut.Set(traceFamily, column, btTS, []byte(strconv.Itoa(int(digestID))))
		// Delete anything that existed at this cell before now.
		mut.DeleteTimestampRange(traceFamily, column, 0, before)
		mutations = append(mutations, mut)
	}
	return rowNames, mutations, nil
}

// GetTile implements the TraceStore interface.
func (b *BTTraceStore) GetTile(ctx context.Context, nCommits int, isSparse bool) (*tiling.Tile, []*tiling.Commit, error) {
	defer metrics2.FuncTimer().Stop()

	// Look up the commits we need to query from BT
	idxCommits := b.vcs.LastNIndex(nCommits)
	if len(idxCommits) == 0 {
		return nil, nil, skerr.Fmt("No commits found.")
	}

	// These commits could span across multiple tiles, so derive the tiles we need to query.
	c := idxCommits[0]
	startTileKey, startCommitOffset, err := b.getTileKey(ctx, c.Hash)
	if err != nil {
		return nil, nil, skerr.Fmt("could not derive tile key for start commit %s: %s", c.Hash, err)
	}

	c = idxCommits[len(idxCommits)-1]
	endTileKey, endCommitOffset, err := b.getTileKey(ctx, c.Hash)
	if err != nil {
		return nil, nil, skerr.Fmt("could not derive tile key for end commit %s: %s", c.Hash, err)
	}

	// Query those tiles.
	nTiles := int(startTileKey - endTileKey + 1)
	encTiles := make([]*EncTile, nTiles)
	var egroup errgroup.Group
	tileKey := startTileKey
	for idx := 0; idx < nTiles; idx++ {
		func(idx int, tileKey TileKey) {
			egroup.Go(func() error {
				var err error
				encTiles[idx], err = b.loadTile(ctx, tileKey)
				if err != nil {
					return skerr.Fmt("could not load tile with key %d to index %d: %s", tileKey, idx, err)
				}
				return nil
			})
		}(idx, tileKey)
		tileKey--
	}

	var digestMap *DigestMap
	egroup.Go(func() error {
		var err error
		digestMap, err = b.getDigestMap(ctx)
		if err != nil {
			return skerr.Fmt("could not load digestMap: %s", err)
		}
		return nil
	})

	if err := egroup.Wait(); err != nil {
		return nil, nil, skerr.Fmt("could not load %d tiles: %s", nTiles, err)
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
			// If we are on the last tile, stop early (that is, at
			// endCommitOffset)
			ecOffset = endCommitOffset
		}
		segLen := ecOffset - startCommitOffset + 1

		for encodedKey, encValues := range encTile.traces {
			// at this point, the encodedKey looks like ,0=1,1=3,3=0,
			// See params.paramsEncoder
			params, err := encTile.ops.DecodeParamsFromString(string(encodedKey))
			if err != nil {
				sklog.Warningf("Incomplete OPS: %#v\n", encTile.ops)
				return nil, nil, skerr.Fmt("corrupted trace key - could not decode %s: %s", encodedKey, err)
			}

			// Turn the params into the tiling.TraceId we expect elsewhere.
			traceKey := tracestore.TraceIDFromParams(params)
			if _, ok := tileTraces[traceKey]; !ok {
				tileTraces[traceKey] = types.NewGoldenTraceN(nCommits)
			}
			gt := tileTraces[traceKey].(*types.GoldenTrace)
			gt.Keys = params
			// Build up the total set of params
			paramSet.AddParams(params)

			// Convert the digests from integer IDs to strings.
			digestIDs := encValues[startCommitOffset : startCommitOffset+segLen]
			digests, err := digestMap.DecodeIDs(digestIDs)
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
			return nil, nil, skerr.Fmt("could not fetch commit data for %q: %s", hashes, err)
		}

		for idx, lc := range longCommits {
			if lc == nil {
				return nil, nil, skerr.Fmt("commit %s not found from VCS", hashes[idx])
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

	// Sort the params for determinism.
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
	// Index 0 is the very first commit in the repo.
	commitIndex, err := b.vcs.IndexOf(ctx, commitHash)
	if err != nil {
		return 0, 0, skerr.Fmt("could not look up commit %s: %s", commitHash, err)
	}

	tileOffset := int32(commitIndex) / b.tileSize
	commitOffset := commitIndex % int(b.tileSize)
	return TileKeyFromOffset(tileOffset), commitOffset, nil
}

// loadTile returns an *EncTile corresponding to the tileKey.
func (b *BTTraceStore) loadTile(ctx context.Context, tileKey TileKey) (*EncTile, error) {
	var egroup errgroup.Group

	// Load the OrderedParamSet so the caller can decode the data from the tile.

	var ops *paramtools.OrderedParamSet
	egroup.Go(func() error {
		opsEntry, _, err := b.getOPS(ctx, tileKey)
		if err != nil {
			return skerr.Fmt("could not load OPS: %s", err)
		}
		ops = opsEntry.ops
		return nil
	})

	var traces map[EncodedTraceId][]DigestID
	egroup.Go(func() error {
		var err error
		traces, err = b.loadEncodedTraces(ctx, tileKey)
		if err != nil {
			return skerr.Fmt("could not load traces: %s", err)
		}
		return nil
	})

	if err := egroup.Wait(); err != nil {
		return nil, err
	}

	return &EncTile{
		ops:    ops,
		traces: traces,
	}, nil
}

// loadEncodedTraces returns all traces belonging to the given tileKey.
// As outlined in BIGTABLE.md, the trace ids and the digest ids they
// map to are in an encoded form and will need to be expanded prior to use.
func (b *BTTraceStore) loadEncodedTraces(ctx context.Context, tileKey TileKey) (map[EncodedTraceId][]DigestID, error) {
	var egroup errgroup.Group
	shardResults := make([]map[EncodedTraceId][]DigestID, b.shards)
	traceCount := int64(0)

	// Query all shards in parallel.
	for shard := int32(0); shard < b.shards; shard++ {
		func(shard int32) {
			egroup.Go(func() error {
				// This prefix will match all traces belonging to the
				// current shard in the current tile.
				prefixRange := bigtable.PrefixRange(shardedRowName(shard, typeTrace, tileKey, ""))
				target := map[EncodedTraceId][]DigestID{}
				shardResults[shard] = target
				var parseErr error
				err := b.table.ReadRows(ctx, prefixRange, func(row bigtable.Row) bool {
					// The encoded trace id is the "key" part of the row name.
					traceKey := EncodedTraceId(extractKey(row.Key()))
					// If this is the first time we've seen the trace, initialize the
					// slice of digest ids for it.
					if _, ok := target[traceKey]; !ok {
						target[traceKey] = make([]DigestID, b.tileSize)
						atomic.AddInt64(&traceCount, 1)
					}

					for _, col := range row[traceFamily] {
						// The columns are something like T:35 where the part
						// after the colon is the commitOffset i.e. the index
						// of this commit in the current tile.
						idx, err := strconv.Atoi(strings.TrimPrefix(col.Column, traceFamilyPrefix))
						if err != nil {
							// Should never happen
							parseErr = err
							return false
						}
						digestID, err := strconv.ParseInt(string(col.Value), 10, 64)
						if err != nil {
							// This should never happen
							parseErr = err
							return false
						}
						if idx < 0 || idx >= int(b.tileSize) {
							// This would happen if the tile size changed from a past
							// value. It shouldn't be changed, even if the Gold tile size
							// (n_commits) changes.
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

	// Merge all the results together
	ret := make(map[EncodedTraceId][]DigestID, traceCount)
	for _, r := range shardResults {
		for traceKey, digestIDs := range r {
			if found, ok := ret[traceKey]; ok {
				for idx, digestID := range digestIDs {
					if digestID != MissingDigestId {
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

// applyBulkBatched writes the given rowNames/mutation pairs to BigTable in batches that are
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
				return skerr.Fmt("error writing batch [%d:%d]: %s", chunkStart, chunkEnd, err)
			}
			if errs != nil {
				return skerr.Fmt("error writing some portions of batch [%d:%d]: %s", chunkStart, chunkEnd, errs)
			}
			return nil
		})
		return nil
	})
	if err != nil {
		return skerr.Fmt("error running ChunkIter: %s", err)
	}
	return egroup.Wait()
}

// calcShardedRowName deterministically assigns a given key (e.g. traceID)
// Once this is done, the shard, rowtype, tileKey and the key are combined into a
// single string to be used as a row name in BT.
func (b *BTTraceStore) calcShardedRowName(tileKey TileKey, rowType, key string) string {
	shard := int32(crc32.ChecksumIEEE([]byte(key)) % uint32(b.shards))
	return shardedRowName(shard, rowType, tileKey, key)
}

// To avoid having one monolithic row, we take the first two characters of the digest
// and use it as a key in the row. Then, what remains is used as the column name.
// In practice this means our digests will be split using 2 hexadecimal characters, so
// we will have 16*16 rows for our digest map.
func (b *BTTraceStore) rowAndColNameFromDigest(digest types.Digest) (string, string) {
	key := string(digest[:3])
	colName := string(digest[3:])
	return b.calcShardedRowName(digestMapTile, typeDigestMap, key), colName
}

// getDigestMap gets the global (i.e. same for all tiles) digestMap.
func (b *BTTraceStore) getDigestMap(ctx context.Context) (*DigestMap, error) {
	// Query all shards in parallel.
	var egroup errgroup.Group
	resultCh := make(chan map[types.Digest]DigestID, b.shards)
	total := int64(0)
	for shard := int32(0); shard < b.shards; shard++ {
		func(shard int32) {
			egroup.Go(func() error {
				prefRange := bigtable.PrefixRange(shardedRowName(shard, typeDigestMap, digestMapTile, ""))
				var idx int64
				var parseErr error = nil
				ret := map[types.Digest]DigestID{}
				err := b.table.ReadRows(ctx, prefRange, func(row bigtable.Row) bool {
					digestPrefix := extractKey(row.Key())
					for _, col := range row[digestMapFamily] {
						idx, parseErr = strconv.ParseInt(string(col.Value), 10, 64)
						if parseErr != nil {
							// Should never happen
							return false
						}
						digest := types.Digest(digestPrefix + strings.TrimPrefix(col.Column, digestMapFamilyPrefix))
						ret[digest] = DigestID(idx)
					}
					return true
				})

				if err != nil {
					return skerr.Fmt("problem fetching shard %d of digestmap: %s", shard, err)
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
		return nil, skerr.Fmt("problem fetching digestmap: %s", err)
	}
	close(resultCh)

	ret := NewDigestMap(int(total))
	for dm := range resultCh {
		if err := ret.Add(dm); err != nil {
			return nil, skerr.Fmt("could not build DigestMap with results %#v: %s", dm, err)
		}
	}
	return ret, nil
}

// getIDs returns a []DigestID of length n where each of the
// digestIDs are unique (even between processes).
func (b *BTTraceStore) getIDs(ctx context.Context, n int) ([]DigestID, error) {
	// Extract up to n ids from those we have already cached.
	b.availIDsMutex.Lock()
	defer b.availIDsMutex.Unlock()
	toExtract := util.MinInt(len(b.availIDs), n)

	ids := make([]DigestID, 0, n)
	ids = append(ids, b.availIDs[:toExtract]...)
	b.availIDs = b.availIDs[toExtract:]

	// missing is how many ids we are short
	missing := int64(n - len(ids))
	if missing == 0 {
		return ids, nil
	}
	// request 1 more to handle the special case of 0, which is the default value
	// and is assigned to MISSING_DIGEST.
	toRequest := missing + 1
	// For performance reasons, make a few big requests for ids instead of many small ones.
	if toRequest < batchIdRequest {
		toRequest = batchIdRequest
	}

	// Reserve new IDs via the ID counter
	rmw := bigtable.NewReadModifyWrite()
	rmw.Increment(idCounterFamily, idCounterColumn, toRequest)
	row, err := b.table.ApplyReadModifyWrite(ctx, idCounterRow, rmw)
	if err != nil {
		return nil, skerr.Fmt("could not fetch counter from BT: %s", err)
	}

	// ri are the rows in Row of the given counter family
	// This should be 1 row with 1 column.
	ri, ok := row[idCounterFamily]
	if !ok {
		// should never happen
		return nil, skerr.Fmt("malformed response - no id counter family: %#v", ri)
	}
	if len(ri) != 1 {
		// should never happen
		return nil, skerr.Fmt("malformed response - expected 1 cell: %#v", ri)
	}

	encMaxID := []byte(ri[0].Value)
	maxID := DigestID(binary.BigEndian.Uint64(encMaxID))

	lastID := maxID - DigestID(toRequest)
	// ID of 0 is a special case - it's already assigned to MISSING_DIGEST, so skip it.
	if lastID == MissingDigestId {
		lastID++
	}
	for i := lastID; i < maxID; i++ {
		// Give the first ids to the current allocation request...
		if missing > 0 {
			ids = append(ids, i)
		} else {
			// ... and put the remainder in the store for later.
			b.availIDs = append(b.availIDs, i)
		}
		missing--
	}

	return ids, nil
}

// returnIDs can be called with a []DigestID of ids that were not actually
// assigned to digests. This allows them to be used by future requests to
// getIDs.
func (b *BTTraceStore) returnIDs(unusedIDs []DigestID) {
	b.availIDsMutex.Lock()
	defer b.availIDsMutex.Unlock()
	b.availIDs = append(b.availIDs, unusedIDs...)
}

// getOrAddDigests fills the given digestMap with the given digests
// assigned to a DigestID if they don't already have an assignment.
// TODO(kjlubick): This currently makes a lot of requests to BT -
// Should there be some caching done here to prevent that?
func (b *BTTraceStore) getOrAddDigests(ctx context.Context, digests []types.Digest, digestMap *DigestMap) (*DigestMap, error) {
	availIDs, err := b.getIDs(ctx, len(digests))
	if err != nil {
		return nil, err
	}

	newIDMapping := make(map[types.Digest]DigestID, len(digests))
	unusedIDs := make([]DigestID, 0, len(availIDs))
	for idx, digest := range digests {
		idVal := availIDs[idx]
		if _, err := digestMap.ID(digest); err == nil {
			// digestMap already has a mapping for this digest, no need to check
			// if BT has seen it yet (because it has).
			unusedIDs = append(unusedIDs, idVal)
			continue
		}
		rowName, colName := b.rowAndColNameFromDigest(digest)
		// This mutation says "Add an entry to the map for digest -> idVal iff
		// the digest doesn't already have a mapping".
		addMut := bigtable.NewMutation()
		addMut.Set(digestMapFamily, colName, bigtable.ServerTime, []byte(strconv.FormatInt(int64(idVal), 10)))
		filter := bigtable.ColumnFilter(colName)
		// Note that we only add the value if filter is false, i.e. the column does not
		// already exist.
		condMut := bigtable.NewCondMutation(filter, nil, addMut)
		var digestAlreadyHadId bool
		if err := b.table.Apply(ctx, rowName, condMut, bigtable.GetCondMutationResult(&digestAlreadyHadId)); err != nil {
			return nil, skerr.Fmt("could not check if row %s col %s already had a DigestID: %s", rowName, colName, err)
		}

		// We didn't need this ID so let's re-use it later.
		if digestAlreadyHadId {
			unusedIDs = append(unusedIDs, idVal)
		} else {
			newIDMapping[digest] = idVal
		}
	}

	// If all ids were added to BT, then we know our newIDMapping can simply be added
	// to what we already have, since there were no collisions between digests and what
	// was in the table already.
	if len(unusedIDs) == 0 {
		if err := digestMap.Add(newIDMapping); err != nil {
			return nil, err
		}
		return digestMap, nil
	}
	// At this point, some of the digests already had ids, so we should reload
	// the entire digestMap to make sure we have the full picture.
	// TODO(kjlubick): Can we not just add what new ones we saw to what we already have?

	// Return the unused IDs for later use.
	b.returnIDs(unusedIDs)
	return b.getDigestMap(ctx)
}

// updateDigestMap returns the current global DigestMap.
func (b *BTTraceStore) updateDigestMap(ctx context.Context, digests types.DigestSet) (*DigestMap, error) {
	// Load the digest map from BT.
	// TODO(kjlubick): should we cache this map and first check to see if the digests
	// are all in there?
	digestMap, err := b.getDigestMap(ctx)
	if err != nil {
		return nil, err
	}

	delta := digestMap.Delta(digests)
	if len(delta) == 0 {
		return digestMap, nil
	}

	return b.getOrAddDigests(ctx, delta, digestMap)
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
			return nil, skerr.Fmt("failed to get OPS: %s", err)
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
			return nil, skerr.Fmt("failed to fetch ops from cache: %s", err)
		}
		encodedOps, err := newEntry.ops.Encode()
		if err != nil {
			return nil, skerr.Fmt("failed to encode new ops: %s", err)
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
// getOps returns true if the returned value came from BT, false if it came
// from the cache.
func (b *BTTraceStore) getOPS(ctx context.Context, tileKey TileKey) (*OpsCacheEntry, bool, error) {
	if b.cacheOps {
		entry, ok := b.opsCache.Load(tileKey.OpsRowName())
		if ok {
			return entry.(*OpsCacheEntry), false, nil
		}
	}
	tctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	row, err := b.table.ReadRow(tctx, tileKey.OpsRowName(), bigtable.RowFilter(bigtable.LatestNFilter(1)))
	if err != nil {
		return nil, false, skerr.Fmt("failed to read OPS from BigTable for %s: %s", tileKey.OpsRowName(), err)
	}
	// If there is no entry in BigTable then return an empty OPS.
	if len(row) == 0 {
		sklog.Warningf("Failed to read OPS from BT for %s.", tileKey.OpsRowName())
		entry, err := NewOpsCacheEntry()
		return entry, false, err
	}
	entry, err := NewOpsCacheEntryFromRow(row)
	if err == nil && b.cacheOps {
		b.opsCache.Store(tileKey.OpsRowName(), entry)
	}
	return entry, true, err
}

// Make sure BTTraceStore fulfills the TraceStore Interface
var _ tracestore.TraceStore = (*BTTraceStore)(nil)
