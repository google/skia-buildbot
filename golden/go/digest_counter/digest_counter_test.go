package digest_counter

import (
	"net/url"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

func TestDigestCountCalculate(t *testing.T) {
	testutils.SmallTest(t)
	tile := makePartialTileOne()

	dc := New()
	dc.Calculate(tile)

	assert.Equal(t, map[string]DigestCount{
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

	assert.Equal(t, map[string]DigestCount{
		AlphaTest: {
			// AlphaTest was the only test, so these are the counts for both configs.
			FirstDigest:  3,
			SecondDigest: 1,
			ThirdDigest:  1,
		},
	}, dc.ByTest())

	assert.Equal(t, map[string]util.StringSet{
		AlphaTest: {
			// AlphaTest had the most of any digest in this test (see above)
			FirstDigest: true,
		},
	}, dc.MaxDigestsByTest())
}

// Check that counts and byTest work with ties and multiple tests
func TestDigestCountCalculateTies(t *testing.T) {
	testutils.SmallTest(t)
	tile := makePartialTileTwo()

	dc := New()
	dc.Calculate(tile)

	assert.Equal(t, map[string]util.StringSet{
		AlphaTest: {
			FirstDigest:  true,
			SecondDigest: true,
		},
		BetaTest: {
			FirstDigest: true,
			ThirdDigest: true,
		},
	}, dc.MaxDigestsByTest())

	assert.Equal(t, map[string]DigestCount{
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
	testutils.SmallTest(t)
	tile := makePartialTileOne()

	dc := New()
	dc.Calculate(tile)

	bq := dc.ByQuery(tile, url.Values{
		types.CORPUS_FIELD: []string{"gm"},
	})

	assert.Equal(t, DigestCount{
		FirstDigest:  2,
		SecondDigest: 1,
	}, bq)

	bq = dc.ByQuery(tile, url.Values{
		types.CORPUS_FIELD: []string{"image"},
	})

	assert.Equal(t, DigestCount{
		FirstDigest: 1,
		ThirdDigest: 1,
	}, bq)

	bq = dc.ByQuery(tile, url.Values{
		types.PRIMARY_KEY_FIELD: []string{AlphaTest},
	})

	assert.Equal(t, DigestCount{
		FirstDigest:  3,
		SecondDigest: 1,
		ThirdDigest:  1,
	}, bq)
}

// arbitrary, but valid md5 hashes
const (
	FirstDigest  = "aaa4bc0a9335c27f086f24ba207a4912"
	SecondDigest = "bbbd0bd836b90d08f4cf640b4c298e7c"
	ThirdDigest  = "ccc23a9039add2978bf5b49550572c7c"

	AlphaTest = "test_alpha"
	BetaTest  = "test_beta"

	x86TestAlphaTraceID = "x86:test_alpha:gm"
	x64TestAlphaTraceID = "x86_64:test_alpha:image"

	x64TestBetaTraceID = "x86_64:test_beta:image"
)

func makePartialTileOne() *tiling.Tile {
	return &tiling.Tile{
		// Commits, Scale and Tile Index omitted (should not affect things)

		Traces: map[string]tiling.Trace{
			// Reminder that the ids for the traces are created by concatenating
			// all the values in alphabetical order of the keys.
			x86TestAlphaTraceID: &types.GoldenTrace{
				Digests: []string{FirstDigest, FirstDigest, SecondDigest},
				Keys: map[string]string{
					"config":                "x86",
					types.PRIMARY_KEY_FIELD: AlphaTest,
					types.CORPUS_FIELD:      "gm",
				},
			},
			x64TestAlphaTraceID: &types.GoldenTrace{
				Digests: []string{ThirdDigest, FirstDigest, types.MISSING_DIGEST},
				Keys: map[string]string{
					"config":                "x86_64",
					types.PRIMARY_KEY_FIELD: AlphaTest,
					types.CORPUS_FIELD:      "image",
				},
			},
		},
	}
}

// This tile intentionally introduces ties in counts
func makePartialTileTwo() *tiling.Tile {
	return &tiling.Tile{
		// Commits, Scale and Tile Index omitted (should not affect things)

		Traces: map[string]tiling.Trace{
			// Reminder that the ids for the traces are created by concatenating
			// all the values in alphabetical order of the keys.
			x86TestAlphaTraceID: &types.GoldenTrace{
				Digests: []string{FirstDigest, FirstDigest, SecondDigest, SecondDigest},
				Keys: map[string]string{
					"config":                "x86",
					types.PRIMARY_KEY_FIELD: AlphaTest,
					types.CORPUS_FIELD:      "gm",
				},
			},
			x64TestBetaTraceID: &types.GoldenTrace{
				Digests: []string{ThirdDigest, FirstDigest, ThirdDigest, FirstDigest},
				Keys: map[string]string{
					"config":                "x86_64",
					types.PRIMARY_KEY_FIELD: BetaTest,
					types.CORPUS_FIELD:      "image",
				},
			},
		},
	}
}
