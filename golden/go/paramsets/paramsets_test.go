package paramsets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

func TestParamsetByTraceForTile(t *testing.T) {
	unittest.SmallTest(t)

	tile := makePartialTestTile()
	counts := makeTestDigestCounts()

	byTest := byTestForTile(tile, counts)

	// The calls to normalize are for test determinism.
	// Spot check the data by looking at various test/digest
	// combos.
	ps := byTest[testOne][DigestB]
	assert.NotNil(t, ps)
	ps.Normalize()
	assert.Equal(t, paramtools.ParamSet{
		"config":              []string{"8888"},
		types.CorpusField:     []string{"gm"},
		types.PrimaryKeyField: []string{string(testOne)},
	}, ps)

	ps = byTest[testOne][DigestA]
	assert.NotNil(t, ps)
	ps.Normalize()
	assert.Equal(t, paramtools.ParamSet{
		"config":              []string{"565", "8888"},
		types.CorpusField:     []string{"gm"},
		types.PrimaryKeyField: []string{string(testOne)},
	}, ps)

	ps = byTest[testTwo][DigestG]
	assert.NotNil(t, ps)
	ps.Normalize()
	assert.Equal(t, paramtools.ParamSet{
		"config":              []string{"565"},
		types.CorpusField:     []string{"gm"},
		types.PrimaryKeyField: []string{string(testTwo)},
	}, ps)

	ps = byTest[testTwo][DigestF]
	assert.NotNil(t, ps)
	ps.Normalize()
	assert.Equal(t, paramtools.ParamSet{
		"config":              []string{"565", "gpu"},
		types.CorpusField:     []string{"gm"},
		types.PrimaryKeyField: []string{string(testTwo)},
	}, ps)

	assert.Nil(t, byTest[nonExistentTest])
	// testOne did not see this digest
	assert.Nil(t, byTest[testOne][DigestG])
}

func TestParamsetCalculate(t *testing.T) {
	unittest.SmallTest(t)

	tile := makePartialTestTile()
	counts := makeTestDigestCounts()

	md := &mocks.DigestCounter{}
	defer md.AssertExpectations(t)

	md.On("ByTrace").Return(counts)

	ps := NewParamSummary(tile, md)

	p := ps.Get(testTwo, DigestG)
	assert.NotNil(t, p)
	p.Normalize()
	assert.Equal(t, paramtools.ParamSet{
		"config":              []string{"565"},
		types.CorpusField:     []string{"gm"},
		types.PrimaryKeyField: []string{string(testTwo)},
	}, p)

}

func TestParamsetCalculateNoCounts(t *testing.T) {
	unittest.SmallTest(t)

	tile := makePartialTestTile()
	noCounts := map[tiling.TraceID]digest_counter.DigestCount{}

	md := &mocks.DigestCounter{}
	defer md.AssertExpectations(t)

	md.On("ByTrace").Return(noCounts)

	ps := NewParamSummary(tile, md)
	assert.Nil(t, ps.Get(testTwo, DigestG))
}

const (
	testOne = types.TestName("test_one")
	testTwo = types.TestName("test_two")

	nonExistentTest = types.TestName("test_not_exist_nope")

	// Arbitrary, but valid md5 hashes
	DigestA = types.Digest("aaa65156b09fc699a7f8892b108ee7e3")
	DigestB = types.Digest("bbb8e0260c64418510cefb2b06eee5cd")
	DigestC = types.Digest("ccc25df8f8f22eefed0ef135c19b8394")
	DigestD = types.Digest("ddd8984c6e72a0289a1dfde0b36df79d")
	DigestE = types.Digest("eee789257fd5ba858522462608b079bb")
	DigestF = types.Digest("fff1ff99147118958954b57e0223f1ba")
	DigestG = types.Digest("000cfe8dbf645d61325257224ee8aec5")
)

// These counts include some of the data from the testTile, but
// also some made up data
func makeTestDigestCounts() map[tiling.TraceID]digest_counter.DigestCount {
	return map[tiling.TraceID]digest_counter.DigestCount{
		"a": {
			DigestA: 1,
			DigestB: 1,
		},
		"b": {
			DigestA: 1,
			DigestC: 1,
			DigestD: 1,
		},
		"c": {
			DigestE: 1,
		},
		"e": {
			DigestF: 1,
			DigestG: 2,
		},
		"f": {
			DigestF: 1,
		},
		"unknown": {
			DigestA: 1,
			DigestC: 1,
			DigestD: 1,
		},
	}
}

// This test tile intentionally has some traces of different lengths
// than others (2 vs 3) to make sure the code is robust to that, even
// though real data should not be like that (all traces should be equal length).
func makePartialTestTile() *tiling.Tile {
	return &tiling.Tile{
		// Commits, Scale and Tile Index omitted (should not affect things)
		Traces: map[tiling.TraceID]*tiling.Trace{
			// These trace ids have been shortened for test terseness.
			// A real trace id would be like ",config=8888,source_type=gm,name=foo,"
			"a": tiling.NewTrace(
				[]types.Digest{DigestA, DigestB},
				map[string]string{
					"config":              "8888",
					types.CorpusField:     "gm",
					types.PrimaryKeyField: string(testOne),
				}),
			"b": tiling.NewTrace(
				[]types.Digest{DigestC, DigestD, DigestA},
				map[string]string{
					"config":              "565",
					types.CorpusField:     "gm",
					types.PrimaryKeyField: string(testOne),
				}),
			"c": tiling.NewTrace(
				[]types.Digest{DigestE, tiling.MissingDigest},
				map[string]string{
					"config":              "gpu",
					types.CorpusField:     "gm",
					types.PrimaryKeyField: string(testOne),
				}),
			"e": tiling.NewTrace(
				[]types.Digest{DigestF, DigestG, DigestG},
				map[string]string{
					"config":              "565",
					types.CorpusField:     "gm",
					types.PrimaryKeyField: string(testTwo),
				}),
			"f": tiling.NewTrace(
				[]types.Digest{DigestF, tiling.MissingDigest},
				map[string]string{
					"config":              "gpu",
					types.CorpusField:     "gm",
					types.PrimaryKeyField: string(testTwo),
				}),
		},
	}
}
