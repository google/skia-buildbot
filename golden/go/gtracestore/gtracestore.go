package gtracestore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"golang.org/x/sync/errgroup"
)

const (
	// Column families.
	cfBranches = "B"
	cfCommit   = "C"
	cfTsCommit = "T"
	cfMeta     = "M"

	// meta data columns.
	colMetaID        = "metaID"
	colMetaIDCounter = "metaIDCounter"

	// Keys of meta data rows.
	metaVarRepo      = "repo"
	metaVarIDCounter = "idcounter"

	// index commit
	colHash      = "h"
	colTimestamp = "t"

	// shortcommit
	colAuthor   = "a"
	colSubject  = "s"
	colParents  = "p"
	colBody     = "b"
	colBranches = "br"

	// Define the row types.
	typIndex     = "i"
	typTimeStamp = "z"
	typCommit    = "k"
	typMeta      = "!"

	// allCommitsBranch is a pseudo branch name to index all commits in a repo.
	allCommitsBranch = "@all-commits"

	// getBatchSize is the batchsize for the Get operation. Each call to bigtable is made with maximally
	// this number of git hashes. This is a conservative number to stay within the 1M request
	// size limit. Since requests are sharded this will not limit throughput in practice.
	getBatchSize = 5000

	// writeBatchSize is the number of mutations to write at once. This could be fine tuned for different
	// row types.
	writeBatchSize = 1000

	// Default number of shards used, if not shards provided in BTConfig.
	DefaultShards = 32

	// Default tile size if no tilesize provided in BTConfig.
	DefaultTileSize = 256
)

// Constants copied from btts.go
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

// InitBT initializes the BT instance for the given configuration. It uses the default way
// to get auth information from the environment and must be called with an account that has
// admin rights.
func InitBT(conf *BTConfig) error {
	return bt.InitBigtable(conf.ProjectID, conf.InstanceID, conf.TableID, []string{
		cfCommit,
		cfMeta,
		cfBranches,
		cfTsCommit,
	})
}

type TraceStore interface {
	Put(ctx context.Context, commit *vcsinfo.LongCommit, entries []*Entry) error
	GetTile(ctx context.Context, nCommits int, isSparse bool) (*tiling.Tile, []*tiling.Commit, []int, error)
}

type Entry struct {
	Params map[string]string
	Value  string
}

type BTConfig struct {
	ProjectID  string
	InstanceID string
	TableID    string
	VCS        vcsinfo.VCS
	TileSize   int32
	Shards     int32
}

type btTraceStore struct {
	vcs      vcsinfo.VCS
	client   *bigtable.Client
	table    *bigtable.Table
	mutex    sync.RWMutex
	tileSize int32
	shards   int32
	opsCache map[string]*OpsCacheEntry
	cacheOps bool
}

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
	}
	return ret, nil
}

func (b *btTraceStore) Put(ctx context.Context, commit *vcsinfo.LongCommit, entries []*Entry) error {
	// if there are no entries this becomes a no-op.
	if len(entries) == 0 {
		return nil
	}

	// Find the index of the commit in the branch of interest.
	commitIndex, err := b.vcs.IndexOf(ctx, commit.Hash)
	if err != nil {
		return err
	}

	// Accumulate all parameters into a paramset and collect all the digests.
	paramSet := make(paramtools.ParamSet, len(entries[0].Params))
	digestSet := make(util.StringSet, len(entries))
	for _, entry := range entries {
		paramSet.AddParams(entry.Params)
		digestSet[entry.Value] = true
	}

	tileOffset := int32(commitIndex) / b.tileSize
	tileKey := TileKeyFromOffset(tileOffset)
	ops, err := b.updateOrderedParamSet(tileKey, paramSet)
	if err != nil {
		return err
	}

	digestMap, err := b.updateDigestMap(tileKey, digestSet)
	if err != nil {
		return err
	}

	commitOffset := int32(commitIndex) % b.tileSize
	mutations := make([]*bigtable.Mutation, 0, len(entries))
	rowNames := make([]string, 0, len(entries))

	for _, entry := range entries {
		traceKey, err := ops.EncodeParamsAsString(entry.Params)
		if err != nil {
			return err
		}

		rowName := b.traceRowName(tileKey, traceKey)
		rowNames = append(rowNames, rowName)

		digestID, err := digestMap.ID(entry.Value)
		if err != nil {
			return err
		}

	}

	// Write the trace data. We pick a batchsize based on the assumption
	// that the whole batch should be 2MB large and each entry is ~200 Bytes of data.
	// 2MB / 200B = 10000. This is extremely conservative but should not be a problem
	// since the batches are written in parallel.
	return b.applyBulkBatched(ctx, rowNames, mutations, 10000)
}

func (b *btTraceStore) traceRowName(tileKey TileKey, traceKey string) string {
	return ""
}

func (b *btTraceStore) GetTile(ctx context.Context, nCommits int, isSparse bool) (*tiling.Tile, []*tiling.Commit, []int, error) {
	return nil, nil, nil, nil
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

func (b *btTraceStore) updateDigestMap(tileKey TileKey, digests util.StringSet) (*DigestMap, error) {
	return nil, nil
}

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

// getOps returns the OpsCacheEntry for a given tile.
//
// Note that it will create a new OpsCacheEntry if none exists.
//
// getOps returns true if the returned value came from BT, false if it came
// from the cache.
func (b *btTraceStore) getOPS(tileKey TileKey) (*OpsCacheEntry, bool, error) {
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

func (b *btTraceStore) getTable() *bigtable.Table {
	return b.table
}

type DigestMap struct{}

func (d *DigestMap) ID(digest string) (int32, error) {
	return 0, nil
}
