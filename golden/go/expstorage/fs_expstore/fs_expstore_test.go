package fs_expstore

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
	"go.skia.org/infra/go/eventbus/mocks"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/expstorage"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/types/expectations"
)

// TestGetExpectations writes some changes and then reads back the
// aggregated results.
func TestGetExpectations(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	// Brand new instance should have no expectations
	e, err := f.Get(ctx)
	require.NoError(t, err)
	require.True(t, e.Empty())

	err = f.AddChange(ctx, []expstorage.Delta{
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

	err = f.AddChange(ctx, []expstorage.Delta{
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
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	err = f.AddChange(ctx, []expstorage.Delta{
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

	err = f.AddChange(ctx, []expstorage.Delta{
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
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
			err := f.AddChange(ctx, []expstorage.Delta{
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
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f, err := New(ctx, c, nil, ReadOnly)
	require.NoError(t, err)

	err = f.AddChange(ctx, []expstorage.Delta{
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
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	fillWith4Entries(t, f)

	entries, n, err := f.QueryLog(ctx, 0, 100, false)
	require.NoError(t, err)
	require.Equal(t, 4, n) // 4 operations
	require.Len(t, entries, 4)

	now := time.Now()
	normalizeEntries(t, now, entries)
	require.Equal(t, []expstorage.TriageLogEntry{
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
	require.Equal(t, expstorage.CountMany, n)
	require.Len(t, entries, 2)

	normalizeEntries(t, now, entries)
	require.Equal(t, []expstorage.TriageLogEntry{
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
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	fillWith4Entries(t, f)

	entries, n, err := f.QueryLog(ctx, 0, 100, true)
	require.NoError(t, err)
	require.Equal(t, 4, n) // 4 operations
	require.Len(t, entries, 4)

	// These should be sorted, starting with the most recent
	require.Equal(t, []expstorage.Delta{
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
	require.Equal(t, []expstorage.Delta{
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaGood1Digest,
			Label:    expectations.Positive,
		},
	}, entries[1].Details)
	require.Equal(t, []expstorage.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    expectations.Positive,
		},
	}, entries[2].Details)
	require.Equal(t, []expstorage.Delta{
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
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	// 800 should spread us across 3 "shards", which are ~250 expectations.
	const numExp = 800
	delta := make([]expstorage.Delta, 0, numExp)
	for i := uint64(0); i < numExp; i++ {
		n := types.TestName(fmt.Sprintf("test_%03d", i))
		// An MD5 hash is 128 bits, which is 32 chars
		d := types.Digest(fmt.Sprintf("%032d", i))
		delta = append(delta, expstorage.Delta{
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
	require.Equal(t, expstorage.Delta{
		Grouping: "test_000",
		Digest:   "00000000000000000000000000000000",
		Label:    expectations.Positive,
	}, entry.Details[0])
	require.Equal(t, expstorage.Delta{
		Grouping: "test_200",
		Digest:   "00000000000000000000000000000200",
		Label:    expectations.Positive,
	}, entry.Details[200])
	require.Equal(t, expstorage.Delta{
		Grouping: "test_400",
		Digest:   "00000000000000000000000000000400",
		Label:    expectations.Positive,
	}, entry.Details[400])
	require.Equal(t, expstorage.Delta{
		Grouping: "test_600",
		Digest:   "00000000000000000000000000000600",
		Label:    expectations.Positive,
	}, entry.Details[600])
	require.Equal(t, expstorage.Delta{
		Grouping: "test_799",
		Digest:   "00000000000000000000000000000799",
		Label:    expectations.Positive,
	}, entry.Details[799])
}

// TestUndoChangeSunnyDay checks undoing entries that exist.
func TestUndoChangeSunnyDay(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	fillWith4Entries(t, f)

	entries, n, err := f.QueryLog(ctx, 0, 4, false)
	require.NoError(t, err)
	require.Equal(t, expstorage.CountMany, n)
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
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	require.NoError(t, f.AddChange(ctx, []expstorage.Delta{
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

	require.NoError(t, f.AddChange(ctx, []expstorage.Delta{
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
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	err = f.UndoChange(ctx, "doesnotexist", "userTwo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not find change")
}

// TestEventBusAddMaster makes sure proper eventbus signals are sent
// when changes are made to the master branch.
func TestEventBusAddMaster(t *testing.T) {
	unittest.LargeTest(t)

	meb := &mocks.EventBus{}
	defer meb.AssertExpectations(t)

	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f, err := New(ctx, c, meb, ReadWrite)
	require.NoError(t, err)

	change1 := []expstorage.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    expectations.Positive,
		},
	}
	change2 := []expstorage.Delta{
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

	meb.On("Publish", expstorage.ExpectationsChangedTopic, &expstorage.EventExpectationChange{
		ExpectationDelta: change1[0],
		CRSAndCLID:       "",
	}, /*global=*/ true).Once()
	// This was two entries, which are split up into two firestore records. Thus, we should
	// see two events, one for each of them.
	meb.On("Publish", expstorage.ExpectationsChangedTopic, &expstorage.EventExpectationChange{
		ExpectationDelta: change2[0],
		CRSAndCLID:       "",
	}, /*global=*/ true).Once()
	meb.On("Publish", expstorage.ExpectationsChangedTopic, &expstorage.EventExpectationChange{
		ExpectationDelta: change2[1],
		CRSAndCLID:       "",
	}, /*global=*/ true).Once()

	require.NoError(t, f.AddChange(ctx, change1, userOne))
	require.NoError(t, f.AddChange(ctx, change2, userTwo))
}

// TestEventBusUndo tests that eventbus signals are properly sent during Undo.
func TestEventBusUndo(t *testing.T) {
	unittest.LargeTest(t)

	meb := &mocks.EventBus{}
	defer meb.AssertExpectations(t)

	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f, err := New(ctx, c, meb, ReadWrite)
	require.NoError(t, err)

	change := expstorage.Delta{
		Grouping: data.AlphaTest,
		Digest:   data.AlphaGood1Digest,
		Label:    expectations.Negative,
	}
	expectedUndo := expstorage.Delta{
		Grouping: data.AlphaTest,
		Digest:   data.AlphaGood1Digest,
		Label:    expectations.Untriaged,
	}

	meb.On("Publish", expstorage.ExpectationsChangedTopic, &expstorage.EventExpectationChange{
		ExpectationDelta: change,
		CRSAndCLID:       "",
	}, /*global=*/ true).Once()
	meb.On("Publish", expstorage.ExpectationsChangedTopic, &expstorage.EventExpectationChange{
		ExpectationDelta: expectedUndo,
		CRSAndCLID:       "",
	}, /*global=*/ true).Once()

	require.NoError(t, f.AddChange(ctx, []expstorage.Delta{change}, userOne))

	entries, _, err := f.QueryLog(ctx, 0, 1, false)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	err = f.UndoChange(ctx, entries[0].ID, userOne)
	require.NoError(t, err)
}

// TestCLExpectationsAddGet tests the separation of the MasterExpectations
// and the CLExpectations. It starts with a shared history, then
// adds some expectations to both, before requiring that they are properly dealt
// with. Specifically, the CLExpectations should be treated as a delta to
// the MasterExpectations (but doesn't actually contain MasterExpectations).
func TestCLExpectationsAddGet(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mb, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	require.NoError(t, mb.AddChange(ctx, []expstorage.Delta{
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
	require.NoError(t, ib.AddChange(ctx, []expstorage.Delta{
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
	require.NoError(t, mb.AddChange(ctx, []expstorage.Delta{
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
	assert.Equal(t, masterE.Classification(data.AlphaTest, data.AlphaGood1Digest), expectations.Negative)
	assert.Equal(t, masterE.Classification(data.AlphaTest, data.AlphaBad1Digest), expectations.Negative)
	assert.Equal(t, masterE.Classification(data.BetaTest, data.BetaGood1Digest), expectations.Untriaged)
	assert.Equal(t, 2, masterE.Len())

	// Make sure the CLExpectations are separate from the MasterExpectations.
	assert.Equal(t, clExp.Classification(data.AlphaTest, data.AlphaGood1Digest), expectations.Positive)
	assert.Equal(t, clExp.Classification(data.AlphaTest, data.AlphaBad1Digest), expectations.Untriaged)
	assert.Equal(t, clExp.Classification(data.BetaTest, data.BetaGood1Digest), expectations.Positive)
	assert.Equal(t, 2, clExp.Len())
}

// TestCLExpectationsQueryLog makes sure the QueryLogs interacts
// with the CLExpectations as expected. Which is to say, the two
// logs are separate.
func TestCLExpectationsQueryLog(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mb, err := New(ctx, c, nil, ReadWrite)
	require.NoError(t, err)

	require.NoError(t, mb.AddChange(ctx, []expstorage.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    expectations.Positive,
		},
	}, userTwo))

	ib := mb.ForChangeList("117", "gerrit") // arbitrary cl id

	require.NoError(t, ib.AddChange(ctx, []expstorage.Delta{
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
	require.Equal(t, expstorage.TriageLogEntry{
		ID:          "was_random_0",
		User:        userTwo,
		TS:          now,
		ChangeCount: 1,
		Details: []expstorage.Delta{
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
	require.Equal(t, expstorage.TriageLogEntry{
		ID:          "was_random_0",
		User:        userOne,
		TS:          now,
		ChangeCount: 1,
		Details: []expstorage.Delta{
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

// fillWith4Entries fills a given Store with 4 triaged records of a few digests.
func fillWith4Entries(t *testing.T, f *Store) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, f.AddChange(ctx, []expstorage.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    expectations.Negative,
		},
	}, userOne))
	require.NoError(t, f.AddChange(ctx, []expstorage.Delta{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    expectations.Positive, // overwrites previous value
		},
	}, userTwo))
	require.NoError(t, f.AddChange(ctx, []expstorage.Delta{
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaGood1Digest,
			Label:    expectations.Positive,
		},
	}, userOne))
	require.NoError(t, f.AddChange(ctx, []expstorage.Delta{
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
func normalizeEntries(t *testing.T, now time.Time, entries []expstorage.TriageLogEntry) {
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
func makeBigExpectations(start, end int) (*expectations.Expectations, []expstorage.Delta) {
	var e expectations.Expectations
	var delta []expstorage.Delta
	for i := start; i < end; i++ {
		for j := 0; j < 32; j++ {
			tn := types.TestName(fmt.Sprintf("test-%03d", i))
			d := types.Digest(fmt.Sprintf("digest-%03d", j))
			e.Set(tn, d, expectations.Positive)
			delta = append(delta, expstorage.Delta{
				Grouping: tn,
				Digest:   d,
				Label:    expectations.Positive,
			})

		}
	}
	return &e, delta
}

const (
	userOne = "userOne@example.com"
	userTwo = "userTwo@example.com"
)
