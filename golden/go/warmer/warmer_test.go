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
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/summary"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/types/expectations"
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

	mdf.On("Precompute", testutils.AnyContext).Return(nil).Once()

	// Can return nil because warmer shouldn't care about what is actually the closest.
	mdf.On("ClosestDigest", testutils.AnyContext, data.AlphaTest, data.AlphaUntriaged1Digest, expectations.Positive).Return(nil, nil).Once()
	mdf.On("ClosestDigest", testutils.AnyContext, data.AlphaTest, data.AlphaUntriaged1Digest, expectations.Negative).Return(nil, nil).Once()
	mdf.On("ClosestDigest", testutils.AnyContext, data.BetaTest, data.BetaUntriaged1Digest, expectations.Positive).Return(nil, nil).Once()
	mdf.On("ClosestDigest", testutils.AnyContext, data.BetaTest, data.BetaUntriaged1Digest, expectations.Negative).Return(nil, nil).Once()

	w := New()
	require.NoError(t, w.PrecomputeDiffs(context.Background(), makeComputedSummaries(), nil, mdc, mdf))
}

// TestPrecomputeDiffsErrors tests to see if we keep going after some diffstore errors happen
// (maybe something transient with GCS)
func TestPrecomputeDiffsErrors(t *testing.T) {
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

	mdf.On("Precompute", testutils.AnyContext).Return(nil).Once()

	// Can return nil because warmer shouldn't care about what is actually the closest.
	mdf.On("ClosestDigest", testutils.AnyContext, data.AlphaTest, data.AlphaUntriaged1Digest, expectations.Positive).Return(nil, nil).Once()
	mdf.On("ClosestDigest", testutils.AnyContext, data.AlphaTest, data.AlphaUntriaged1Digest, expectations.Negative).Return(nil, errors.New("transient gcs error")).Once()
	mdf.On("ClosestDigest", testutils.AnyContext, data.BetaTest, data.BetaUntriaged1Digest, expectations.Positive).Return(nil, nil).Once()
	mdf.On("ClosestDigest", testutils.AnyContext, data.BetaTest, data.BetaUntriaged1Digest, expectations.Negative).Return(nil, errors.New("sentient AI error")).Once()

	w := New()
	err := w.PrecomputeDiffs(context.Background(), makeComputedSummaries(), nil, mdc, mdf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "and 1 other error")
}

// TestPrecomputeDiffsContextError tests to see if we stop with cancelled context.
func TestPrecomputeDiffsContextError(t *testing.T) {
	unittest.SmallTest(t)

	mdc := &mocks.DigestCounter{}
	mdf := &mocks.ClosestDiffFinder{}
	defer mdc.AssertExpectations(t)
	defer mdf.AssertExpectations(t)

	mdf.On("Precompute", testutils.AnyContext).Return(nil).Once()

	// No calls to ClosestDigest, since we have a cancelled context.

	w := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := w.PrecomputeDiffs(ctx, makeComputedSummaries(), nil, mdc, mdf)
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
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

	mdf.On("Precompute", testutils.AnyContext).Return(nil).Once()

	// Can return nil because warmer shouldn't care about what is actually the closest.
	// Should not call ClosestDigest on AlphaTest because only BetaTest is in testNames.
	mdf.On("ClosestDigest", testutils.AnyContext, data.BetaTest, data.BetaUntriaged1Digest, expectations.Positive).Return(nil, nil).Once()
	mdf.On("ClosestDigest", testutils.AnyContext, data.BetaTest, data.BetaUntriaged1Digest, expectations.Negative).Return(nil, nil).Once()

	w := New()
	require.NoError(t, w.PrecomputeDiffs(context.Background(), makeComputedSummaries(), types.TestNameSet{data.BetaTest: true}, mdc, mdf))
}

func makeComputedSummaries() []*summary.TriageStatus {
	return []*summary.TriageStatus{
		{
			Name:      data.AlphaTest,
			Untriaged: 1,
			UntHashes: types.DigestSlice{data.AlphaUntriaged1Digest},
			// warmer doesn't care about elided fields
		},
		{
			Name:      data.BetaTest,
			Untriaged: 1,
			UntHashes: types.DigestSlice{data.BetaUntriaged1Digest},
		},
	}
}
