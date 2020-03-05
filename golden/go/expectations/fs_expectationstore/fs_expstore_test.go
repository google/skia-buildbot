package fs_expectationstore

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/expectations"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
)

// TestGetExpectations writes some changes and then reads back the
// aggregated results.
func TestGetExpectations(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	// Brand new instance should have no expectations
	e, err := f.Get(ctx)
	require.NoError(t, err)
	require.True(t, e.Empty())

	err = f.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaUntriaged1Digest,
			Label:    expectations.Positive,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    expectations.Positive,
		},
	}, userOne)
	require.NoError(t, err)

	err = f.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaBad1Digest,
			Label:    expectations.Negative,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaUntriaged1Digest, // overwrites previous
			Label:    expectations.Untriaged,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaGood1Digest,
			Label:    expectations.Positive,
		},
	}, userTwo)
	require.NoError(t, err)

	e, err = f.Get(ctx)
	require.NoError(t, err)
	assertExpectationsMatchDefaults(t, e)
	// Make sure that if we create a new view, we can read the results immediately.
	fr, err := New(ctx, c, nil, ReadOnly)
	require.NoError(t, err)
	e, err = fr.Get(ctx)
	require.NoError(t, err)
	assertExpectationsMatchDefaults(t, e)
}

func assertExpectationsMatchDefaults(t *testing.T, e expectations.ReadOnly) {
	assert.Equal(t, expectations.Positive, e.Classification(data.AlphaTest, data.AlphaGood1Digest))
	assert.Equal(t, expectations.Negative, e.Classification(data.AlphaTest, data.AlphaBad1Digest))
	assert.Equal(t, expectations.Untriaged, e.Classification(data.AlphaTest, data.AlphaUntriaged1Digest))
	assert.Equal(t, expectations.Positive, e.Classification(data.BetaTest, data.BetaGood1Digest))
	assert.Equal(t, expectations.Untriaged, e.Classification(data.BetaTest, data.BetaUntriaged1Digest))
	assert.Equal(t, 3, e.Len())
}

// TestGetExpectationsSnapShot has both a read-write and a read version and makes sure
// that the changes to the read-write version eventually propagate to the read version
// via the QuerySnapshot.
func TestGetExpectationsSnapShot(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	err = f.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaUntriaged1Digest,
			Label:    expectations.Positive,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    expectations.Positive,
		},
	}, userOne)
	require.NoError(t, err)

	ro, err := New(ctx, c, nil, ReadOnly)
	require.NoError(t, err)
	require.NotNil(t, ro)

	exp, err := ro.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, expectations.Positive, exp.Classification(data.AlphaTest, data.AlphaUntriaged1Digest))
	require.Equal(t, expectations.Positive, exp.Classification(data.AlphaTest, data.AlphaGood1Digest))
	require.Equal(t, expectations.Untriaged, exp.Classification(data.AlphaTest, data.AlphaBad1Digest))
	require.Equal(t, 2, exp.Len())

	err = f.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaBad1Digest,
			Label:    expectations.Negative,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaUntriaged1Digest, // overwrites previous
			Label:    expectations.Untriaged,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaGood1Digest,
			Label:    expectations.Positive,
		},
	}, userTwo)
	require.NoError(t, err)

	e, err := ro.Get(ctx)
	require.NoError(t, err)
	assertExpectationsMatchDefaults(t, e)
}

// TestGetExpectationsRace writes a bunch of data from many go routines
// in an effort to catch any race conditions in the caching layer.
func TestGetExpectationsRace(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	type entry struct {
		Grouping types.TestName
		Digest   types.Digest
		Label    expectations.Label
	}

	entries := []entry{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaUntriaged1Digest,
			Label:    expectations.Untriaged,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaBad1Digest,
			Label:    expectations.Negative,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    expectations.Positive,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaGood1Digest,
			Label:    expectations.Positive,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaUntriaged1Digest,
			Label:    expectations.Untriaged,
		},
	}

	wg := sync.WaitGroup{}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e := entries[i%len(entries)]
			err := f.AddChange(ctx, []expectations.Delta{
				{
					Grouping: e.Grouping,
					Digest:   e.Digest,
					Label:    e.Label,
				},
			}, userOne)
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

// TestGetExpectationsBig writes 32^2=1024 entries
// to test the batch writing.
func TestGetExpectationsBig(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	// Write the expectations in two, non-overlapping blocks.
	exp1, delta1 := makeBigExpectations(0, 16)
	exp2, delta2 := makeBigExpectations(16, 32)

	expected := exp1.DeepCopy()
	expected.MergeExpectations(exp2)

	wg := sync.WaitGroup{}

	// Write them concurrently to test for races.
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

	// We wait for the query snapshots to be notified about the change.
	require.Eventually(t, func() bool {
		// Fetch a copy to avoid a race between Get() and DeepEqual
		e, err := f.GetCopy(ctx)
		assert.NoError(t, err)
		return deepequal.DeepEqual(expected, e)
	}, 10*time.Second, 100*time.Millisecond)

	// Make sure that if we create a new view, we can read the results
	// from the table to make the expectations
	fr, err := New(ctx, c, nil, ReadOnly)
	require.NoError(t, err)
	e, err := fr.GetCopy(ctx)
	require.NoError(t, err)
	require.Equal(t, expected, e)
}

