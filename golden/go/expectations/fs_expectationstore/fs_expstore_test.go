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

// TestExpectationEntry_ID_ReplacesInvalidCharacters tests edge cases for malformed names.
func TestExpectationEntry_ID_ReplacesInvalidCharacters(t *testing.T) {
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
	masterStore := New(c, nil, ReadWrite)
	clStore := masterStore.ForChangeList(clID, crs)

	// Brand new instance should have no expectations
	clExps, err := clStore.Get(ctx)
	require.NoError(t, err)
	require.True(t, clExps.Empty())

	err = clStore.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaUntriagedDigest,
			Label:    expectations.PositiveStr, // Intentionally wrong. Will be fixed by the next AddChange.
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaPositiveDigest,
			Label:    expectations.PositiveStr,
		},
	}, userOne)
	require.NoError(t, err)

	err = clStore.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
			Label:    expectations.NegativeStr,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaUntriagedDigest, // overwrites previous
			Label:    expectations.UntriagedStr,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaPositiveDigest,
			Label:    expectations.PositiveStr,
		},
	}, userTwo)
	require.NoError(t, err)

	clExps, err = clStore.Get(ctx)
	require.NoError(t, err)
	assertExpectationsMatchDefaults(t, clExps)

	// Make sure that if we create a new view, we can still read the results.
	masterStore = New(c, nil, ReadOnly)
	clStore = masterStore.ForChangeList(clID, crs)
	clExps, err = clStore.Get(ctx)
	require.NoError(t, err)
	assertExpectationsMatchDefaults(t, clExps)
}

// TestGet_ExpectationsInMasterPartition_Success writes some changes, one of which overwrites a
// previous expectation and asserts that we can call Get to extract the correct output.
func TestGet_ExpectationsInMasterPartition_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)
	require.NoError(t, masterStore.Initialize(ctx))

	// Brand new instance should have no expectations
	masterExps, err := masterStore.Get(ctx)
	require.NoError(t, err)
	require.True(t, masterExps.Empty())

	err = masterStore.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaUntriagedDigest,
			Label:    expectations.PositiveStr,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaPositiveDigest,
			Label:    expectations.PositiveStr,
		},
	}, userOne)
	require.NoError(t, err)

	err = masterStore.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
			Label:    expectations.NegativeStr,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaUntriagedDigest, // overwrites previous
			Label:    expectations.UntriagedStr,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaPositiveDigest,
			Label:    expectations.PositiveStr,
		},
	}, userTwo)
	require.NoError(t, err)

	// Wait for the cache to sync
	assert.Eventually(t, func() bool {
		masterStore.entryCacheMutex.RLock()
		defer masterStore.entryCacheMutex.RUnlock()
		return len(masterStore.entryCache) == 4
	}, 10*time.Second, 100*time.Millisecond)

	masterExps, err = masterStore.Get(ctx)
	require.NoError(t, err)
	assertExpectationsMatchDefaults(t, masterExps)

	// Make sure that if we create a new view, we can still read the results.
	readOnly := New(c, nil, ReadOnly)
	roExps, err := readOnly.Get(ctx)
	require.NoError(t, err)
	assertExpectationsMatchDefaults(t, roExps)

	assert.Equal(t, 5, countExpectationChanges(ctx, t, masterStore))
	assert.Equal(t, 2, countTriageRecords(ctx, t, masterStore))
	assert.Equal(t, 5, countExpectationChanges(ctx, t, readOnly))
	assert.Equal(t, 2, countTriageRecords(ctx, t, readOnly))
}

func assertExpectationsMatchDefaults(t *testing.T, e expectations.ReadOnly) {
	assert.Equal(t, expectations.PositiveStr, e.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.NegativeStr, e.Classification(data.AlphaTest, data.AlphaNegativeDigest))
	assert.Equal(t, expectations.UntriagedStr, e.Classification(data.AlphaTest, data.AlphaUntriagedDigest))
	assert.Equal(t, expectations.PositiveStr, e.Classification(data.BetaTest, data.BetaPositiveDigest))
	assert.Equal(t, expectations.UntriagedStr, e.Classification(data.BetaTest, data.BetaUntriagedDigest))
	assert.Equal(t, 3, e.Len())
}

// TestGetCopy_CLPartition_CallerMutatesReturnValue_StoreUnaffected mutates the result of GetCopy
// and makes sure that future calls to GetCopy are not affected.
func TestGetCopy_CLPartition_CallerMutatesReturnValue_StoreUnaffected(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)
	clStore := masterStore.ForChangeList("123", "github") // These are arbitrary
	putEntry(ctx, t, clStore, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)

	clExps, err := clStore.GetCopy(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, clExps.Len())
	assert.Equal(t, expectations.PositiveStr, clExps.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.UntriagedStr, clExps.Classification(data.AlphaTest, data.AlphaUntriagedDigest))

	clExps.Set(data.AlphaTest, data.AlphaPositiveDigest, expectations.NegativeStr)
	clExps.Set(data.AlphaTest, data.AlphaUntriagedDigest, expectations.PositiveStr)

	shouldBeUnaffected, err := clStore.GetCopy(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, shouldBeUnaffected.Len())
	assert.Equal(t, expectations.PositiveStr, shouldBeUnaffected.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.UntriagedStr, shouldBeUnaffected.Classification(data.AlphaTest, data.AlphaUntriagedDigest))
}

// TestGetCopy_MasterPartition_CallerMutatesReturnValue_StoreUnaffected mutates the result of
// GetCopy and makes sure that future calls to GetCopy are not affected.
func TestGetCopy_MasterPartition_CallerMutatesReturnValue_StoreUnaffected(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)
	require.NoError(t, masterStore.Initialize(ctx))
	putEntry(ctx, t, masterStore, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)

	// Wait for the query snapshot to show up in the RAM cache.
	assert.Eventually(t, func() bool {
		masterStore.entryCacheMutex.RLock()
		defer masterStore.entryCacheMutex.RUnlock()
		return len(masterStore.entryCache) == 1
	}, 10*time.Second, 100*time.Millisecond)

	// Warm the local cache
	_, err := masterStore.Get(ctx)
	require.NoError(t, err)

	masterExps, err := masterStore.GetCopy(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, masterExps.Len())
	assert.Equal(t, expectations.PositiveStr, masterExps.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.UntriagedStr, masterExps.Classification(data.AlphaTest, data.AlphaUntriagedDigest))

	masterExps.Set(data.AlphaTest, data.AlphaPositiveDigest, expectations.NegativeStr)
	masterExps.Set(data.AlphaTest, data.AlphaUntriagedDigest, expectations.PositiveStr)

	shouldBeUnaffected, err := masterStore.GetCopy(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, shouldBeUnaffected.Len())
	assert.Equal(t, expectations.PositiveStr, shouldBeUnaffected.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.UntriagedStr, shouldBeUnaffected.Classification(data.AlphaTest, data.AlphaUntriagedDigest))
}

