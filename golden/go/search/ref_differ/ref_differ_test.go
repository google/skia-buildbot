package ref_differ

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff"
	mock_diffstore "go.skia.org/infra/golden/go/diffstore/mocks"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/expectations"
	mock_index "go.skia.org/infra/golden/go/indexer/mocks"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/search/frontend"
	"go.skia.org/infra/golden/go/types"
)

// TestGetRefDiffsSunnyDay tests getting the refs
// for an untriaged diff in a test that has two
// previously marked positive digests and one such negative digest.
func TestGetRefDiffsSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	es := makeExpClassifier()

	mis := &mock_index.IndexSearcher{}
	mds := &mock_diffstore.DiffStore{}
	defer mis.AssertExpectations(t)
	defer mds.AssertExpectations(t)

	mis.On("GetParamsetSummaryByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]map[types.Digest]paramtools.ParamSet{
			testName: {
				alphaPositiveDigest: makeAlphaParamSet(),
				betaNegativeDigest:  makeBetaParamSet(),
				gammaPositiveDigest: makeGammaParamSet(),
				untriagedDigest:     makeUntriagedParamSet(),
			},
		},
	)

	mis.On("DigestCountsByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]digest_counter.DigestCount{
			testName: {
				alphaPositiveDigest: 117,
				betaNegativeDigest:  8,
				gammaPositiveDigest: 93,
				untriagedDigest:     7,
			},
		},
	)

	mds.On("Get", testutils.AnyContext, untriagedDigest, types.DigestSlice{alphaPositiveDigest, gammaPositiveDigest}).Return(
		map[types.Digest]*diff.DiffMetrics{
			alphaPositiveDigest: makeDiffMetric(8),
			gammaPositiveDigest: makeDiffMetric(2),
		}, nil)

	mds.On("Get", testutils.AnyContext, untriagedDigest, types.DigestSlice{betaNegativeDigest}).Return(
		map[types.Digest]*diff.DiffMetrics{
			betaNegativeDigest: makeDiffMetric(9),
		}, nil)

	rd := New(es, mds, mis)

	metric := diff.CombinedMetric
	matches := []string{types.PrimaryKeyField} // This is the default for several gold queries.
	input := frontend.SearchResult{
		ParamSet: makeUntriagedParamSet(),
		Digest:   untriagedDigest,
		Test:     testName,
	}
	err := rd.FillRefDiffs(context.Background(), &input, metric, matches, matchAll, types.ExcludeIgnoredTraces)

	require.NoError(t, err)
	require.Equal(t, common.PositiveRef, input.ClosestRef)
	require.Equal(t, map[common.RefClosest]*frontend.SRDiffDigest{
		common.PositiveRef: {
			DiffMetrics:       makeDiffMetric(2),
			Digest:            gammaPositiveDigest,
			Status:            "positive",
			ParamSet:          makeGammaParamSet(),
			OccurrencesInTile: 93, // These are the arbitrary numbers from DigestCountsByTest
		},
		common.NegativeRef: {
			DiffMetrics:       makeDiffMetric(9),
			Digest:            betaNegativeDigest,
			Status:            "negative",
			ParamSet:          makeBetaParamSet(),
			OccurrencesInTile: 8, // These are the arbitrary numbers from DigestCountsByTest
		},
	}, input.RefDiffs)
}