// TestReadOnly ensures a read-only instance fails to write data.
func TestReadOnly(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f, err := New(ctx, c, nil, ReadOnly)
	require.NoError(t, err)

	err = f.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    expectations.Positive,
		},
	}, userOne)
	require.Error(t, err)
	require.Contains(t, err.Error(), "read-only")
}

// TestQueryLog tests that we can query logs at a given place
func TestQueryLog(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	fillWith4Entries(t, f)

	entries, n, err := f.QueryLog(ctx, 0, 100, false)
	require.NoError(t, err)
	require.Equal(t, 4, n) // 4 operations
	require.Len(t, entries, 4)

	now := time.Now()
	normalizeEntries(t, now, entries)
	require.Equal(t, []expectations.TriageLogEntry{
		{
			ID:          "was_random_0",
			User:        userTwo,
			TS:          now,
			ChangeCount: 2,
			Details:     nil,
		},
		{
			ID:          "was_random_1",
			User:        userOne,
			TS:          now,
			ChangeCount: 1,
			Details:     nil,
		},
		{
			ID:          "was_random_2",
			User:        userTwo,
			TS:          now,
			ChangeCount: 1,
			Details:     nil,
		},
		{
			ID:          "was_random_3",
			User:        userOne,
			TS:          now,
			ChangeCount: 1,
			Details:     nil,
		},
	}, entries)

	entries, n, err = f.QueryLog(ctx, 1, 2, false)
	require.NoError(t, err)
	require.Equal(t, expectations.CountMany, n)
	require.Len(t, entries, 2)

	normalizeEntries(t, now, entries)
	require.Equal(t, []expectations.TriageLogEntry{
		{
			ID:          "was_random_0",
			User:        userOne,
			TS:          now,
			ChangeCount: 1,
			Details:     nil,
		},
		{
			ID:          "was_random_1",
			User:        userTwo,
			TS:          now,
			ChangeCount: 1,
			Details:     nil,
		},
	}, entries)

	// Make sure we can handle an invalid offset
	entries, n, err = f.QueryLog(ctx, 500, 100, false)
	require.NoError(t, err)
	require.Equal(t, 500, n) // The system guesses that there are 500 or fewer items.
	require.Empty(t, entries)
}

// TestQueryLogDetails checks that the details are filled in when requested.
func TestQueryLogDetails(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	fillWith4Entries(t, f)

	entries, n, err := f.QueryLog(ctx, 0, 100, true)
	require.NoError(t, err)
	require.Equal(t, 4, n) // 4 operations
	require.Len(t, entries, 4)

	// These should be sorted, starting with the most recent
	require.Equal(t, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaBad1Digest,
			Label:    expectations.Negative,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaUntriaged1Digest,
			Label:    expectations.Untriaged,
		},
	}, entries[0].Details)
	require.Equal(t, []expectations.Delta{
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaGood1Digest,
			Label:    expectations.Positive,
		},
	}, entries[1].Details)
	require.Equal(t, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    expectations.Positive,
		},
	}, entries[2].Details)
	require.Equal(t, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    expectations.Negative,
		},
	}, entries[3].Details)
}

