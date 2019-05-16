package warmer

import (
	"testing"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/summary"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
)

// TestPrecomputeDiffsSunnyDay tests a typical call of PrecomputeDiffs in which
// each test has one untriaged image.
func TestPrecomputeDiffsSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mdc := &mocks.DigestCounter{}
	mdf := &mocks.ClosestDiffFinder{}
	defer mdc.AssertExpectations(t)
	defer mdf.AssertExpectations(t)

	byTest := map[types.TestName]digest_counter.DigestCount{
		data.AlphaTest: {
			data.AlphaGood1Digest:      2,
			data.AlphaBad1Digest:       6,
			data.AlphaUntriaged1Digest: 1,
		},
		data.BetaTest: {
			data.BetaGood1Digest:      6,
			data.BetaUntriaged1Digest: 1,
		},
	}

	mdc.On("ByTest").Return(byTest)

	mdf.On("Precompute").Once()

	// Can return nil because warmer shouldn't care about what is actually the closest.
	mdf.On("ClosestDigest", data.AlphaTest, data.AlphaUntriaged1Digest, types.POSITIVE).Return(nil).Once()
	mdf.On("ClosestDigest", data.AlphaTest, data.AlphaUntriaged1Digest, types.NEGATIVE).Return(nil).Once()
	mdf.On("ClosestDigest", data.BetaTest, data.BetaUntriaged1Digest, types.POSITIVE).Return(nil).Once()
	mdf.On("ClosestDigest", data.BetaTest, data.BetaUntriaged1Digest, types.NEGATIVE).Return(nil).Once()

	sm := summary.SummaryMap{
		data.AlphaTest: &summary.Summary{
			Name:      data.AlphaTest,
			Untriaged: 1,
			UntHashes: types.DigestSlice{data.AlphaUntriaged1Digest},
			// warmer doesn't care about elided fields
		},
		data.BetaTest: &summary.Summary{
			Name:      data.BetaTest,
			Untriaged: 1,
			UntHashes: types.DigestSlice{data.BetaUntriaged1Digest},
		},
	}

	w := New()
	w.PrecomputeDiffs(sm, nil, mdc, mdf)
}

// TestPrecomputeDiffsTestName tests a partial update scenario. An example would be
// when the expectations change and only some of the digests need to be recalculated.
func TestPrecomputeDiffsTestName(t *testing.T) {
	unittest.SmallTest(t)

	mdc := &mocks.DigestCounter{}
	mdf := &mocks.ClosestDiffFinder{}
	defer mdc.AssertExpectations(t)
	defer mdf.AssertExpectations(t)

	byTest := map[types.TestName]digest_counter.DigestCount{
		data.AlphaTest: {
			data.AlphaGood1Digest:      2,
			data.AlphaBad1Digest:       6,
			data.AlphaUntriaged1Digest: 1,
		},
		data.BetaTest: {
			data.BetaGood1Digest:      6,
			data.BetaUntriaged1Digest: 1,
		},
	}

	mdc.On("ByTest").Return(byTest)

	mdf.On("Precompute").Once()

	// Can return nil because warmer shouldn't care about what is actually the closest.
	// Should not call ClosestDigest on AlphaTest because only BetaTest is in testNames.
	mdf.On("ClosestDigest", data.BetaTest, data.BetaUntriaged1Digest, types.POSITIVE).Return(nil).Once()
	mdf.On("ClosestDigest", data.BetaTest, data.BetaUntriaged1Digest, types.NEGATIVE).Return(nil).Once()

	sm := summary.SummaryMap{
		data.AlphaTest: &summary.Summary{
			Name:      data.AlphaTest,
			Untriaged: 1,
			UntHashes: types.DigestSlice{data.AlphaUntriaged1Digest},
			// warmer doesn't care about elided fields
		},
		data.BetaTest: &summary.Summary{
			Name:      data.BetaTest,
			Untriaged: 1,
			UntHashes: types.DigestSlice{data.BetaUntriaged1Digest},
		},
	}

	w := New()
	w.PrecomputeDiffs(sm, types.TestNameSet{data.BetaTest: true}, mdc, mdf)
}
