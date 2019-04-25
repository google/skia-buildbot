package gtracestore

import (
	"crypto/md5"
	"fmt"
	"hash/crc32"
	"math"
	"strconv"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
)

// TileKey is the identifier for each tile held in BigTable.
//
// Note that tile keys are in the opposite order of tile offset, that is, the first
// tile for a repo would be 0, and then 1, etc. Those are the offsets, and the most
// recent tile has the largest offset. To make it easy to find the most recent
// tile we calculate tilekey as math.MaxInt32 - tileoffset, so that more recent
// tiles come first in sort order.
type TileKey int32

// BadTileKey is returned in error conditions.
const BadTileKey = TileKey(-1)

// TileKeyFromOffset returns a TileKey from the tile offset.
func TileKeyFromOffset(tileOffset int32) TileKey {
	if tileOffset < 0 {
		return BadTileKey
	}
	return TileKey(math.MaxInt32 - tileOffset)
}

func (t TileKey) PrevTile() TileKey {
	return TileKeyFromOffset(t.Offset() - 1)
}

// OpsRowName returns the name of the BigTable row that the OrderedParamSet for this tile is stored at.
func (t TileKey) OpsRowName() string {
	return fmt.Sprintf("@%07d", t)
}

// TraceRowPrefix returns the prefix of a BigTable row name for any Trace in this tile.
func (t TileKey) TraceRowPrefix(shard int32) string {
	return fmt.Sprintf("%d:%07d:", shard, t)
}

// TraceRowPrefix returns the BigTable row name for the given trace, for the given number of shards.
// TraceRowName(",0=1,", 3) -> 2:2147483647:,0=1,
func (t TileKey) TraceRowName(traceId string, shards int32) string {
	return fmt.Sprintf("%d:%07d:%s", crc32.ChecksumIEEE([]byte(traceId))%uint32(shards), t, traceId)
}

// Offset returns the tile offset, i.e. the not-reversed number.
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
