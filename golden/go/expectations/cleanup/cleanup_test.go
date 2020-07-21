package cleanup

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/expectations"
	mock_expectations "go.skia.org/infra/golden/go/expectations/mocks"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
)

func TestStart_InvalidPolicy_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	invalidPolicy := Policy{
		PositiveMaxLastUsed: -time.Minute,
	}
	err := Start(context.Background(), nil, nil, nil, invalidPolicy)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "negative")
}

func TestStart_CancelledContex_DoesNothing(t *testing.T) {
	unittest.SmallTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Start(ctx, nil, nil, nil, Policy{})
	require.NoError(t, err)
}

func TestUpdate_OnlyUpdateTriagedDigests(t *testing.T) {
	unittest.SmallTest(t)

	now := time.Date(2020, time.February, 14, 15, 16, 17, 0, time.UTC)

	// Make sure we call GarbageCollector.UpdateLastUsed with only triaged inputs.
	mc := &mock_expectations.GarbageCollector{}
	defer mc.AssertExpectations(t)
	// Notice there are no references to Untriaged digests here even though they are in the input
	// data.
	expectedIDs := []expectations.ID{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaPositiveDigest,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaPositiveDigest,
		},
	}
	idMatcher := mock.MatchedBy(func(ids []expectations.ID) bool {
		// The order doesn't matter when calling into UpdateLastUsed.
		assert.ElementsMatch(t, expectedIDs, ids)
		return true
	})
	mc.On("UpdateLastUsed", testutils.AnyContext, idMatcher, now).Return(nil)

	err := update(context.Background(), makeThreeDevicesDigestCounterByTest(), mc, data.MakeTestExpectations(), now)
	require.NoError(t, err)
}

func TestUpdate_EverythingUntriaged_UpdateNothing(t *testing.T) {
	unittest.SmallTest(t)

	now := time.Date(2020, time.February, 14, 15, 16, 17, 0, time.UTC)

	// We expect no calls to mc because everything is untriaged.
	mc := &mock_expectations.GarbageCollector{}

	// By passing EmptyClassifier to the test, all digests will be considered untriaged.
	err := update(context.Background(), makeThreeDevicesDigestCounterByTest(), mc, expectations.EmptyClassifier(), now)
	require.NoError(t, err)
}

func TestCleanup_NoPolicySet_OnlyGarbageCollect(t *testing.T) {
	unittest.SmallTest(t)

	// Make sure we call GarbageCollector.GarbageCollect as expected.
	mc := &mock_expectations.GarbageCollector{}
	defer mc.AssertExpectations(t)

	mc.On("GarbageCollect", testutils.AnyContext).Return(0, nil)

	now := time.Date(2020, time.February, 14, 15, 16, 17, 0, time.UTC)
	noGCPolicy := Policy{}
	err := cleanup(context.Background(), mc, noGCPolicy, now)
	require.NoError(t, err)
}

// TestCleanup_InvalidPolicySet_OnlyGarbageCollect makes sure that if an invalid policy makes it
// into the cleanup function, we still do the right thing (and only GarbageCollect).
func TestCleanup_InvalidPolicySet_OnlyGarbageCollect(t *testing.T) {
	unittest.SmallTest(t)

	// Make sure we call GarbageCollector.GarbageCollect as expected.
	mc := &mock_expectations.GarbageCollector{}
	defer mc.AssertExpectations(t)

	mc.On("GarbageCollect", testutils.AnyContext).Return(0, nil)

	now := time.Date(2020, time.February, 14, 15, 16, 17, 0, time.UTC)
	invalidPolicy := Policy{
		PositiveMaxLastUsed: -time.Minute,
		NegativeMaxLastUsed: -time.Minute,
	}
	require.Error(t, invalidPolicy.Validate())
	err := cleanup(context.Background(), mc, invalidPolicy, now)
	require.NoError(t, err)
}

func TestCleanup_PositiveDigestPolicy_MarkPositiveForGCAndGarbageCollect(t *testing.T) {
	unittest.SmallTest(t)

	// Make sure we call GarbageCollector as expected.
	mc := &mock_expectations.GarbageCollector{}
	defer mc.AssertExpectations(t)

	now := time.Date(2020, time.February, 14, 15, 16, 17, 0, time.UTC)
	oneHourAgo := mock.MatchedBy(func(ts time.Time) bool {
		return now.Sub(ts) == time.Hour
	})
	mc.On("MarkUnusedEntriesForGC", testutils.AnyContext, expectations.PositiveInt, oneHourAgo).Return(0, nil)
	mc.On("GarbageCollect", testutils.AnyContext).Return(0, nil)

	positiveOnlyPolicy := Policy{
		PositiveMaxLastUsed: time.Hour,
	}
	err := cleanup(context.Background(), mc, positiveOnlyPolicy, now)
	require.NoError(t, err)
}

func TestCleanup_NegativeDigestPolicy_MarkNegativeForGCAndGarbageCollect(t *testing.T) {
	unittest.SmallTest(t)

	// Make sure we call GarbageCollector as expected.
	mc := &mock_expectations.GarbageCollector{}
	defer mc.AssertExpectations(t)

	now := time.Date(2020, time.February, 14, 15, 16, 17, 0, time.UTC)
	twoHoursAgo := mock.MatchedBy(func(ts time.Time) bool {
		return now.Sub(ts) == 2*time.Hour
	})
	mc.On("MarkUnusedEntriesForGC", testutils.AnyContext, expectations.NegativeInt, twoHoursAgo).Return(0, nil)
	mc.On("GarbageCollect", testutils.AnyContext).Return(0, nil)

	negativeOnlyPolicy := Policy{
		NegativeMaxLastUsed: 2 * time.Hour,
	}
	err := cleanup(context.Background(), mc, negativeOnlyPolicy, now)
	require.NoError(t, err)
}

func TestCleanup_PositiveAndNegativePolicy_BothMarkedForGCAndGarbageCollect(t *testing.T) {
	unittest.SmallTest(t)

	// Make sure we call GarbageCollector as expected.
	mc := &mock_expectations.GarbageCollector{}
	defer mc.AssertExpectations(t)

	now := time.Date(2020, time.February, 14, 15, 16, 17, 0, time.UTC)
	twoHoursAgo := mock.MatchedBy(func(ts time.Time) bool {
		return now.Sub(ts) == 2*time.Hour
	})
	oneHourAgo := mock.MatchedBy(func(ts time.Time) bool {
		return now.Sub(ts) == time.Hour
	})
	mc.On("MarkUnusedEntriesForGC", testutils.AnyContext, expectations.NegativeInt, twoHoursAgo).Return(0, nil)
	mc.On("MarkUnusedEntriesForGC", testutils.AnyContext, expectations.PositiveInt, oneHourAgo).Return(0, nil)
	mc.On("GarbageCollect", testutils.AnyContext).Return(0, nil)

	policy := Policy{
		PositiveMaxLastUsed: 1 * time.Hour,
		NegativeMaxLastUsed: 2 * time.Hour,
	}
	err := cleanup(context.Background(), mc, policy, now)
	require.NoError(t, err)
}

// makeThreeDevicesDigestCounterByTest returns a hard-coded version of what the SearchIndex.ByTest()
// would return for the three_devices test corpus.
func makeThreeDevicesDigestCounterByTest() map[types.TestName]digest_counter.DigestCount {
	return map[types.TestName]digest_counter.DigestCount{
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
}
