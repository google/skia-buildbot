package bt_tracestore

import (
	"crypto/md5"
	"fmt"
	"time"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

// Constants adapted from btts.go
// See BIGTABLE.md for an overview of how the data is stored in BT.
const (
	// Namespace for this package's data. Due to the fact that there is one table per instance,
	// This makes sure we don't have collisions between traces and something else
	// in BT (i.e. git info)
	traceStoreNameSpace = "ts"

	// Column Families.
	// https://cloud.google.com/bigtable/docs/schema-design#column_families_and_column_qualifiers
	opsFamily   = "O" // ops short for Ordered Param Set
	traceFamily = "T" // Holds "0"..."tilesize-1" columns with a DigestID at each cell

	// Columns in the OrderedParamSet column family.
	opsHashColumn   = "H"
	opsOpsColumn    = "OPS"
	hashFullColName = opsFamily + ":" + opsHashColumn
	opsFullColName  = opsFamily + ":" + opsOpsColumn

	// The columns in the trace family are "0", "1", "2"..."N" where N is
	// the BT tile size (default below). These values correspond to the commitOffset,
	// where 0 is the first (most recent) commit in the tile and N is the last (oldest)
	// commit in the tile.
	// They will all have the following prefix.
	traceFamilyPrefix = traceFamily + ":"

	// Define the row types.
	typeOPS   = "o"
	typeTrace = "t"

	// This is the size of the tile in Big Table. That is, how many commits do we store in one tile.
	// We can have up to 2^32 tiles in big table, so this would let us store 1 trillion
	// commits worth of data. This tile size does not need to be related to the tile size that
	// Gold operates on (although when tuning, it should be greater than, or an even divisor
	// of the Gold tile size). The first commit in the repo belongs to tile 2^32-1 and tile numbers
	// decrease for newer commits.
	DefaultTileSize = 256

	// Default number of shards used. A shard splits the traces up on a tile.
	// If a trace exists on shard N in tile A, it will be on shard N for all tiles.
	// Having traces on shards lets BT split up the work more evenly.
	DefaultShards = 32

	readTimeout  = 4 * time.Minute
	writeTimeout = 10 * time.Minute

	// BadTileKey is returned in error conditions.
	badTileKey = tileKey(-1)
)

var (
	// missingDigestBytes is the sentinel for types.MISSING_DIGEST
	missingDigestBytes = []byte{0}
)

// List of families (conceptually similar to tables) we are creating in BT.
var btColumnFamilies = []string{
	opsFamily,
	traceFamily,
}

// tileKey is the identifier for each tile held in BigTable.
//
// Note that tile keys are in the opposite order of tile offset, that is, the first commit
// in a repo goes in the first tile, which has key 2^32-1. We do this so more recent
// tiles come first in sort order.
type tileKey int32

// encodedTraceID is a shortened form of a tiling.TraceId, e.g. 0=1,1=3,3=0,
// Those indices are references to the OrderedParamSet stored in encTile.
// See params.paramsEncoder
type encodedTraceID string

// encTile contains an encoded tile.
type encTile struct {
	// maps a trace id to the list of digests. The list corresponds to the commits, with index
	// 0 being the oldest commit and the last commit being the most recent.
	traces map[encodedTraceID][]types.Digest
	ops    *paramtools.OrderedParamSet
}

// When ingesting we keep a cache of the OrderedParamSets we have seen per-tile.
type opsCacheEntry struct {
	ops  *paramtools.OrderedParamSet
	hash string // md5 has of the serialized ops - used for deterministic querying.
}

// opsCacheEntryFromOPS creates and fills in an OpsCacheEntry from the given
// OrderedParamSet and sets the hash appropriately.
func opsCacheEntryFromOPS(ops *paramtools.OrderedParamSet) (*opsCacheEntry, error) {
	buf, err := ops.Encode()
	if err != nil {
		return nil, skerr.Fmt("could not encode the given ops to bytes: %s", err)
	}
	hash := fmt.Sprintf("%x", md5.Sum(buf))
	return &opsCacheEntry{
		ops:  ops,
		hash: hash,
	}, nil
}

// newOpsCacheEntry returns an empty OpsCacheEntry.
func newOpsCacheEntry() (*opsCacheEntry, error) {
	return opsCacheEntryFromOPS(paramtools.NewOrderedParamSet())
}

// newOpsCacheEntryFromRow loads the appropriate data from the given BT row
// and returns a OpsCacheEntry with that data.
func newOpsCacheEntryFromRow(row bigtable.Row) (*opsCacheEntry, error) {
	family := row[opsFamily]
	if len(family) != 2 {
		// This should never happen
		return nil, skerr.Fmt("incorrect number of of OPS columns in BT for key %s, %d != 2", row.Key(), len(family))
	}
	ops := &paramtools.OrderedParamSet{}
	hash := ""
	for _, col := range family {
		if col.Column == opsFullColName {
			var err error
			ops, err = paramtools.NewOrderedParamSetFromBytes(col.Value)
			if err != nil {
				// should never happen
				return nil, skerr.Fmt("corrupted paramset in BT for key %s: %s", row.Key(), err)
			}
		} else if col.Column == hashFullColName {
			hash = string(col.Value)
		}
	}
	if hash == "" {
		return nil, skerr.Fmt("missing hash for OPS for key %s: %#v", row.Key(), ops)
	}
	// You might be tempted to use opsCacheEntryFromOps and
	// check that entry.hash == hash here, but that will fail
	// because GoB encoding of maps is not deterministic.
	entry := opsCacheEntry{
		ops:  ops,
		hash: hash,
	}
	return &entry, nil
}

// Define this as a type so we can define some helper functions.
type traceMap map[tiling.TraceId]tiling.Trace

// CommitIndicesWithData returns the indexes of the commits with at least one non-missing
// digest in at least one trace.
func (t traceMap) CommitIndicesWithData() []int {
	if len(t) == 0 {
		return nil
	}
	numCommits := 0
	for _, trace := range t {
		gt := trace.(*types.GoldenTrace)
		numCommits = len(gt.Digests)
		break
	}
	var haveData []int
	for i := 0; i < numCommits; i++ {
		for _, trace := range t {
			gt := trace.(*types.GoldenTrace)
			if !gt.IsMissing(i) {
				haveData = append(haveData, i)
				break
			}
		}
	}
	return haveData
}

// MakeFromCommitIndexes creates a new traceMap from the data in this one that
// only has the digests belonging to the given commit indices. Conceptually,
// this grabs a subset of the commit columns from the tile.
func (t traceMap) MakeFromCommitIndexes(indices []int) traceMap {
	if len(indices) == 0 {
		return traceMap{}
	}
	r := make(traceMap, len(t))
	for id, trace := range t {
		gt := trace.(*types.GoldenTrace)

		newDigests := make([]types.Digest, len(indices))
		for i, idx := range indices {
			newDigests[i] = gt.Digests[idx]
		}

		r[id] = &types.GoldenTrace{
			Keys:    gt.Keys,
			Digests: newDigests,
		}
	}
	return r
}

// PrependTraces augments this traceMap with the data from the given one.
// Specifically, it prepends that data, assuming the "other" data came
// before the data in this map.
// TODO(kjlubick): Deduplicate this with tiling.Merge
func (t traceMap) PrependTraces(other traceMap) {
	numCommits := 0
	for _, trace := range t {
		gt := trace.(*types.GoldenTrace)
		numCommits = len(gt.Digests)
		break
	}

	numOtherCommits := 0
	for id, trace := range other {
		numOtherCommits = trace.Len()
		original, ok := t[id]
		if ok {
			// Keys are constant and are what the id is derived from
			t[id] = trace.Merge(original)
		} else {
			// if we stopped seeing the trace in t, we need to pad the end with MISSING_DIGEST
			trace.Grow(numOtherCommits+numCommits, tiling.FILL_AFTER) // Assumes we can modify other
			t[id] = trace
		}
	}

	// if we saw a trace in t, but not in other, we need to pad the beginning with MISSING_DIGEST
	for id, trace := range t {
		if _, ok := other[id]; !ok {
			trace.Grow(numOtherCommits+numCommits, tiling.FILL_BEFORE)
		}
	}
}
