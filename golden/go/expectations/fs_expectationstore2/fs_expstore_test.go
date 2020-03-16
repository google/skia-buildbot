package fs_expectationstore

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/expectations"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
)

// TestExpectationEntryID_ReplacesInvalidCharacters tests edge cases for malformed names.
func TestExpectationEntryID_ReplacesInvalidCharacters(t *testing.T) {
	unittest.SmallTest(t)
	// Based on real data
	e := expectationEntry{
		Grouping: "downsample/images/mandrill_512.png",
		Digest:   "36bc7da524f2869c97f0a0f1d7042110",
	}
	assert.Equal(t, "downsample-images-mandrill_512.png|36bc7da524f2869c97f0a0f1d7042110",
		e.ID())
}

// TestGet_ExpectationsInCLPartition_Success writes some changes, one of which overwrites a
// previous expectation and asserts that we can call Get to extract the correct output.
func TestGet_ExpectationsInCLPartition_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()
	// These are arbitrary
	const clID = "123"
	const crs = "github"
	master := New(c, nil, ReadWrite)
	f := master.ForChangeList(clID, crs)

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
	master = New(c, nil, ReadOnly)
	fr := master.ForChangeList(clID, crs)
	e, err = fr.Get(ctx)
	require.NoError(t, err)
	assertExpectationsMatchDefaults(t, e)
}

// TestGet_ExpectationsInMasterPartition_Success writes some changes, one of which overwrites a
// previous expectation and asserts that we can call Get to extract the correct output.
func TestGet_ExpectationsInMasterPartition_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New(c, nil, ReadWrite)
	require.NoError(t, f.Initialize(ctx))

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

	// Wait for the cache to sync
	assert.Eventually(t, func() bool {
		f.entryCacheMutex.RLock()
		defer f.entryCacheMutex.RUnlock()
		return len(f.entryCache) == 4
	}, 10*time.Second, 100*time.Millisecond)

	e, err = f.Get(ctx)
	require.NoError(t, err)
	assertExpectationsMatchDefaults(t, e)
	// Make sure that if we create a new view, we can still read the results.
	fr := New(c, nil, ReadOnly)
	e, err = fr.Get(ctx)
	require.NoError(t, err)
	assertExpectationsMatchDefaults(t, e)

	assert.Equal(t, 5, countExpectationChanges(ctx, t, f))
	assert.Equal(t, 2, countTriageRecords(ctx, t, f))
	assert.Equal(t, 5, countExpectationChanges(ctx, t, fr))
	assert.Equal(t, 2, countTriageRecords(ctx, t, fr))
}

func assertExpectationsMatchDefaults(t *testing.T, e expectations.ReadOnly) {
	assert.Equal(t, expectations.Positive, e.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.Negative, e.Classification(data.AlphaTest, data.AlphaNegativeDigest))
	assert.Equal(t, expectations.Untriaged, e.Classification(data.AlphaTest, data.AlphaUntriagedDigest))
	assert.Equal(t, expectations.Positive, e.Classification(data.BetaTest, data.BetaPositiveDigest))
	assert.Equal(t, expectations.Untriaged, e.Classification(data.BetaTest, data.BetaUntriagedDigest))
	assert.Equal(t, 3, e.Len())
}

// TestGetCopy_CLExpectations_CallerMutatesReturnValue_Success mutates the result of GetCopy and
// makes sure that future calls to GetCopy are not affected.
func TestGetCopy_CLExpectations_CallerMutatesReturnValue_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	master := New(c, nil, ReadWrite)
	f := master.ForChangeList("123", "github") // These are arbitrary
	putEntry(ctx, t, f, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)

	exp, err := f.GetCopy(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, exp.Len())
	assert.Equal(t, expectations.Positive, exp.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.Untriaged, exp.Classification(data.AlphaTest, data.AlphaUntriagedDigest))

	exp.Set(data.AlphaTest, data.AlphaPositiveDigest, expectations.Negative)
	exp.Set(data.AlphaTest, data.AlphaUntriagedDigest, expectations.Positive)

	shouldBeUnaffected, err := f.GetCopy(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, shouldBeUnaffected.Len())
	assert.Equal(t, expectations.Positive, shouldBeUnaffected.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.Untriaged, shouldBeUnaffected.Classification(data.AlphaTest, data.AlphaUntriagedDigest))
}