// TestGetRefDiffsTryJobSunnyDay tests getting the refs
// for an untriaged diff in a tryjob test that has two
// previously marked positive digests and one such negative digest.
func TestGetRefDiffsTryJobSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	es := makeExpClassifier()

	mis := &mock_index.IndexSearcher{}
	mds := &mock_diffstore.DiffStore{}
	defer mis.AssertExpectations(t)
	defer mds.AssertExpectations(t)

	mis.On("GetParamsetSummaryByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]map[types.Digest]paramtools.ParamSet{
			testName: {
				alphaPositiveDigest: makeAlphaParamSet(),
				betaNegativeDigest:  makeBetaParamSet(),
				gammaPositiveDigest: makeGammaParamSet(),
				// untriagedDigest isn't here to emulate a tryjob run
			},
		},
	)

	mis.On("DigestCountsByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]digest_counter.DigestCount{
			testName: {
				alphaPositiveDigest: 117,
				betaNegativeDigest:  8,
				gammaPositiveDigest: 93,
				// untriagedDigest isn't here to emulate a tryjob run
			},
		},
	)

	mds.On("Get", testutils.AnyContext, untriagedDigest, types.DigestSlice{alphaPositiveDigest, gammaPositiveDigest}).Return(
		map[types.Digest]*diff.DiffMetrics{
			alphaPositiveDigest: makeDiffMetric(8),
			gammaPositiveDigest: makeDiffMetric(2),
		}, nil)

	mds.On("Get", testutils.AnyContext, untriagedDigest, types.DigestSlice{betaNegativeDigest}).Return(
		map[types.Digest]*diff.DiffMetrics{
			betaNegativeDigest: makeDiffMetric(9),
		}, nil)

	rd := New(es, mds, mis)

	metric := diff.CombinedMetric
	matches := []string{types.PrimaryKeyField} // This is the default for several gold queries.
	input := frontend.SearchResult{
		ParamSet: makeUntriagedParamSet(),
		Digest:   untriagedDigest,
		Test:     testName,
	}
	err := rd.FillRefDiffs(context.Background(), &input, metric, matches, matchAll, types.ExcludeIgnoredTraces)

	require.NoError(t, err)
	require.Equal(t, common.PositiveRef, input.ClosestRef)
	require.Equal(t, map[common.RefClosest]*frontend.SRDiffDigest{
		common.PositiveRef: {
			DiffMetrics:       makeDiffMetric(2),
			Digest:            gammaPositiveDigest,
			Status:            "positive",
			ParamSet:          makeGammaParamSet(),
			OccurrencesInTile: 93, // These are the arbitrary numbers from DigestCountsByTest
		},
		common.NegativeRef: {
			DiffMetrics:       makeDiffMetric(9),
			Digest:            betaNegativeDigest,
			Status:            "negative",
			ParamSet:          makeBetaParamSet(),
			OccurrencesInTile: 8, // These are the arbitrary numbers from DigestCountsByTest
		},
	}, input.RefDiffs)
}

// TestGetRefDiffsAllUntriaged tests the case when there are a few untriaged digests
// on master, including the one we are trying to find a diff for.
func TestGetRefDiffsAllUntriaged(t *testing.T) {
	unittest.SmallTest(t)

	// Empty expectations => everything is untriaged.
	es := expectations.EmptyClassifier()

	mis := &mock_index.IndexSearcher{}
	mds := &mock_diffstore.DiffStore{}
	defer mis.AssertExpectations(t)
	defer mds.AssertExpectations(t)

	mis.On("GetParamsetSummaryByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]map[types.Digest]paramtools.ParamSet{
			testName: {
				alphaPositiveDigest: makeAlphaParamSet(),
				betaNegativeDigest:  makeBetaParamSet(),
				gammaPositiveDigest: makeGammaParamSet(),
				untriagedDigest:     makeUntriagedParamSet(),
			},
		},
	)

	mis.On("DigestCountsByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]digest_counter.DigestCount{
			testName: {
				alphaPositiveDigest: 117,
				betaNegativeDigest:  8,
				gammaPositiveDigest: 93,
				untriagedDigest:     3,
			},
		},
	)

	rd := New(es, mds, mis)

	metric := diff.CombinedMetric
	matches := []string{types.PrimaryKeyField}
	input := frontend.SearchResult{
		ParamSet: makeUntriagedParamSet(),
		Digest:   untriagedDigest,
		Test:     testName,
	}
	err := rd.FillRefDiffs(context.Background(), &input, metric, matches, matchAll, types.ExcludeIgnoredTraces)

	require.NoError(t, err)
	require.Equal(t, common.NoRef, input.ClosestRef)
	require.Equal(t, map[common.RefClosest]*frontend.SRDiffDigest{
		common.PositiveRef: nil,
		common.NegativeRef: nil,
	}, input.RefDiffs)
}