// TestQueryLogDetailsLarge checks that the details are filled in correctly, even in cases
// where we had to write in multiple chunks. (skbug.com/9485)
func TestQueryLogDetailsLarge(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	// 800 should spread us across 3 "shards", which are ~250 expectations.
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
	err = f.AddChange(ctx, delta, "test@example.com")
	require.NoError(t, err)

	entries, n, err := f.QueryLog(ctx, 0, 2, true)
	require.NoError(t, err)
	require.Equal(t, 1, n) // 1 big operation
	require.Len(t, entries, 1)

	entry := entries[0]
	require.Equal(t, numExp, entry.ChangeCount)
	require.Len(t, entry.Details, numExp)

	// spot check some details
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

// TestUndoChangeSunnyDay checks undoing entries that exist.
func TestUndoChangeSunnyDay(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	fillWith4Entries(t, f)

	entries, n, err := f.QueryLog(ctx, 0, 4, false)
	require.NoError(t, err)
	require.Equal(t, expectations.CountMany, n)
	require.Len(t, entries, 4)

	err = f.UndoChange(ctx, entries[0].ID, userOne)
	require.NoError(t, err)

	err = f.UndoChange(ctx, entries[2].ID, userOne)
	require.NoError(t, err)

	// Check that the undone items were applied
	exp, err := f.Get(ctx)
	require.NoError(t, err)

	assertMatches := func(e expectations.ReadOnly) {
		assert.Equal(t, e.Classification(data.AlphaTest, data.AlphaGood1Digest), expectations.Negative)
		assert.Equal(t, e.Classification(data.AlphaTest, data.AlphaBad1Digest), expectations.Untriaged)
		assert.Equal(t, e.Classification(data.BetaTest, data.BetaGood1Digest), expectations.Positive)
		assert.Equal(t, e.Classification(data.BetaTest, data.BetaUntriaged1Digest), expectations.Untriaged)
		assert.Equal(t, 2, e.Len())
	}
	assertMatches(exp)

	// Make sure that if we create a new view, we can read the results
	// from the table to make the expectations
	fr, err := New(ctx, c, nil, ReadOnly)
	require.NoError(t, err)
	exp, err = fr.Get(ctx)
	require.NoError(t, err)
	assertMatches(exp)
}

// TestUndoChangeUntriaged checks undoing entries that were set to Untriaged. For example,
// a user accidentally marks something as untriaged and then undoes that.
func TestUndoChangeUntriaged(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	require.NoError(t, f.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    expectations.Positive,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaBad1Digest,
			Label:    expectations.Negative,
		},
	}, userOne))

	require.NoError(t, f.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaBad1Digest,
			Label:    expectations.Untriaged,
		},
	}, userTwo))

	// Make sure the "oops" marking of untriaged was applied:
	exp, err := f.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, expectations.Positive, exp.Classification(data.AlphaTest, data.AlphaGood1Digest))
	require.Equal(t, expectations.Untriaged, exp.Classification(data.AlphaTest, data.AlphaBad1Digest))
	require.Equal(t, 1, exp.Len())

	entries, _, err := f.QueryLog(ctx, 0, 1, false)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	err = f.UndoChange(ctx, entries[0].ID, userTwo)
	require.NoError(t, err)

	// Check that we reset from Untriaged back to Negative.
	exp, err = f.Get(ctx)
	require.NoError(t, err)

	assertMatches := func(e expectations.ReadOnly) {
		assert.Equal(t, expectations.Positive, e.Classification(data.AlphaTest, data.AlphaGood1Digest))
		assert.Equal(t, expectations.Negative, e.Classification(data.AlphaTest, data.AlphaBad1Digest))
		assert.Equal(t, 2, e.Len())
	}
	assertMatches(exp)

	// Make sure that if we create a new view, we can read the results
	// from the table to make the expectations
	fr, err := New(ctx, c, nil, ReadOnly)
	require.NoError(t, err)
	exp, err = fr.Get(ctx)
	require.NoError(t, err)
	assertMatches(exp)
}

// TestUndoChangeNoExist checks undoing an entry that does not exist.
func TestUndoChangeNoExist(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	err = f.UndoChange(ctx, "doesnotexist", "userTwo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not find change")
}

// TestAddChange_MasterBranch_NotifierEventsCorrect makes sure the notifier is called when changes
// are made to the master branch.
func TestAddChange_MasterBranch_NotifierEventsCorrect(t *testing.T) {
	unittest.LargeTest(t)

	notifier := expectations.NewEventDispatcherForTesting()
	var calledMutex sync.Mutex
	var calledWith []expectations.Delta
	notifier.ListenForChange(func(e expectations.Delta) {
		calledMutex.Lock()
		defer calledMutex.Unlock()
		calledWith = append(calledWith, e)
	})

	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f, err := New(ctx, c, notifier, ReadWrite)
	require.NoError(t, err)

	change1 := []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    expectations.Positive,
		},
	}
	change2 := []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaBad1Digest,
			Label:    expectations.Negative,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaGood1Digest,
			Label:    expectations.Positive,
		},
	}

	require.NoError(t, f.AddChange(ctx, change1, userOne))
	require.NoError(t, f.AddChange(ctx, change2, userTwo))

	assert.Eventually(t, func() bool {
		calledMutex.Lock()
		defer calledMutex.Unlock()
		expected := []expectations.Delta{change1[0], change2[0], change2[1]}
		return assert.ElementsMatch(t, expected, calledWith)
	}, 5*time.Second, 100*time.Millisecond)
}

