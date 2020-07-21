package warmer

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/summary"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
)

// TestPrecomputeDiffsSunnyDay tests a typical call of PrecomputeDiffs in which
// each test has one untriaged image.
func TestPrecomputeDiffsSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mdf := &mocks.ClosestDiffFinder{}
	defer mdf.AssertExpectations(t)

	byTest := map[types.TestName]digest_counter.DigestCount{
		data.AlphaTest: {
			data.AlphaPositiveDigest:  2,
			data.AlphaNegativeDigest:  6,
			data.AlphaUntriagedDigest: 1,
		},
		data.BetaTest: {
			data.BetaPositiveDigest:  6,
			data.BetaUntriagedDigest: 1,
		},
	}

	mdf.On("Precompute", testutils.AnyContext).Return(nil).Once()

	// Can return nil because warmer shouldn't care about what is actually the closest.
	mdf.On("ClosestDigest", testutils.AnyContext, data.AlphaTest, data.AlphaUntriagedDigest, expectations.PositiveStr).Return(nil, nil).Once()
	mdf.On("ClosestDigest", testutils.AnyContext, data.AlphaTest, data.AlphaUntriagedDigest, expectations.NegativeStr).Return(nil, nil).Once()
	mdf.On("ClosestDigest", testutils.AnyContext, data.BetaTest, data.BetaUntriagedDigest, expectations.PositiveStr).Return(nil, nil).Once()
	mdf.On("ClosestDigest", testutils.AnyContext, data.BetaTest, data.BetaUntriagedDigest, expectations.NegativeStr).Return(nil, nil).Once()

	w := New()
	wd := Data{
		TestSummaries: makeComputedSummaries(),
		DigestsByTest: byTest,
		SubsetOfTests: nil,
	}
	require.NoError(t, w.PrecomputeDiffs(context.Background(), wd, mdf))
}

// TestPrecomputeDiffsErrors tests to see if we keep going after some diffstore errors happen
// (maybe something transient with GCS)
func TestPrecomputeDiffsErrors(t *testing.T) {
	unittest.SmallTest(t)

	mdf := &mocks.ClosestDiffFinder{}
	defer mdf.AssertExpectations(t)

	byTest := map[types.TestName]digest_counter.DigestCount{
		data.AlphaTest: {
			data.AlphaPositiveDigest:  2,
			data.AlphaNegativeDigest:  6,
			data.AlphaUntriagedDigest: 1,
		},
		data.BetaTest: {
			data.BetaPositiveDigest:  6,
			data.BetaUntriagedDigest: 1,
		},
	}

	mdf.On("Precompute", testutils.AnyContext).Return(nil).Once()

	// Can return nil because warmer shouldn't care about what is actually the closest.
	mdf.On("ClosestDigest", testutils.AnyContext, data.AlphaTest, data.AlphaUntriagedDigest, expectations.PositiveStr).Return(nil, nil).Once()
	mdf.On("ClosestDigest", testutils.AnyContext, data.AlphaTest, data.AlphaUntriagedDigest, expectations.NegativeStr).Return(nil, errors.New("transient gcs error")).Once()
	mdf.On("ClosestDigest", testutils.AnyContext, data.BetaTest, data.BetaUntriagedDigest, expectations.PositiveStr).Return(nil, nil).Once()
	mdf.On("ClosestDigest", testutils.AnyContext, data.BetaTest, data.BetaUntriagedDigest, expectations.NegativeStr).Return(nil, errors.New("sentient AI error")).Once()

	w := New()
	wd := Data{
		TestSummaries: makeComputedSummaries(),
		DigestsByTest: byTest,
		SubsetOfTests: nil,
	}
	err := w.PrecomputeDiffs(context.Background(), wd, mdf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "and 1 other error")
}

// TestPrecomputeDiffsContextError tests to see if we stop with cancelled context.
func TestPrecomputeDiffsContextError(t *testing.T) {
	unittest.SmallTest(t)

	mdf := &mocks.ClosestDiffFinder{}
	defer mdf.AssertExpectations(t)

	mdf.On("Precompute", testutils.AnyContext).Return(nil).Once()

	w := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	wd := Data{
		TestSummaries: makeComputedSummaries(),
		DigestsByTest: nil, // should be unused
		SubsetOfTests: nil, // should be unused
	}
	err := w.PrecomputeDiffs(ctx, wd, mdf)
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

// TestPrecomputeDiffsTestName tests a partial update scenario. An example would be
// when the expectations change and only some of the digests need to be recalculated.
func TestPrecomputeDiffsTestName(t *testing.T) {
	unittest.SmallTest(t)

	mdf := &mocks.ClosestDiffFinder{}
	defer mdf.AssertExpectations(t)

	byTest := map[types.TestName]digest_counter.DigestCount{
		data.AlphaTest: {
			data.AlphaPositiveDigest:  2,
			data.AlphaNegativeDigest:  6,
			data.AlphaUntriagedDigest: 1,
		},
		data.BetaTest: {
			data.BetaPositiveDigest:  6,
			data.BetaUntriagedDigest: 1,
		},
	}

	mdf.On("Precompute", testutils.AnyContext).Return(nil).Once()

	// Can return nil because warmer shouldn't care about what is actually the closest.
	// Should not call ClosestDigest on AlphaTest because only BetaTest is in testNames.
	mdf.On("ClosestDigest", testutils.AnyContext, data.BetaTest, data.BetaUntriagedDigest, expectations.PositiveStr).Return(nil, nil).Once()
	mdf.On("ClosestDigest", testutils.AnyContext, data.BetaTest, data.BetaUntriagedDigest, expectations.NegativeStr).Return(nil, nil).Once()

	w := New()
	wd := Data{
		TestSummaries: makeComputedSummaries(),
		DigestsByTest: byTest,
		SubsetOfTests: types.TestNameSet{data.BetaTest: true},
	}
	require.NoError(t, w.PrecomputeDiffs(context.Background(), wd, mdf))
}

func makeComputedSummaries() []*summary.TriageStatus {
	return []*summary.TriageStatus{
		{
			Name:      data.AlphaTest,
			Untriaged: 1,
			UntHashes: types.DigestSlice{data.AlphaUntriagedDigest},
			// warmer doesn't care about elided fields
		},
		{
			Name:      data.BetaTest,
			Untriaged: 1,
			UntHashes: types.DigestSlice{data.BetaUntriagedDigest},
		},
	}
}
