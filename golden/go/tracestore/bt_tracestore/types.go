package bt_tracestore

import (
	"crypto/md5"
	"fmt"
	"runtime"
	"sync"
	"time"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/tiling"
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
	opsFamily     = "O" // ops short for OrderedParamSet
	optionsFamily = "P" // oPtions map for a trace
	traceFamily   = "T" // Holds "0"..."tilesize-1" columns with a DigestID at each cell

	// Columns in the OrderedParamSet column family.
	opsHashColumn   = "H"
	opsOpsColumn    = "OPS"
	hashFullColName = opsFamily + ":" + opsHashColumn
	opsFullColName  = opsFamily + ":" + opsOpsColumn

	// The only column in the optionsFamily - used to store the param map encoded
	// liked a trace id: `,key1=value1,`
	optionsBytesColumn = "B"

	// The columns in the trace family are "0", "1", "2"..."N" where N is
	// the BT tile size (default below). These values correspond to the commitOffset,
	// where 0 is the first (most recent) commit in the tile and N is the last (oldest)
	// commit in the tile.
	// They will all have the following prefix.
	traceFamilyPrefix = traceFamily + ":"

	// Define the row types.
	typeOPS     = "o"
	typeOptions = "p"
	typeTrace   = "t"

	// We pad the columns so they are properly sorted lexicographically.
	columnPad = "%03d"

	// This is the size of the tile in Big Table. That is, how many commits do we store in one tile.
	// We can have up to 2^32 tiles in big table, so this would let us store 1 trillion
	// commits worth of data. This tile size does not need to be related to the tile size that
	// Gold operates on (although when tuning, it should be greater than, or an even divisor
	// of the Gold tile size). The first commit in the repo belongs to tile 2^32-1 and tile numbers
	// decrease for newer commits. The columnPad const also depends on the number of digits of
	// DefaultTileSize.
	DefaultTileSize = 256

	// Default number of shards used. A shard splits the traces up on a tile.
	// If a trace exists on shard N in tile A, it will be on shard N for all tiles.
	// Having traces on shards lets BT split up the work more evenly.
	DefaultShards = 32

	readTimeout  = 4 * time.Minute
	writeTimeout = 10 * time.Minute

	maxTilesForDenseTile = 50

	// BadTileKey is returned in error conditions.
	badTileKey = TileKey(-1)
)

var (
	// missingDigestBytes is the sentinel for types.MissingDigest
	missingDigestBytes = []byte("")
)

// TileKey is the identifier for each tile held in BigTable.
//
// Note that tile keys are in the opposite order of tile offset, that is, the first commit
// in a repo goes in the first tile, which has key 2^32-1. We do this so more recent
// tiles come first in sort order.
type TileKey int32

// encodedTraceID is a shortened form of a tiling.TraceID, e.g. 0=1,1=3,3=0,
// Those indices are references to the OrderedParamSet stored in encTile.
// See params.paramsEncoder
type encodedTraceID string

// encTile contains an encoded tile.
type encTile struct {
	// This being a slice is more performant than a map, since we only really
	// needed to iterate over the data structure, not look anything up by trace.
	traces []*encodedTracePair
	ops    *paramtools.OrderedParamSet
}

