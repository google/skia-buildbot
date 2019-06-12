package bt_tracestore

import (
	"fmt"
	"math"
	"strings"
)

// tileKeyFromIndex converts the tile index to the tileKey.
// See BIGTABLE.md for more on this conversion.
func tileKeyFromIndex(tileIndex int32) tileKey {
	if tileIndex < 0 {
		return badTileKey
	}
	return tileKey(math.MaxInt32 - tileIndex)
}

// OpsRowName returns the name of the BigTable row which stores the OrderedParamSet
// for this tile.
func (t tileKey) OpsRowName() string {
	return unshardedRowName(typeOPS, t)
}

// unshardedRowName calculates the row for the given data which all has the same format:
// :[namespace]:[type]:[tile]:
func unshardedRowName(rowType string, tileKey tileKey) string {
	return fmt.Sprintf(":%s:%s:%010d:", traceStoreNameSpace, rowType, tileKey)
}

// shardedRowName calculates the row for the given data which all has the same format:
// [shard]:[namespace]:[type]:[tile]:[subkey]
// For some data types, where there is only one row,  or when doing a prefix-match,
// subkey may be "".
func shardedRowName(shard int32, rowType string, tileKey tileKey, subkey string) string {
	return fmt.Sprintf("%02d:%s:%s:%010d:%s", shard, traceStoreNameSpace, rowType, tileKey, subkey)
}

// extractKey returns the subkey from the given row name. This could be "".
func extractSubkey(rowName string) string {
	parts := strings.Split(rowName, ":")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