// TestGetCopy_MasterExpectations_CallerMutatesReturnValue_Success mutates the result of GetCopy and
// makes sure that future calls to GetCopy are not affected.
func TestGetCopy_MasterExpectations_CallerMutatesReturnValue_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New(c, nil, ReadWrite)
	require.NoError(t, f.Initialize(ctx))
	putEntry(ctx, t, f, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)

	// Wait for the query snapshot to show up in the RAM cache.
	assert.Eventually(t, func() bool {
		f.entryCacheMutex.RLock()
		defer f.entryCacheMutex.RUnlock()
		return len(f.entryCache) == 1
	}, 10*time.Second, 100*time.Millisecond)

	// Warm the local cache
	_, err := f.Get(ctx)
	require.NoError(t, err)

	exp, err := f.GetCopy(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, exp.Len())
	assert.Equal(t, expectations.Positive, exp.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.Untriaged, exp.Classification(data.AlphaTest, data.AlphaUntriagedDigest))

	exp.Set(data.AlphaTest, data.AlphaPositiveDigest, expectations.Negative)
	exp.Set(data.AlphaTest, data.AlphaUntriagedDigest, expectations.Positive)

	shouldBeUnaffected, err := f.GetCopy(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, shouldBeUnaffected.Len())
	assert.Equal(t, expectations.Positive, shouldBeUnaffected.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.Untriaged, shouldBeUnaffected.Classification(data.AlphaTest, data.AlphaUntriagedDigest))
}

// TestInitialize_ExpectationCacheIsFilledAndUpdated_Success has both a read-write and a read-only
// version and makes sure that the changes to the read-write version eventually propagate to the
// read-only version via the snapshots.
func TestInitialize_ExpectationCacheIsFilledAndUpdated_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()
	const firstPositiveThenUntriaged = types.Digest("abcd")

	f := New(c, nil, ReadWrite)
	putEntry(ctx, t, f, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)
	putEntry(ctx, t, f, data.AlphaTest, data.AlphaNegativeDigest, expectations.Negative, userOne)
	putEntry(ctx, t, f, data.AlphaTest, firstPositiveThenUntriaged, expectations.Positive, userOne)

	ro := New(c, nil, ReadOnly)
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

	assert.Equal(t, 5, countExpectationChanges(ctx, t, f))
	assert.Equal(t, 5, countTriageRecords(ctx, t, f))
}

// TestAddChange_MasterPartition_FromManyGoroutines_Success writes a bunch of data from many
// go routines in an effort to catch any race conditions in the caching layer.
func TestAddChange_MasterPartition_FromManyGoroutines_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New(c, nil, ReadWrite)
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
	assert.Equal(t, 50, countExpectationChanges(ctx, t, f))
	assert.Equal(t, 50, countTriageRecords(ctx, t, f))
}