// TestInitialize_ExpectationCacheIsFilledAndUpdated_Success has both a read-write and a read-only
// version and makes sure that the changes to the read-write version eventually propagate to the
// read-only version via the snapshots.
func TestInitialize_ExpectationCacheIsFilledAndUpdated_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	const firstPositiveThenUntriaged = types.Digest("abcd")

	// Initialize store with some expectations.
	masterStore := New(c, nil, ReadWrite)
	putEntry(ctx, t, masterStore, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)
	putEntry(ctx, t, masterStore, data.AlphaTest, data.AlphaNegativeDigest, expectations.Negative, userOne)
	putEntry(ctx, t, masterStore, data.AlphaTest, firstPositiveThenUntriaged, expectations.Positive, userOne)

	// Create a read-only store and assert the cache is empty before we call Initialize.
	readOnly := New(c, nil, ReadOnly)
	assert.Empty(t, readOnly.entryCache)
	assert.False(t, readOnly.hasSnapshotsRunning)
	require.NoError(t, readOnly.Initialize(ctx))

	assert.True(t, readOnly.hasSnapshotsRunning)

	// Check that the read-only copy has been loaded with the existing 3 entries as a result of
	// the Initialize method.
	assert.Len(t, readOnly.entryCache, 3)
	roExps, err := readOnly.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, expectations.PositiveStr, roExps.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.NegativeStr, roExps.Classification(data.AlphaTest, data.AlphaNegativeDigest))
	assert.Equal(t, expectations.PositiveStr, roExps.Classification(data.AlphaTest, firstPositiveThenUntriaged))

	// This should update the existing entry, leaving us with 4 total entries, not 5
	putEntry(ctx, t, masterStore, data.AlphaTest, firstPositiveThenUntriaged, expectations.Untriaged, userOne)
	putEntry(ctx, t, masterStore, data.BetaTest, data.BetaPositiveDigest, expectations.Positive, userOne)

	assert.Eventually(t, func() bool {
		readOnly.entryCacheMutex.RLock()
		defer readOnly.entryCacheMutex.RUnlock()
		return len(readOnly.entryCache) == 4
	}, 10*time.Second, 100*time.Millisecond)

	roExps2, err := readOnly.Get(ctx)
	require.NoError(t, err)
	assertExpectationsMatchDefaults(t, roExps2)
	assert.Equal(t, expectations.UntriagedStr, roExps2.Classification(data.AlphaTest, firstPositiveThenUntriaged))

	// Spot check that the expectations we got first were not impacted by the new expectations
	// coming in or the second call to Get.
	assert.Equal(t, expectations.PositiveStr, roExps.Classification(data.AlphaTest, firstPositiveThenUntriaged))

	assert.Equal(t, 5, countExpectationChanges(ctx, t, masterStore))
	assert.Equal(t, 5, countTriageRecords(ctx, t, masterStore))
	assert.Equal(t, 5, countExpectationChanges(ctx, t, readOnly))
	assert.Equal(t, 5, countTriageRecords(ctx, t, readOnly))
}

// TestAddChange_MasterPartition_FromManyGoroutines_Success writes a bunch of data from many
// go routines in an effort to catch any race conditions in the caching layer.
func TestAddChange_MasterPartition_FromManyGoroutines_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)
	require.NoError(t, masterStore.Initialize(ctx))

	entries := []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaUntriagedDigest,
			Label:    expectations.UntriagedStr,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
			Label:    expectations.NegativeStr,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaPositiveDigest,
			Label:    expectations.PositiveStr,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaPositiveDigest,
			Label:    expectations.PositiveStr,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaUntriagedDigest,
			Label:    expectations.UntriagedStr,
		},
	}

	wg := sync.WaitGroup{}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e := entries[i%len(entries)]
			err := masterStore.AddChange(ctx, []expectations.Delta{e}, userOne)
			require.NoError(t, err)
		}(i)

		// Make sure we can read and write at the same time. We run these tests with golang's -race
		// option which can help identify race conditions.
		if i%5 == 0 {
			_, err := masterStore.Get(ctx)
			require.NoError(t, err)
		}
	}

	wg.Wait()

	e, err := masterStore.Get(ctx)
	require.NoError(t, err)
	assertExpectationsMatchDefaults(t, e)
	assert.Equal(t, 50, countExpectationChanges(ctx, t, masterStore))
	assert.Equal(t, 50, countTriageRecords(ctx, t, masterStore))
}

// TestAddChange_ExpectationsDoNotConflictBetweenMasterAndCLPartition tests the separation of
// the master expectations and the CL expectations. It starts with a single expectation, then adds
// some expectations to both, including changing the expectation. Specifically, the CL expectations
// should be treated as a delta to the master expectations (but doesn't actually contain
// master expectations).
func TestAddChange_ExpectationsDoNotConflictBetweenMasterAndCLPartition(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)
	require.NoError(t, masterStore.Initialize(ctx))
	putEntry(ctx, t, masterStore, data.AlphaTest, data.AlphaPositiveDigest, expectations.Negative, userTwo)

	clStore := masterStore.ForChangeList("117", "gerrit") // arbitrary cl id
	// Check that it starts out blank.
	clExps, err := clStore.Get(ctx)
	require.NoError(t, err)
	require.True(t, clExps.Empty())

	// Add to the CL expectations
	putEntry(ctx, t, clStore, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)
	putEntry(ctx, t, clStore, data.BetaTest, data.BetaPositiveDigest, expectations.Positive, userTwo)

	// Add to the master expectations
	putEntry(ctx, t, masterStore, data.AlphaTest, data.AlphaNegativeDigest, expectations.Negative, userOne)

	// Wait for the entries to sync.
	assert.Eventually(t, func() bool {
		masterStore.entryCacheMutex.RLock()
		defer masterStore.entryCacheMutex.RUnlock()
		return len(masterStore.entryCache) == 2
	}, 10*time.Second, 100*time.Millisecond)

	masterExps, err := masterStore.Get(ctx)
	require.NoError(t, err)
	clExps, err = clStore.Get(ctx)
	require.NoError(t, err)

	// Make sure the CL expectations did not leak to the master expectations
	assert.Equal(t, expectations.NegativeStr, masterExps.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.NegativeStr, masterExps.Classification(data.AlphaTest, data.AlphaNegativeDigest))
	assert.Equal(t, expectations.UntriagedStr, masterExps.Classification(data.BetaTest, data.BetaPositiveDigest))
	assert.Equal(t, 2, masterExps.Len())

	// Make sure the CL expectations are separate from the master expectations.
	assert.Equal(t, expectations.PositiveStr, clExps.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.UntriagedStr, clExps.Classification(data.AlphaTest, data.AlphaNegativeDigest))
	assert.Equal(t, expectations.PositiveStr, clExps.Classification(data.BetaTest, data.BetaPositiveDigest))
	assert.Equal(t, 2, clExps.Len())
}