// TestAddUndo_NotifierEventsCorrect tests that the notifier calls are correct during Undo
// operations on the master branch.
func TestAddUndo_NotifierEventsCorrect(t *testing.T) {
	unittest.LargeTest(t)

	notifier := expectations.NewEventDispatcherForTesting()
	var calledMutex sync.Mutex
	var calledWith []expectations.Delta
	notifier.ListenForChange(func(e expectations.Delta) {
		calledMutex.Lock()
		defer calledMutex.Unlock()
		calledWith = append(calledWith, e)
	})

	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	f, err := New(ctx, c, notifier, ReadWrite)
	require.NoError(t, err)

	change := expectations.Delta{
		Grouping: data.AlphaTest,
		Digest:   data.AlphaGood1Digest,
		Label:    expectations.Negative,
	}
	expectedUndo := expectations.Delta{
		Grouping: data.AlphaTest,
		Digest:   data.AlphaGood1Digest,
		Label:    expectations.Untriaged,
	}

	require.NoError(t, f.AddChange(ctx, []expectations.Delta{change}, userOne))

	entries, _, err := f.QueryLog(ctx, 0, 1, false)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	err = f.UndoChange(ctx, entries[0].ID, userOne)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		calledMutex.Lock()
		defer calledMutex.Unlock()
		expected := []expectations.Delta{change, expectedUndo}
		return assert.ElementsMatch(t, expected, calledWith)
	}, 5*time.Second, 100*time.Millisecond)
}

// TestCLExpectationsAddGet tests the separation of the MasterExpectations and the CLExpectations.
// It starts with a shared history, then adds some expectations to both, before requiring that
// they are properly dealt with. Specifically, the CLExpectations should be treated as a delta to
// the MasterExpectations (but doesn't actually contain MasterExpectations).
func TestCLExpectationsAddGet(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	// Notice notifier is nil; this verifies we do not send events when ChangeList
	// expectations change
	mb, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	require.NoError(t, mb.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    expectations.Negative,
		},
	}, userTwo))

	ib := mb.ForChangeList("117", "gerrit") // arbitrary cl id

	// Check that it starts out blank.
	clExp, err := ib.Get(ctx)
	require.NoError(t, err)
	require.True(t, clExp.Empty())

	// Add to the CLExpectations
	require.NoError(t, ib.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    expectations.Positive,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaGood1Digest,
			Label:    expectations.Positive,
		},
	}, userOne))

	// Add to the MasterExpectations
	require.NoError(t, mb.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaBad1Digest,
			Label:    expectations.Negative,
		},
	}, userOne))

	masterE, err := mb.Get(ctx)
	require.NoError(t, err)
	clExp, err = ib.Get(ctx)
	require.NoError(t, err)

	// Make sure the CLExpectations did not leak to the MasterExpectations
	assert.Equal(t, expectations.Negative, masterE.Classification(data.AlphaTest, data.AlphaGood1Digest))
	assert.Equal(t, expectations.Negative, masterE.Classification(data.AlphaTest, data.AlphaBad1Digest))
	assert.Equal(t, expectations.Untriaged, masterE.Classification(data.BetaTest, data.BetaGood1Digest))
	assert.Equal(t, 2, masterE.Len())

	// Make sure the CLExpectations are separate from the MasterExpectations.
	assert.Equal(t, expectations.Positive, clExp.Classification(data.AlphaTest, data.AlphaGood1Digest))
	assert.Equal(t, expectations.Untriaged, clExp.Classification(data.AlphaTest, data.AlphaBad1Digest))
	assert.Equal(t, expectations.Positive, clExp.Classification(data.BetaTest, data.BetaGood1Digest))
	assert.Equal(t, 2, clExp.Len())
}

// TestCLExpectationsQueryLog makes sure the QueryLogs interacts
// with the CLExpectations as expected. Which is to say, the two
// logs are separate.
func TestCLExpectationsQueryLog(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	mb, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	require.NoError(t, mb.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    expectations.Positive,
		},
	}, userTwo))

	ib := mb.ForChangeList("117", "gerrit") // arbitrary cl id

	require.NoError(t, ib.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaGood1Digest,
			Label:    expectations.Positive,
		},
	}, userOne))

	// Make sure the master logs are separate from the cl logs.
	// request up to 10 to make sure we would get the cl
	// change (if the filtering was wrong).
	entries, n, err := mb.QueryLog(ctx, 0, 10, true)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	now := time.Now()
	normalizeEntries(t, now, entries)
	require.Equal(t, expectations.TriageLogEntry{
		ID:          "was_random_0",
		User:        userTwo,
		TS:          now,
		ChangeCount: 1,
		Details: []expectations.Delta{
			{
				Grouping: data.AlphaTest,
				Digest:   data.AlphaGood1Digest,
				Label:    expectations.Positive,
			},
		},
	}, entries[0])

	// Make sure the cl logs are separate from the master logs.
	// Unlike when getting the expectations, the cl logs are
	// *only* those logs that affected this cl. Not, for example,
	// all the master logs with the cl logs tacked on.
	entries, n, err = ib.QueryLog(ctx, 0, 10, true)
	require.NoError(t, err)
	require.Equal(t, 1, n) // only one change on this branch

	normalizeEntries(t, now, entries)
	require.Equal(t, expectations.TriageLogEntry{
		ID:          "was_random_0",
		User:        userOne,
		TS:          now,
		ChangeCount: 1,
		Details: []expectations.Delta{
			{
				Grouping: data.BetaTest,
				Digest:   data.BetaGood1Digest,
				Label:    expectations.Positive,
			},
		},
	}, entries[0])
}