// TestAddChange_ExpectationsDontConflictBetweenMasterAndCLPartition_Success tests the separation
// of the MasterExpectations and the CLExpectations. It starts with a single expectation, then adds
// some expectations to both, including changing the expectation. Specifically, the CLExpectations
// should be treated as a delta to the MasterExpectations (but doesn't actually contain
// MasterExpectations).
func TestAddChange_ExpectationsDontConflictBetweenMasterAndCLPartition_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterP := New(c, nil, ReadWrite)
	require.NoError(t, masterP.Initialize(ctx))
	putEntry(ctx, t, masterP, data.AlphaTest, data.AlphaPositiveDigest, expectations.Negative, userTwo)

	clPartition := masterP.ForChangeList("117", "gerrit") // arbitrary cl id
	// Check that it starts out blank.
	clExp, err := clPartition.Get(ctx)
	require.NoError(t, err)
	require.True(t, clExp.Empty())

	// Add to the CLExpectations
	putEntry(ctx, t, clPartition, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)
	putEntry(ctx, t, clPartition, data.BetaTest, data.BetaPositiveDigest, expectations.Positive, userTwo)

	// Add to the MasterExpectations
	putEntry(ctx, t, masterP, data.AlphaTest, data.AlphaNegativeDigest, expectations.Negative, userOne)

	// Wait for the entries to sync.
	assert.Eventually(t, func() bool {
		masterP.entryCacheMutex.RLock()
		defer masterP.entryCacheMutex.RUnlock()
		return len(masterP.entryCache) == 2
	}, 10*time.Second, 100*time.Millisecond)

	masterE, err := masterP.Get(ctx)
	require.NoError(t, err)
	clExp, err = clPartition.Get(ctx)
	require.NoError(t, err)

	// Make sure the CLExpectations did not leak to the MasterExpectations
	assert.Equal(t, expectations.Negative, masterE.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.Negative, masterE.Classification(data.AlphaTest, data.AlphaNegativeDigest))
	assert.Equal(t, expectations.Untriaged, masterE.Classification(data.BetaTest, data.BetaPositiveDigest))
	assert.Equal(t, 2, masterE.Len())

	// Make sure the CLExpectations are separate from the MasterExpectations.
	assert.Equal(t, expectations.Positive, clExp.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.Untriaged, clExp.Classification(data.AlphaTest, data.AlphaNegativeDigest))
	assert.Equal(t, expectations.Positive, clExp.Classification(data.BetaTest, data.BetaPositiveDigest))
	assert.Equal(t, 2, clExp.Len())
}

// TestAddChange_MasterPartition_TwoLargeSimultaneousBatches writes two batches of 512 entries
// to test the batch writing that happens for large amounts of expectation changes.
func TestAddChange_MasterPartition_TwoLargeSimultaneousBatches(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New(c, nil, ReadWrite)
	require.NoError(t, f.Initialize(ctx))

	// Write the expectations in two non-overlapping blocks of 16*32=512 entries, which should take
	// 3 batches to write them all. This is because Firestore has a limit of 500 writes per batch,
	// and we write both the expectation entry and the expectation change, so ~250 deltas can be
	// written per batch.
	exp1, delta1 := makeBigExpectations(0, 16)
	exp2, delta2 := makeBigExpectations(16, 32)

	expected := exp1.DeepCopy()
	expected.MergeExpectations(exp2)

	wg := sync.WaitGroup{}

	// Write them concurrently to test for potential race conditions.
	wg.Add(2)
	go func() {
		defer wg.Done()
		err := f.AddChange(ctx, delta1, userOne)
		require.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		err := f.AddChange(ctx, delta2, userTwo)
		require.NoError(t, err)
	}()
	wg.Wait()

	require.Eventually(t, func() bool {
		e, err := f.Get(ctx)
		assert.NoError(t, err)
		return deepequal.DeepEqual(expected, e)
	}, 10*time.Second, 500*time.Millisecond)

	assert.Equal(t, 1024, countExpectationChanges(ctx, t, f))
	assert.Equal(t, 2, countTriageRecords(ctx, t, f))
}

