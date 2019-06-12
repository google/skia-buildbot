package bt_tracestore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/types"
)

// TestDigestMapMissingDigest verifies a newly created digest map
// starts off with the mapping for a missing digest and nothing else.
func TestDigestMapMissingDigest(t *testing.T) {
	unittest.SmallTest(t)

	digestMap := newDigestMap(1000)
	assert.Equal(t, 1, digestMap.Len())
	id, err := digestMap.ID(types.MISSING_DIGEST)
	assert.NoError(t, err)
	assert.Equal(t, id, missingDigestID)

	_, err = digestMap.ID(AlphaDigest)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to find")
}

// TestDigestMapAddGet tests the three ways to get data out of the map.
func TestDigestMapAddGet(t *testing.T) {
	unittest.SmallTest(t)

	digestMap := newDigestMap(1)
	err := digestMap.Add(map[types.Digest]digestID{
		AlphaDigest: AlphaID,
		BetaDigest:  BetaID,
		GammaDigest: GammaID,
	})
	assert.NoError(t, err)
	assert.Equal(t, 4, digestMap.Len())

	digests, err := digestMap.DecodeIDs([]digestID{GammaID, AlphaID})
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

	notExistID := digestID(99)
	notExistDigest := types.Digest("fffe39544765f38baab53350aef79966")

	digestMap := newDigestMap(1)
	err := digestMap.Add(map[types.Digest]digestID{
		AlphaDigest: AlphaID,
		BetaDigest:  BetaID,
		GammaDigest: GammaID,
	})
	assert.NoError(t, err)
	_, err = digestMap.DecodeIDs([]digestID{AlphaID, notExistID})
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

	digestMap := newDigestMap(1)
	err := digestMap.Add(map[types.Digest]digestID{AlphaDigest: AlphaID})
	assert.NoError(t, err)
	err = digestMap.Add(map[types.Digest]digestID{BetaDigest: BetaID})
	assert.NoError(t, err)

	// Adding something multiple times is no error
	err = digestMap.Add(map[types.Digest]digestID{BetaDigest: BetaID})
	assert.NoError(t, err)
	err = digestMap.Add(map[types.Digest]digestID{BetaDigest: BetaID})
	assert.NoError(t, err)

	// Can't add something with MISSING_DIGEST as a key ...
	err = digestMap.Add(map[types.Digest]digestID{types.MISSING_DIGEST: 5})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input id")
	// ... or as a value.
	err = digestMap.Add(map[types.Digest]digestID{BetaDigest: missingDigestID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input id")

	// Can't add something that is already in the map as a key
	err = digestMap.Add(map[types.Digest]digestID{BetaDigest: GammaID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "internal inconsistency")
	// ... or as a value.
	err = digestMap.Add(map[types.Digest]digestID{GammaDigest: BetaID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "internal inconsistency")

	// Can't mix up data that has already been seen before
	err = digestMap.Add(map[types.Digest]digestID{AlphaDigest: BetaID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "inconsistent data")
}

func TestTraceMapCommitIndicesWithData(t *testing.T) {
	unittest.SmallTest(t)

	tm := traceMap{
		",key=first,": &types.GoldenTrace{
			Digests: []types.Digest{
				types.MISSING_DIGEST, types.MISSING_DIGEST, AlphaDigest,
				AlphaDigest, types.MISSING_DIGEST, BetaDigest,
			},
		},
		",key=second,": &types.GoldenTrace{
			Digests: []types.Digest{
				GammaDigest, types.MISSING_DIGEST, types.MISSING_DIGEST,
				GammaDigest, types.MISSING_DIGEST, GammaDigest,
			},
		},
	}
	assert.Equal(t, []int{0, 2, 3, 5}, tm.CommitIndicesWithData())

	empty := traceMap{
		",key=first,": &types.GoldenTrace{
			Digests: []types.Digest{
				types.MISSING_DIGEST, types.MISSING_DIGEST, types.MISSING_DIGEST,
			},
		},
		",key=second,": &types.GoldenTrace{
			Digests: []types.Digest{
				types.MISSING_DIGEST, types.MISSING_DIGEST, types.MISSING_DIGEST,
			},
		},
	}
	assert.Nil(t, empty.CommitIndicesWithData())

	assert.Nil(t, traceMap{}.CommitIndicesWithData())
}

func TestTraceMapMakeFromCommitIndexes(t *testing.T) {
	unittest.SmallTest(t)

	tm := traceMap{
		",key=first,": &types.GoldenTrace{
			Digests: []types.Digest{
				types.MISSING_DIGEST, types.MISSING_DIGEST, AlphaDigest,
				AlphaDigest, types.MISSING_DIGEST, BetaDigest,
			},
		},
		",key=second,": &types.GoldenTrace{
			Digests: []types.Digest{
				GammaDigest, types.MISSING_DIGEST, types.MISSING_DIGEST,
				GammaDigest, types.MISSING_DIGEST, GammaDigest,
			},
		},
	}

	assert.Equal(t, traceMap{
		",key=first,": &types.GoldenTrace{
			Digests: []types.Digest{
				types.MISSING_DIGEST, AlphaDigest,
				AlphaDigest, BetaDigest,
			},
		},
		",key=second,": &types.GoldenTrace{
			Digests: []types.Digest{
				GammaDigest, types.MISSING_DIGEST,
				GammaDigest, GammaDigest,
			},
		},
	}, tm.MakeFromCommitIndexes([]int{0, 2, 3, 5}))

	assert.Equal(t, traceMap{
		",key=first,": &types.GoldenTrace{
			Digests: []types.Digest{
				types.MISSING_DIGEST, types.MISSING_DIGEST, types.MISSING_DIGEST,
			},
		},
		",key=second,": &types.GoldenTrace{
			Digests: []types.Digest{
				GammaDigest, types.MISSING_DIGEST, types.MISSING_DIGEST,
			},
		},
	}, tm.MakeFromCommitIndexes([]int{0, 1, 4}))

	assert.Equal(t, traceMap{}, tm.MakeFromCommitIndexes([]int{}))
	assert.Equal(t, traceMap{}, tm.MakeFromCommitIndexes(nil))
}

func TestTraceMapPrependTraces(t *testing.T) {
	unittest.SmallTest(t)

	tm1 := traceMap{
		",key=first,": &types.GoldenTrace{
			Digests: []types.Digest{
				types.MISSING_DIGEST, types.MISSING_DIGEST, AlphaDigest,
			},
		},
		",key=second,": &types.GoldenTrace{
			Digests: []types.Digest{
				GammaDigest, types.MISSING_DIGEST, types.MISSING_DIGEST,
			},
		},
	}

	tm2 := traceMap{
		",key=first,": &types.GoldenTrace{
			Digests: []types.Digest{
				types.MISSING_DIGEST, GammaDigest,
			},
		},
		",key=third,": &types.GoldenTrace{
			Digests: []types.Digest{
				GammaDigest, BetaDigest,
			},
		},
	}

	tm1.PrependTraces(tm2)

	assert.Equal(t, traceMap{
		",key=first,": &types.GoldenTrace{
			Digests: []types.Digest{
				types.MISSING_DIGEST, GammaDigest,
				types.MISSING_DIGEST, types.MISSING_DIGEST, AlphaDigest,
			},
		},
		",key=second,": &types.GoldenTrace{
			Digests: []types.Digest{
				types.MISSING_DIGEST, types.MISSING_DIGEST,
				GammaDigest, types.MISSING_DIGEST, types.MISSING_DIGEST,
			},
		},
		",key=third,": &types.GoldenTrace{
			Digests: []types.Digest{
				GammaDigest, BetaDigest,
				types.MISSING_DIGEST, types.MISSING_DIGEST, types.MISSING_DIGEST,
			},
		},
	}, tm1)
}

const (
	AlphaDigest = types.Digest("aaa6fc936d06e6569788366f1e3fda4e")
	BetaDigest  = types.Digest("bbb15c047d150d961573062854f35a55")
	GammaDigest = types.Digest("cccd42f3ee0b02687f63963adb36a580")

	AlphaID = digestID(1)
	BetaID  = digestID(2)
	GammaID = digestID(3)
)