// maps a trace id to the list of digests.
type encodedTracePair struct {
	ID encodedTraceID
	// This corresponds to the commits, with index 0 being the oldest commit
	// and the last commit being the most recent.
	Digests [DefaultTileSize]types.Digest
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
type traceMap map[tiling.TraceID]*tiling.Trace

// CommitIndicesWithData returns the indexes of the commits with at least one non-missing
// digest in at least one trace. Since the traces always have DefaultTraceSize commits
// and might not be fully filled in, maxIndex tell us where to cut off the search so as
// to avoid unneeded work.
func (t traceMap) CommitIndicesWithData(maxIndex int) []int {
	if len(t) == 0 {
		return nil
	}
	nCommits := 0
	for _, trace := range t {
		nCommits = trace.Len()
		break
	}
	// This is pretty expensive, as in the worst case, it will have to go through all digests of
	// all traces.
	// One optimization is: break the work up into chunks and run each chunk on a
	// goroutine. That way all the CPUs can get involved searching every trace (if needed).
	// Finding the best params for breaking the data up is potentially tricky, so right now we do
	// the naive thing and break it up based on the number of CPUs. On a 4 core laptop, this improved
	// the sparse case by 2x and the dense case by 1.5x over the naive implementation.
	chunkSize := maxIndex / runtime.NumCPU()
	// Prevent integer division from making this 0 (or other small numbers where the overhead
	// may not be worth it).
	if chunkSize < 4 {
		chunkSize = 4
	}
	wg := sync.WaitGroup{}
	// store data to a slice of bools so we can safely share it between the goroutines without
	// a mutex (which had some contention problems in the dense case).
	haveData := make([]bool, maxIndex)
	for i := 0; i < maxIndex; i += chunkSize {
		wg.Add(1)
		go func(start int) {
			defer wg.Done()
			for i := start; i < start+chunkSize && i < maxIndex && i < nCommits; i++ {
				for _, trace := range t {
					if !trace.IsMissing(i) {
						haveData[i] = true
						break
					}
				}
			}
		}(i)
	}
	wg.Wait()
	var indices []int
	for i, b := range haveData {
		if b {
			indices = append(indices, i)
		}
	}
	return indices
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

		newDigests := make([]types.Digest, len(indices))
		for i, idx := range indices {
			newDigests[i] = trace.Digests[idx]
		}

		r[id] = tiling.NewTrace(newDigests, trace.Keys())
	}
	return r
}

// PrependTraces augments this traceMap with the data from the given one.
// Specifically, it prepends that data, assuming the "other" data came
// before the data in this map.
func (t traceMap) PrependTraces(other traceMap) {
	numCommits := 0
	for _, trace := range t {
		numCommits = trace.Len()
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
			// if we stopped seeing the trace in t, we need to pad the end with MissingDigest
			trace.Grow(numOtherCommits+numCommits, tiling.FillAfter) // Assumes we can modify other
			t[id] = trace
		}
	}

	// if we saw a trace in t, but not in other, we need to pad the beginning with MissingDigest
	for id, trace := range t {
		if _, ok := other[id]; !ok {
			trace.Grow(numOtherCommits+numCommits, tiling.FillBefore)
		}
	}
}

// Most of the strings aren't unique. For example, in a single, stable trace,
// the same digest may be used for all commits in the row. Thus, we don't want
// to have to allocate memory on the heap for each of those strings, we can
// just reuse those (immutable) strings.
// THIS IS NOT SAFE to be shared between goroutines.
type digestCache map[[md5.Size]byte]types.Digest

func (c digestCache) FromBytesOrCache(b []byte) types.Digest {
	if len(b) != md5.Size {
		return tiling.MissingDigest
	}
	// Allocate a small array on the stack, then copy the bytes
	// into it and use that as the key in the map.
	// This is faster than k := string(b), maybe because of
	// extra copies or simply that runtime.mapaccess2_faststr is slower
	// than runtime.mapaccess2 (used by array).
	k := [md5.Size]byte{}
	copy(k[:], b)
	d, ok := c[k]
	if ok {
		return d
	}
	d = fromBytes(k[:])
	c[k] = d
	return d
}

// Most of the options aren't unique. For example, in a single tile,
// the same options can be shared across most traces with the same name.
// THIS IS NOT SAFE to be shared between goroutines.
type paramCache map[string]paramtools.Params

func (c paramCache) FromBytesOrCache(b []byte) paramtools.Params {
	if len(b) == 0 {
		return paramtools.Params{}
	}
	k := string(b)
	p, ok := c[k]
	if ok {
		return p
	}
	p = decodeParams(k)
	c[k] = p
	return p
}