func TestAddChange_MasterPartition_NotifierEventsCorrect(t *testing.T) {
	unittest.LargeTest(t)

	notifier := expectations.NewEventDispatcherForTesting()
	var calledMutex sync.Mutex
	var calledWith []expectations.ID
	notifier.ListenForChange(func(e expectations.ID) {
		calledMutex.Lock()
		defer calledMutex.Unlock()
		calledWith = append(calledWith, e)
	})

	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New(c, notifier, ReadWrite)
	require.NoError(t, f.Initialize(ctx))

	change1 := []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaPositiveDigest,
			Label:    expectations.Positive,
		},
	}
	change2 := []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
			Label:    expectations.Negative,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaPositiveDigest,
			Label:    expectations.Positive,
		},
	}

	require.NoError(t, f.AddChange(ctx, change1, userOne))
	require.NoError(t, f.AddChange(ctx, change2, userTwo))

	assert.Eventually(t, func() bool {
		f.entryCacheMutex.RLock()
		defer f.entryCacheMutex.RUnlock()
		return len(f.entryCache) == 3
	}, 10*time.Second, 100*time.Millisecond)

	assert.ElementsMatch(t, []expectations.ID{change1[0].ID(), change2[0].ID(), change2[1].ID()}, calledWith)
}

// TestGetTriageHistory_MasterPartition_RepeatedlyOverwriteOneEntry_Success repeatedly overwrites
//  a single entry to make sure the cache reflects reality and that our history is complete.
func TestGetTriageHistory_MasterPartition_RepeatedlyOverwriteOneEntry_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New(c, nil, ReadWrite)
	require.NoError(t, f.Initialize(ctx))

	fakeNow := time.Date(2020, time.March, 1, 2, 3, 57, 0, time.UTC)
	f.now = func() time.Time {
		return fakeNow
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

	fakeNow = fakeNow.Add(time.Minute)
	putEntry(ctx, t, f, theEntry.Grouping, theEntry.Digest, expectations.Negative, userOne)
	waitForCacheToBe(expectations.Negative)

	fakeNow = fakeNow.Add(time.Minute)
	putEntry(ctx, t, f, theEntry.Grouping, theEntry.Digest, expectations.Untriaged, userTwo)
	waitForCacheToBe(expectations.Untriaged)

	fakeNow = fakeNow.Add(time.Minute)
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
	assert.Equal(t, 4, countExpectationChanges(ctx, t, f))
	assert.Equal(t, 4, countTriageRecords(ctx, t, f))
}

// TestGetTriageHistory_MasterPartition_Success writes some changes and then gets the triage
// history for those changes. Even if we query for records that don't exist, we should not see
// errors.
func TestGetTriageHistory_MasterPartition_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New(c, nil, ReadWrite)

	err := f.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
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
	}, userTwo)
	require.NoError(t, err)

	// Just make sure the time in the record was recent - the exact time does not really matter.
	assertTimeCorrect := func(t *testing.T, ts time.Time) {
		assert.True(t, ts.Before(time.Now()))
		assert.True(t, ts.After(time.Now().Add(-time.Minute)))
	}

	th, err := f.GetTriageHistory(ctx, data.AlphaTest, data.AlphaPositiveDigest)
	require.NoError(t, err)
	require.Len(t, th, 1)
	assert.Equal(t, userOne, th[0].User)
	assertTimeCorrect(t, th[0].TS)

	th, err = f.GetTriageHistory(ctx, data.AlphaTest, data.AlphaNegativeDigest)
	require.NoError(t, err)
	require.Len(t, th, 2)
	// Make sure the most recent change is first
	assert.Equal(t, userTwo, th[0].User)
	assertTimeCorrect(t, th[0].TS)
	assert.Equal(t, userOne, th[1].User)
	assertTimeCorrect(t, th[1].TS)
	assert.True(t, th[0].TS.After(th[1].TS))

	th, err = f.GetTriageHistory(ctx, "does not exist", "nope")
	require.NoError(t, err)
	assert.Empty(t, th)
}

