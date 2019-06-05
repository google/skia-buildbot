package fs_expstore

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/eventbus/mocks"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/expstorage"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
)

// TODO(kjlubick): These tests are marked as manual because the
// Firestore Emulator is not yet on the bots, due to some more complicated
// setup (e.g. chmod)

// TestGetExpectations writes some changes and then reads back the
// aggregated results.
func TestGetExpectations(t *testing.T) {
	unittest.ManualTest(t)
	unittest.RequiresFirestoreEmulator(t)

	c := getTestFirestoreInstance(t)

	f := New(c, nil, ReadWrite)

	// Brand new instance should have no expectations
	e, err := f.Get()
	assert.NoError(t, err)
	assert.Equal(t, types.Expectations{}, e)

	ctx := context.Background()
	err = f.AddChange(ctx, types.Expectations{
		data.AlphaTest: {
			data.AlphaUntriaged1Digest: types.POSITIVE,
			data.AlphaGood1Digest:      types.POSITIVE,
		},
	}, userOne)
	assert.NoError(t, err)

	err = f.AddChange(ctx, types.Expectations{
		data.AlphaTest: {
			data.AlphaBad1Digest:       types.NEGATIVE,
			data.AlphaUntriaged1Digest: types.UNTRIAGED, // overwrites previous
		},
		data.BetaTest: {
			data.BetaGood1Digest: types.POSITIVE,
		},
	}, userTwo)
	assert.NoError(t, err)

	expected := types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest:      types.POSITIVE,
			data.AlphaBad1Digest:       types.NEGATIVE,
			data.AlphaUntriaged1Digest: types.UNTRIAGED,
		},
		data.BetaTest: {
			data.BetaGood1Digest: types.POSITIVE,
		},
	}

	e, err = f.Get()
	assert.NoError(t, err)
	assert.Equal(t, expected, e)

	// Make sure that if we create a new view, we can read the results
	// from the table to make the expectations
	fr := New(c, nil, ReadOnly)
	e, err = fr.Get()
	assert.NoError(t, err)
	assert.Equal(t, expected, e)
}

// TestGetExpectationsRace writes a bunch of data from many go routines
// in an effort to catch any race conditions in the caching layer.
func TestGetExpectationsRace(t *testing.T) {
	unittest.ManualTest(t)
	unittest.RequiresFirestoreEmulator(t)

	c := getTestFirestoreInstance(t)

	f := New(c, nil, ReadWrite)

	type entry struct {
		Grouping types.TestName
		Digest   types.Digest
		Label    types.Label
	}

	entries := []entry{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaUntriaged1Digest,
			Label:    types.UNTRIAGED,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaBad1Digest,
			Label:    types.NEGATIVE,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    types.POSITIVE,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaGood1Digest,
			Label:    types.POSITIVE,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaUntriaged1Digest,
			Label:    types.UNTRIAGED,
		},
	}

	ctx := context.Background()
	wg := sync.WaitGroup{}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e := entries[i%len(entries)]
			err := f.AddChange(ctx, types.Expectations{
				e.Grouping: {
					e.Digest: e.Label,
				},
			}, userOne)
			assert.NoError(t, err)
		}(i)

		// Make sure we can read and write w/o races
		if i%5 == 0 {
			_, err := f.Get()
			assert.NoError(t, err)
		}
	}

	wg.Wait()

	e, err := f.Get()
	assert.NoError(t, err)
	assert.Equal(t, types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest:      types.POSITIVE,
			data.AlphaBad1Digest:       types.NEGATIVE,
			data.AlphaUntriaged1Digest: types.UNTRIAGED,
		},
		data.BetaTest: {
			data.BetaGood1Digest:      types.POSITIVE,
			data.BetaUntriaged1Digest: types.UNTRIAGED,
		},
	}, e)
}

// TestGetExpectationsBig writes 32^2=1024 entries
// to test the batch writing.
func TestGetExpectationsBig(t *testing.T) {
	unittest.ManualTest(t)
	unittest.RequiresFirestoreEmulator(t)

	c := getTestFirestoreInstance(t)

	f := New(c, nil, ReadWrite)

	// Write the expectations in two, non-overlapping blocks.
	exp1 := makeBigExpectations(0, 16)
	exp2 := makeBigExpectations(16, 32)

	expected := exp1.DeepCopy()
	expected.MergeExpectations(exp2)

	ctx := context.Background()
	wg := sync.WaitGroup{}

	// Write them concurrently to test for races.
	wg.Add(2)
	go func() {
		defer wg.Done()
		err := f.AddChange(ctx, exp1, userOne)
		assert.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		err := f.AddChange(ctx, exp2, userTwo)
		assert.NoError(t, err)
	}()
	wg.Wait()

	e, err := f.Get()
	assert.NoError(t, err)
	assert.Equal(t, expected, e)

	// Make sure that if we create a new view, we can read the results
	// from the table to make the expectations
	fr := New(c, nil, ReadOnly)
	e, err = fr.Get()
	assert.NoError(t, err)
	assert.Equal(t, expected, e)
}

