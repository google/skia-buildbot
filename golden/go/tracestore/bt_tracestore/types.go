package bt_tracestore

import (
	"crypto/md5"
	"fmt"
	"time"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/types"
)

// Constants adapted from btts.go
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
	// hashes. The row will be the first two characters of the hash and the column will
	// be the remaining characters (see b.rowAndColNameFromDigest)
	// All entries will have the following prefix
	digestMapFamilyPrefix = digestMapFamily + ":"

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
	typDigestMap  = "d"
	typeIdCounter = "i"
	typOPS        = "o"
	typTrace      = "t"

	// This is the size of the tile in Big Table. That is, how many commits do we store in one tile.
	// We can have up to 2^32 tiles in big table, so this would let us store 1 trillion
	// commits worth of data. This tile size does not  need to be related to the tile size that
	// Gold operates on (although when tuning, it should be greater than, or an even divisor
	// of the Gold tile size). The first commit in the repo belongs to tile 2^32-1 and tile numbers
	// decrease for newer commits.
	DefaultTileSize = 256

	// Default number of shards used, if not shards provided in BTConfig. A shard splits the traces
	// up on a tile. If a tile exists on shard N in tile A, it will be on shard N for all tiles.
	// Having traces on shards lets BT split up the work more evenly.
	DefaultShards = 32

	// To avoid many increment calls to the id counter cell, we request a number of
	// ids at once. This number is arbitrarily picked and can be increased if need be.
	batchIdRequest = 256

	readTimeout  = 4 * time.Minute
	writeTimeout = 10 * time.Minute

	// BadTileKey is returned in error conditions.
	BadTileKey = TileKey(-1)
)

// List of families (conceptually similar to tables) we are creating in BT.
var btColumnFamilies = []string{
	traceFamily,
	opsFamily,
	idCounterFamily,
	digestMapFamily,
}

// We have one global digest id counter, so just hard-code it to tile 0.
var idCounterRow = rowName(typeIdCounter, 0, "")

// The TileKey type was copied from btts.go

// TileKey is the identifier for each tile held in BigTable.
//
// Note that tile keys are in the opposite order of tile offset, that is, the first commit
// in a repo goes in the first tile, which has key 2^32-1. We do this so more recent
// tiles come first in sort order.
type TileKey int32

// DigestID is an arbitrary number for referring to a types.Digest (string)
// that is stored in a DigestMap.
type DigestID int64

// EncodedTraceId is a shortened form of a tiling.TraceId, e.g. 0=1,1=3,3=0,
// Those indices are references to the OrderedParamSet stored in EncTile.
// See  params.paramsEncoder
type EncodedTraceId string

// EncTile contains an encoded tile.
type EncTile struct {
	// maps a trace id to the list of digest ids. The list corresponds to the commits, with index
	// 0 being the most recent commit.
	traces    map[EncodedTraceId][]DigestID
	ops       *paramtools.OrderedParamSet
	digestMap *DigestMap
}

// When ingesting we keep a cache of the OrderedParamSets we have seen per-tile.
type OpsCacheEntry struct {
	ops  *paramtools.OrderedParamSet
	hash string // md5 has of the serialized ops.
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
	family := row[opsFamily]
	if len(family) != 2 {
		// This should never happen
		return nil, skerr.Fmt("incorrect number of of OPS columns in BT, %d != 2", len(family))
	}
	ops := &paramtools.OrderedParamSet{}
	hash := ""
	for _, col := range family {
		if col.Column == opsFullColName {
			var err error
			ops, err = paramtools.NewOrderedParamSetFromBytes(col.Value)
			if err != nil {
				// should never happen
				return nil, skerr.Fmt("corrupted paramset in BT: %s", err)
			}
		} else if col.Column == hashFullColName {
			hash = string(col.Value)
		}
	}
	if hash == "" {
		return nil, skerr.Fmt("missing hash for OPS %#v", ops)
	}
	// You might be tempted to use opsCacheEntryFromOps and
	// check that entry.hash == hash here, but that will fail
	// because GoB encoding of maps is not deterministic.
	entry := OpsCacheEntry{
		ops:  ops,
		hash: hash,
	}
	return &entry, nil
}

type DigestMap struct {
	intMap map[DigestID]types.Digest
	strMap map[types.Digest]DigestID
}

func NewDigestMap(cap int) *DigestMap {
	ret := &DigestMap{
		intMap: make(map[DigestID]types.Digest, cap),
		strMap: make(map[types.Digest]DigestID, cap),
	}
	ret.intMap[0] = types.MISSING_DIGEST
	ret.strMap[types.MISSING_DIGEST] = 0
	return ret
}

func (d *DigestMap) Delta(digests map[types.Digest]bool) []types.Digest {
	ret := make([]types.Digest, 0, len(digests))
	for digest := range digests {
		if _, ok := d.strMap[digest]; !ok {
			ret = append(ret, digest)
		}
	}
	return ret
}

func (d *DigestMap) Add(newEntries map[types.Digest]DigestID) error {
	for digest, id := range newEntries {
		if digest == types.MISSING_DIGEST || id <= 0 {
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
			return skerr.Fmt("internal inconsistency - input pair (%q -> %d) resolves to (%d -> %q)", digest, foundID, id, foundDigest)
		}

		// New mapping. Add it.
		d.intMap[id] = digest
		d.strMap[digest] = id
	}
	return nil
}

func (d *DigestMap) ID(digest types.Digest) (DigestID, error) {
	ret, ok := d.strMap[digest]
	if !ok {
		return 0, skerr.Fmt("unable to find id for %q", digest)
	}
	return ret, nil
}

func (d *DigestMap) DecodeIDs(ids []DigestID) ([]types.Digest, error) {
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

func (d *DigestMap) Digest(id DigestID) (types.Digest, error) {
	ret, ok := d.intMap[id]
	if !ok {
		return "", skerr.Fmt("unable to find digest for %d", id)
	}
	return ret, nil
}

func (d *DigestMap) Len() int {
	return len(d.strMap)
}