// TestGetTriageHistory_MasterAndCLPartitionsDontConflict_Success writes some changes to the master
// partition and then to a CL partition and makes sure they don't conflict.
func TestGetTriageHistory_MasterAndCLPartitionsDontConflict_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterP := New(c, nil, ReadWrite)

	err := masterP.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
			Label:    expectations.Positive,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaPositiveDigest,
			Label:    expectations.Positive,
		},
	}, userOne)
	require.NoError(t, err)

	clPartition := masterP.ForChangeList("123", "gerrit") // arbitrary CL

	err = clPartition.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
			Label:    expectations.Negative,
		},
	}, userTwo)
	require.NoError(t, err)

	// Just make sure the time in the record was recent - the exact time does not really matter.
	assertTimeCorrect := func(t *testing.T, ts time.Time) {
		assert.True(t, ts.Before(time.Now()))
		assert.True(t, ts.After(time.Now().Add(-time.Minute)))
	}

	th, err := masterP.GetTriageHistory(ctx, data.AlphaTest, data.AlphaPositiveDigest)
	require.NoError(t, err)
	require.Len(t, th, 1)
	assert.Equal(t, userOne, th[0].User)
	assertTimeCorrect(t, th[0].TS)

	th, err = masterP.GetTriageHistory(ctx, data.AlphaTest, data.AlphaNegativeDigest)
	require.NoError(t, err)
	require.Len(t, th, 1)
	assert.Equal(t, userOne, th[0].User)
	assertTimeCorrect(t, th[0].TS)

	th, err = clPartition.GetTriageHistory(ctx, data.AlphaTest, data.AlphaPositiveDigest)
	require.NoError(t, err)
	require.Empty(t, th)

	th, err = clPartition.GetTriageHistory(ctx, data.AlphaTest, data.AlphaNegativeDigest)
	require.NoError(t, err)
	require.Len(t, th, 1)
	assert.Equal(t, userTwo, th[0].User)
	assertTimeCorrect(t, th[0].TS)

}

func TestQueryLog_WithoutDetails_OffsetsAndLimitsAreRespected(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New(c, nil, ReadWrite)
	firstTime := time.Date(2020, time.March, 1, 2, 3, 4, 0, time.UTC)
	fakeNow := firstTime
	f.now = func() time.Time {
		return fakeNow
	}

	putEntry(ctx, t, f, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)
	secondTime := time.Date(2020, time.March, 14, 2, 3, 4, 0, time.UTC)
	fakeNow = secondTime

	err := f.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
			Label:    expectations.Negative,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaPositiveDigest,
			Label:    expectations.Positive,
		},
	}, userTwo)
	require.NoError(t, err)

	entries, n, err := f.QueryLog(ctx, 0, 100, false)
	require.NoError(t, err)
	require.Equal(t, 2, n) // 2 operations in total
	assert.Equal(t, 3, countExpectationChanges(ctx, t, f))
	assert.Equal(t, 2, countTriageRecords(ctx, t, f))

	normalizeEntries2(t, entries)
	require.Equal(t, []expectations.TriageLogEntry{
		{
			ID:          "was_random_0",
			User:        userTwo,
			TS:          secondTime,
			ChangeCount: 2,
		},
		{
			ID:          "was_random_1",
			User:        userOne,
			TS:          firstTime,
			ChangeCount: 1,
		},
	}, entries)

	entries, n, err = f.QueryLog(ctx, 0, 1, false)
	require.NoError(t, err)
	require.Equal(t, expectations.CountMany, n)

	normalizeEntries2(t, entries)
	require.Equal(t, []expectations.TriageLogEntry{
		{
			ID:          "was_random_0",
			User:        userTwo,
			TS:          secondTime,
			ChangeCount: 2,
		},
	}, entries)

	// Now try for an offset way past the end of the data.
	entries, n, err = f.QueryLog(ctx, 500, 100, false)
	require.NoError(t, err)
	require.Equal(t, 500, n) // The system guesses that there are 500 or fewer items.
	require.Empty(t, entries)
}

