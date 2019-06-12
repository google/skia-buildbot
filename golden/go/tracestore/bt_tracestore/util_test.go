package bt_tracestore

import (
	"math"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
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

	// Example RowName for a digest
	// digests are stored in a row based on the first three characters and a
	// column with the remaining characters.
	digestPrefix := string(AlphaDigest[:3])
	assert.Equal(t, "03:ts:d:2147483646:aaa", shardedRowName(shard, typeDigestMap, tileZeroKey, digestPrefix))
}

func TestExtractKeyFromRowName(t *testing.T) {
	unittest.SmallTest(t)

	assert.Equal(t, "ae3", extractSubkey("07:ts:d:2147483646:ae3"))
	assert.Equal(t, "", extractSubkey(":ts:o:2147483646:"))
	assert.Equal(t, ",0=1,1=3,3=0,", extractSubkey("03:ts:t:2147483646:,0=1,1=3,3=0,"))
}
