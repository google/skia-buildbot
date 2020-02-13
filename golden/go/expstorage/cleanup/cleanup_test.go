package cleanup

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/expstorage/mocks"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/paramsets"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/types/expectations"
)

func TestUpdate_ThreeDevicesCorpus(t *testing.T) {
	unittest.SmallTest(t)

	fis := makeThreeDevicesIndex()
	now := time.Date(2020, time.February, 14, 15, 16, 17, 0, time.UTC)

	// Make sure we call cleaner.SetUsed with only triaged inputs.
	mc := &mocks.Cleaner{}
	defer mc.AssertExpectations(t)
	expectedDeltas := []expstorage.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaBad1Digest,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaGood1Digest,
		},
	}
	deltaMatcher := mock.MatchedBy(func(deltas []expstorage.Delta) bool {
		sortDeltas(deltas)
		sortDeltas(expectedDeltas)
		assert.Equal(t, expectedDeltas, deltas)
		return true
	})
	mc.On("SetUsed", testutils.AnyContext, deltaMatcher, now).Return(nil)

	err := update(context.Background(), fis, mc, data.MakeTestExpectations(), now)
	require.NoError(t, err)
}

func TestUpdate_NoMaxAgesSet_OnlyUntriagedPruned(t *testing.T) {
	unittest.SmallTest(t)

	// Make sure we call cleaner.DeleteEntriesUnusedAndModifiedBefore as expected.
	mc := &mocks.Cleaner{}
	defer mc.AssertExpectations(t)

	now := time.Date(2020, time.February, 14, 15, 16, 17, 0, time.UTC)
	futureTime := mock.MatchedBy(func(ts time.Time) bool {
		return now.Before(ts)
	})
	mc.On("DeleteEntriesUnusedAndModifiedBefore", testutils.AnyContext, expectations.Untriaged, futureTime).Return(0, nil)

	err := cleanup(context.Background(), mc, 0, 0, now)
	require.NoError(t, err)
}

func TestUpdate_NegativeMaxAgesSet_OnlyUntriagedPruned(t *testing.T) {
	unittest.SmallTest(t)

	// Make sure we call cleaner.DeleteEntriesUnusedAndModifiedBefore as expected.
	mc := &mocks.Cleaner{}
	defer mc.AssertExpectations(t)

	now := time.Date(2020, time.February, 14, 15, 16, 17, 0, time.UTC)
	futureTime := mock.MatchedBy(func(ts time.Time) bool {
		return now.Before(ts)
	})
	mc.On("DeleteEntriesUnusedAndModifiedBefore", testutils.AnyContext, expectations.Untriaged, futureTime).Return(0, nil)

	err := cleanup(context.Background(), mc, -time.Minute, -time.Minute, now)
	require.NoError(t, err)
}

func TestUpdate_MaxPosAgeSet_OnlyUntriagedPruned(t *testing.T) {
	unittest.SmallTest(t)

	// Make sure we call cleaner.DeleteEntriesUnusedAndModifiedBefore as expected.
	mc := &mocks.Cleaner{}
	defer mc.AssertExpectations(t)

	now := time.Date(2020, time.February, 14, 15, 16, 17, 0, time.UTC)
	futureTime := mock.MatchedBy(func(ts time.Time) bool {
		return now.Before(ts)
	})
	oneHourAgo := mock.MatchedBy(func(ts time.Time) bool {
		return now.Sub(ts) == time.Hour
	})
	mc.On("DeleteEntriesUnusedAndModifiedBefore", testutils.AnyContext, expectations.Positive, oneHourAgo).Return(0, nil)
	mc.On("DeleteEntriesUnusedAndModifiedBefore", testutils.AnyContext, expectations.Untriaged, futureTime).Return(0, nil)

	err := cleanup(context.Background(), mc, time.Hour, 0, now)
	require.NoError(t, err)
}

func TestUpdate_MaxNegAgeSet_OnlyUntriagedPruned(t *testing.T) {
	unittest.SmallTest(t)

	// Make sure we call cleaner.DeleteEntriesUnusedAndModifiedBefore as expected.
	mc := &mocks.Cleaner{}
	defer mc.AssertExpectations(t)

	now := time.Date(2020, time.February, 14, 15, 16, 17, 0, time.UTC)
	futureTime := mock.MatchedBy(func(ts time.Time) bool {
		return now.Before(ts)
	})
	twoHoursAgo := mock.MatchedBy(func(ts time.Time) bool {
		return now.Sub(ts) == 2*time.Hour
	})
	mc.On("DeleteEntriesUnusedAndModifiedBefore", testutils.AnyContext, expectations.Negative, twoHoursAgo).Return(0, nil)
	mc.On("DeleteEntriesUnusedAndModifiedBefore", testutils.AnyContext, expectations.Untriaged, futureTime).Return(0, nil)

	err := cleanup(context.Background(), mc, 0, 2*time.Hour, now)
	require.NoError(t, err)
}

// makeThreeDevicesIndex returns a minimal search index corresponding to the three_devices_data
// (which currently has nothing ignored).
func makeThreeDevicesIndex() *indexer.SearchIndex {
	cpxTile := types.NewComplexTile(data.MakeTestTile())
	dc := digest_counter.New(data.MakeTestTile())
	si, err := indexer.SearchIndexForTesting(cpxTile, [2]digest_counter.DigestCounter{dc, dc}, [2]paramsets.ParamSummary{}, nil, nil)
	if err != nil {
		// Something is horribly broken with our test data/setup
		panic(err.Error())
	}
	return si
}

func sortDeltas(deltas []expstorage.Delta) {
	sort.Slice(deltas, func(i, j int) bool {
		if deltas[i].Grouping < deltas[j].Grouping {
			return true
		} else if deltas[i].Grouping == deltas[j].Grouping {
			return deltas[i].Digest < deltas[j].Digest
		}
		return false
	})
}
