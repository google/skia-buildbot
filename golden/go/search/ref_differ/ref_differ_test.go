package ref_differ

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digest_counter"
	mock_index "go.skia.org/infra/golden/go/indexer/mocks"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/search/frontend"
	"go.skia.org/infra/golden/go/types"
)

// TestGetRefDiffsSunnyDay tests getting the refs
// for an untriaged diff in a test that has two
// previously marked positive digests and one such negative digest.
func TestGetRefDiffsSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	untriagedDigest := types.Digest("7bf4d4e913605c0781697df4004191c5")

	es := common.ExpSlice{types.Expectations{
		TestName: {
			AlphaPositiveDigest: types.POSITIVE,
			GammaPositiveDigest: types.POSITIVE,
			BetaNegativeDigest:  types.NEGATIVE,
		},
	}}

	mis := &mock_index.IndexSearcher{}
	mds := &mocks.DiffStore{}
	defer mis.AssertExpectations(t)
	defer mds.AssertExpectations(t)

	mds.On("UnavailableDigests").Return(map[types.Digest]*diff.DigestFailure{})

	mis.On("GetParamsetSummaryByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]map[types.Digest]paramtools.ParamSet{
			TestName: {
				AlphaPositiveDigest: makeAlphaParamSet(),
				BetaNegativeDigest:  makeBetaParamSet(),
				GammaPositiveDigest: makeGammaParamSet(),
			},
		},
	)

	mis.On("DigestCountsByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]digest_counter.DigestCount{
			TestName: {
				AlphaPositiveDigest: 117,
				BetaNegativeDigest:  8,
				GammaPositiveDigest: 93,
			},
		},
	)

	mds.On("Get", diff.PRIORITY_NOW, untriagedDigest, types.DigestSlice{AlphaPositiveDigest, GammaPositiveDigest}).Return(
		map[types.Digest]interface{}{
			AlphaPositiveDigest: makeDiffMetric(8),
			GammaPositiveDigest: makeDiffMetric(2),
		}, nil)

	mds.On("Get", diff.PRIORITY_NOW, untriagedDigest, types.DigestSlice{BetaNegativeDigest}).Return(
		map[types.Digest]interface{}{
			BetaNegativeDigest: makeDiffMetric(9),
		}, nil)

	rd := New(es, mds, mis)

	metric := diff.METRIC_COMBINED
	matches := []string{types.PRIMARY_KEY_FIELD} // This is the default for several gold queries.
	input := frontend.SRDigest{
		ParamSet: paramtools.ParamSet{
			"arch":                  []string{"x86"},
			types.PRIMARY_KEY_FIELD: []string{string(TestName)},
			"os":                    []string{"iPhone 38 Maxx"},
		},
		Digest: untriagedDigest,
		Test:   TestName,
	}
	rd.FillRefDiffs(&input, metric, matches, matchAll, types.ExcludeIgnoredTraces)

	assert.Equal(t, common.PositiveRef, input.ClosestRef)
	assert.Equal(t, map[common.RefClosest]*frontend.SRDiffDigest{
		common.PositiveRef: {
			DiffMetrics:       makeDiffMetric(2),
			Digest:            GammaPositiveDigest,
			Status:            "positive",
			ParamSet:          makeGammaParamSet(),
			OccurrencesInTile: 93, // These are the arbitrary numbers from DigestCountsByTest
		},
		common.NegativeRef: {
			DiffMetrics:       makeDiffMetric(9),
			Digest:            BetaNegativeDigest,
			Status:            "negative",
			ParamSet:          makeBetaParamSet(),
			OccurrencesInTile: 8, // These are the arbitrary numbers from DigestCountsByTest
		},
	}, input.RefDiffs)
}

// TestGetRefDiffsNoPrevious tests the case when the first digest for a test
// is uploaded an there are no positive nor negative matches seen previously.
func TestGetRefDiffsNoPrevious(t *testing.T) {
	unittest.SmallTest(t)

	untriagedDigest := types.Digest("7bf4d4e913605c0781697df4004191c5")

	es := common.ExpSlice{types.Expectations{}}

	mis := &mock_index.IndexSearcher{}
	mds := &mocks.DiffStore{}
	defer mis.AssertExpectations(t)
	defer mds.AssertExpectations(t)

	mds.On("UnavailableDigests").Return(map[types.Digest]*diff.DigestFailure{})

	mis.On("GetParamsetSummaryByTest", types.ExcludeIgnoredTraces).Return(map[types.TestName]map[types.Digest]paramtools.ParamSet{})

	mis.On("DigestCountsByTest", types.ExcludeIgnoredTraces).Return(map[types.TestName]digest_counter.DigestCount{})

	rd := New(es, mds, mis)

	metric := diff.METRIC_COMBINED
	matches := []string{types.PRIMARY_KEY_FIELD}
	input := frontend.SRDigest{
		ParamSet: paramtools.ParamSet{
			"arch":                  []string{"x86"},
			types.PRIMARY_KEY_FIELD: []string{string(TestName)},
			"os":                    []string{"iPhone 38 Maxx"},
		},
		Digest: untriagedDigest,
		Test:   TestName,
	}
	rd.FillRefDiffs(&input, metric, matches, matchAll, types.ExcludeIgnoredTraces)

	assert.Equal(t, common.NoRef, input.ClosestRef)
	assert.Equal(t, map[common.RefClosest]*frontend.SRDiffDigest{
		common.PositiveRef: nil,
		common.NegativeRef: nil,
	}, input.RefDiffs)
}

