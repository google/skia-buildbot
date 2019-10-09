package bt_tracestore

import (
	"crypto/md5"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
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
	require.Equal(t, []int{0, 2, 3, 5}, tm.CommitIndicesWithData(10))

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
	require.Empty(t, empty.CommitIndicesWithData(10))

	require.Empty(t, traceMap{}.CommitIndicesWithData(10))
}

func TestTraceMapCommitIndicesWithDataTricky(t *testing.T) {
	unittest.SmallTest(t)

	for i := 0; i < 100; i++ {
		tm := traceMap{
			",key=first,": &types.GoldenTrace{
				Digests: []types.Digest{
					AlphaDigest, types.MISSING_DIGEST,
				},
			},
			",key=second,": &types.GoldenTrace{
				Digests: []types.Digest{
					types.MISSING_DIGEST, GammaDigest,
				},
			},
			",key=third,": &types.GoldenTrace{
				Digests: []types.Digest{
					AlphaDigest, types.MISSING_DIGEST,
				},
			},
		}
		require.Equal(t, []int{0, 1}, tm.CommitIndicesWithData(10))
	}
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

	require.Equal(t, traceMap{
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

	require.Equal(t, traceMap{
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

	require.Equal(t, traceMap{}, tm.MakeFromCommitIndexes([]int{}))
	require.Equal(t, traceMap{}, tm.MakeFromCommitIndexes(nil))
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

	require.Equal(t, traceMap{
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

var benchResult []int

func BenchmarkCommitIndicesWithDataDense(b *testing.B) {
	tm := makeRandomTraceMap(0.9)
	b.ResetTimer()
	var r []int
	for n := 0; n < b.N; n++ {
		r = tm.CommitIndicesWithData(numCommits)
	}
	benchResult = r
}

func BenchmarkCommitIndicesWithDataSparse(b *testing.B) {
	tm := makeRandomTraceMap(0.1)
	b.ResetTimer()
	var r []int
	for n := 0; n < b.N; n++ {
		r = tm.CommitIndicesWithData(numCommits)
	}
	benchResult = r
}

const numTraces = 10000
const numCommits = 500

func makeRandomTraceMap(density float32) traceMap {
	rand.Seed(0)
	commitsWithData := make([]bool, numCommits)
	for i := 0; i < numCommits; i++ {
		if rand.Float32() < density {
			commitsWithData[i] = true
		}
	}
	tm := make(traceMap, numTraces)
	for i := 0; i < numTraces; i++ {
		traceID := tiling.TraceId(randomDigest()) // any string should do
		gt := &types.GoldenTrace{
			// Keys can be blank for the CommitIndicesWithData bench
			Digests: make([]types.Digest, numCommits),
		}
		for j := 0; j < numCommits; j++ {
			if commitsWithData[j] && rand.Float32() < density {
				gt.Digests[j] = randomDigest()
			} else {
				gt.Digests[j] = types.MISSING_DIGEST
			}
		}
		tm[traceID] = gt
	}
	return tm
}

func randomDigest() types.Digest {
	b := make([]byte, md5.Size)
	_, _ = rand.Read(b)
	return fromBytes(b)
}
