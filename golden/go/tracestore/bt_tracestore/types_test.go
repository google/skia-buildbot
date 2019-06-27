package bt_tracestore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/types"
)

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
)