// TestReadOnly ensures a read-only instance fails to write data.
func TestReadOnly(t *testing.T) {
	unittest.SmallTest(t)

	f := New(nil, nil, ReadOnly)

	err := f.AddChange(context.Background(), types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest: types.POSITIVE,
		},
	}, userOne)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read-only")
}

// TestQueryLog tests that we can query logs at a given place
func TestQueryLog(t *testing.T) {
	unittest.ManualTest(t)
	unittest.RequiresFirestoreEmulator(t)

	c := getTestFirestoreInstance(t)
	f := New(c, nil, ReadWrite)

	fillWith4Entries(t, f)

	ctx := context.Background()
	entries, n, err := f.QueryLog(ctx, 0, 100, false)
	assert.NoError(t, err)
	assert.Equal(t, 4, n) // 4 operations

	now := time.Now()
	normalizeEntries(t, now, entries)
	assert.Equal(t, []expstorage.TriageLogEntry{
		{
			ID:          "was_random_0",
			Name:        userTwo,
			TS:          now.Unix(),
			ChangeCount: 2,
			Details:     nil,
		},
		{
			ID:          "was_random_1",
			Name:        userOne,
			TS:          now.Unix(),
			ChangeCount: 1,
			Details:     nil,
		},
		{
			ID:          "was_random_2",
			Name:        userTwo,
			TS:          now.Unix(),
			ChangeCount: 1,
			Details:     nil,
		},
		{
			ID:          "was_random_3",
			Name:        userOne,
			TS:          now.Unix(),
			ChangeCount: 1,
			Details:     nil,
		},
	}, entries)

	entries, n, err = f.QueryLog(ctx, 1, 2, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, n)
	normalizeEntries(t, now, entries)
	assert.Equal(t, []expstorage.TriageLogEntry{
		{
			ID:          "was_random_0",
			Name:        userOne,
			TS:          now.Unix(),
			ChangeCount: 1,
			Details:     nil,
		},
		{
			ID:          "was_random_1",
			Name:        userTwo,
			TS:          now.Unix(),
			ChangeCount: 1,
			Details:     nil,
		},
	}, entries)

	// Make sure we can handle an invalid offset
	entries, n, err = f.QueryLog(ctx, 500, 100, false)
	assert.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Nil(t, entries)
}

// TestQueryLogDetails checks that the details are filled in when requested.
func TestQueryLogDetails(t *testing.T) {
	unittest.ManualTest(t)
	unittest.RequiresFirestoreEmulator(t)

	c := getTestFirestoreInstance(t)
	f := New(c, nil, ReadWrite)

	fillWith4Entries(t, f)

	ctx := context.Background()
	entries, n, err := f.QueryLog(ctx, 0, 100, true)
	assert.NoError(t, err)
	assert.Equal(t, 4, n) // 4 operations

	assert.Equal(t, []expstorage.TriageDetail{
		{
			TestName: data.AlphaTest,
			Digest:   data.AlphaBad1Digest,
			Label:    types.NEGATIVE.String(),
		},
		{
			TestName: data.BetaTest,
			Digest:   data.BetaUntriaged1Digest,
			Label:    types.UNTRIAGED.String(),
		},
	}, entries[0].Details)
	assert.Equal(t, []expstorage.TriageDetail{
		{
			TestName: data.BetaTest,
			Digest:   data.BetaGood1Digest,
			Label:    types.POSITIVE.String(),
		},
	}, entries[1].Details)
	assert.Equal(t, []expstorage.TriageDetail{
		{
			TestName: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    types.POSITIVE.String(),
		},
	}, entries[2].Details)
	assert.Equal(t, []expstorage.TriageDetail{
		{
			TestName: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    types.NEGATIVE.String(),
		},
	}, entries[3].Details)
}