func TestQueryLog_MasterAndCLPartitionsDontConflict_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterP := New(c, nil, ReadWrite)
	firstTime := time.Date(2020, time.March, 1, 2, 3, 4, 0, time.UTC)
	fakeNow := firstTime
	masterP.now = func() time.Time {
		return fakeNow
	}

	putEntry(ctx, t, masterP, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)

	clPartition := masterP.ForChangeList("1687", "gerrit") // this is arbitrary
	secondTime := time.Date(2020, time.March, 14, 2, 3, 4, 0, time.UTC)
	fakeNow = secondTime

	err := clPartition.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
			Label:    expectations.Negative,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaPositiveDigest,
			Label:    expectations.Positive,
		},
	}, userTwo)
	require.NoError(t, err)

	entries, n, err := masterP.QueryLog(ctx, 0, 10, false)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	assert.Equal(t, 1, countExpectationChanges(ctx, t, masterP))
	assert.Equal(t, 1, countTriageRecords(ctx, t, masterP))

	normalizeEntries2(t, entries)
	require.Equal(t, []expectations.TriageLogEntry{
		{
			ID:          "was_random_0",
			User:        userOne,
			TS:          firstTime,
			ChangeCount: 1,
		},
	}, entries)

	entries, n, err = clPartition.QueryLog(ctx, 0, 10, false)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	assert.Equal(t, 2, countExpectationChanges(ctx, t, clPartition.(*Store)))
	assert.Equal(t, 1, countTriageRecords(ctx, t, clPartition.(*Store)))

	normalizeEntries2(t, entries)
	require.Equal(t, []expectations.TriageLogEntry{
		{
			ID:          "was_random_0",
			User:        userTwo,
			TS:          secondTime,
			ChangeCount: 2,
		},
	}, entries)
}

func TestQueryLog_InvalidOffsets_Error(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New(c, nil, ReadWrite)
	putEntry(ctx, t, f, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)

	_, _, err := f.QueryLog(ctx, -1, 100, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "be positive")

	_, _, err = f.QueryLog(ctx, 0, -100, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "be positive")
}

func TestQueryLog_WithDetails_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New(c, nil, ReadWrite)
	firstTime := time.Date(2020, time.March, 1, 2, 3, 4, 0, time.UTC)
	fakeNow := firstTime
	f.now = func() time.Time {
		return fakeNow
	}

	putEntry(ctx, t, f, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)
	secondTime := time.Date(2020, time.March, 14, 2, 3, 4, 0, time.UTC)
	fakeNow = secondTime

	err := f.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
			Label:    expectations.Negative,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaPositiveDigest,
			Label:    expectations.Positive,
		},
	}, userTwo)
	require.NoError(t, err)

	entries, n, err := f.QueryLog(ctx, 0, 100, true)
	require.NoError(t, err)
	require.Equal(t, 2, n) // 2 operations in total

	normalizeEntries2(t, entries)
	require.Equal(t, []expectations.TriageLogEntry{
		{
			ID:          "was_random_0",
			User:        userTwo,
			TS:          secondTime,
			ChangeCount: 2,
			Details: []expectations.Delta{
				{
					Grouping: data.AlphaTest,
					Digest:   data.AlphaNegativeDigest,
					Label:    expectations.Negative,
				},
				{
					Grouping: data.BetaTest,
					Digest:   data.BetaPositiveDigest,
					Label:    expectations.Positive,
				},
			},
		},
		{
			ID:          "was_random_1",
			User:        userOne,
			TS:          firstTime,
			ChangeCount: 1,
			Details: []expectations.Delta{
				{
					Grouping: data.AlphaTest,
					Digest:   data.AlphaPositiveDigest,
					Label:    expectations.Positive,
				},
			},
		},
	}, entries)
}

