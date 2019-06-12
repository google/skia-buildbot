package bt_tracestore

import (
	"crypto/md5"
	"encoding"
	"encoding/binary"
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
	digestMapFamily = "D" // Store a mapping of Digest->DigestID
	idCounterFamily = "I" // Keeps a monotonically increasing number to generate DigestIDs
	opsFamily       = "O" // ops short for Ordered Param Set
	traceFamily     = "T" // Holds "0"..."tilesize-1" columns with a DigestID at each cell

	// The columns (and rows) in the digest map family are derived from the digest, which are md5
	// hashes. The row will be the first three characters of the hash and the column will
	// be the remaining characters (see b.rowAndColNameFromDigest)
	// All entries will have the following prefix
	digestMapFamilyPrefix = digestMapFamily + ":"
	// All entries are part of a global map (set tile to 0)
	digestMapTile = tileKey(0)

	// Columns in the ID counter family. There is only one row and one column.
	idCounterColumn = "idc"

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
	typeDigestMap = "d"
	typeIdCounter = "i"
	typeOPS       = "o"
	typeTrace     = "t"

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

	// To avoid many successive increment calls to the id counter cell, we request a number of
	// ids at once. This number is arbitrarily picked and can be increased if need be.
	numReservedIds = 256

	readTimeout  = 4 * time.Minute
	writeTimeout = 10 * time.Minute

	// BadTileKey is returned in error conditions.
	badTileKey = tileKey(-1)

	// missingDigestID is the id for types.MISSING_DIGEST
	missingDigestID = digestID(0)
)

// List of families (conceptually similar to tables) we are creating in BT.
var btColumnFamilies = []string{
	traceFamily,
	opsFamily,
	idCounterFamily,
	digestMapFamily,
}

// We have one global digest id counter, so just hard-code it to tile 0.
var idCounterRow = unshardedRowName(typeIdCounter, 0)

// tileKey is the identifier for each tile held in BigTable.
//
// Note that tile keys are in the opposite order of tile offset, that is, the first commit
// in a repo goes in the first tile, which has key 2^32-1. We do this so more recent
// tiles come first in sort order.
type tileKey int32

// digestID is an arbitrary number for referring to a types.Digest (string)
// that is stored in a digestMap.
type digestID uint64

// MarshalBinary implements the encoding.BinaryMarshaler interface, allowing for
// us to compactly store these to BT (about 50% space savings over string representation).
func (d digestID) MarshalBinary() ([]byte, error) {
	rv := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(rv, uint64(d))
	return rv[:n], nil
}

var _ encoding.BinaryMarshaler = digestID(0)

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface, allowing for
// us to compactly store these to BT (about 50% space savings over string representation).
func (d *digestID) UnmarshalBinary(data []byte) error {
	val, n := binary.Uvarint(data)
	if n != len(data) {
		return skerr.Fmt("Error decoding digestID from %x; Uvarint consumed %d bytes instead of %d", data, n, len(data))
	}
	*d = digestID(val)
	return nil
}

var _ encoding.BinaryUnmarshaler = (*digestID)(nil)

// encodedTraceID is a shortened form of a tiling.TraceId, e.g. 0=1,1=3,3=0,
// Those indices are references to the OrderedParamSet stored in encTile.
// See params.paramsEncoder
type encodedTraceID string

// encTile contains an encoded tile.
type encTile struct {
	// maps a trace id to the list of digest ids. The list corresponds to the commits, with index
	// 0 being the oldest commit and the last commit being the most recent.
	traces map[encodedTraceID][]digestID
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

// A digestMap keeps track of the mapping between encoded digestID and their corresponding
// types.Digest.
type digestMap struct {
	intMap map[digestID]types.Digest
	strMap map[types.Digest]digestID
}

// newDigestMap creates an empty digestMap with the given capacity.
func newDigestMap(cap int) *digestMap {
	ret := &digestMap{
		intMap: make(map[digestID]types.Digest, cap),
		strMap: make(map[types.Digest]digestID, cap),
	}
	ret.intMap[missingDigestID] = types.MISSING_DIGEST
	ret.strMap[types.MISSING_DIGEST] = missingDigestID
	return ret
}

// Delta returns a []types.Digest of those passed in digests that are
// not in this mapping currently.
func (d *digestMap) Delta(digests map[types.Digest]bool) []types.Digest {
	ret := make([]types.Digest, 0, len(digests))
	for digest := range digests {
		if _, ok := d.strMap[digest]; !ok {
			ret = append(ret, digest)
		}
	}
	return ret
}

// Add expands the map with the given entries. It fails if any key or any value
// is already in the mapping.
func (d *digestMap) Add(newEntries map[types.Digest]digestID) error {
	for digest, id := range newEntries {
		if digest == types.MISSING_DIGEST || id == 0 {
			return skerr.Fmt("invalid input id or digest: (%q -> %d)", digest, id)
		}

		foundID, strExists := d.strMap[digest]
		foundDigest, intExists := d.intMap[id]
		if strExists && intExists {
			if (foundID != id) || (foundDigest != digest) {
				return skerr.Fmt("inconsistent data - got (%q -> %d) when (%q -> %d) was expected", digest, id, foundDigest, foundID)
			}
			// Already contained so this is a no-op.
			return nil
		}

		if strExists || intExists {
			return skerr.Fmt("internal inconsistency - expected forward mapping (%q -> %d) and reverse mapping (%d -> %q) to both be present", digest, foundID, id, foundDigest)
		}

		// New mapping. Add it.
		d.intMap[id] = digest
		d.strMap[digest] = id
	}
	return nil
}

// ID returns the DigestID for a given types.Digest.
func (d *digestMap) ID(digest types.Digest) (digestID, error) {
	ret, ok := d.strMap[digest]
	if !ok {
		return 0, skerr.Fmt("unable to find id for %q", digest)
	}
	return ret, nil
}

// DecodeIDs is like Digest but in bulk.
func (d *digestMap) DecodeIDs(ids []digestID) ([]types.Digest, error) {
	ret := make([]types.Digest, len(ids))
	var ok bool
	for idx, id := range ids {
		ret[idx], ok = d.intMap[id]
		if !ok {
			return nil, skerr.Fmt("unable to find id %d in intMap", id)
		}
	}
	return ret, nil
}

// Digest returns the types.Digest for a given DigestID.
func (d *digestMap) Digest(id digestID) (types.Digest, error) {
	ret, ok := d.intMap[id]
	if !ok {
		return "", skerr.Fmt("unable to find digest for %d", id)
	}
	return ret, nil
}

// Len returns how many map entries are in this map.
func (d *digestMap) Len() int {
	return len(d.strMap)
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
