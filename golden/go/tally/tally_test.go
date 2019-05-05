package tally

import (
	"net/url"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

func TestTallyCalculate(t *testing.T) {
	testutils.SmallTest(t)
	tile := makeTestTile()

	tallies := New()
	tallies.Calculate(tile)

	assert.Equal(t, map[string]Tally{
		"foo:x86": Tally{
			"aaa": 2,
			"bbb": 1,
		},
		"foo:x86_64": Tally{
			"aaa": 1,
			"ccc": 1,
		},
	}, tallies.ByTrace())

	assert.Equal(t, map[string]Tally{
		"foo": Tally{
			"aaa": 3,
			"bbb": 1,
			"ccc": 1,
		},
	}, tallies.ByTest())

	assert.Equal(t, map[string]util.StringSet{
		"foo": util.StringSet{
			"aaa": true,
		},
	}, tallies.MaxDigestsByTest())
}

func TestTallyByQuery(t *testing.T) {
	testutils.SmallTest(t)
	tile := makeTestTile()

	tallies := New()
	tallies.Calculate(tile)

	bq := tallies.ByQuery(tile, url.Values{
		types.CORPUS_FIELD: []string{"gm"},
	})

	assert.Equal(t, Tally{
		"aaa": 2,
		"bbb": 1,
	}, bq)

	bq = tallies.ByQuery(tile, url.Values{
		types.CORPUS_FIELD: []string{"image"},
	})

	assert.Equal(t, Tally{
		"aaa": 1,
		"ccc": 1,
	}, bq)

	bq = tallies.ByQuery(tile, url.Values{
		types.PRIMARY_KEY_FIELD: []string{"foo"},
	})

	assert.Equal(t, Tally{
		"aaa": 3,
		"bbb": 1,
		"ccc": 1,
	}, bq)
}

func makeTestTile() *tiling.Tile {
	// Create a tile to test against.
	tile := tiling.NewTile()
	trace1 := types.NewGoldenTrace()
	trace1.Digests[0] = "aaa"
	trace1.Digests[1] = "aaa"
	trace1.Digests[2] = "bbb"
	trace1.Keys[types.PRIMARY_KEY_FIELD] = "foo"
	trace1.Keys[types.CORPUS_FIELD] = "gm"
	tile.Traces["foo:x86"] = trace1

	trace2 := types.NewGoldenTrace()
	trace2.Digests[0] = "ccc"
	trace2.Digests[1] = "aaa"
	trace2.Keys[types.PRIMARY_KEY_FIELD] = "foo"
	trace2.Keys[types.CORPUS_FIELD] = "image"
	tile.Traces["foo:x86_64"] = trace2
	return tile
}