// TestQueryLogDetailsLarge checks that the details are filled in correctly, even in cases
// where we had to write in multiple chunks. (skbug.com/9485)
func TestQueryLog_WritingManyExpectations_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New(c, nil, ReadWrite)

	// 800 should spread us across 4 "batches", which are ~250 expectations each.
	const numExp = 800
	delta := make([]expectations.Delta, 0, numExp)
	for i := uint64(0); i < numExp; i++ {
		n := types.TestName(fmt.Sprintf("test_%03d", i))
		// An MD5 hash is 128 bits, which is 32 chars
		d := types.Digest(fmt.Sprintf("%032d", i))
		delta = append(delta, expectations.Delta{
			Grouping: n,
			Digest:   d,
			Label:    expectations.Positive,
		})
	}
	err := f.AddChange(ctx, delta, "test@example.com")
	require.NoError(t, err)

	entries, n, err := f.QueryLog(ctx, 0, 2, true)
	require.NoError(t, err)
	require.Equal(t, 1, n) // 1 big operation
	require.Len(t, entries, 1)

	entry := entries[0]
	require.Equal(t, numExp, entry.ChangeCount)
	require.Len(t, entry.Details, numExp)

	// Spot check some details across the various batches.
	require.Equal(t, expectations.Delta{
		Grouping: "test_000",
		Digest:   "00000000000000000000000000000000",
		Label:    expectations.Positive,
	}, entry.Details[0])
	require.Equal(t, expectations.Delta{
		Grouping: "test_200",
		Digest:   "00000000000000000000000000000200",
		Label:    expectations.Positive,
	}, entry.Details[200])
	require.Equal(t, expectations.Delta{
		Grouping: "test_400",
		Digest:   "00000000000000000000000000000400",
		Label:    expectations.Positive,
	}, entry.Details[400])
	require.Equal(t, expectations.Delta{
		Grouping: "test_600",
		Digest:   "00000000000000000000000000000600",
		Label:    expectations.Positive,
	}, entry.Details[600])
	require.Equal(t, expectations.Delta{
		Grouping: "test_799",
		Digest:   "00000000000000000000000000000799",
		Label:    expectations.Positive,
	}, entry.Details[799])
}

// TestUndo_MasterPartition_EntriesExist_Success makes sure we can undo changes properly.
func TestUndo_MasterPartition_EntriesExist_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New(c, nil, ReadWrite)
	require.NoError(t, f.Initialize(ctx))

	putEntry(ctx, t, f, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)
	putEntry(ctx, t, f, data.AlphaTest, data.AlphaPositiveDigest, expectations.Negative, userOne) // will be undone
	putEntry(ctx, t, f, data.AlphaTest, data.AlphaNegativeDigest, expectations.Negative, userOne)

	entries, _, err := f.QueryLog(ctx, 0, 10, false)
	require.NoError(t, err)
	require.Len(t, entries, 3)

	toUndo := entries[1].ID
	require.NotEmpty(t, toUndo)

	require.NoError(t, f.UndoChange(ctx, toUndo, userTwo))

	exp, err := f.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, expectations.Positive, exp.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.Negative, exp.Classification(data.AlphaTest, data.AlphaNegativeDigest))

	// Check that the undo shows up as the most recent entry.
	entries, _, err = f.QueryLog(ctx, 0, 10, true)
	require.NoError(t, err)
	require.Len(t, entries, 4)
	undidEntry := entries[0]
	assert.Equal(t, userTwo, undidEntry.User)
	assert.Equal(t, 1, undidEntry.ChangeCount)
	assert.Equal(t, expectations.Delta{
		Grouping: data.AlphaTest,
		Digest:   data.AlphaPositiveDigest,
		Label:    expectations.Positive,
	}, undidEntry.Details[0])
}