// TestUndoChangeSunnyDay checks undoing entries that exist.
func TestUndoChangeSunnyDay(t *testing.T) {
	unittest.ManualTest(t)
	unittest.RequiresFirestoreEmulator(t)

	c := getTestFirestoreInstance(t)
	f := New(c, nil, ReadWrite)

	fillWith4Entries(t, f)

	ctx := context.Background()
	entries, n, err := f.QueryLog(ctx, 0, 4, false)
	assert.NoError(t, err)
	assert.Equal(t, 4, n)

	exp, err := f.UndoChange(ctx, entries[0].ID, userOne)
	assert.NoError(t, err)
	assert.Equal(t, types.Expectations{
		data.AlphaTest: {
			data.AlphaBad1Digest: types.UNTRIAGED,
		},
		data.BetaTest: {
			data.BetaUntriaged1Digest: types.UNTRIAGED,
		},
	}, exp)

	exp, err = f.UndoChange(ctx, entries[2].ID, userOne)
	assert.NoError(t, err)
	assert.Equal(t, types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest: types.NEGATIVE,
		},
	}, exp)

	expected := types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest: types.NEGATIVE,
			data.AlphaBad1Digest:  types.UNTRIAGED,
		},
		data.BetaTest: {
			data.BetaGood1Digest:      types.POSITIVE,
			data.BetaUntriaged1Digest: types.UNTRIAGED,
		},
	}

	// Check that the undone items were applied
	exp, err = f.Get()
	assert.NoError(t, err)
	assert.Equal(t, expected, exp)

	// Make sure that if we create a new view, we can read the results
	// from the table to make the expectations
	fr := New(c, nil, ReadOnly)
	exp, err = fr.Get()
	assert.NoError(t, err)
	assert.Equal(t, expected, exp)
}

// TestUndoChangeNoExist checks undoing an entry that does not exist.
func TestUndoChangeNoExist(t *testing.T) {
	unittest.ManualTest(t)
	unittest.RequiresFirestoreEmulator(t)

	c := getTestFirestoreInstance(t)
	f := New(c, nil, ReadWrite)

	_, err := f.UndoChange(context.Background(), "doesnotexist", "userTwo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not find change")
}

// TestEventBusAddMaster makes sure proper eventbus signals are sent
// when changes are made to the master branch.
func TestEventBusAddMaster(t *testing.T) {
	unittest.ManualTest(t)
	unittest.RequiresFirestoreEmulator(t)

	meb := &mocks.EventBus{}
	defer meb.AssertExpectations(t)

	c := getTestFirestoreInstance(t)
	f := New(c, meb, ReadWrite)

	change1 := types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest: types.POSITIVE,
		},
	}
	change2 := types.Expectations{
		data.AlphaTest: {
			data.AlphaBad1Digest: types.NEGATIVE,
		},
		data.BetaTest: {
			data.BetaGood1Digest: types.POSITIVE,
		},
	}

	meb.On("Publish", expstorage.EV_EXPSTORAGE_CHANGED, &expstorage.EventExpectationChange{
		TestChanges: change1,
		IssueID:     expstorage.MasterBranch,
	}, /*global=*/ true).Once()
	meb.On("Publish", expstorage.EV_EXPSTORAGE_CHANGED, &expstorage.EventExpectationChange{
		TestChanges: change2,
		IssueID:     expstorage.MasterBranch,
	}, /*global=*/ true).Once()

	ctx := context.Background()
	assert.NoError(t, f.AddChange(ctx, change1, userOne))
	assert.NoError(t, f.AddChange(ctx, change2, userTwo))
}

// TestEventBusAddIssue makes sure proper eventbus signals are sent
// when changes are made to an IssueExpectations.
func TestEventBusAddIssue(t *testing.T) {
	unittest.ManualTest(t)
	unittest.RequiresFirestoreEmulator(t)

	meb := &mocks.EventBus{}
	defer meb.AssertExpectations(t)

	c := getTestFirestoreInstance(t)
	e := New(c, meb, ReadWrite)
	issue := int64(117)
	f := e.ForIssue(issue) // arbitrary issue

	change1 := types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest: types.POSITIVE,
		},
	}
	change2 := types.Expectations{
		data.AlphaTest: {
			data.AlphaBad1Digest: types.NEGATIVE,
		},
		data.BetaTest: {
			data.BetaGood1Digest: types.POSITIVE,
		},
	}

	meb.On("Publish", expstorage.EV_TRYJOB_EXP_CHANGED, &expstorage.EventExpectationChange{
		TestChanges: change1,
		IssueID:     issue,
	}, /*global=*/ false).Once()
	meb.On("Publish", expstorage.EV_TRYJOB_EXP_CHANGED, &expstorage.EventExpectationChange{
		TestChanges: change2,
		IssueID:     issue,
	}, /*global=*/ false).Once()

	ctx := context.Background()
	assert.NoError(t, f.AddChange(ctx, change1, userOne))
	assert.NoError(t, f.AddChange(ctx, change2, userTwo))
}

