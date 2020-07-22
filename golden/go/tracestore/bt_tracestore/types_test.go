package bt_tracestore

import (
	"crypto/md5"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

func TestTraceMapCommitIndicesWithData(t *testing.T) {
	unittest.SmallTest(t)

	tm := traceMap{
		",key=first,": &tiling.Trace{
			Digests: []types.Digest{
				tiling.MissingDigest, tiling.MissingDigest, AlphaDigest,
				AlphaDigest, tiling.MissingDigest, BetaDigest,
			},
		},
		",key=second,": &tiling.Trace{
			Digests: []types.Digest{
				GammaDigest, tiling.MissingDigest, tiling.MissingDigest,
				GammaDigest, tiling.MissingDigest, GammaDigest,
			},
		},
	}
	require.Equal(t, []int{0, 2, 3, 5}, tm.CommitIndicesWithData(10))

	empty := traceMap{
		",key=first,": &tiling.Trace{
			Digests: []types.Digest{
				tiling.MissingDigest, tiling.MissingDigest, tiling.MissingDigest,
			},
		},
		",key=second,": &tiling.Trace{
			Digests: []types.Digest{
				tiling.MissingDigest, tiling.MissingDigest, tiling.MissingDigest,
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
			",key=first,": &tiling.Trace{
				Digests: []types.Digest{
					AlphaDigest, tiling.MissingDigest,
				},
			},
			",key=second,": &tiling.Trace{
				Digests: []types.Digest{
					tiling.MissingDigest, GammaDigest,
				},
			},
			",key=third,": &tiling.Trace{
				Digests: []types.Digest{
					AlphaDigest, tiling.MissingDigest,
				},
			},
		}
		require.Equal(t, []int{0, 1}, tm.CommitIndicesWithData(10))
	}
}

func TestTraceMapMakeFromCommitIndexes(t *testing.T) {
	unittest.SmallTest(t)

	tm := traceMap{
		",key=first,": &tiling.Trace{
			Digests: []types.Digest{
				tiling.MissingDigest, tiling.MissingDigest, AlphaDigest,
				AlphaDigest, tiling.MissingDigest, BetaDigest,
			},
		},
		",key=second,": &tiling.Trace{
			Digests: []types.Digest{
				GammaDigest, tiling.MissingDigest, tiling.MissingDigest,
				GammaDigest, tiling.MissingDigest, GammaDigest,
			},
		},
	}

	require.Equal(t, traceMap{
		",key=first,": &tiling.Trace{
			Digests: []types.Digest{
				tiling.MissingDigest, AlphaDigest,
				AlphaDigest, BetaDigest,
			},
		},
		",key=second,": &tiling.Trace{
			Digests: []types.Digest{
				GammaDigest, tiling.MissingDigest,
				GammaDigest, GammaDigest,
			},
		},
	}, tm.MakeFromCommitIndexes([]int{0, 2, 3, 5}))

	require.Equal(t, traceMap{
		",key=first,": &tiling.Trace{
			Digests: []types.Digest{
				tiling.MissingDigest, tiling.MissingDigest, tiling.MissingDigest,
			},
		},
		",key=second,": &tiling.Trace{
			Digests: []types.Digest{
				GammaDigest, tiling.MissingDigest, tiling.MissingDigest,
			},
		},
	}, tm.MakeFromCommitIndexes([]int{0, 1, 4}))

	require.Equal(t, traceMap{}, tm.MakeFromCommitIndexes([]int{}))
	require.Equal(t, traceMap{}, tm.MakeFromCommitIndexes(nil))
}

func TestTraceMapPrependTraces(t *testing.T) {
	unittest.SmallTest(t)

	tm1 := traceMap{
		",key=first,": tiling.NewTrace(
			[]types.Digest{tiling.MissingDigest, tiling.MissingDigest, AlphaDigest},
			map[string]string{"key": "first"},
			map[string]string{"opt": "foo"}),
		",key=second,": tiling.NewTrace(
			[]types.Digest{GammaDigest, tiling.MissingDigest, tiling.MissingDigest},
			map[string]string{"key": "second"},
			map[string]string{"opt": "foo"}),
	}

	tm2 := traceMap{
		",key=first,": tiling.NewTrace(
			[]types.Digest{tiling.MissingDigest, GammaDigest},
			map[string]string{"key": "first"},
			map[string]string{"opt": "bar"}),
		",key=third,": tiling.NewTrace(
			[]types.Digest{GammaDigest, BetaDigest},
			map[string]string{"key": "third"},
			map[string]string{"opt": "bar"}),
	}

	tm1.PrependTraces(tm2)

	require.Equal(t, traceMap{
		",key=first,": tiling.NewTrace(
			[]types.Digest{tiling.MissingDigest, GammaDigest, tiling.MissingDigest, tiling.MissingDigest, AlphaDigest},
			map[string]string{"key": "first"},
			map[string]string{"opt": "foo"}),
		",key=second,": tiling.NewTrace(
			[]types.Digest{tiling.MissingDigest, tiling.MissingDigest, GammaDigest, tiling.MissingDigest, tiling.MissingDigest},
			map[string]string{"key": "second"},
			map[string]string{"opt": "foo"}),
		",key=third,": tiling.NewTrace(
			[]types.Digest{GammaDigest, BetaDigest, tiling.MissingDigest, tiling.MissingDigest, tiling.MissingDigest},
			map[string]string{"key": "third"},
			map[string]string{"opt": "bar"}),
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
		traceID := tiling.TraceID(randomDigest()) // any string should do
		gt := &tiling.Trace{
			// Keys can be blank for the CommitIndicesWithData bench
			Digests: make([]types.Digest, numCommits),
		}
		for j := 0; j < numCommits; j++ {
			if commitsWithData[j] && rand.Float32() < density {
				gt.Digests[j] = randomDigest()
			} else {
				gt.Digests[j] = tiling.MissingDigest
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