// TestAddChange_MasterPartition_TwoLargeSimultaneousBatches_Success writes two batches of 512
// entries to test the batch writing that happens for large amounts of expectation changes.
func TestAddChange_MasterPartition_TwoLargeSimultaneousBatches_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)
	require.NoError(t, masterStore.Initialize(ctx))

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
		err := masterStore.AddChange(ctx, delta1, userOne)
		require.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		err := masterStore.AddChange(ctx, delta2, userTwo)
		require.NoError(t, err)
	}()
	wg.Wait()

	require.Eventually(t, func() bool {
		e, err := masterStore.Get(ctx)
		assert.NoError(t, err)
		return deepequal.DeepEqual(expected, e)
	}, 10*time.Second, 500*time.Millisecond)

	assert.Equal(t, 1024, countExpectationChanges(ctx, t, masterStore))
	assert.Equal(t, 2, countTriageRecords(ctx, t, masterStore))
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

	masterStore := New(c, notifier, ReadWrite)
	require.NoError(t, masterStore.Initialize(ctx))

	change1 := []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaPositiveDigest,
			Label:    expectations.PositiveStr,
		},
	}
	change2 := []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
			Label:    expectations.NegativeStr,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaPositiveDigest,
			Label:    expectations.PositiveStr,
		},
	}

	require.NoError(t, masterStore.AddChange(ctx, change1, userOne))
	require.NoError(t, masterStore.AddChange(ctx, change2, userTwo))

	assert.Eventually(t, func() bool {
		masterStore.entryCacheMutex.RLock()
		defer masterStore.entryCacheMutex.RUnlock()
		return len(masterStore.entryCache) == 3
	}, 10*time.Second, 100*time.Millisecond)

	assert.ElementsMatch(t, []expectations.ID{change1[0].ID(), change2[0].ID(), change2[1].ID()}, calledWith)
}

// TestGetTriageHistory_MasterPartition_Success writes some changes and then gets the triage
// history for those changes. Even if we query for records that don't exist, we should not see
// errors.
func TestGetTriageHistory_MasterPartition_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)

	err := masterStore.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
			Label:    expectations.PositiveStr,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaPositiveDigest,
			Label:    expectations.PositiveStr,
		},
	}, userOne)
	require.NoError(t, err)

	err = masterStore.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
			Label:    expectations.NegativeStr,
		},
	}, userTwo)
	require.NoError(t, err)

	// Just make sure the time in the record was recent - the exact time does not really matter.
	assertTimeCorrect := func(t *testing.T, ts time.Time) {
		assert.True(t, ts.Before(time.Now()))
		assert.True(t, ts.After(time.Now().Add(-time.Minute)))
	}

	th, err := masterStore.GetTriageHistory(ctx, data.AlphaTest, data.AlphaPositiveDigest)
	require.NoError(t, err)
	require.Len(t, th, 1)
	assert.Equal(t, userOne, th[0].User)
	assertTimeCorrect(t, th[0].TS)

	th, err = masterStore.GetTriageHistory(ctx, data.AlphaTest, data.AlphaNegativeDigest)
	require.NoError(t, err)
	require.Len(t, th, 2)
	// Make sure the most recent change is first
	assert.Equal(t, userTwo, th[0].User)
	assertTimeCorrect(t, th[0].TS)
	assert.Equal(t, userOne, th[1].User)
	assertTimeCorrect(t, th[1].TS)
	assert.True(t, th[0].TS.After(th[1].TS))

	th, err = masterStore.GetTriageHistory(ctx, "does not exist", "nope")
	require.NoError(t, err)
	assert.Empty(t, th)
}

// TestGetTriageHistory_MasterPartition_RepeatedlyOverwriteOneEntry_Success repeatedly overwrites
// a single entry to make sure the cache reflects reality and that our history is complete.
func TestGetTriageHistory_MasterPartition_RepeatedlyOverwriteOneEntry_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)
	require.NoError(t, masterStore.Initialize(ctx))

	fakeNow := time.Date(2020, time.March, 1, 2, 3, 57, 0, time.UTC)
	masterStore.now = func() time.Time {
		return fakeNow
	}

	theEntry := expectations.ID{Grouping: data.AlphaTest, Digest: data.AlphaPositiveDigest}

	// This will wait for the firestore query snapshots to update the cache to have the entry we care
	// about to have the given label.
	waitForCacheToBe := func(label expectations.Label) {
		require.Eventually(t, func() bool {
			masterStore.entryCacheMutex.RLock()
			defer masterStore.entryCacheMutex.RUnlock()
			if len(masterStore.entryCache) != 1 {
				return false
			}
			actualEntry := masterStore.entryCache[theEntry]
			// Make sure we don't append to Ranges (since we are currently overwriting at master).
			assert.Len(t, actualEntry.Ranges, 1)
			return actualEntry.Ranges[0].Label == label
		}, 10*time.Second, 100*time.Millisecond)
	}

	putEntry(ctx, t, masterStore, theEntry.Grouping, theEntry.Digest, expectations.Positive, userOne)
	waitForCacheToBe(expectations.Positive)

	fakeNow = fakeNow.Add(time.Minute)
	putEntry(ctx, t, masterStore, theEntry.Grouping, theEntry.Digest, expectations.Negative, userOne)
	waitForCacheToBe(expectations.Negative)

	fakeNow = fakeNow.Add(time.Minute)
	putEntry(ctx, t, masterStore, theEntry.Grouping, theEntry.Digest, expectations.Untriaged, userTwo)
	waitForCacheToBe(expectations.Untriaged)

	fakeNow = fakeNow.Add(time.Minute)
	putEntry(ctx, t, masterStore, theEntry.Grouping, theEntry.Digest, expectations.Positive, userTwo)
	waitForCacheToBe(expectations.Positive)

	xth, err := masterStore.GetTriageHistory(ctx, theEntry.Grouping, theEntry.Digest)
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
	assert.Equal(t, 4, countExpectationChanges(ctx, t, masterStore))
	assert.Equal(t, 4, countTriageRecords(ctx, t, masterStore))
}