// TestExpectationEntryID tests edge cases for malformed names
func TestExpectationEntryID(t *testing.T) {
	unittest.SmallTest(t)
	// Based on real data
	e := expectationEntry{
		Grouping: "downsample/images/mandrill_512.png",
		Digest:   "36bc7da524f2869c97f0a0f1d7042110",
	}
	require.Equal(t, "downsample-images-mandrill_512.png|36bc7da524f2869c97f0a0f1d7042110",
		e.ID())
}

func TestUpdateLastUsed_NoEntriesToUpdate_NothingChanges(t *testing.T) {
	unittest.LargeTest(t)
	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	exp, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)
	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)

	newUsedTime := time.Date(2020, time.February, 5, 0, 0, 0, 0, time.UTC)
	err = exp.UpdateLastUsed(ctx, nil, newUsedTime)
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

	exp, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)
	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)

	newUsedTime := time.Date(2020, time.February, 5, 0, 0, 0, 0, time.UTC)
	err = exp.UpdateLastUsed(ctx, []expectations.ID{
		{
			Grouping: entryOneGrouping,
			Digest:   entryOneDigest,
		},
	}, newUsedTime)
	require.NoError(t, err)

	actualEntryOne := getRawEntry(ctx, t, c, entryOneGrouping, entryOneDigest)
	require.NotNil(t, actualEntryOne)
	assert.Equal(t, entryOne.Label, actualEntryOne.Label)          // no change
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

	exp, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)
	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)

	newUsedTime := time.Date(2020, time.February, 5, 0, 0, 0, 0, time.UTC)
	err = exp.UpdateLastUsed(ctx, []expectations.ID{
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
	assert.Equal(t, entryOne.Label, actualEntryOne.Label)          // no change
	assert.True(t, entryOne.Updated.Equal(actualEntryOne.Updated)) // no change
	assert.True(t, newUsedTime.Equal(actualEntryOne.LastUsed))     // change expected

	actualEntryTwo := getRawEntry(ctx, t, c, entryTwoGrouping, entryTwoDigest)
	require.NotNil(t, actualEntryTwo)
	assert.Equal(t, entryTwo.Label, actualEntryTwo.Label)          // no change
	assert.True(t, entryTwo.Updated.Equal(actualEntryTwo.Updated)) // no change
	assert.True(t, newUsedTime.Equal(actualEntryTwo.LastUsed))     // change expected

	actualEntryThree := getRawEntry(ctx, t, c, entryThreeGrouping, entryThreeDigest)
	assertUnchanged(t, &entryThree, actualEntryThree)
}

func TestMarkUnusedEntriesForGC_Untriaged_Error(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()
	exp := Store{}
	_, err := exp.MarkUnusedEntriesForGC(ctx, expectations.Untriaged, time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be untriaged")
}

// TestMarkUnusedEntriesForGC_EntriesRecentlyUsed_NoEntriesMarked_Success checks that we don't mark
// entries for garbage collection (untriage them) that are have been used more recently than the
// cutoff time.
func TestMarkUnusedEntriesForGC_EntriesRecentlyUsed_NoEntriesMarked_Success(t *testing.T) {
	unittest.LargeTest(t)

	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	exp, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)
	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)

	// The time passed here is before all entries
	n, err := exp.MarkUnusedEntriesForGC(ctx, expectations.Positive, entryOne.LastUsed.Add(-time.Second))
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	// The time passed here is before all negative entries. It is after entryOne (which is positive)
	// so we still expect nothing to have changed.
	n, err = exp.MarkUnusedEntriesForGC(ctx, expectations.Negative, entryTwo.LastUsed.Add(-time.Second))
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
// is marked for garbage collection (i.e. untriaged).
func TestMarkUnusedEntriesForGC_OnePositiveEntryMarked_Success(t *testing.T) {
	unittest.LargeTest(t)

	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	exp, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)
	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)

	// The time here is selected to be after both entryOne and entryTwo were last used, to make
	// sure that we are respecting the label.
	cutoff := entryThree.LastUsed.Add(-time.Minute)
	assert.True(t, cutoff.After(entryOne.LastUsed))
	assert.True(t, cutoff.After(entryTwo.LastUsed))
	n, err := exp.MarkUnusedEntriesForGC(ctx, expectations.Positive, cutoff)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	// Make sure all entries are still there, just entryOne is Untriaged
	actualEntryOne := getRawEntry(ctx, t, c, entryOneGrouping, entryOneDigest)
	require.NotNil(t, actualEntryOne)
	assert.Equal(t, expectations.Untriaged, actualEntryOne.Label)
	actualEntryTwo := getRawEntry(ctx, t, c, entryTwoGrouping, entryTwoDigest)
	assertUnchanged(t, &entryTwo, actualEntryTwo)
	actualEntryThree := getRawEntry(ctx, t, c, entryThreeGrouping, entryThreeDigest)
	assertUnchanged(t, &entryThree, actualEntryThree)
}