// TestEventBusUndo tests that eventbus signals are properly sent during Undo.
func TestEventBusUndo(t *testing.T) {
	unittest.ManualTest(t)
	unittest.RequiresFirestoreEmulator(t)

	meb := &mocks.EventBus{}
	defer meb.AssertExpectations(t)

	c := getTestFirestoreInstance(t)
	f := New(c, meb, ReadWrite)

	change := types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest: types.NEGATIVE,
		},
	}
	expectedUndo := types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest: types.UNTRIAGED,
		},
	}

	meb.On("Publish", expstorage.EV_EXPSTORAGE_CHANGED, &expstorage.EventExpectationChange{
		TestChanges: change,
		IssueID:     expstorage.MasterBranch,
	}, /*global=*/ true).Once()
	meb.On("Publish", expstorage.EV_EXPSTORAGE_CHANGED, &expstorage.EventExpectationChange{
		TestChanges: expectedUndo,
		IssueID:     expstorage.MasterBranch,
	}, /*global=*/ true).Once()

	ctx := context.Background()
	assert.NoError(t, f.AddChange(ctx, change, userOne))

	entries, n, err := f.QueryLog(ctx, 0, 1, false)
	assert.NoError(t, err)
	assert.Equal(t, 1, n)

	exp, err := f.UndoChange(ctx, entries[0].ID, userOne)
	assert.NoError(t, err)
	assert.Equal(t, expectedUndo, exp)
}

// TestIssueExpectationsAddGet tests the separation of the MasterExpectations
// and the IssueExpectations. It starts with a shared history, then
// adds some expectations to both, before asserting that they are properly dealt
// with. Specifically, the IssueExpectations should be applied as a delta to
// the MasterExpectations.
func TestIssueExpectationsAddGet(t *testing.T) {
	unittest.ManualTest(t)
	unittest.RequiresFirestoreEmulator(t)

	c := getTestFirestoreInstance(t)
	mb := New(c, nil, ReadWrite)

	ctx := context.Background()
	assert.NoError(t, mb.AddChange(ctx, types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest: types.NEGATIVE,
		},
	}, userTwo))

	ib := mb.ForIssue(117) // arbitrary issue id

	masterE, err := mb.Get()
	assert.NoError(t, err)
	issueE, err := ib.Get()
	assert.NoError(t, err)
	assert.Equal(t, masterE, issueE)

	// Add to the IssueExpectations
	assert.NoError(t, ib.AddChange(ctx, types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest: types.POSITIVE, // overwrites previous
		},
		data.BetaTest: {
			data.BetaGood1Digest: types.POSITIVE,
		},
	}, userOne))

	// Add to the MasterExpectations
	assert.NoError(t, mb.AddChange(ctx, types.Expectations{
		data.AlphaTest: {
			data.AlphaBad1Digest: types.NEGATIVE,
		},
	}, userOne))

	masterE, err = mb.Get()
	assert.NoError(t, err)
	issueE, err = ib.Get()
	assert.NoError(t, err)

	// Make sure the IssueExpectations did not leak to the MasterExpectations
	assert.Equal(t, types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest: types.NEGATIVE,
			data.AlphaBad1Digest:  types.NEGATIVE,
		},
	}, masterE)

	// Make sure the IssueExpectations are applied on top of the updated
	// MasterExpectations.
	assert.Equal(t, types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest: types.POSITIVE,
			data.AlphaBad1Digest:  types.NEGATIVE,
		},
		data.BetaTest: {
			data.BetaGood1Digest: types.POSITIVE,
		},
	}, issueE)
}