// TestGetTriageHistory_MasterAndCLPartitionsDoNotConflict_Success writes some changes to the master
// partition and then to a CL partition and makes sure they don't conflict.
func TestGetTriageHistory_MasterAndCLPartitionsDoNotConflict_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)

	err := masterStore.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
			Label:    expectations.PositiveStr,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaPositiveDigest,
			Label:    expectations.PositiveStr,
		},
	}, userOne)
	require.NoError(t, err)

	clStore := masterStore.ForChangeList("123", "gerrit") // arbitrary CL

	err = clStore.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
			Label:    expectations.NegativeStr,
		},
	}, userTwo)
	require.NoError(t, err)

	// Just make sure the time in the record was recent - the exact time does not really matter.
	assertTimeCorrect := func(t *testing.T, ts time.Time) {
		assert.True(t, ts.Before(time.Now()))
		assert.True(t, ts.After(time.Now().Add(-time.Minute)))
	}

	th, err := masterStore.GetTriageHistory(ctx, data.AlphaTest, data.AlphaPositiveDigest)
	require.NoError(t, err)
	require.Len(t, th, 1)
	assert.Equal(t, userOne, th[0].User)
	assertTimeCorrect(t, th[0].TS)

	th, err = masterStore.GetTriageHistory(ctx, data.AlphaTest, data.AlphaNegativeDigest)
	require.NoError(t, err)
	require.Len(t, th, 1)
	assert.Equal(t, userOne, th[0].User)
	assertTimeCorrect(t, th[0].TS)

	th, err = clStore.GetTriageHistory(ctx, data.AlphaTest, data.AlphaPositiveDigest)
	require.NoError(t, err)
	require.Empty(t, th)

	th, err = clStore.GetTriageHistory(ctx, data.AlphaTest, data.AlphaNegativeDigest)
	require.NoError(t, err)
	require.Len(t, th, 1)
	assert.Equal(t, userTwo, th[0].User)
	assertTimeCorrect(t, th[0].TS)

}

func TestQueryLog_WithoutDetails_OffsetsAndLimitsAreRespected(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)
	firstTime := time.Date(2020, time.March, 1, 2, 3, 4, 0, time.UTC)
	fakeNow := firstTime
	masterStore.now = func() time.Time {
		return fakeNow
	}

	putEntry(ctx, t, masterStore, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)
	secondTime := time.Date(2020, time.March, 14, 2, 3, 4, 0, time.UTC)
	fakeNow = secondTime

	err := masterStore.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
			Label:    expectations.NegativeStr,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaPositiveDigest,
			Label:    expectations.PositiveStr,
		},
	}, userTwo)
	require.NoError(t, err)

	entries, n, err := masterStore.QueryLog(ctx, 0, 100, false)
	require.NoError(t, err)
	require.Equal(t, 2, n) // 2 operations in total
	assert.Equal(t, 3, countExpectationChanges(ctx, t, masterStore))
	assert.Equal(t, 2, countTriageRecords(ctx, t, masterStore))

	normalizeEntries(t, entries)
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

	entries, n, err = masterStore.QueryLog(ctx, 0, 1, false)
	require.NoError(t, err)
	require.Equal(t, expectations.CountMany, n)

	normalizeEntries(t, entries)
	require.Equal(t, []expectations.TriageLogEntry{
		{
			ID:          "was_random_0",
			User:        userTwo,
			TS:          secondTime,
			ChangeCount: 2,
		},
	}, entries)

	// Now try for an offset way past the end of the data.
	entries, n, err = masterStore.QueryLog(ctx, 500, 100, false)
	require.NoError(t, err)
	require.Equal(t, 500, n) // The system guesses that there are 500 or fewer items.
	require.Empty(t, entries)
}

func TestQueryLog_MasterAndCLPartitionsDoNotConflict_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)
	firstTime := time.Date(2020, time.March, 1, 2, 3, 4, 0, time.UTC)
	fakeNow := firstTime
	masterStore.now = func() time.Time {
		return fakeNow
	}

	putEntry(ctx, t, masterStore, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)

	clStore := masterStore.ForChangeList("1687", "gerrit") // this is arbitrary
	secondTime := time.Date(2020, time.March, 14, 2, 3, 4, 0, time.UTC)
	fakeNow = secondTime

	err := clStore.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
			Label:    expectations.NegativeStr,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaPositiveDigest,
			Label:    expectations.PositiveStr,
		},
	}, userTwo)
	require.NoError(t, err)

	entries, n, err := masterStore.QueryLog(ctx, 0, 10, false)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	assert.Equal(t, 1, countExpectationChanges(ctx, t, masterStore))
	assert.Equal(t, 1, countTriageRecords(ctx, t, masterStore))

	normalizeEntries(t, entries)
	require.Equal(t, []expectations.TriageLogEntry{
		{
			ID:          "was_random_0",
			User:        userOne,
			TS:          firstTime,
			ChangeCount: 1,
		},
	}, entries)

	entries, n, err = clStore.QueryLog(ctx, 0, 10, false)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	assert.Equal(t, 2, countExpectationChanges(ctx, t, clStore.(*Store)))
	assert.Equal(t, 1, countTriageRecords(ctx, t, clStore.(*Store)))

	normalizeEntries(t, entries)
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

	masterStore := New(c, nil, ReadWrite)
	putEntry(ctx, t, masterStore, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)

	_, _, err := masterStore.QueryLog(ctx, -1, 100, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "be positive")

	_, _, err = masterStore.QueryLog(ctx, 0, -100, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "be positive")
}

