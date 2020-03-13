package fs_expectationstore

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/expectations"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
)

// TestGet_ExpectationsInMasterPartition_Success writes some changes, one of which overwrites a
// previous expectation and asserts that we can call Get to extract the correct output.
func TestGet_ExpectationsInMasterPartition_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New2(c, nil, ReadWrite)

	// Brand new instance should have no expectations
	e, err := f.Get(ctx)
	require.NoError(t, err)
	require.True(t, e.Empty())

	err = f.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaUntriagedDigest,
			Label:    expectations.Positive,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaPositiveDigest,
			Label:    expectations.Positive,
		},
	}, userOne)
	require.NoError(t, err)

	err = f.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
			Label:    expectations.Negative,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaUntriagedDigest, // overwrites previous
			Label:    expectations.Untriaged,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaPositiveDigest,
			Label:    expectations.Positive,
		},
	}, userTwo)
	require.NoError(t, err)

	e, err = f.Get(ctx)
	require.NoError(t, err)
	assertExpectationsMatchDefaults(t, e)
	// Make sure that if we create a new view, we can still read the results.
	fr := New2(c, nil, ReadOnly)
	e, err = fr.Get(ctx)
	require.NoError(t, err)
	assertExpectationsMatchDefaults(t, e)
}

func assertExpectationsMatchDefaults(t *testing.T, e expectations.ReadOnly) {
	assert.Equal(t, expectations.Positive, e.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.Negative, e.Classification(data.AlphaTest, data.AlphaNegativeDigest))
	assert.Equal(t, expectations.Untriaged, e.Classification(data.AlphaTest, data.AlphaUntriagedDigest))
	assert.Equal(t, expectations.Positive, e.Classification(data.BetaTest, data.BetaPositiveDigest))
	assert.Equal(t, expectations.Untriaged, e.Classification(data.BetaTest, data.BetaUntriagedDigest))
	assert.Equal(t, 3, e.Len())
}

// TestInitialize_ExpectationCacheIsFilledAndUpdated_Success has both a read-write and a read-only
// version and makes sure that the changes to the read-write version eventually propagate to the
// read-only version via the snapshots.
func TestInitialize_ExpectationCacheIsFilledAndUpdated_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()
	const firstPositiveThenUntriaged = types.Digest("abcd")

	f := New2(c, nil, ReadWrite)
	putEntry(ctx, t, f, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)
	putEntry(ctx, t, f, data.AlphaTest, data.AlphaNegativeDigest, expectations.Negative, userOne)
	putEntry(ctx, t, f, data.AlphaTest, firstPositiveThenUntriaged, expectations.Positive, userOne)

	ro := New2(c, nil, ReadOnly)
	assert.Empty(t, ro.entryCache)
	assert.False(t, ro.hasSnapshotsRunning)

	require.NoError(t, ro.Initialize(ctx))

	assert.True(t, ro.hasSnapshotsRunning)
	// Check that the read-only copy has been loaded with the existing 3 entries as a result of
	// the Initialize method.
	assert.Len(t, ro.entryCache, 3)
	e, err := ro.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, expectations.Positive, e.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.Negative, e.Classification(data.AlphaTest, data.AlphaNegativeDigest))
	assert.Equal(t, expectations.Positive, e.Classification(data.AlphaTest, firstPositiveThenUntriaged))

	// This should update the existing entry, leaving us with 4 total entries, not 5
	putEntry(ctx, t, f, data.AlphaTest, firstPositiveThenUntriaged, expectations.Untriaged, userOne)
	putEntry(ctx, t, f, data.BetaTest, data.BetaPositiveDigest, expectations.Positive, userOne)

	assert.Eventually(t, func() bool {
		ro.entryCacheMutex.RLock()
		defer ro.entryCacheMutex.RUnlock()
		return len(ro.entryCache) == 4
	}, 10*time.Second, 100*time.Millisecond)

	e2, err := ro.Get(ctx)
	require.NoError(t, err)
	assertExpectationsMatchDefaults(t, e2)
	assert.Equal(t, expectations.Untriaged, e2.Classification(data.AlphaTest, firstPositiveThenUntriaged))
	// Spot check that the expectations we got first were not impacted by the new expectations
	// coming in or the future call to Get.
	assert.Equal(t, expectations.Positive, e.Classification(data.AlphaTest, firstPositiveThenUntriaged))
}