// TestMarkUnusedEntriesForGC_OneNegativeEntryMarked_Success tests where the middle entry (the
// only negative) entry is marked for garbage collection (i.e. untriaged).
func TestMarkUnusedEntriesForGC_OneNegativeEntryMarked_Success(t *testing.T) {
	unittest.LargeTest(t)

	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	exp, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)
	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)
	// This time is picked to be after all entries
	cutoff := entryThree.LastUsed.Add(time.Minute)
	assert.True(t, cutoff.After(entryOne.LastUsed))
	assert.True(t, cutoff.After(entryTwo.LastUsed))
	n, err := exp.MarkUnusedEntriesForGC(ctx, expectations.Negative, cutoff)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	// Make sure all entries are still there, just entryTwo is Untriaged
	actualEntryOne := getRawEntry(ctx, t, c, entryOneGrouping, entryOneDigest)
	assertUnchanged(t, &entryOne, actualEntryOne)
	actualEntryTwo := getRawEntry(ctx, t, c, entryTwoGrouping, entryTwoDigest)
	require.NotNil(t, actualEntryTwo)
	assert.Equal(t, expectations.Untriaged, actualEntryTwo.Label)
	actualEntryThree := getRawEntry(ctx, t, c, entryThreeGrouping, entryThreeDigest)
	assertUnchanged(t, &entryThree, actualEntryThree)
}

// TestMarkUnusedEntriesForGC_MultiplePositiveEntriesAffected tests where we mark both positive
// entries as untriaged (not matching the negative one in the middle).
func TestMarkUnusedEntriesForGC_MultiplePositiveEntriesAffected(t *testing.T) {
	unittest.LargeTest(t)

	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	exp, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)
	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)

	// This time is picked to be after all entries
	cutoff := entryThree.LastUsed.Add(time.Minute)
	assert.True(t, cutoff.After(entryOne.LastUsed))
	assert.True(t, cutoff.After(entryTwo.LastUsed))
	n, err := exp.MarkUnusedEntriesForGC(ctx, expectations.Positive, cutoff)
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	// Make sure all entries are still there, entryOne and entryThree are Untriaged
	actualEntryOne := getRawEntry(ctx, t, c, entryOneGrouping, entryOneDigest)
	require.NotNil(t, actualEntryOne)
	assert.Equal(t, expectations.Untriaged, actualEntryOne.Label)
	actualEntryTwo := getRawEntry(ctx, t, c, entryTwoGrouping, entryTwoDigest)
	assertUnchanged(t, &entryTwo, actualEntryTwo)
	actualEntryThree := getRawEntry(ctx, t, c, entryThreeGrouping, entryThreeDigest)
	require.NotNil(t, actualEntryThree)
	assert.Equal(t, expectations.Untriaged, actualEntryThree.Label)
}

// TestMarkUnusedEntriesForGC_LastUsedLongAgo_UpdatedRecently_NoEntriesMarked_Success tests where
// we don't untriage digests that have not been seen in a while, but were modified recently.
func TestMarkUnusedEntriesForGC_LastUsedLongAgo_UpdatedRecently_NoEntriesMarked_Success(t *testing.T) {
	unittest.LargeTest(t)

	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	exp, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)
	// This is well after entryThree.LastUsed
	moreRecently := time.Date(2020, time.March, 1, 1, 1, 1, 0, time.UTC)
	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, moreRecently)
	assert.True(t, moreRecently.After(entryThree.LastUsed))

	// This time is picked to be after all entries
	cutoff := entryThree.LastUsed.Add(time.Minute)
	assert.True(t, cutoff.After(entryOne.LastUsed))
	assert.True(t, cutoff.After(entryTwo.LastUsed))
	n, err := exp.MarkUnusedEntriesForGC(ctx, expectations.Positive, cutoff)
	require.NoError(t, err)
	// None should be affected because the modified stamp is too new.
	assert.Equal(t, 0, n)

	// Make sure all entries are still there, just entryOne is Untriaged
	actualEntryOne := getRawEntry(ctx, t, c, entryOneGrouping, entryOneDigest)
	assertUnchanged(t, &entryOne, actualEntryOne)
	actualEntryTwo := getRawEntry(ctx, t, c, entryTwoGrouping, entryTwoDigest)
	assertUnchanged(t, &entryTwo, actualEntryTwo)
	actualEntryThree := getRawEntry(ctx, t, c, entryThreeGrouping, entryThreeDigest)
	assertUnchanged(t, &entryThree, actualEntryThree)
}