func TestQueryLog_WithDetails_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)
	firstTime := time.Date(2020, time.March, 1, 2, 3, 4, 0, time.UTC)
	fakeNow := firstTime
	masterStore.now = func() time.Time {
		return fakeNow
	}

	putEntry(ctx, t, masterStore, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)
	secondTime := time.Date(2020, time.March, 14, 2, 3, 4, 0, time.UTC)
	fakeNow = secondTime

	err := masterStore.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaNegativeDigest,
			Label:    expectations.NegativeStr,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaPositiveDigest,
			Label:    expectations.PositiveStr,
		},
	}, userTwo)
	require.NoError(t, err)

	entries, n, err := masterStore.QueryLog(ctx, 0, 100, true)
	require.NoError(t, err)
	require.Equal(t, 2, n) // 2 operations in total

	normalizeEntries(t, entries)
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
					Label:    expectations.NegativeStr,
				},
				{
					Grouping: data.BetaTest,
					Digest:   data.BetaPositiveDigest,
					Label:    expectations.PositiveStr,
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
					Label:    expectations.PositiveStr,
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

	masterStore := New(c, nil, ReadWrite)

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
			Label:    expectations.PositiveStr,
		})
	}
	err := masterStore.AddChange(ctx, delta, "test@example.com")
	require.NoError(t, err)

	entries, n, err := masterStore.QueryLog(ctx, 0, 2, true)
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
		Label:    expectations.PositiveStr,
	}, entry.Details[0])
	require.Equal(t, expectations.Delta{
		Grouping: "test_200",
		Digest:   "00000000000000000000000000000200",
		Label:    expectations.PositiveStr,
	}, entry.Details[200])
	require.Equal(t, expectations.Delta{
		Grouping: "test_400",
		Digest:   "00000000000000000000000000000400",
		Label:    expectations.PositiveStr,
	}, entry.Details[400])
	require.Equal(t, expectations.Delta{
		Grouping: "test_600",
		Digest:   "00000000000000000000000000000600",
		Label:    expectations.PositiveStr,
	}, entry.Details[600])
	require.Equal(t, expectations.Delta{
		Grouping: "test_799",
		Digest:   "00000000000000000000000000000799",
		Label:    expectations.PositiveStr,
	}, entry.Details[799])
}

// TestUndo_MasterPartition_EntriesExist_Success makes sure we can undo changes properly.
func TestUndo_MasterPartition_EntriesExist_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)
	require.NoError(t, masterStore.Initialize(ctx))

	putEntry(ctx, t, masterStore, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)
	putEntry(ctx, t, masterStore, data.AlphaTest, data.AlphaPositiveDigest, expectations.Negative, userOne) // will be undone
	putEntry(ctx, t, masterStore, data.AlphaTest, data.AlphaNegativeDigest, expectations.Negative, userOne)

	entries, _, err := masterStore.QueryLog(ctx, 0, 10, false)
	require.NoError(t, err)
	require.Len(t, entries, 3)

	toUndo := entries[1].ID
	require.NotEmpty(t, toUndo)

	require.NoError(t, masterStore.UndoChange(ctx, toUndo, userTwo))

	masterExps, err := masterStore.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, expectations.PositiveStr, masterExps.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.NegativeStr, masterExps.Classification(data.AlphaTest, data.AlphaNegativeDigest))

	// Check that the undo shows up as the most recent entry.
	entries, _, err = masterStore.QueryLog(ctx, 0, 10, true)
	require.NoError(t, err)
	require.Len(t, entries, 4)
	undidEntry := entries[0]
	assert.Equal(t, userTwo, undidEntry.User)
	assert.Equal(t, 1, undidEntry.ChangeCount)
	assert.Equal(t, expectations.Delta{
		Grouping: data.AlphaTest,
		Digest:   data.AlphaPositiveDigest,
		Label:    expectations.PositiveStr,
	}, undidEntry.Details[0])
}

// TestUndo_CLPartition_EntriesExist_Success makes sure we can undo changes properly, even if the
// background firestore snapshots are not running (e.g. for CL Expectations).
func TestUndo_CLPartition_EntriesExist_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)
	clStore := masterStore.ForChangeList("123", "github") // These are arbitrary

	putEntry(ctx, t, clStore, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive, userOne)
	putEntry(ctx, t, clStore, data.AlphaTest, data.AlphaPositiveDigest, expectations.Negative, userOne) // will be undone
	putEntry(ctx, t, clStore, data.AlphaTest, data.AlphaNegativeDigest, expectations.Negative, userOne)

	entries, _, err := clStore.QueryLog(ctx, 0, 10, false)
	require.NoError(t, err)
	require.Len(t, entries, 3)

	toUndo := entries[1].ID
	require.NotEmpty(t, toUndo)

	require.NoError(t, clStore.UndoChange(ctx, toUndo, userTwo))

	exp, err := clStore.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, expectations.PositiveStr, exp.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.NegativeStr, exp.Classification(data.AlphaTest, data.AlphaNegativeDigest))

	// Check that the undo shows up as the most recent entry.
	entries, _, err = clStore.QueryLog(ctx, 0, 10, true)
	require.NoError(t, err)
	require.Len(t, entries, 4)
	undidEntry := entries[0]
	assert.Equal(t, userTwo, undidEntry.User)
	assert.Equal(t, 1, undidEntry.ChangeCount)
	assert.Equal(t, expectations.Delta{
		Grouping: data.AlphaTest,
		Digest:   data.AlphaPositiveDigest,
		Label:    expectations.PositiveStr,
	}, undidEntry.Details[0])
}

func TestUpdateLastUsed_NoEntriesToUpdate_NothingChanges(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)

	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)

	newUsedTime := time.Date(2020, time.February, 5, 0, 0, 0, 0, time.UTC)
	err := masterStore.UpdateLastUsed(ctx, nil, newUsedTime)
	require.NoError(t, err)

	actualEntryOne := getRawEntry(ctx, t, c, entryOneGrouping, entryOneDigest)
	assertUnchanged(t, &entryOne, actualEntryOne)

	actualEntryTwo := getRawEntry(ctx, t, c, entryTwoGrouping, entryTwoDigest)
	assertUnchanged(t, &entryTwo, actualEntryTwo)

	actualEntryThree := getRawEntry(ctx, t, c, entryThreeGrouping, entryThreeDigest)
	assertUnchanged(t, &entryThree, actualEntryThree)
}

