package bt_tracestore

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

// The tests in this package are mainly to make sure changes that
// are not backward-compatible are detected.

func TestTileKeyFromIndex(t *testing.T) {
	unittest.SmallTest(t)

	// spot-check some arbitrary values
	require.Equal(t, TileKey(2147483647), tileKeyFromIndex(0))
	require.Equal(t, TileKey(2147483451), tileKeyFromIndex(196))
	require.Equal(t, TileKey(908536335), tileKeyFromIndex(1238947312))
}

func TestOpsRowName(t *testing.T) {
	unittest.SmallTest(t)

	// spot-check some arbitrary values
	require.Equal(t, ":ts:o:2147483647:", tileKeyFromIndex(0).OpsRowName())
	require.Equal(t, ":ts:o:2147483451:", tileKeyFromIndex(196).OpsRowName())
	require.Equal(t, ":ts:o:0908536335:", tileKeyFromIndex(1238947312).OpsRowName())
}

func TestShardedRowName(t *testing.T) {
	unittest.SmallTest(t)

	shard := int32(3) // arbitrarily picked
	tileZeroKey := TileKey(math.MaxInt32 - 1)
	veryNewTileKey := TileKey(57)

	// Example RowName for a trace
	encodedTrace := ",0=1,1=3,3=0,"
	require.Equal(t, "03:ts:t:2147483646:,0=1,1=3,3=0,", shardedRowName(shard, typeTrace, tileZeroKey, encodedTrace))
	require.Equal(t, "03:ts:t:0000000057:,0=1,1=3,3=0,", shardedRowName(shard, typeTrace, veryNewTileKey, encodedTrace))
}

func TestExtractKeyFromRowName(t *testing.T) {
	unittest.SmallTest(t)

	require.Equal(t, "", extractSubkey(":ts:o:2147483646:"))
	require.Equal(t, ",0=1,1=3,3=0,", extractSubkey("03:ts:t:2147483646:,0=1,1=3,3=0,"))
}

func TestDigestBytesSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	require.Equal(t, missingDigestBytes, toBytes(tiling.MissingDigest))
	require.Equal(t, tiling.MissingDigest, fromBytes(missingDigestBytes))
	require.Equal(t, tiling.MissingDigest, fromBytes(nil))
	require.Equal(t, tiling.MissingDigest, fromBytes([]byte{}))

	require.Equal(t, arbitraryDigestBytes, toBytes(arbitraryDigest))
	require.Equal(t, arbitraryDigest, fromBytes(arbitraryDigestBytes))
}

func TestDigestBytesBadData(t *testing.T) {
	unittest.SmallTest(t)
	require.Equal(t, arbitraryDigestBytes, toBytes(arbitraryDigestCap))

	require.Equal(t, missingDigestBytes, toBytes(corruptDigest))

	require.Equal(t, missingDigestBytes, toBytes(truncatedDigest))
	require.Equal(t, tiling.MissingDigest, fromBytes(truncatedDigestBytes))
}

const (
	arbitraryDigest    = types.Digest("8db9913366b07bd14df9a68459e2958b")
	arbitraryDigestCap = types.Digest("8DB9913366B07BD14DF9A68459E2958B")
	corruptDigest      = types.Digest("NOPE")
	truncatedDigest    = types.Digest("8db9913366b07b")
)

var (
	arbitraryDigestBytes = []byte{0x8d, 0xb9, 0x91, 0x33, 0x66, 0xb0, 0x7b, 0xd1,
		0x4d, 0xf9, 0xa6, 0x84, 0x59, 0xe2, 0x95, 0x8b}

	truncatedDigestBytes = []byte{0x8d, 0xb9, 0x91, 0x33, 0x66, 0xb0, 0x7b}
)
