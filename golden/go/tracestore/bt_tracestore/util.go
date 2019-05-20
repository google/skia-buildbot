package bt_tracestore

import (
	"fmt"
	"math"
	"strings"
)

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

// OpsRowName returns the name of the BigTable row which stores the OrderedParamSet for this tile.
func (t TileKey) OpsRowName() string {
	return rowName(typOPS, t, "")
}

// Offset returns the tile offset, i.e. the not-reversed number.
func (t TileKey) Offset() int32 {
	return math.MaxInt32 - int32(t)
}

// rowName calculates the row for the given data which all has the same format:
// :[namespace]:[type]:[tile]:[key]
// For some data types, where there is only one row, or when doing a prefix-match,
// key may be "".
func rowName(rowType string, tileKey TileKey, key string) string {
	return fmt.Sprintf(":%s:%s:%010d:%s", traceStoreNameSpace, rowType, tileKey, key)
}

// shardedRowName calculates the row for the given data which all has the same format:
// [shard]:[namespace]:[type]:[tile]:[key]
// For some data types, where there is only one row, key may be "".
func shardedRowName(shard int32, rowType string, tileKey TileKey, key string) string {
	return fmt.Sprintf("%02d%s", shard, rowName(rowType, tileKey, key))
}

// extractKey returns the key from the given row name. This could be "".
func extractKey(rowName string) string {
	parts := strings.Split(rowName, ":")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