// TestUpdateLastUsed_OneEntryToUpdate_Success calls UpdateLastUsed with one entry and verifies
// that only the last_used field is modified and only for the specified entry.
func TestUpdateLastUsed_OneEntryToUpdate_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)

	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)

	newUsedTime := time.Date(2020, time.February, 5, 0, 0, 0, 0, time.UTC)
	err := masterStore.UpdateLastUsed(ctx, []expectations.ID{
		{
			Grouping: entryOneGrouping,
			Digest:   entryOneDigest,
		},
	}, newUsedTime)
	require.NoError(t, err)

	actualEntryOne := getRawEntry(ctx, t, c, entryOneGrouping, entryOneDigest)
	require.NotNil(t, actualEntryOne)
	assert.Equal(t, entryOne.Ranges, actualEntryOne.Ranges)        // no change
	assert.True(t, entryOne.Updated.Equal(actualEntryOne.Updated)) // no change
	assert.True(t, newUsedTime.Equal(actualEntryOne.LastUsed))     // change expected

	actualEntryTwo := getRawEntry(ctx, t, c, entryTwoGrouping, entryTwoDigest)
	assertUnchanged(t, &entryTwo, actualEntryTwo)

	actualEntryThree := getRawEntry(ctx, t, c, entryThreeGrouping, entryThreeDigest)
	assertUnchanged(t, &entryThree, actualEntryThree)
}

// TestUpdateLastUsed_MultipleEntriesToUpdate_Success is like the OneEntry case, except two of the
// three entries should now be updated with the new time.
func TestUpdateLastUsed_MultipleEntriesToUpdate_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)

	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)

	newUsedTime := time.Date(2020, time.February, 5, 0, 0, 0, 0, time.UTC)
	err := masterStore.UpdateLastUsed(ctx, []expectations.ID{
		// order shouldn't matter, so might as well do it "backwards"
		{
			Grouping: entryTwoGrouping,
			Digest:   entryTwoDigest,
		},
		{
			Grouping: entryOneGrouping,
			Digest:   entryOneDigest,
		},
	}, newUsedTime)
	require.NoError(t, err)

	actualEntryOne := getRawEntry(ctx, t, c, entryOneGrouping, entryOneDigest)
	require.NotNil(t, actualEntryOne)
	assert.Equal(t, entryOne.Ranges, actualEntryOne.Ranges)        // no change
	assert.True(t, entryOne.Updated.Equal(actualEntryOne.Updated)) // no change
	assert.True(t, newUsedTime.Equal(actualEntryOne.LastUsed))     // change expected

	actualEntryTwo := getRawEntry(ctx, t, c, entryTwoGrouping, entryTwoDigest)
	require.NotNil(t, actualEntryTwo)
	assert.Equal(t, entryTwo.Ranges, actualEntryTwo.Ranges)        // no change
	assert.True(t, entryTwo.Updated.Equal(actualEntryTwo.Updated)) // no change
	assert.True(t, newUsedTime.Equal(actualEntryTwo.LastUsed))     // change expected

	actualEntryThree := getRawEntry(ctx, t, c, entryThreeGrouping, entryThreeDigest)
	assertUnchanged(t, &entryThree, actualEntryThree)
}

// TestMarkUnusedEntriesForGC_EntriesRecentlyUsed_NoEntriesMarked_Success checks that we don't mark
// entries for garbage collection (untriage them) that are have been used more recently than the
// cutoff time.
func TestMarkUnusedEntriesForGC_EntriesRecentlyUsed_NoEntriesMarked_Success(t *testing.T) {
	unittest.LargeTest(t)

	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)
	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)

	// The time passed here is before all entries
	n, err := masterStore.MarkUnusedEntriesForGC(ctx, expectations.Positive, entryOne.LastUsed.Add(-time.Second))
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	// The time passed here is before all negative entries. It is after entryOne (which is positive)
	// so we still expect nothing to have changed.
	n, err = masterStore.MarkUnusedEntriesForGC(ctx, expectations.Negative, entryTwo.LastUsed.Add(-time.Second))
	require.NoError(t, err)
	assert.Equal(t, 0, n)

	// Make sure all entries are there and not marked as untriaged.
	actualEntryOne := getRawEntry(ctx, t, c, entryOneGrouping, entryOneDigest)
	assertUnchanged(t, &entryOne, actualEntryOne)
	actualEntryTwo := getRawEntry(ctx, t, c, entryTwoGrouping, entryTwoDigest)
	assertUnchanged(t, &entryTwo, actualEntryTwo)
	actualEntryThree := getRawEntry(ctx, t, c, entryThreeGrouping, entryThreeDigest)
	assertUnchanged(t, &entryThree, actualEntryThree)
}

// TestMarkUnusedEntriesForGC_OnePositiveEntryMarked_Success tests where a single entry (the first)
// is marked for garbage collection.
func TestMarkUnusedEntriesForGC_OnePositiveEntryMarked_Success(t *testing.T) {
	unittest.LargeTest(t)

	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)
	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)

	// The time here is selected to be after both entryOne and entryTwo were last used, to make
	// sure that we are respecting the label.
	cutoff := entryThree.LastUsed.Add(-time.Minute)
	assert.True(t, cutoff.After(entryOne.LastUsed))
	assert.True(t, cutoff.After(entryTwo.LastUsed))
	n, err := masterStore.MarkUnusedEntriesForGC(ctx, expectations.Positive, cutoff)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	// Make sure all entries are still there, just entryOne is Untriaged
	actualEntryOne := getRawEntry(ctx, t, c, entryOneGrouping, entryOneDigest)
	require.NotNil(t, actualEntryOne)
	assert.True(t, actualEntryOne.NeedsGC)
	actualEntryTwo := getRawEntry(ctx, t, c, entryTwoGrouping, entryTwoDigest)
	assertUnchanged(t, &entryTwo, actualEntryTwo)
	actualEntryThree := getRawEntry(ctx, t, c, entryThreeGrouping, entryThreeDigest)
	assertUnchanged(t, &entryThree, actualEntryThree)
}

