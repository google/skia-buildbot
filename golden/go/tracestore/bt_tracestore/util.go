package bt_tracestore

import (
	"fmt"
	"math"
	"strings"

	"go.skia.org/infra/golden/go/types"
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

// OpsRowName returns the name of the BigTable row that the OrderedParamSet for this tile
// is stored at.
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

// To avoid having one monolithic row, we take the first character of the digest
// and use it as a key in the row. Then, what remains is used as the column name.
// In practice this means our digests will be split using 1 hexadecimal characters, so
// we will have 16 rows for our digest map. See digestPrefixes.
func rowAndColNameFromDigest(tileKey TileKey, digest types.Digest) (string, string) {
	key := string(digest[:1])
	colName := string(digest[1:])
	return rowName(typDigestMap, tileKey, key), colName
}

// digestPrefixes is a list of all possible prefixes from rowAndColNameFromDigest.
var digestPrefixes = [16]string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c", "d", "e", "f"}

// extractKey returns the key from the given row name. This could be "".
func extractKey(rowName string) string {
	parts := strings.Split(rowName, ":")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