// TestIssueExpectationsQueryLog makes sure the QueryLogs interacts
// with the IssueExpectations as expected. Which is to say, the two
// logs are separate.
func TestIssueExpectationsQueryLog(t *testing.T) {
	unittest.ManualTest(t)
	unittest.RequiresFirestoreEmulator(t)

	c := getTestFirestoreInstance(t)
	mb := New(c, nil, ReadWrite)

	ctx := context.Background()
	assert.NoError(t, mb.AddChange(ctx, types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest: types.POSITIVE,
		},
	}, userTwo))

	ib := mb.ForIssue(117) // arbitrary issue id

	assert.NoError(t, ib.AddChange(ctx, types.Expectations{
		data.BetaTest: {
			data.BetaGood1Digest: types.POSITIVE,
		},
	}, userOne))

	// Make sure the master logs are separate from the issue logs.
	// request up to 10 to make sure we would get the issue
	// change (if the filtering was wrong).
	entries, n, err := mb.QueryLog(ctx, 0, 10, true)
	assert.NoError(t, err)
	assert.Equal(t, 1, n)

	now := time.Now()
	normalizeEntries(t, now, entries)
	assert.Equal(t, expstorage.TriageLogEntry{
		ID:          "was_random_0",
		Name:        userTwo,
		TS:          now.Unix(),
		ChangeCount: 1,
		Details: []expstorage.TriageDetail{
			{
				TestName: data.AlphaTest,
				Digest:   data.AlphaGood1Digest,
				Label:    types.POSITIVE.String(),
			},
		},
	}, entries[0])

	// Make sure the issue logs are separate from the master logs.
	// Unlike when getting the expectations, the issue logs are
	// *only* those logs that affected this issue. Not, for example,
	// all the master logs with the issue logs tacked on.
	entries, n, err = ib.QueryLog(ctx, 0, 10, true)
	assert.NoError(t, err)
	assert.Equal(t, 1, n) // only one change on this branch

	normalizeEntries(t, now, entries)
	assert.Equal(t, expstorage.TriageLogEntry{
		ID:          "was_random_0",
		Name:        userOne,
		TS:          now.Unix(),
		ChangeCount: 1,
		Details: []expstorage.TriageDetail{
			{
				TestName: data.BetaTest,
				Digest:   data.BetaGood1Digest,
				Label:    types.POSITIVE.String(),
			},
		},
	}, entries[0])
}

// fillWith4Entries fills a given Store with 4 triaged records of a few digests.
func fillWith4Entries(t *testing.T, f *Store) {
	ctx := context.Background()
	assert.NoError(t, f.AddChange(ctx, types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest: types.NEGATIVE,
		},
	}, userOne))
	assert.NoError(t, f.AddChange(ctx, types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest: types.POSITIVE, // overwrites previous value
		},
	}, userTwo))
	assert.NoError(t, f.AddChange(ctx, types.Expectations{
		data.BetaTest: {
			data.BetaGood1Digest: types.POSITIVE,
		},
	}, userOne))
	assert.NoError(t, f.AddChange(ctx, types.Expectations{
		data.AlphaTest: {
			data.AlphaBad1Digest: types.NEGATIVE,
		},
		data.BetaTest: {
			data.BetaUntriaged1Digest: types.UNTRIAGED,
		},
	}, userTwo))
}

// Some parts of the entries (timestamp and id) are non-deterministic
// Make sure they are valid, then replace them with deterministic values
// for an easier comparison.
func normalizeEntries(t *testing.T, now time.Time, entries []expstorage.TriageLogEntry) {
	for i, te := range entries {
		assert.NotEqual(t, "", te.ID)
		te.ID = "was_random_" + strconv.Itoa(i)
		ts := time.Unix(te.TS, 0)
		assert.False(t, ts.IsZero())
		assert.True(t, now.After(ts))
		te.TS = now.Unix()
		entries[i] = te
	}
}

// Creates an empty firestore instance. The emulator keeps the tables in ram, but
// by appending a random nonce, we can be assured the collection we get is empty.
func getTestFirestoreInstance(t *testing.T) *firestore.Client {
	randInstance := uuid.New().String()
	c, err := firestore.NewClient(context.Background(), "should-use-emulator", "gold-test", ExpectationStoreCollection+randInstance, nil)
	assert.NoError(t, err)
	return c
}

// makeBigExpectations makes n tests named from start to end that each have 32 digests.
func makeBigExpectations(start, end int) types.Expectations {
	e := types.Expectations{}
	for i := start; i < end; i++ {
		for j := 0; j < 32; j++ {
			e.AddDigest(types.TestName(fmt.Sprintf("test-%03d", i)),
				types.Digest(fmt.Sprintf("digest-%03d", j)), types.POSITIVE)
		}
	}
	return e
}

const (
	userOne = "userOne@example.com"
	userTwo = "userTwo@example.com"
)
