package bt_tracestore

import (
	"math"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/types"
)

// TestDigestMapMissingDigest verifies a newly created digest map
// starts off with the mapping for a missing digest and nothing else.
func TestDigestMapMissingDigest(t *testing.T) {
	unittest.SmallTest(t)

	digestMap := NewDigestMap(1000)
	assert.Equal(t, 1, digestMap.Len())
	id, err := digestMap.ID(types.MISSING_DIGEST)
	assert.NoError(t, err)
	assert.Equal(t, id, MissingID)

	_, err = digestMap.ID(AlphaDigest)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to find")
}

// TestDigestMapAddGet tests the three ways to get data out of the map.
func TestDigestMapAddGet(t *testing.T) {
	unittest.SmallTest(t)

	digestMap := NewDigestMap(1)
	digestMap.Add(map[types.Digest]DigestID{
		AlphaDigest: 1,
		BetaDigest:  2,
		GammaDigest: 3,
	})
	assert.Equal(t, 4, digestMap.Len())

	digests, err := digestMap.DecodeIDs([]DigestID{GammaID, AlphaID})
	assert.NoError(t, err)
	assert.Equal(t, digests, []types.Digest{GammaDigest, AlphaDigest})

	d, err := digestMap.Digest(BetaID)
	assert.NoError(t, err)
	assert.Equal(t, BetaDigest, d)

	id, err := digestMap.ID(BetaDigest)
	assert.NoError(t, err)
	assert.Equal(t, BetaID, id)
}

// TestDigestMapAddBadGet tests getting things out of the map that don't exist.
func TestDigestMapAddBadGet(t *testing.T) {
	unittest.SmallTest(t)

	notExistID := DigestID(99)
	notExistDigest := types.Digest("fffe39544765f38baab53350aef79966")

	digestMap := NewDigestMap(1)
	digestMap.Add(map[types.Digest]DigestID{
		AlphaDigest: 1,
		BetaDigest:  2,
		GammaDigest: 3,
	})
	_, err := digestMap.DecodeIDs([]DigestID{AlphaID, notExistID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to find id")

	_, err = digestMap.Digest(notExistID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to find digest")

	_, err = digestMap.ID(notExistDigest)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to find id")
}

// TestDigestMapBadAdd tests some corner cases with adding things to the map.
func TestDigestMapBadAdd(t *testing.T) {
	unittest.SmallTest(t)

	digestMap := NewDigestMap(1)
	err := digestMap.Add(map[types.Digest]DigestID{AlphaDigest: AlphaID})
	assert.NoError(t, err)
	err = digestMap.Add(map[types.Digest]DigestID{BetaDigest: BetaID})
	assert.NoError(t, err)

	// Adding something multiple times is no error
	err = digestMap.Add(map[types.Digest]DigestID{BetaDigest: BetaID})
	assert.NoError(t, err)
	err = digestMap.Add(map[types.Digest]DigestID{BetaDigest: BetaID})
	assert.NoError(t, err)

	// Can't add something with MISSING_DIGEST as a key ...
	err = digestMap.Add(map[types.Digest]DigestID{types.MISSING_DIGEST: 5})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input id")
	// ... or as a value.
	err = digestMap.Add(map[types.Digest]DigestID{BetaDigest: MissingID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input id")

	// Can't add something that is already in the map as a key
	err = digestMap.Add(map[types.Digest]DigestID{BetaDigest: GammaID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "internal inconsistency")
	// ... or as a value.
	err = digestMap.Add(map[types.Digest]DigestID{GammaDigest: BetaID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "internal inconsistency")

	// Can't mix up data that has already been seen before
	err = digestMap.Add(map[types.Digest]DigestID{AlphaDigest: BetaID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "inconsistent data")
}

func TestShardedRowName(t *testing.T) {
	unittest.SmallTest(t)

	shard := int32(3) // arbitrarily picked
	tileZeroKey := TileKey(math.MaxInt32 - 1)
	veryNewTileKey := TileKey(57)

	// Example RowName for a trace
	encodedTrace := ",0=1,1=3,3=0,"
	assert.Equal(t, "03:ts:t:2147483646:,0=1,1=3,3=0,", shardedRowName(shard, typTrace, tileZeroKey, encodedTrace))
	assert.Equal(t, "03:ts:t:0000000057:,0=1,1=3,3=0,", shardedRowName(shard, typTrace, veryNewTileKey, encodedTrace))

	// Example RowName for a digest
	// digests are stored in a row based on the first three characters and a
	// column with the remaining characters.
	digestPrefix := string(AlphaDigest[:3])
	assert.Equal(t, "03:ts:d:2147483646:aaa", shardedRowName(shard, typDigestMap, tileZeroKey, digestPrefix))
}

func TestExtractKeyFromRowName(t *testing.T) {
	unittest.SmallTest(t)

	assert.Equal(t, "ae3", extractKey("07:ts:d:2147483646:ae3"))
	assert.Equal(t, "", extractKey(":ts:o:2147483646:"))
	assert.Equal(t, ",0=1,1=3,3=0,", extractKey("03:ts:t:2147483646:,0=1,1=3,3=0,"))
}

const (
	AlphaDigest = types.Digest("aaa6fc936d06e6569788366f1e3fda4e")
	BetaDigest  = types.Digest("bbb15c047d150d961573062854f35a55")
	GammaDigest = types.Digest("cccd42f3ee0b02687f63963adb36a580")

	MissingID = DigestID(0)
	AlphaID   = DigestID(1)
	BetaID    = DigestID(2)
	GammaID   = DigestID(3)
)