// TestMarkUnusedEntriesForGC_OneNegativeEntryMarked_Success tests where the middle entry (the
// only negative) entry is marked for garbage collection.
func TestMarkUnusedEntriesForGC_OneNegativeEntryMarked_Success(t *testing.T) {
	unittest.LargeTest(t)

	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)

	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)
	// This time is picked to be after all entries
	cutoff := entryThree.LastUsed.Add(time.Minute)
	assert.True(t, cutoff.After(entryOne.LastUsed))
	assert.True(t, cutoff.After(entryTwo.LastUsed))
	n, err := masterStore.MarkUnusedEntriesForGC(ctx, expectations.Negative, cutoff)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	// Make sure all entries are still there, just entryTwo is marked for GC.
	actualEntryOne := getRawEntry(ctx, t, c, entryOneGrouping, entryOneDigest)
	assertUnchanged(t, &entryOne, actualEntryOne)
	actualEntryTwo := getRawEntry(ctx, t, c, entryTwoGrouping, entryTwoDigest)
	require.NotNil(t, actualEntryTwo)
	assert.True(t, actualEntryTwo.NeedsGC)
	actualEntryThree := getRawEntry(ctx, t, c, entryThreeGrouping, entryThreeDigest)
	assertUnchanged(t, &entryThree, actualEntryThree)
}

// TestMarkUnusedEntriesForGC_MultiplePositiveEntriesAffected tests where we mark both positive
// entries for garbage collecting (not matching the negative one in the middle).
func TestMarkUnusedEntriesForGC_MultiplePositiveEntriesAffected(t *testing.T) {
	unittest.LargeTest(t)

	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)
	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)

	// This time is picked to be after all entries
	cutoff := entryThree.LastUsed.Add(time.Minute)
	assert.True(t, cutoff.After(entryOne.LastUsed))
	assert.True(t, cutoff.After(entryTwo.LastUsed))
	n, err := masterStore.MarkUnusedEntriesForGC(ctx, expectations.Positive, cutoff)
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	// Make sure all entries are still there, entryOne and entryThree are marked for GC.
	actualEntryOne := getRawEntry(ctx, t, c, entryOneGrouping, entryOneDigest)
	require.NotNil(t, actualEntryOne)
	assert.True(t, actualEntryOne.NeedsGC)
	actualEntryTwo := getRawEntry(ctx, t, c, entryTwoGrouping, entryTwoDigest)
	assertUnchanged(t, &entryTwo, actualEntryTwo)
	actualEntryThree := getRawEntry(ctx, t, c, entryThreeGrouping, entryThreeDigest)
	require.NotNil(t, actualEntryThree)
	assert.True(t, actualEntryThree.NeedsGC)
}

// TestMarkUnusedEntriesForGC_LastUsedLongAgo_UpdatedRecently_NoEntriesMarked_Success tests where
// we don't mark entries for GC which have not been seen in a while, but were modified recently.
func TestMarkUnusedEntriesForGC_LastUsedLongAgo_UpdatedRecently_NoEntriesMarked_Success(t *testing.T) {
	unittest.LargeTest(t)

	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)
	// This is well after entryThree.LastUsed
	moreRecently := time.Date(2020, time.March, 1, 1, 1, 1, 0, time.UTC)
	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, moreRecently)
	assert.True(t, moreRecently.After(entryThree.LastUsed))

	// This time is picked to be after all entries
	cutoff := entryThree.LastUsed.Add(time.Minute)
	assert.True(t, cutoff.After(entryOne.LastUsed))
	assert.True(t, cutoff.After(entryTwo.LastUsed))
	n, err := masterStore.MarkUnusedEntriesForGC(ctx, expectations.Positive, cutoff)
	require.NoError(t, err)
	// None should be affected because the modified stamp is too new.
	assert.Equal(t, 0, n)

	// Make sure all entries are still there and none were marked for GC.
	actualEntryOne := getRawEntry(ctx, t, c, entryOneGrouping, entryOneDigest)
	assertUnchanged(t, &entryOne, actualEntryOne)
	actualEntryTwo := getRawEntry(ctx, t, c, entryTwoGrouping, entryTwoDigest)
	assertUnchanged(t, &entryTwo, actualEntryTwo)
	actualEntryThree := getRawEntry(ctx, t, c, entryThreeGrouping, entryThreeDigest)
	assertUnchanged(t, &entryThree, actualEntryThree)
}

// TestGarbageCollect_MultipleEntriesDeleted tests case where we mark two entries for GC and then
// cleanup those entries so they are not in Firestore anymore.
func TestGarbageCollect_MultipleEntriesDeleted(t *testing.T) {
	unittest.LargeTest(t)

	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)
	_, entryTwo, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)

	n, err := masterStore.MarkUnusedEntriesForGC(ctx, expectations.Positive, entryThree.LastUsed.Add(time.Minute))
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	n, err = masterStore.GarbageCollect(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	// Make sure entryOne and entryThree are not there (e.g. now nil)
	actualEntryOne := getRawEntry(ctx, t, c, entryOneGrouping, entryOneDigest)
	assert.Nil(t, actualEntryOne)
	actualEntryTwo := getRawEntry(ctx, t, c, entryTwoGrouping, entryTwoDigest)
	require.NotNil(t, actualEntryTwo)
	assertUnchanged(t, &entryTwo, actualEntryTwo)
	actualEntryThree := getRawEntry(ctx, t, c, entryThreeGrouping, entryThreeDigest)
	assert.Nil(t, actualEntryThree)
}

// TestGarbageCollect_NoEntriesDeleted tests case where there are no entries to clean up.
// Of note, trying to call .Commit() on an empty firestore.Batch() results in an error in
// production (and a hang in the test using the emulator). This test makes sure we avoid that.
func TestGarbageCollect_NoEntriesDeleted(t *testing.T) {
	unittest.LargeTest(t)

	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)
	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)

	n, err := masterStore.GarbageCollect(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, n)

	// Make sure entryOne and entryTwo are not there (e.g. now nil)
	actualEntryOne := getRawEntry(ctx, t, c, entryOneGrouping, entryOneDigest)
	assertUnchanged(t, &entryOne, actualEntryOne)
	actualEntryTwo := getRawEntry(ctx, t, c, entryTwoGrouping, entryTwoDigest)
	assertUnchanged(t, &entryTwo, actualEntryTwo)
	actualEntryThree := getRawEntry(ctx, t, c, entryThreeGrouping, entryThreeDigest)
	assertUnchanged(t, &entryThree, actualEntryThree)
}

