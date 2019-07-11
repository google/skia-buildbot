package bt_tracestore

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math"
	"strings"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/tracestore"
	"go.skia.org/infra/golden/go/types"
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

// toBytes turns a Digest into the bytes that will be stored in the table.
func toBytes(d types.Digest) []byte {
	if d == types.MISSING_DIGEST {
		return missingDigestBytes
	}
	b, err := hex.DecodeString(string(d))
	if err != nil || len(b) != md5.Size {
		sklog.Errorf("Invalid digest %q: %v", d, err)
		return missingDigestBytes
	}
	return b
}

// fromBytes does the opposite of toBytes.
func fromBytes(b []byte) types.Digest {
	// Be extra cautious - if we don't have enough bytes for an md5 hash,
	// just assume it's corrupted or something and say missing.
	if len(b) != md5.Size {
		if len(b) > len(missingDigestBytes) {
			sklog.Warningf("Possibly corrupt data: %#v", b)
		}
		return types.MISSING_DIGEST
	}
	return types.Digest(hex.EncodeToString(b))
}

// encodeParams encodes params to bytes. Specifically, it encodes them
// like a traceID
func encodeParams(p map[string]string) []byte {
	id := tracestore.TraceIDFromParams(p)
	return []byte(id)
}

// decodeParams decodes bytes to Params.
func decodeParams(b []byte) paramtools.Params {
	if len(b) == 0 {
		return paramtools.Params{}
	}
	p, err := query.ParseKeyFast(string(b))
	if err != nil {
		sklog.Errorf("Invalid params bytes %s: %s", string(b), err)
		return paramtools.Params{}
	}
	return p
}