// TestUndo_CLPartition_EntriesExist_Success makes sure we can undo changes properly, even if the
// background firestore snapshots are not running (e.g. for CL Expectations).
func TestUndo_CLPartition_EntriesExist_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	master := New(c, nil, ReadWrite)
	f := master.ForChangeList("123", "github") // These are arbitrary

	putEntry(ctx, t, f, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)
	putEntry(ctx, t, f, data.AlphaTest, data.AlphaPositiveDigest, expectations.Negative, userOne) // will be undone
	putEntry(ctx, t, f, data.AlphaTest, data.AlphaNegativeDigest, expectations.Negative, userOne)

	entries, _, err := f.QueryLog(ctx, 0, 10, false)
	require.NoError(t, err)
	require.Len(t, entries, 3)

	toUndo := entries[1].ID
	require.NotEmpty(t, toUndo)

	require.NoError(t, f.UndoChange(ctx, toUndo, userTwo))

	exp, err := f.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, expectations.Positive, exp.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.Negative, exp.Classification(data.AlphaTest, data.AlphaNegativeDigest))

	// Check that the undo shows up as the most recent entry.
	entries, _, err = f.QueryLog(ctx, 0, 10, true)
	require.NoError(t, err)
	require.Len(t, entries, 4)
	undidEntry := entries[0]
	assert.Equal(t, userTwo, undidEntry.User)
	assert.Equal(t, 1, undidEntry.ChangeCount)
	assert.Equal(t, expectations.Delta{
		Grouping: data.AlphaTest,
		Digest:   data.AlphaPositiveDigest,
		Label:    expectations.Positive,
	}, undidEntry.Details[0])
}

// normalizeEntries2 fixes the non-deterministic parts of TriageLogEntry to be deterministic
func normalizeEntries2(t *testing.T, entries []expectations.TriageLogEntry) {
	for i, te := range entries {
		require.NotEqual(t, "", te.ID)
		te.ID = "was_random_" + strconv.Itoa(i)
		entries[i] = te
	}
}

func countExpectationChanges(ctx context.Context, t *testing.T, f *Store) int {
	q := f.changesCollection().Offset(0)
	count := 0
	require.NoError(t, f.client.IterDocs(ctx, "", "", q, 3, 30*time.Second, func(ds *firestore.DocumentSnapshot) error {
		if ds == nil {
			return nil
		}
		count++
		return nil
	}))
	return count
}

func countTriageRecords(ctx context.Context, t *testing.T, f *Store) int {
	q := f.recordsCollection().Offset(0)
	count := 0
	require.NoError(t, f.client.IterDocs(ctx, "", "", q, 3, 30*time.Second, func(ds *firestore.DocumentSnapshot) error {
		if ds == nil {
			return nil
		}
		count++
		return nil
	}))
	return count
}

func putEntry(ctx context.Context, t *testing.T, f expectations.Store, name types.TestName, digest types.Digest, label expectations.Label, user string) {
	require.NoError(t, f.AddChange(ctx, []expectations.Delta{
		{
			Grouping: name,
			Digest:   digest,
			Label:    label,
		},
	}, user))
}

const (
	userOne = "userOne@example.com"
	userTwo = "userTwo@example.com"
)

// makeBigExpectations makes (end-start) tests named from start to end that each have 32 digests.
func makeBigExpectations(start, end int) (*expectations.Expectations, []expectations.Delta) {
	var e expectations.Expectations
	var delta []expectations.Delta
	for i := start; i < end; i++ {
		for j := 0; j < 32; j++ {
			tn := types.TestName(fmt.Sprintf("test-%03d", i))
			d := types.Digest(fmt.Sprintf("digest-%03d", j))
			e.Set(tn, d, expectations.Positive)
			delta = append(delta, expectations.Delta{
				Grouping: tn,
				Digest:   d,
				Label:    expectations.Positive,
			})

		}
	}
	return &e, delta
}

// makeTestFirestoreClient returns a firestore.Client and a context.Context. When the third return
// value is called, the Context will be cancelled and the Client will be cleaned up.
func makeTestFirestoreClient(t *testing.T) (*ifirestore.Client, context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	c, cleanup := ifirestore.NewClientForTesting(ctx, t)
	return c, ctx, func() {
		cancel()
		cleanup()
	}
}
