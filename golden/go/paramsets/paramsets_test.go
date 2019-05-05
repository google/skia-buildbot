package paramsets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/types"
)

const (
	testOne = "foo"
	testTwo = "bar"
)

func TestParamsetByTraceForTile(t *testing.T) {
	testutils.SmallTest(t)
	tile := &tiling.Tile{
		Traces: map[string]tiling.Trace{
			// These trace ids have been shortened for test terseness.
			// A real trace id would be like "8888:gm:foo"
			"a": &types.GoldenTrace{
				Digests: []string{"aaa", "bbb"},
				Keys: map[string]string{
					"config":                "8888",
					types.CORPUS_FIELD:      "gm",
					types.PRIMARY_KEY_FIELD: testOne,
				},
			},
			"b": &types.GoldenTrace{
				Digests: []string{"ccc", "ddd", "aaa"},
				Keys: map[string]string{
					"config":                "565",
					types.CORPUS_FIELD:      "gm",
					types.PRIMARY_KEY_FIELD: testOne,
				},
			},
			"c": &types.GoldenTrace{
				Digests: []string{"eee", types.MISSING_DIGEST},
				Keys: map[string]string{
					"config":                "gpu",
					types.CORPUS_FIELD:      "gm",
					types.PRIMARY_KEY_FIELD: testOne,
				},
			},
			"e": &types.GoldenTrace{
				Digests: []string{"xxx", "yyy", "yyy"},
				Keys: map[string]string{
					"config":                "565",
					types.CORPUS_FIELD:      "gm",
					types.PRIMARY_KEY_FIELD: testTwo,
				},
			},
			"f": &types.GoldenTrace{
				Digests: []string{"xxx", types.MISSING_DIGEST},
				Keys: map[string]string{
					"config":                "gpu",
					types.CORPUS_FIELD:      "gm",
					types.PRIMARY_KEY_FIELD: testTwo,
				},
			},
		},
	}

	tallies := map[string]digest_counter.DigestCount{
		"a": {
			"aaa": 1,
			"bbb": 1,
		},
		"b": {
			"ccc": 1,
			"ddd": 1,
			"aaa": 1,
		},
		"c": {
			"eee": 1,
		},
		"e": {
			"xxx": 1,
			"yyy": 2,
		},
		"f": {
			"xxx": 1,
		},
		"unknown": {
			"ccc": 1,
			"ddd": 1,
			"aaa": 1,
		},
	}

	byTrace := byTraceForTile(tile, tallies)

	// Test that we are robust to traces appearing in tallies, but not in the tile, and vice-versa.
	assert.Equal(t, paramtools.ParamSet{
		"config":                []string{"8888"},
		types.CORPUS_FIELD:      []string{"gm"},
		types.PRIMARY_KEY_FIELD: []string{testOne},
	}, byTrace[testOne]["bbb"])
	assert.Equal(t, paramtools.ParamSet{
		"config":                []string{"8888", "565"},
		types.CORPUS_FIELD:      []string{"gm"},
		types.PRIMARY_KEY_FIELD: []string{testOne},
	}, byTrace[testOne]["aaa"])

	assert.Equal(t, paramtools.ParamSet{
		"config":                []string{"565"},
		types.CORPUS_FIELD:      []string{"gm"},
		types.PRIMARY_KEY_FIELD: []string{testTwo},
	}, byTrace[testTwo]["yyy"])

	assert.Equal(t, paramtools.ParamSet{
		"config":                []string{"565", "gpu"},
		types.CORPUS_FIELD:      []string{"gm"},
		types.PRIMARY_KEY_FIELD: []string{testTwo},
	}, byTrace[testTwo]["xxx"])

	assert.Nil(t, byTrace["bar:fff"])
	assert.Nil(t, byTrace[testOne]["yyy"])
}
