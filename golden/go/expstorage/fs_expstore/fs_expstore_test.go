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

	f := New(c, MasterBranch, ReadWrite)

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
	fr := New(c, MasterBranch, ReadOnly)
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

	f := New(c, MasterBranch, ReadWrite)

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

	f := New(c, MasterBranch, ReadWrite)

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
	fr := New(c, MasterBranch, ReadOnly)
	e, err = fr.Get()
	assert.NoError(t, err)
	assert.Equal(t, expected, e)
}

// TestReadOnly ensures a read-only instance fails to write data.
func TestReadOnly(t *testing.T) {
	unittest.SmallTest(t)

	f := New(nil, MasterBranch, ReadOnly)

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
	f := New(c, MasterBranch, ReadWrite)
	ctx := context.Background()

	fillWith4Entries(t, f)

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
	f := New(c, MasterBranch, ReadWrite)
	ctx := context.Background()

	fillWith4Entries(t, f)

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
	f := New(c, MasterBranch, ReadWrite)
	ctx := context.Background()

	fillWith4Entries(t, f)
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
	fr := New(c, MasterBranch, ReadOnly)
	exp, err = fr.Get()
	assert.NoError(t, err)
	assert.Equal(t, expected, exp)
}

// TestUndoChangeNoExist checks undoing an entry that does not exist.
func TestUndoChangeNoExist(t *testing.T) {
	unittest.ManualTest(t)
	unittest.RequiresFirestoreEmulator(t)

	c := getTestFirestoreInstance(t)
	f := New(c, MasterBranch, ReadWrite)
	ctx := context.Background()

	_, err := f.UndoChange(ctx, "doesnotexist", "userTwo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not find change")
}

// TODO(kjlubick): implement tests for branch expectations.
// func TestBranchExpectationsGet(t *testing.T) {
// 	unittest.ManualTest(t)
// 	unittest.RequiresFirestoreEmulator(t)

// 	c := getTestFirestoreInstance(t)
// 	m := New(c, MasterBranch, ReadWrite)
// 	b := New(c, 117, ReadWrite) // arbitrary branch id
// 	ctx := context.Background()

// }

// fillWith4Entries fills a given Store with 4 triaged records of a few digests.
func fillWith4Entries(t *testing.T, f *Store) {
	assert.NoError(t, f.AddChange(context.Background(), types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest: types.NEGATIVE,
		},
	}, userOne))
	assert.NoError(t, f.AddChange(context.Background(), types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest: types.POSITIVE, // overwrites previous value
		},
	}, userTwo))
	assert.NoError(t, f.AddChange(context.Background(), types.Expectations{
		data.BetaTest: {
			data.BetaGood1Digest: types.POSITIVE,
		},
	}, userOne))
	assert.NoError(t, f.AddChange(context.Background(), types.Expectations{
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
