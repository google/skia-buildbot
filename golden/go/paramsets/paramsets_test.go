package paramsets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/types"
)

func TestParamset(t *testing.T) {
	testutils.SmallTest(t)
	tile := &tiling.Tile{
		Traces: map[string]tiling.Trace{
			"a": &types.GoldenTrace{
				Digests: []string{"aaa", "bbb"},
				Keys: map[string]string{
					"config":                "8888",
					types.CORPUS_FIELD:      "gm",
					types.PRIMARY_KEY_FIELD: "foo",
				},
			},
			"b": &types.GoldenTrace{
				Digests: []string{"ccc", "ddd", "aaa"},
				Keys: map[string]string{
					"config":                "565",
					types.CORPUS_FIELD:      "gm",
					types.PRIMARY_KEY_FIELD: "foo",
				},
			},
			"c": &types.GoldenTrace{
				Digests: []string{"eee", types.MISSING_DIGEST},
				Keys: map[string]string{
					"config":                "gpu",
					types.CORPUS_FIELD:      "gm",
					types.PRIMARY_KEY_FIELD: "foo",
				},
			},
			"e": &types.GoldenTrace{
				Digests: []string{"xxx", "yyy", "yyy"},
				Keys: map[string]string{
					"config":                "565",
					types.CORPUS_FIELD:      "gm",
					types.PRIMARY_KEY_FIELD: "bar",
				},
			},
			"f": &types.GoldenTrace{
				Digests: []string{"xxx", types.MISSING_DIGEST},
				Keys: map[string]string{
					"config":                "gpu",
					types.CORPUS_FIELD:      "gm",
					types.PRIMARY_KEY_FIELD: "bar",
				},
			},
		},
	}

	tallies := map[string]tally.Tally{
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
	assert.Equal(t, byTrace["foo"]["bbb"]["config"], []string{"8888"})
	assert.Equal(t, byTrace["foo"]["aaa"][types.PRIMARY_KEY_FIELD], []string{"foo"})
	assert.Equal(t, byTrace["bar"]["yyy"]["config"], []string{"565"})
	assert.Equal(t, util.NewStringSet([]string{"565", "gpu"}), util.NewStringSet(byTrace["bar"]["xxx"]["config"]))
	assert.Equal(t, util.NewStringSet([]string{"565", "8888"}), util.NewStringSet(byTrace["foo"]["aaa"]["config"]))
	assert.Nil(t, byTrace["bar:fff"])
}
