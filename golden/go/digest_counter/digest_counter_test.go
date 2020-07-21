package digest_counter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

func TestDigestCountNew(t *testing.T) {
	unittest.SmallTest(t)
	tile := makePartialTileOne()

	dc := New(tile)

	require.Equal(t, map[tiling.TraceID]DigestCount{
		x86TestAlphaTraceID: {
			// FirstDigest showed up twice for this test+config and SecondDigest only once.
			FirstDigest:  2,
			SecondDigest: 1,
		},
		x64TestAlphaTraceID: {
			// FirstDigest and ThirdDigest showed up once each for this test+config.
			FirstDigest: 1,
			ThirdDigest: 1,
		},
	}, dc.ByTrace())

	require.Equal(t, map[types.TestName]DigestCount{
		AlphaTest: {
			// AlphaTest was the only test, so these are the counts for both configs.
			FirstDigest:  3,
			SecondDigest: 1,
			ThirdDigest:  1,
		},
	}, dc.ByTest())

	require.Equal(t, map[types.TestName]types.DigestSet{
		AlphaTest: {
			// AlphaTest had the most of any digest in this test (see above)
			FirstDigest: true,
		},
	}, dc.MaxDigestsByTest())
}

// Check that counts and byTest work with ties and multiple tests
func TestDigestCountTies(t *testing.T) {
	unittest.SmallTest(t)
	tile := makePartialTileTwo()

	dc := New(tile)

	require.Equal(t, map[types.TestName]types.DigestSet{
		AlphaTest: {
			FirstDigest:  true,
			SecondDigest: true,
		},
		BetaTest: {
			FirstDigest: true,
			ThirdDigest: true,
		},
	}, dc.MaxDigestsByTest())

	require.Equal(t, map[types.TestName]DigestCount{
		AlphaTest: {
			FirstDigest:  2,
			SecondDigest: 2,
		},
		BetaTest: {
			FirstDigest: 2,
			ThirdDigest: 2,
		},
	}, dc.ByTest())
}

func TestDigestCountByQuery(t *testing.T) {
	unittest.SmallTest(t)
	tile := makePartialTileOne()

	dc := New(tile)

	bq := dc.ByQuery(tile, paramtools.ParamSet{
		types.CorpusField: []string{"gm"},
	})

	require.Equal(t, DigestCount{
		FirstDigest:  2,
		SecondDigest: 1,
	}, bq)

	bq = dc.ByQuery(tile, paramtools.ParamSet{
		types.CorpusField: []string{"image"},
	})

	require.Equal(t, DigestCount{
		FirstDigest: 1,
		ThirdDigest: 1,
	}, bq)

	bq = dc.ByQuery(tile, paramtools.ParamSet{
		types.PrimaryKeyField: []string{string(AlphaTest)},
	})

	require.Equal(t, DigestCount{
		FirstDigest:  3,
		SecondDigest: 1,
		ThirdDigest:  1,
	}, bq)
}

// valid, but arbitrary md5 hashes
const (
	FirstDigest  = types.Digest("aaa4bc0a9335c27f086f24ba207a4912")
	SecondDigest = types.Digest("bbbd0bd836b90d08f4cf640b4c298e7c")
	ThirdDigest  = types.Digest("ccc23a9039add2978bf5b49550572c7c")

	AlphaTest = types.TestName("test_alpha")
	BetaTest  = types.TestName("test_beta")

	// TraceIDs are created like tracestore.TraceIDFromParams
	x86TestAlphaTraceID = tiling.TraceID(",config=x86,source_type=test_alpha,name=gm")
	x64TestAlphaTraceID = tiling.TraceID(",config=x86_64,source_type=test_alpha,name=image")

	x64TestBetaTraceID = tiling.TraceID(",config=x86_64,source_type=test_beta,name=image")
)

func makePartialTileOne() *tiling.Tile {
	return &tiling.Tile{
		// Commits, Scale and Tile Index omitted (should not affect things)
		Traces: map[tiling.TraceID]*tiling.Trace{
			x86TestAlphaTraceID: tiling.NewTrace(
				[]types.Digest{FirstDigest, FirstDigest, SecondDigest},
				map[string]string{
					"config":              "x86",
					types.PrimaryKeyField: string(AlphaTest),
					types.CorpusField:     "gm",
				}),
			x64TestAlphaTraceID: tiling.NewTrace(
				[]types.Digest{ThirdDigest, FirstDigest, tiling.MissingDigest},
				map[string]string{
					"config":              "x86_64",
					types.PrimaryKeyField: string(AlphaTest),
					types.CorpusField:     "image",
				}),
		},
	}
}

// This tile intentionally introduces ties in counts
func makePartialTileTwo() *tiling.Tile {
	return &tiling.Tile{
		// Commits, Scale and Tile Index omitted (should not affect things)

		Traces: map[tiling.TraceID]*tiling.Trace{
			// Reminder that the ids for the traces are created by concatenating
			// all the values in alphabetical order of the keys.
			x86TestAlphaTraceID: tiling.NewTrace(
				types.DigestSlice{FirstDigest, FirstDigest, SecondDigest, SecondDigest},
				map[string]string{
					"config":              "x86",
					types.PrimaryKeyField: string(AlphaTest),
					types.CorpusField:     "gm",
				}),
			x64TestBetaTraceID: tiling.NewTrace(
				types.DigestSlice{ThirdDigest, FirstDigest, ThirdDigest, FirstDigest},
				map[string]string{
					"config":              "x86_64",
					types.PrimaryKeyField: string(BetaTest),
					types.CorpusField:     "image",
				}),
		},
	}
}