// TestGarbageCollect_MultipleEntriesDeleted tests case where we untriage two entries and then
// delete those untriaged entries so they are not in Firestore anymore.
func TestGarbageCollect_MultipleEntriesDeleted(t *testing.T) {
	unittest.LargeTest(t)

	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	exp, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)
	_, _, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)

	n, err := exp.MarkUnusedEntriesForGC(ctx, expectations.Positive, entryThree.LastUsed.Add(time.Minute))
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	n, err = exp.GarbageCollect(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	// Make sure entryOne and entryTwo are not there (e.g. now nil)
	actualEntryOne := getRawEntry(ctx, t, c, entryOneGrouping, entryOneDigest)
	require.Nil(t, actualEntryOne)
	actualEntryTwo := getRawEntry(ctx, t, c, entryTwoGrouping, entryTwoDigest)
	require.NotNil(t, actualEntryTwo)
	assert.Equal(t, expectations.Negative, actualEntryTwo.Label)
	actualEntryThree := getRawEntry(ctx, t, c, entryThreeGrouping, entryThreeDigest)
	require.Nil(t, actualEntryThree)
}

// TestGarbageCollect_NoEntriesDeleted tests case where there are no entries to clean up.
// Of note, trying to call .Commit() on an empty firestore.Batch() results in an error in
// production (and a hang in the test using the emulator). This test makes sure we avoid that.
func TestGarbageCollect_NoEntriesDeleted(t *testing.T) {
	unittest.LargeTest(t)

	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	exp, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)
	entryOne, entryTwo, entryThree := populateFirestore(ctx, t, c, updatedLongAgo)

	n, err := exp.GarbageCollect(ctx)
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

	exp, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	clExp := exp.ForChangeList("foo", "bar")
	err = clExp.AddChange(ctx, []expectations.Delta{
		{
			Grouping: entryOneGrouping,
			Digest:   entryOneDigest,
			Label:    expectations.Positive,
		},
	}, "test@example.com")
	require.NoError(t, err)

	cutoff := time.Now().Add(time.Hour)
	n, err := exp.MarkUnusedEntriesForGC(ctx, expectations.Positive, cutoff)
	require.NoError(t, err)
	assert.Equal(t, 0, n)

	// Make sure the original CL entry is there, still positive.
	actualEntryOne := getRawEntry(ctx, t, c, entryOneGrouping, entryOneDigest)
	require.NotNil(t, actualEntryOne)
	assert.Equal(t, expectations.Positive, actualEntryOne.Label)
	assert.NotEqual(t, masterBranch, actualEntryOne.CRSAndCLID)
}

// TestMarkUnusedEntriesForGC_LegacyEntriesRemoved_Success tests that legacy entries (created w/o
// a LastUsed field set) get cleaned up if their Updated field is old enough. This is tolerable
// because if the garbage collection process has been running for a while, then the legacy
// expectation was at least not seen in the most recent tile, so it is unlikely to be fresh anyway.
// This test can go away in Fall 2020 when the MarkUnusedEntriesForGC is updated to search first by
// LastUsed.
func TestMarkUnusedEntriesForGC_LegacyEntriesRemoved_Success(t *testing.T) {
	unittest.LargeTest(t)

	c, ctx, cleanup := makeTestFirestoreClient(t)
	defer cleanup()

	exp, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)
	lastYear := time.Date(2019, time.February, 27, 0, 0, 0, 0, time.UTC)
	today := time.Date(2020, time.February, 27, 0, 0, 0, 0, time.UTC)
	entryOne := expectationEntry{
		Grouping:   entryOneGrouping,
		Digest:     entryOneDigest,
		Label:      expectations.Positive,
		Updated:    lastYear,
		CRSAndCLID: masterBranch,
		LastUsed:   time.Time{},
	}
	entryTwo := expectationEntry{
		Grouping:   entryTwoGrouping,
		Digest:     entryTwoDigest,
		Label:      expectations.Positive,
		Updated:    today,
		CRSAndCLID: masterBranch,
		LastUsed:   time.Time{},
	}
	createRawEntry(ctx, t, c, entryOne)
	createRawEntry(ctx, t, c, entryTwo)

	cutoff := time.Date(2020, time.February, 26, 0, 0, 0, 0, time.UTC)
	assert.True(t, cutoff.After(lastYear))
	assert.True(t, cutoff.Before(today))
	n, err := exp.MarkUnusedEntriesForGC(ctx, expectations.Positive, cutoff)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	// Make sure both entries are there
	actualEntryOne := getRawEntry(ctx, t, c, entryOneGrouping, entryOneDigest)
	require.NotNil(t, actualEntryOne)
	assert.Equal(t, expectations.Untriaged, actualEntryOne.Label)
	actualEntryTwo := getRawEntry(ctx, t, c, entryTwoGrouping, entryTwoDigest)
	assertUnchanged(t, &entryTwo, actualEntryTwo)
}

// An arbitrary date a long time before the times used in populateFirestore.
var updatedLongAgo = time.Date(2019, time.January, 1, 1, 1, 1, 0, time.UTC)