// TestGetRefDiffsNoPrevious tests the case when the first digest for a test
// is uploaded an there are no positive nor negative matches seen previously.
func TestGetRefDiffsNoPrevious(t *testing.T) {
	unittest.SmallTest(t)

	es := expectations.EmptyClassifier()

	mis := &mock_index.IndexSearcher{}
	mds := &mock_diffstore.DiffStore{}
	defer mis.AssertExpectations(t)
	defer mds.AssertExpectations(t)

	mis.On("GetParamsetSummaryByTest", types.ExcludeIgnoredTraces).Return(map[types.TestName]map[types.Digest]paramtools.ParamSet{})

	mis.On("DigestCountsByTest", types.ExcludeIgnoredTraces).Return(map[types.TestName]digest_counter.DigestCount{})

	rd := New(es, mds, mis)

	metric := diff.CombinedMetric
	matches := []string{types.PrimaryKeyField}
	input := frontend.SearchResult{
		ParamSet: makeUntriagedParamSet(),
		Digest:   untriagedDigest,
		Test:     testName,
	}
	err := rd.FillRefDiffs(context.Background(), &input, metric, matches, matchAll, types.ExcludeIgnoredTraces)

	require.NoError(t, err)
	require.Equal(t, common.NoRef, input.ClosestRef)
	require.Equal(t, map[common.RefClosest]*frontend.SRDiffDigest{
		common.PositiveRef: nil,
		common.NegativeRef: nil,
	}, input.RefDiffs)
}

// TestGetRefDiffsMatches tests that we can supply multiple keys to
// match against.
func TestGetRefDiffsMatches(t *testing.T) {
	unittest.SmallTest(t)

	es := makeExpClassifier()

	mis := &mock_index.IndexSearcher{}
	mds := &mock_diffstore.DiffStore{}
	defer mis.AssertExpectations(t)
	defer mds.AssertExpectations(t)

	mis.On("GetParamsetSummaryByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]map[types.Digest]paramtools.ParamSet{
			testName: {
				alphaPositiveDigest: makeAlphaParamSet(),
				betaNegativeDigest:  makeBetaParamSet(),
				gammaPositiveDigest: makeGammaParamSet(),
			},
		},
	)

	mis.On("DigestCountsByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]digest_counter.DigestCount{
			testName: {
				alphaPositiveDigest: 117,
				betaNegativeDigest:  8,
				gammaPositiveDigest: 93,
			},
		},
	)

	mds.On("Get", testutils.AnyContext, untriagedDigest, types.DigestSlice{gammaPositiveDigest}).Return(
		map[types.Digest]*diff.DiffMetrics{
			gammaPositiveDigest: makeDiffMetric(2),
		}, nil)

	rd := New(es, mds, mis)

	metric := diff.CombinedMetric
	matches := []string{"arch", types.PrimaryKeyField} // Only Gamma has x86 in the "arch" values.
	input := frontend.SearchResult{
		ParamSet: makeUntriagedParamSet(),
		Digest:   untriagedDigest,
		Test:     testName,
	}
	err := rd.FillRefDiffs(context.Background(), &input, metric, matches, matchAll, types.ExcludeIgnoredTraces)

	require.NoError(t, err)
	require.Equal(t, common.PositiveRef, input.ClosestRef)
	require.Equal(t, map[common.RefClosest]*frontend.SRDiffDigest{
		common.PositiveRef: {
			DiffMetrics:       makeDiffMetric(2),
			Digest:            gammaPositiveDigest,
			Status:            "positive",
			ParamSet:          makeGammaParamSet(),
			OccurrencesInTile: 93, // These are the arbitrary numbers from DigestCountsByTest
		},
		common.NegativeRef: nil,
	}, input.RefDiffs)
}