// TestMarkUnusedEntriesForGC_CLEntriesNotAffected_Success tests that CL expectations are immune
// from being marked for cleanup.
func TestMarkUnusedEntriesForGC_CLEntriesNotAffected_Success(t *testing.T) {
	unittest.LargeTest(t)

	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	masterStore := New(c, nil, ReadWrite)

	clExp := masterStore.ForChangeList("cl1234", "crs")
	err := clExp.AddChange(ctx, []expectations.Delta{
		{
			Grouping: entryOneGrouping,
			Digest:   entryOneDigest,
			Label:    expectations.PositiveStr,
		},
	}, "test@example.com")
	require.NoError(t, err)

	cutoff := time.Now().Add(time.Hour)
	n, err := masterStore.MarkUnusedEntriesForGC(ctx, expectations.Positive, cutoff)
	require.NoError(t, err)
	assert.Equal(t, 0, n)

	// Make sure the original CL entry is there, still positive.
	actualEntryOne := getRawCLEntry(ctx, t, c, entryOneGrouping, entryOneDigest, "crs_cl1234")
	require.NotNil(t, actualEntryOne)
	assert.False(t, actualEntryOne.NeedsGC)
	assert.Equal(t, []triageRange{
		{
			FirstIndex: beginningOfTime,
			LastIndex:  endOfTime,
			Label:      expectations.Positive,
		},
	}, actualEntryOne.Ranges)
}

// normalizeEntries fixes the non-deterministic parts of TriageLogEntry to be deterministic
func normalizeEntries(t *testing.T, entries []expectations.TriageLogEntry) {
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
			Label:    label.String(),
		},
	}, user))
}

// An arbitrary date a long time before the times used in populateFirestore.
var updatedLongAgo = time.Date(2019, time.January, 1, 1, 1, 1, 0, time.UTC)

const (
	entryOneGrouping   = data.AlphaTest
	entryOneDigest     = data.AlphaPositiveDigest
	entryTwoGrouping   = data.AlphaTest
	entryTwoDigest     = data.AlphaNegativeDigest
	entryThreeGrouping = data.BetaTest
	entryThreeDigest   = data.BetaPositiveDigest

	userOne = "userOne@example.com"
	userTwo = "userTwo@example.com"
)

// populateFirestore creates three manual entries in firestore, corresponding to the
// three_devices data. It uses three different times for LastUsed and the same (provided) time
// for modified for each of the entries. Then, it returns the created entries for use in asserts.
func populateFirestore(ctx context.Context, t *testing.T, c *ifirestore.Client, modified time.Time) (expectationEntry, expectationEntry, expectationEntry) {
	// For convenience, these times are spaced a few days apart at midnight in ascending order.
	var entryOneUsed = time.Date(2020, time.January, 28, 0, 0, 0, 0, time.UTC)
	var entryTwoUsed = time.Date(2020, time.January, 30, 0, 0, 0, 0, time.UTC)
	var entryThreeUsed = time.Date(2020, time.February, 2, 0, 0, 0, 0, time.UTC)

	entryOne := expectationEntry{
		Grouping: entryOneGrouping,
		Digest:   entryOneDigest,
		Ranges: []triageRange{
			{FirstIndex: beginningOfTime, LastIndex: endOfTime, Label: expectations.Positive},
		},
		Updated:  modified,
		LastUsed: entryOneUsed,
	}
	entryTwo := expectationEntry{
		Grouping: entryTwoGrouping,
		Digest:   entryTwoDigest,
		Ranges: []triageRange{
			{FirstIndex: beginningOfTime, LastIndex: endOfTime, Label: expectations.Negative},
		},
		Updated:  modified,
		LastUsed: entryTwoUsed,
	}
	entryThree := expectationEntry{
		Grouping: entryThreeGrouping,
		Digest:   entryThreeDigest,
		Ranges: []triageRange{
			{FirstIndex: beginningOfTime, LastIndex: endOfTime, Label: expectations.Positive},
		},
		Updated:  modified,
		LastUsed: entryThreeUsed,
	}
	createRawEntry(ctx, t, c, entryOne)
	createRawEntry(ctx, t, c, entryTwo)
	createRawEntry(ctx, t, c, entryThree)
	return entryOne, entryTwo, entryThree
}

// createRawEntry creates the bare expectationEntry in firestore.
func createRawEntry(ctx context.Context, t *testing.T, c *ifirestore.Client, entry expectationEntry) {
	doc := c.Collection(partitions).Doc(masterPartition).Collection(expectationEntries).Doc(entry.ID())
	_, err := doc.Create(ctx, entry)
	require.NoError(t, err)
}

func assertUnchanged(t *testing.T, expected, actual *expectationEntry) {
	require.NotNil(t, expected)
	require.NotNil(t, actual)
	require.Len(t, actual.Ranges, 1)
	assert.Equal(t, expected.Ranges[0], actual.Ranges[0])
	assert.True(t, expected.Updated.Equal(actual.Updated))
	assert.True(t, expected.LastUsed.Equal(actual.LastUsed))
	assert.Equal(t, expected.NeedsGC, actual.NeedsGC)
}

// getRawEntry returns the bare expectationEntry from firestore for the given name/digest.
func getRawEntry(ctx context.Context, t *testing.T, c *ifirestore.Client, name types.TestName, digest types.Digest) *expectationEntry {
	entry := expectationEntry{Grouping: name, Digest: digest}
	doc := c.Collection(partitions).Doc(masterPartition).Collection(expectationEntries).Doc(entry.ID())
	ds, err := doc.Get(ctx)
	if err != nil {
		// This error could indicated not found, which may be expected by some tests.
		return nil
	}
	err = ds.DataTo(&entry)
	require.NoError(t, err)
	return &entry
}

func getRawCLEntry(ctx context.Context, t *testing.T, c *ifirestore.Client, name types.TestName, digest types.Digest, crsCLID string) *expectationEntry {
	entry := expectationEntry{Grouping: name, Digest: digest}
	doc := c.Collection(partitions).Doc(crsCLID).Collection(expectationEntries).Doc(entry.ID())
	ds, err := doc.Get(ctx)
	if err != nil {
		// This error could indicated not found, which may be expected by some tests.
		return nil
	}
	err = ds.DataTo(&entry)
	require.NoError(t, err)
	return &entry
}

// makeBigExpectations makes (end-start) tests named from start to end that each have 32 digests.
func makeBigExpectations(start, end int) (*expectations.Expectations, []expectations.Delta) {
	var e expectations.Expectations
	var delta []expectations.Delta
	for i := start; i < end; i++ {
		for j := 0; j < 32; j++ {
			tn := types.TestName(fmt.Sprintf("test-%03d", i))
			d := types.Digest(fmt.Sprintf("digest-%03d", j))
			e.Set(tn, d, expectations.PositiveStr)
			delta = append(delta, expectations.Delta{
				Grouping: tn,
				Digest:   d,
				Label:    expectations.PositiveStr,
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