const (
	entryOneGrouping   = data.AlphaTest
	entryOneDigest     = data.AlphaGood1Digest
	entryTwoGrouping   = data.AlphaTest
	entryTwoDigest     = data.AlphaBad1Digest
	entryThreeGrouping = data.BetaTest
	entryThreeDigest   = data.BetaGood1Digest
)

// populateFirestore creates three manual entries in firestore, corresponding to the
// three_devices data. It uses three different times for LastUsed and the same (provided) time
// for modified for each of the entries. Then, it returns the created entries for use in asserts.
func populateFirestore(ctx context.Context, t *testing.T, c *firestore.Client, modified time.Time) (expectationEntry, expectationEntry, expectationEntry) {
	// For convenience, these times are spaced a few days apart at midnight in ascending order.
	var entryOneUsed = time.Date(2020, time.January, 28, 0, 0, 0, 0, time.UTC)
	var entryTwoUsed = time.Date(2020, time.January, 30, 0, 0, 0, 0, time.UTC)
	var entryThreeUsed = time.Date(2020, time.February, 2, 0, 0, 0, 0, time.UTC)

	entryOne := expectationEntry{
		Grouping:   entryOneGrouping,
		Digest:     entryOneDigest,
		Label:      expectations.Positive,
		Updated:    modified,
		CRSAndCLID: masterBranch,
		LastUsed:   entryOneUsed,
	}
	entryTwo := expectationEntry{
		Grouping:   entryTwoGrouping,
		Digest:     entryTwoDigest,
		Label:      expectations.Negative,
		Updated:    modified,
		CRSAndCLID: masterBranch,
		LastUsed:   entryTwoUsed,
	}
	entryThree := expectationEntry{
		Grouping:   entryThreeGrouping,
		Digest:     entryThreeDigest,
		Label:      expectations.Positive,
		Updated:    modified,
		CRSAndCLID: masterBranch,
		LastUsed:   entryThreeUsed,
	}
	createRawEntry(ctx, t, c, entryOne)
	createRawEntry(ctx, t, c, entryTwo)
	createRawEntry(ctx, t, c, entryThree)
	return entryOne, entryTwo, entryThree
}

// createRawEntry creates the bare expectationEntry in firestore.
func createRawEntry(ctx context.Context, t *testing.T, c *firestore.Client, entry expectationEntry) {
	doc := c.Collection(expectationsCollection).Doc(entry.ID())
	_, err := doc.Create(ctx, entry)
	require.NoError(t, err)
}

// getRawEntry returns the bare expectationEntry from firestore for the given name/digest.
func getRawEntry(ctx context.Context, t *testing.T, c *firestore.Client, name types.TestName, digest types.Digest) *expectationEntry {
	entry := expectationEntry{Grouping: name, Digest: digest}
	doc := c.Collection(expectationsCollection).Doc(entry.ID())
	ds, err := doc.Get(ctx)
	if err != nil {
		// This error could indicated not found, which may be expected by some tests.
		return nil
	}
	err = ds.DataTo(&entry)
	require.NoError(t, err)
	return &entry
}

func assertUnchanged(t *testing.T, expected, actual *expectationEntry) {
	require.NotNil(t, expected)
	require.NotNil(t, actual)
	assert.Equal(t, expected.Label, actual.Label)
	assert.True(t, expected.Updated.Equal(actual.Updated))
	assert.True(t, expected.LastUsed.Equal(actual.LastUsed))
}

// fillWith4Entries fills a given Store with 4 triaged records of a few digests.
func fillWith4Entries(t *testing.T, f *Store) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, f.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    expectations.Negative,
		},
	}, userOne))
	require.NoError(t, f.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    expectations.Positive, // overwrites previous value
		},
	}, userTwo))
	require.NoError(t, f.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaGood1Digest,
			Label:    expectations.Positive,
		},
	}, userOne))
	require.NoError(t, f.AddChange(ctx, []expectations.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaBad1Digest,
			Label:    expectations.Negative,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaUntriaged1Digest,
			Label:    expectations.Untriaged,
		},
	}, userTwo))
}

// Some parts of the entries (timestamp and id) are non-deterministic
// Make sure they are valid, then replace them with deterministic values
// for an easier comparison.
func normalizeEntries(t *testing.T, now time.Time, entries []expectations.TriageLogEntry) {
	for i, te := range entries {
		require.NotEqual(t, "", te.ID)
		te.ID = "was_random_" + strconv.Itoa(i)
		ts := te.TS
		require.False(t, ts.IsZero())
		require.True(t, now.After(ts))
		te.TS = now
		entries[i] = te
	}
}

// makeBigExpectations makes n tests named from start to end that each have 32 digests.
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
func makeTestFirestoreClient(t *testing.T) (*firestore.Client, context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	c, cleanup := firestore.NewClientForTesting(ctx, t)
	return c, ctx, func() {
		cancel()
		cleanup()
	}
}

const (
	userOne = "userOne@example.com"
	userTwo = "userTwo@example.com"
)