// TestGetRefDiffsMatches tests that we can supply multiple keys to
// match against.
func TestGetRefDiffsMatches(t *testing.T) {
	unittest.SmallTest(t)

	untriagedDigest := types.Digest("7bf4d4e913605c0781697df4004191c5")

	es := common.ExpSlice{types.Expectations{
		TestName: {
			AlphaPositiveDigest: types.POSITIVE,
			GammaPositiveDigest: types.POSITIVE,
			BetaNegativeDigest:  types.NEGATIVE,
		},
	}}

	mis := &mock_index.IndexSearcher{}
	mds := &mocks.DiffStore{}
	defer mis.AssertExpectations(t)
	defer mds.AssertExpectations(t)

	mds.On("UnavailableDigests").Return(map[types.Digest]*diff.DigestFailure{})

	mis.On("GetParamsetSummaryByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]map[types.Digest]paramtools.ParamSet{
			TestName: {
				AlphaPositiveDigest: makeAlphaParamSet(),
				BetaNegativeDigest:  makeBetaParamSet(),
				GammaPositiveDigest: makeGammaParamSet(),
			},
		},
	)

	mis.On("DigestCountsByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]digest_counter.DigestCount{
			TestName: {
				AlphaPositiveDigest: 117,
				BetaNegativeDigest:  8,
				GammaPositiveDigest: 93,
			},
		},
	)

	mds.On("Get", diff.PRIORITY_NOW, untriagedDigest, types.DigestSlice{GammaPositiveDigest}).Return(
		map[types.Digest]interface{}{
			GammaPositiveDigest: makeDiffMetric(2),
		}, nil)

	rd := New(es, mds, mis)

	metric := diff.METRIC_COMBINED
	matches := []string{"arch", types.PRIMARY_KEY_FIELD} // Only Gamma has x86 in the "arch" values.
	input := frontend.SRDigest{
		ParamSet: paramtools.ParamSet{
			"arch":                  []string{"x86"},
			types.PRIMARY_KEY_FIELD: []string{string(TestName)},
			"os":                    []string{"iPhone 38 Maxx"},
		},
		Digest: untriagedDigest,
		Test:   TestName,
	}
	rd.FillRefDiffs(&input, metric, matches, matchAll, types.ExcludeIgnoredTraces)

	assert.Equal(t, common.PositiveRef, input.ClosestRef)
	assert.Equal(t, map[common.RefClosest]*frontend.SRDiffDigest{
		common.PositiveRef: {
			DiffMetrics:       makeDiffMetric(2),
			Digest:            GammaPositiveDigest,
			Status:            "positive",
			ParamSet:          makeGammaParamSet(),
			OccurrencesInTile: 93, // These are the arbitrary numbers from DigestCountsByTest
		},
		common.NegativeRef: nil,
	}, input.RefDiffs)
}

var matchAll = paramtools.ParamSet{}

// All this test data is valid, but arbitrary.

const (
	AlphaPositiveDigest = types.Digest("aaa884cd5ac3d6785c35cff8f26d2da5")
	BetaNegativeDigest  = types.Digest("bbb8d94852dfde3f3bebcc000be60153")
	GammaPositiveDigest = types.Digest("ccc84ad6f1a0c628d5f27180e497309e")

	TestName = types.TestName("some_test")
)

// makeDiffMetric makes a DiffMetrics object with
// a combined diff metric of n. All other data is
// based off of n, but not technically accurate.
func makeDiffMetric(n int) *diff.DiffMetrics {
	return &diff.DiffMetrics{
		NumDiffPixels:    n * 100,
		PixelDiffPercent: 1.0 - float32(n)/10.0,
		MaxRGBADiffs:     []int{3 * n, 2 * n, n, n},
		DimDiffer:        false,
		Diffs: map[string]float32{
			diff.METRIC_COMBINED: float32(n),
			"percent":            1.0 - float32(n)/10.0,
			"pixel":              float32(n) * 100,
		},
	}
}

// makeAlphaParamSet returns the ParamSet for the AlphaPositiveDigest
func makeAlphaParamSet() paramtools.ParamSet {
	return paramtools.ParamSet{
		"arch": []string{"z80"},
		"name": []string{string(TestName)},
		"os":   []string{"Texas Instruments"},
	}
}

// makeBetaParamSet returns the ParamSet for the AlphaPositiveDigest
func makeBetaParamSet() paramtools.ParamSet {
	return paramtools.ParamSet{
		"arch": []string{"x64"},
		"name": []string{string(TestName)},
		"os":   []string{"Android"},
	}
}

// makeGammaParamSet returns the ParamSet for the AlphaPositiveDigest
func makeGammaParamSet() paramtools.ParamSet {
	// This means that both the arm and x86 bot drew the same thing
	// for the given test.
	return paramtools.ParamSet{
		"arch": []string{"arm", "x86"},
		"name": []string{string(TestName)},
		"os":   []string{"Android"},
	}
}
