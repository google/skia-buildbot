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
	putEntry(ctx, t, f, data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive)
	putEntry(ctx, t, f, data.AlphaTest, data.AlphaNegativeDigest, expectations.Negative)
	putEntry(ctx, t, f, data.AlphaTest, firstPositiveThenUntriaged, expectations.Positive)

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
	putEntry(ctx, t, f, data.AlphaTest, firstPositiveThenUntriaged, expectations.Untriaged)
	putEntry(ctx, t, f, data.BetaTest, data.BetaPositiveDigest, expectations.Positive)

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

func putEntry(ctx context.Context, t *testing.T, f *Store2, name types.TestName, digest types.Digest, label expectations.Label) {
	require.NoError(t, f.AddChange(ctx, []expectations.Delta{
		{
			Grouping: name,
			Digest:   digest,
			Label:    label,
		},
	}, userOne))
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
