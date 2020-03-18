package fs_expectationstore

import (
	"context"
	"fmt"
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
			Label:    expectations.Positive, // Intentionally wrong. Will be fixed by the next AddChange.
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaPositiveDigest,
			Label:    expectations.Positive,
		},
	}, userOne)
	require.NoError(t, err)

	err = clStore.AddChange(ctx, []expectations.Delta{
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
			Label:    expectations.Positive,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaPositiveDigest,
			Label:    expectations.Positive,
		},
	}, userOne)
	require.NoError(t, err)

	err = masterStore.AddChange(ctx, []expectations.Delta{
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
	assert.Equal(t, expectations.Positive, e.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.Negative, e.Classification(data.AlphaTest, data.AlphaNegativeDigest))
	assert.Equal(t, expectations.Untriaged, e.Classification(data.AlphaTest, data.AlphaUntriagedDigest))
	assert.Equal(t, expectations.Positive, e.Classification(data.BetaTest, data.BetaPositiveDigest))
	assert.Equal(t, expectations.Untriaged, e.Classification(data.BetaTest, data.BetaUntriagedDigest))
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
	assert.Equal(t, expectations.Positive, clExps.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.Untriaged, clExps.Classification(data.AlphaTest, data.AlphaUntriagedDigest))

	clExps.Set(data.AlphaTest, data.AlphaPositiveDigest, expectations.Negative)
	clExps.Set(data.AlphaTest, data.AlphaUntriagedDigest, expectations.Positive)

	shouldBeUnaffected, err := clStore.GetCopy(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, shouldBeUnaffected.Len())
	assert.Equal(t, expectations.Positive, shouldBeUnaffected.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.Untriaged, shouldBeUnaffected.Classification(data.AlphaTest, data.AlphaUntriagedDigest))
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
	assert.Equal(t, expectations.Positive, masterExps.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.Untriaged, masterExps.Classification(data.AlphaTest, data.AlphaUntriagedDigest))

	masterExps.Set(data.AlphaTest, data.AlphaPositiveDigest, expectations.Negative)
	masterExps.Set(data.AlphaTest, data.AlphaUntriagedDigest, expectations.Positive)

	shouldBeUnaffected, err := masterStore.GetCopy(ctx)
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
	assert.Equal(t, expectations.Positive, roExps.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.Negative, roExps.Classification(data.AlphaTest, data.AlphaNegativeDigest))
	assert.Equal(t, expectations.Positive, roExps.Classification(data.AlphaTest, firstPositiveThenUntriaged))

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
	assert.Equal(t, expectations.Untriaged, roExps2.Classification(data.AlphaTest, firstPositiveThenUntriaged))

	// Spot check that the expectations we got first were not impacted by the new expectations
	// coming in or the second call to Get.
	assert.Equal(t, expectations.Positive, roExps.Classification(data.AlphaTest, firstPositiveThenUntriaged))

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

// TestAddChange_ExpectationsDontConflictBetweenMasterAndCLPartition tests the separation of
// the master expectations and the CL expectations. It starts with a single expectation, then adds
// some expectations to both, including changing the expectation. Specifically, the CL expectations
// should be treated as a delta to the master expectations (but doesn't actually contain
// master expectations).
func TestAddChange_ExpectationsDontConflictBetweenMasterAndCLPartition(t *testing.T) {
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
	assert.Equal(t, expectations.Negative, masterExps.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.Negative, masterExps.Classification(data.AlphaTest, data.AlphaNegativeDigest))
	assert.Equal(t, expectations.Untriaged, masterExps.Classification(data.BetaTest, data.BetaPositiveDigest))
	assert.Equal(t, 2, masterExps.Len())

	// Make sure the CL expectations are separate from the master expectations.
	assert.Equal(t, expectations.Positive, clExps.Classification(data.AlphaTest, data.AlphaPositiveDigest))
	assert.Equal(t, expectations.Untriaged, clExps.Classification(data.AlphaTest, data.AlphaNegativeDigest))
	assert.Equal(t, expectations.Positive, clExps.Classification(data.BetaTest, data.BetaPositiveDigest))
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

	require.NoError(t, masterStore.AddChange(ctx, change1, userOne))
	require.NoError(t, masterStore.AddChange(ctx, change2, userTwo))

	assert.Eventually(t, func() bool {
		masterStore.entryCacheMutex.RLock()
		defer masterStore.entryCacheMutex.RUnlock()
		return len(masterStore.entryCache) == 3
	}, 10*time.Second, 100*time.Millisecond)

	assert.ElementsMatch(t, []expectations.ID{change1[0].ID(), change2[0].ID(), change2[1].ID()}, calledWith)
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