// TestAddChange_FromManyGoroutines_Success writes a bunch of data from many go routines in an
// effort to catch any race conditions in the caching layer.
func TestAddChange_FromManyGoroutines_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New2(c, nil, ReadWrite)
	require.NoError(t, f.Initialize(ctx))

	entries := []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaUntriagedDigest,
			Label:    expectations.Untriaged,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
			Label:    expectations.Negative,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaPositiveDigest,
			Label:    expectations.Positive,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaPositiveDigest,
			Label:    expectations.Positive,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaUntriagedDigest,
			Label:    expectations.Untriaged,
		},
	}

	wg := sync.WaitGroup{}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e := entries[i%len(entries)]
			err := f.AddChange(ctx, []expectations.Delta{e}, userOne)
			require.NoError(t, err)
		}(i)

		// Make sure we can read and write w/o races
		if i%5 == 0 {
			_, err := f.Get(ctx)
			require.NoError(t, err)
		}
	}

	wg.Wait()

	e, err := f.Get(ctx)
	require.NoError(t, err)
	assertExpectationsMatchDefaults(t, e)
}

// TestGetTriageHistory_RepeatedlyOverwriteOneEntry_Success repeatedly overwrites a single entry to make sure the cache
// reflects reality and that our history is complete
func TestGetTriageHistory_RepeatedlyOverwriteOneEntry_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New2(c, nil, ReadWrite)
	require.NoError(t, f.Initialize(ctx))

	now := time.Date(2020, time.March, 1, 2, 3, 57, 0, time.UTC)
	f.now = func() time.Time {
		return now
	}

	theEntry := expectations.ID{Grouping: data.AlphaTest, Digest: data.AlphaPositiveDigest}

	// This will wait for the firestore query snapshots to update the cache to have the entry we care
	// about to have the given label.
	waitForCacheToBe := func(label expectations.Label) {
		require.Eventually(t, func() bool {
			f.entryCacheMutex.RLock()
			defer f.entryCacheMutex.RUnlock()
			if len(f.entryCache) != 1 {
				return false
			}
			actualEntry := f.entryCache[theEntry]
			// Make sure we don't append to Ranges (since we are currently overwriting at master).
			assert.Len(t, actualEntry.Ranges, 1)
			return actualEntry.Ranges[0].Label == label
		}, 10*time.Second, 100*time.Millisecond)
	}

	putEntry(ctx, t, f, theEntry.Grouping, theEntry.Digest, expectations.Positive, userOne)
	waitForCacheToBe(expectations.Positive)

	now = now.Add(time.Minute)
	putEntry(ctx, t, f, theEntry.Grouping, theEntry.Digest, expectations.Negative, userOne)
	waitForCacheToBe(expectations.Negative)

	now = now.Add(time.Minute)
	putEntry(ctx, t, f, theEntry.Grouping, theEntry.Digest, expectations.Untriaged, userTwo)
	waitForCacheToBe(expectations.Untriaged)

	now = now.Add(time.Minute)
	putEntry(ctx, t, f, theEntry.Grouping, theEntry.Digest, expectations.Positive, userTwo)
	waitForCacheToBe(expectations.Positive)

	xth, err := f.GetTriageHistory(ctx, theEntry.Grouping, theEntry.Digest)
	require.NoError(t, err)
	assert.Equal(t, []expectations.TriageHistory{
		{
			User: userTwo,
			TS:   time.Date(2020, time.March, 1, 2, 6, 57, 0, time.UTC),
		}, {
			User: userTwo,
			TS:   time.Date(2020, time.March, 1, 2, 5, 57, 0, time.UTC),
		}, {
			User: userOne,
			TS:   time.Date(2020, time.March, 1, 2, 4, 57, 0, time.UTC),
		}, {
			User: userOne,
			TS:   time.Date(2020, time.March, 1, 2, 3, 57, 0, time.UTC),
		},
	}, xth)
}

func putEntry(ctx context.Context, t *testing.T, f *Store2, name types.TestName, digest types.Digest, label expectations.Label, user string) {
	require.NoError(t, f.AddChange(ctx, []expectations.Delta{
		{
			Grouping: name,
			Digest:   digest,
			Label:    label,
		},
	}, user))
}

// makeTestFirestoreClient returns a firestore.Client and a context.Context. When the third return
// value is called, the Context will be cancelled and the Client will be cleaned up.
func makeTestFirestoreClient(t *testing.T) (*firestore.Client, context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	c, cleanup := firestore.NewClientForTesting(ctx, t)
	return c, ctx, func() {
		cancel()
		cleanup()
	}
}