// TestGetRefDiffsMatchRHS tests that we can provide a RHS query to match against.
func TestGetRefDiffsMatchRHS(t *testing.T) {
	unittest.SmallTest(t)

	es := makeExpClassifier()

	mis := &mock_index.IndexSearcher{}
	mds := &mock_diffstore.DiffStore{}
	defer mis.AssertExpectations(t)
	defer mds.AssertExpectations(t)

	mis.On("GetParamsetSummaryByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]map[types.Digest]paramtools.ParamSet{
			testName: {
				alphaPositiveDigest: makeAlphaParamSet(),
				betaNegativeDigest:  makeBetaParamSet(),
				gammaPositiveDigest: makeGammaParamSet(),
			},
			"should-be-ignored": { // this is provided to make sure we only match in a given test.
				"ignorabledigest": makeAlphaParamSet(),
			},
		},
	)

	mis.On("DigestCountsByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]digest_counter.DigestCount{
			testName: {
				alphaPositiveDigest: 117,
				betaNegativeDigest:  8,
				gammaPositiveDigest: 93,
			},
			"should-be-ignored": {
				"ignorabledigest": 9999,
			},
		},
	)

	mds.On("Get", testutils.AnyContext, untriagedDigest, types.DigestSlice{alphaPositiveDigest}).Return(
		map[types.Digest]*diff.DiffMetrics{
			alphaPositiveDigest: makeDiffMetric(2),
		}, nil)

	rd := New(es, mds, mis)

	metric := diff.CombinedMetric
	input := frontend.SearchResult{
		ParamSet: makeUntriagedParamSet(),
		Digest:   untriagedDigest,
		Test:     testName,
	}
	rhsQuery := paramtools.ParamSet{
		"arch": []string{"z80"},
	}
	err := rd.FillRefDiffs(context.Background(), &input, metric, nil, rhsQuery, types.ExcludeIgnoredTraces)

	require.NoError(t, err)
	require.Equal(t, common.PositiveRef, input.ClosestRef)
	require.Equal(t, map[common.RefClosest]*frontend.SRDiffDigest{
		common.PositiveRef: {
			DiffMetrics:       makeDiffMetric(2),
			Digest:            alphaPositiveDigest,
			Status:            "positive",
			ParamSet:          makeAlphaParamSet(),
			OccurrencesInTile: 117, // These are the arbitrary numbers from DigestCountsByTest
		},
		common.NegativeRef: nil,
	}, input.RefDiffs)
}

var matchAll = paramtools.ParamSet{}

// All this test data is valid, but arbitrary.

const (
	alphaPositiveDigest = types.Digest("aaa884cd5ac3d6785c35cff8f26d2da5")
	betaNegativeDigest  = types.Digest("bbb8d94852dfde3f3bebcc000be60153")
	gammaPositiveDigest = types.Digest("ccc84ad6f1a0c628d5f27180e497309e")
	untriagedDigest     = types.Digest("7bf4d4e913605c0781697df4004191c5")

	testName = types.TestName("some_test")
)

// makeDiffMetric makes a DiffMetrics object with
// a combined diff metric of n. All other data is
// based off of n, but not technically accurate.
func makeDiffMetric(n int) *diff.DiffMetrics {
	return &diff.DiffMetrics{
		NumDiffPixels:    n * 100,
		PixelDiffPercent: float32(n) / 10.0,
		MaxRGBADiffs:     [4]int{3 * n, 2 * n, n, n},
		DimDiffer:        false,
		Diffs: map[string]float32{
			diff.CombinedMetric: float32(n),
			"percent":           float32(n) / 10.0,
			"pixel":             float32(n) * 100,
		},
	}
}

// makeAlphaParamSet returns the ParamSet for the alphaPositiveDigest
func makeAlphaParamSet() paramtools.ParamSet {
	return paramtools.ParamSet{
		"arch": []string{"z80"},
		"name": []string{string(testName)},
		"os":   []string{"Texas Instruments"},
	}
}

// makeBetaParamSet returns the ParamSet for the betaPositiveDigest
func makeBetaParamSet() paramtools.ParamSet {
	return paramtools.ParamSet{
		"arch": []string{"x64"},
		"name": []string{string(testName)},
		"os":   []string{"Android"},
	}
}

// makeGammaParamSet returns the ParamSet for the gammaPositiveDigest
func makeGammaParamSet() paramtools.ParamSet {
	// This means that both the arm and x86 bot drew the same thing
	// for the given test.
	return paramtools.ParamSet{
		"arch": []string{"arm", "x86"},
		"name": []string{string(testName)},
		"os":   []string{"Android"},
	}
}

// makeUntriagedParamSet returns the ParamSet for the untriagedDigest
func makeUntriagedParamSet() paramtools.ParamSet {
	return paramtools.ParamSet{
		"arch":                []string{"x86"},
		types.PrimaryKeyField: []string{string(testName)},
		"os":                  []string{"iPhone 38 Maxx"},
	}
}

// makeExpClassifier returns a Classifier which has two positive entries and one negative one.
func makeExpClassifier() expectations.Classifier {
	var expOne expectations.Expectations
	expOne.Set(testName, alphaPositiveDigest, expectations.Positive)
	expOne.Set(testName, gammaPositiveDigest, expectations.Positive)

	var expTwo expectations.Expectations
	expTwo.Set(testName, betaNegativeDigest, expectations.Negative)
	return expectations.Join(&expOne, &expTwo)
}
