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
	e := expectationEntry2{
		Grouping: "downsample/images/mandrill_512.png",
		Digest:   "36bc7da524f2869c97f0a0f1d7042110",
	}
	assert.Equal(t, "downsample-images-mandrill_512.png|36bc7da524f2869c97f0a0f1d7042110",
		e.ID())
}

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

// TestGetCopy_NoInitialize_CallerMutatesReturnValue_Success mutates the result of GetCopy and
// makes sure that future calls to GetCopy are not affected.
func TestGetCopy_NoInitialize_CallerMutatesReturnValue_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New2(c, nil, ReadWrite)
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

// TestGetCopy_Initialize_CallerMutatesReturnValue_Success mutates the result of GetCopy and
// makes sure that future calls to GetCopy are not affected.
func TestGetCopy_Initialize_CallerMutatesReturnValue_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New2(c, nil, ReadWrite)
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

	assert.Equal(t, 5, countExpectationChanges(ctx, t, f))
	assert.Equal(t, 5, countTriageRecords(ctx, t, f))
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
	assert.Equal(t, 50, countExpectationChanges(ctx, t, f))
	assert.Equal(t, 50, countTriageRecords(ctx, t, f))
}

// TestGetTriageHistory_RepeatedlyOverwriteOneEntry_Success repeatedly overwrites a single entry to make sure the cache
// reflects reality and that our history is complete.
func TestGetTriageHistory_RepeatedlyOverwriteOneEntry_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New2(c, nil, ReadWrite)
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

// TestGetTriageHistory_SunnyDay writes some changes and then gets the triage history for those
// changes. Even if we query for records that don't exist, we should not see errors.
func TestGetTriageHistory_SunnyDay_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New2(c, nil, ReadWrite)

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

// TestAddChange_TwoLargeSimultaneousBatches writes two batches of 512 entries to test the batch
// writing that happens for large amounts of expectation changes.
func TestAddChange_TwoLargeSimultaneousBatches(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New2(c, nil, ReadWrite)

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

func TestQueryLog_WithoutDetails_OffsetsAndLimitsAreRespected(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New2(c, nil, ReadWrite)
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

func TestQueryLog_InvalidOffsets_Error(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New2(c, nil, ReadWrite)
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

	f := New2(c, nil, ReadWrite)
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

	f := New2(c, nil, ReadWrite)

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

// TestUndo_WithInitialize_EntriesExist_Success makes sure we can undo changes properly.
func TestUndo_WithInitialize_EntriesExist_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New2(c, nil, ReadWrite)
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

// TestUndo_NoInitialize_EntriesExist_Success makes sure we can undo changes properly, even if the
// background firestore snapshots are not running (e.g. for CL Expectations).
func TestUndo_NoInitialize_EntriesExist_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f := New2(c, nil, ReadWrite)

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

func TestUpdateLastUsed_NoEntriesToUpdate_NothingChanges(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	exp := New2(c, nil, ReadWrite)

	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)

	newUsedTime := time.Date(2020, time.February, 5, 0, 0, 0, 0, time.UTC)
	err := exp.UpdateLastUsed(ctx, nil, newUsedTime)
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

	exp := New2(c, nil, ReadWrite)

	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)

	newUsedTime := time.Date(2020, time.February, 5, 0, 0, 0, 0, time.UTC)
	err := exp.UpdateLastUsed(ctx, []expectations.ID{
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

	exp := New2(c, nil, ReadWrite)

	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)

	newUsedTime := time.Date(2020, time.February, 5, 0, 0, 0, 0, time.UTC)
	err := exp.UpdateLastUsed(ctx, []expectations.ID{
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

// normalizeEntries2 fixes the non-deterministic parts of TriageLogEntry to be deterministic
func normalizeEntries2(t *testing.T, entries []expectations.TriageLogEntry) {
	for i, te := range entries {
		require.NotEqual(t, "", te.ID)
		te.ID = "was_random_" + strconv.Itoa(i)
		entries[i] = te
	}
}

func countExpectationChanges(ctx context.Context, t *testing.T, f *Store2) int {
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

func countTriageRecords(ctx context.Context, t *testing.T, f *Store2) int {
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

func putEntry(ctx context.Context, t *testing.T, f *Store2, name types.TestName, digest types.Digest, label expectations.Label, user string) {
	require.NoError(t, f.AddChange(ctx, []expectations.Delta{
		{
			Grouping: name,
			Digest:   digest,
			Label:    label,
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
)

// populateFirestore creates three manual entries in firestore, corresponding to the
// three_devices data. It uses three different times for LastUsed and the same (provided) time
// for modified for each of the entries. Then, it returns the created entries for use in asserts.
func populateFirestore(ctx context.Context, t *testing.T, c *ifirestore.Client, modified time.Time) (expectationEntry2, expectationEntry2, expectationEntry2) {
	// For convenience, these times are spaced a few days apart at midnight in ascending order.
	var entryOneUsed = time.Date(2020, time.January, 28, 0, 0, 0, 0, time.UTC)
	var entryTwoUsed = time.Date(2020, time.January, 30, 0, 0, 0, 0, time.UTC)
	var entryThreeUsed = time.Date(2020, time.February, 2, 0, 0, 0, 0, time.UTC)

	entryOne := expectationEntry2{
		Grouping: entryOneGrouping,
		Digest:   entryOneDigest,
		Ranges: []triageRange{
			{FirstIndex: beginningOfTime, LastIndex: endOfTime, Label: expectations.Positive},
		},
		Updated:  modified,
		LastUsed: entryOneUsed,
	}
	entryTwo := expectationEntry2{
		Grouping: entryTwoGrouping,
		Digest:   entryTwoDigest,
		Ranges: []triageRange{
			{FirstIndex: beginningOfTime, LastIndex: endOfTime, Label: expectations.Negative},
		},
		Updated:  modified,
		LastUsed: entryTwoUsed,
	}
	entryThree := expectationEntry2{
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
func createRawEntry(ctx context.Context, t *testing.T, c *ifirestore.Client, entry expectationEntry2) {
	doc := c.Collection(partitions).Doc(masterPartition).Collection(expectationEntries).Doc(entry.ID())
	_, err := doc.Create(ctx, entry)
	require.NoError(t, err)
}

func assertUnchanged(t *testing.T, expected, actual *expectationEntry2) {
	require.NotNil(t, expected)
	require.NotNil(t, actual)
	require.Len(t, actual.Ranges, 1)
	assert.Equal(t, expected.Ranges[0], actual.Ranges[0])
	assert.True(t, expected.Updated.Equal(actual.Updated))
	assert.True(t, expected.LastUsed.Equal(actual.LastUsed))
}

// getRawEntry returns the bare expectationEntry from firestore for the given name/digest.
func getRawEntry(ctx context.Context, t *testing.T, c *ifirestore.Client, name types.TestName, digest types.Digest) *expectationEntry2 {
	entry := expectationEntry2{Grouping: name, Digest: digest}
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
