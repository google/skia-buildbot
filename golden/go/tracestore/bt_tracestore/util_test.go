package bt_tracestore

import (
	"math"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/types"
)

// The tests in this package are mainly to make sure changes that
// are not backward-compatible are detected.

func TestTileKeyFromIndex(t *testing.T) {
	unittest.SmallTest(t)

	// spot-check some arbitrary values
	assert.Equal(t, tileKey(2147483647), tileKeyFromIndex(0))
	assert.Equal(t, tileKey(2147483451), tileKeyFromIndex(196))
	assert.Equal(t, tileKey(908536335), tileKeyFromIndex(1238947312))
}

func TestOpsRowName(t *testing.T) {
	unittest.SmallTest(t)

	// spot-check some arbitrary values
	assert.Equal(t, ":ts:o:2147483647:", tileKeyFromIndex(0).OpsRowName())
	assert.Equal(t, ":ts:o:2147483451:", tileKeyFromIndex(196).OpsRowName())
	assert.Equal(t, ":ts:o:0908536335:", tileKeyFromIndex(1238947312).OpsRowName())
}

func TestShardedRowName(t *testing.T) {
	unittest.SmallTest(t)

	shard := int32(3) // arbitrarily picked
	tileZeroKey := tileKey(math.MaxInt32 - 1)
	veryNewTileKey := tileKey(57)

	// Example RowName for a trace
	encodedTrace := ",0=1,1=3,3=0,"
	assert.Equal(t, "03:ts:t:2147483646:,0=1,1=3,3=0,", shardedRowName(shard, typeTrace, tileZeroKey, encodedTrace))
	assert.Equal(t, "03:ts:t:0000000057:,0=1,1=3,3=0,", shardedRowName(shard, typeTrace, veryNewTileKey, encodedTrace))
}

func TestExtractKeyFromRowName(t *testing.T) {
	unittest.SmallTest(t)

	assert.Equal(t, "", extractSubkey(":ts:o:2147483646:"))
	assert.Equal(t, ",0=1,1=3,3=0,", extractSubkey("03:ts:t:2147483646:,0=1,1=3,3=0,"))
}

func TestDigestBytesSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	assert.Equal(t, missingDigestBytes, toBytes(types.MISSING_DIGEST))
	assert.Equal(t, types.MISSING_DIGEST, fromBytes(missingDigestBytes))
	assert.Equal(t, types.MISSING_DIGEST, fromBytes(nil))
	assert.Equal(t, types.MISSING_DIGEST, fromBytes([]byte{}))

	assert.Equal(t, arbitraryDigestBytes, toBytes(arbitraryDigest))
	assert.Equal(t, arbitraryDigest, fromBytes(arbitraryDigestBytes))
}

func TestDigestBytesBadData(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, arbitraryDigestBytes, toBytes(arbitraryDigestCap))

	assert.Equal(t, missingDigestBytes, toBytes(corruptDigest))

	assert.Equal(t, missingDigestBytes, toBytes(truncatedDigest))
	assert.Equal(t, types.MISSING_DIGEST, fromBytes(truncatedDigestBytes))
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
