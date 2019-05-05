package paramsets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/types"
)

const (
	testOne = "foo"
	testTwo = "bar"
)

func TestParamsetByTraceForTile(t *testing.T) {
	testutils.SmallTest(t)

	tile := makeTestTile()
	counts := makeTestDigestCounts()

	byTrace := byTraceForTile(tile, counts)

	// Test that we are robust to traces appearing in counts, but not
	// in the tile, and vice-versa.
	// The calls to normalize are for test determinism.
	ps := byTrace[testOne]["bbb"]
	assert.NotNil(t, ps)
	ps.Normalize()
	assert.Equal(t, paramtools.ParamSet{
		"config":                []string{"8888"},
		types.CORPUS_FIELD:      []string{"gm"},
		types.PRIMARY_KEY_FIELD: []string{testOne},
	}, ps)

	ps = byTrace[testOne]["aaa"]
	assert.NotNil(t, ps)
	ps.Normalize()
	assert.Equal(t, paramtools.ParamSet{
		"config":                []string{"565", "8888"},
		types.CORPUS_FIELD:      []string{"gm"},
		types.PRIMARY_KEY_FIELD: []string{testOne},
	}, ps)

	ps = byTrace[testTwo]["yyy"]
	assert.NotNil(t, ps)
	ps.Normalize()
	assert.Equal(t, paramtools.ParamSet{
		"config":                []string{"565"},
		types.CORPUS_FIELD:      []string{"gm"},
		types.PRIMARY_KEY_FIELD: []string{testTwo},
	}, ps)

	ps = byTrace[testTwo]["xxx"]
	assert.NotNil(t, ps)
	ps.Normalize()
	assert.Equal(t, paramtools.ParamSet{
		"config":                []string{"565", "gpu"},
		types.CORPUS_FIELD:      []string{"gm"},
		types.PRIMARY_KEY_FIELD: []string{testTwo},
	}, ps)

	assert.Nil(t, byTrace["bar:fff"])
	assert.Nil(t, byTrace[testOne]["yyy"])
}

func TestParamsetCalculate(t *testing.T) {
	testutils.SmallTest(t)

	tile := makeTestTile()
	counts := makeTestDigestCounts()
	noCounts := map[string]digest_counter.DigestCount{}

	mc := &mocks.ComplexTile{}
	// without ignores
	md := &mocks.DigestCounter{}
	// with ignores
	mdi := &mocks.DigestCounter{}
	defer mc.AssertExpectations(t)
	defer md.AssertExpectations(t)
	defer mdi.AssertExpectations(t)

	mc.On("GetTile", true).Return(tile)
	mc.On("GetTile", false).Return(tile)

	md.On("ByTrace").Return(counts)
	mdi.On("ByTrace").Return(noCounts)

	ps := New()
	ps.Calculate(mc, md, mdi)

	withIgnores := ps.GetByTest(true)
	withoutIgnores := ps.GetByTest(false)
	assert.NotEqual(t, withIgnores, withoutIgnores)
	// spot check one from each

	p := withoutIgnores[testTwo]["yyy"]
	assert.NotNil(t, p)
	p.Normalize()
	assert.Equal(t, paramtools.ParamSet{
		"config":                []string{"565"},
		types.CORPUS_FIELD:      []string{"gm"},
		types.PRIMARY_KEY_FIELD: []string{testTwo},
	}, p)

	assert.Empty(t, withIgnores[testTwo])
}

func makeTestDigestCounts() map[string]digest_counter.DigestCount {
	return map[string]digest_counter.DigestCount{
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
}

func makeTestTile() *tiling.Tile {
	return &tiling.Tile{
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
}
