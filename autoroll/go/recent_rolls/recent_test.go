package recent_rolls

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
)

// TestRecentRolls verifies that we correctly track roll history.
func TestRecentRolls(t *testing.T) {
	ctx := context.Background()
	testutil.InitDatastore(t, ds.KIND_AUTOROLL_ROLL)

	// Create the RecentRolls.
	r, err := NewRecentRolls(ctx, NewDatastoreRollsDB(ctx), "test-roller")
	require.NoError(t, err)

	// Use this function for checking expectations.
	check := func(current, last *autoroll.AutoRollIssue, history []*autoroll.AutoRollIssue) {
		assertdeep.Equal(t, history, r.GetRecentRolls())
		assertdeep.Equal(t, current, r.CurrentRoll())
		assertdeep.Equal(t, last, r.LastRoll())
	}

	// Add one issue.
	now := time.Now().UTC()
	ari1 := &autoroll.AutoRollIssue{
		Closed:     false,
		Committed:  false,
		Created:    now,
		IsDryRun:   false,
		Issue:      1010101,
		Modified:   now,
		Patchsets:  []int64{1},
		Result:     autoroll.ROLL_RESULT_IN_PROGRESS,
		Subject:    "FAKE DEPS ROLL 1",
		TryResults: []*autoroll.TryResult(nil),
	}
	expect := []*autoroll.AutoRollIssue{ari1}
	require.NoError(t, r.Add(ctx, ari1))
	check(ari1, nil, expect)

	// Try to add it again. We should log an error but not fail.
	require.NoError(t, r.Add(ctx, ari1))
	check(ari1, nil, expect)

	// Close the issue as successful. Ensure that it's now the last roll
	// instead of the current roll.
	ari1.Closed = true
	ari1.Committed = true
	ari1.CqFinished = true
	ari1.CqSuccess = true
	ari1.Result = autoroll.ROLL_RESULT_SUCCESS
	require.NoError(t, r.Update(ctx, ari1))
	check(nil, ari1, expect)

	// Add another issue. Ensure that it's the current roll with the
	// previously-added roll as the last roll.
	now = time.Now().UTC()
	ari2 := &autoroll.AutoRollIssue{
		Closed:     false,
		Committed:  false,
		Created:    now,
		Issue:      1010102,
		Modified:   now,
		Patchsets:  []int64{1},
		Result:     autoroll.ROLL_RESULT_IN_PROGRESS,
		Subject:    "FAKE DEPS ROLL 2",
		TryResults: []*autoroll.TryResult(nil),
	}
	require.NoError(t, r.Add(ctx, ari2))
	expect = []*autoroll.AutoRollIssue{ari2, ari1}
	check(ari2, ari1, expect)

	// Try to add another active issue. We should log an error but not fail.
	now = time.Now().UTC()
	ari3 := &autoroll.AutoRollIssue{
		Closed:     false,
		Committed:  false,
		Created:    now,
		Issue:      1010103,
		Modified:   now,
		Patchsets:  []int64{1},
		Result:     autoroll.ROLL_RESULT_IN_PROGRESS,
		Subject:    "FAKE DEPS ROLL 3",
		TryResults: []*autoroll.TryResult(nil),
	}
	require.NoError(t, r.Add(ctx, ari3))
	expect = []*autoroll.AutoRollIssue{ari3, ari2, ari1}
	check(ari3, ari2, expect)

	// Close the issue as failed. Ensure that it's now the last roll
	// instead of the current roll.
	ari2.Closed = true
	ari2.Committed = false
	ari2.CqFinished = true
	ari2.CqSuccess = false
	ari2.Result = autoroll.ROLL_RESULT_FAILURE
	require.NoError(t, r.Update(ctx, ari2))
	check(ari3, ari2, expect)

	// Same with ari3.
	ari3.Closed = true
	ari3.Committed = false
	ari3.CqFinished = true
	ari3.CqSuccess = false
	ari3.Result = autoroll.ROLL_RESULT_FAILURE
	require.NoError(t, r.Update(ctx, ari3))
	check(nil, ari3, expect)

	// Try to add a bogus issue.
	now = time.Now().UTC()
	bad2 := &autoroll.AutoRollIssue{
		Closed:     false,
		Committed:  true,
		Created:    now,
		Issue:      1010104,
		Modified:   now,
		Patchsets:  []int64{1},
		Result:     autoroll.ROLL_RESULT_FAILURE,
		Subject:    "FAKE DEPS ROLL 4",
		TryResults: []*autoroll.TryResult(nil),
	}
	require.Error(t, r.Add(ctx, bad2))

	// Add one more roll. Ensure that it's the current roll.
	now = time.Now().UTC()
	ari4 := &autoroll.AutoRollIssue{
		Closed:     false,
		Committed:  false,
		Created:    now,
		Issue:      1010105,
		Modified:   now,
		Patchsets:  []int64{1},
		Result:     autoroll.ROLL_RESULT_IN_PROGRESS,
		Subject:    "FAKE DEPS ROLL 5",
		TryResults: []*autoroll.TryResult(nil),
	}
	require.NoError(t, r.Add(ctx, ari4))
	expect = []*autoroll.AutoRollIssue{ari4, ari3, ari2, ari1}
	check(ari4, ari3, expect)
}

func TestRecentRolls_NumFailuresAndLastSucessfulRollTime(t *testing.T) {
	ctx := context.Background()
	testutil.InitDatastore(t, ds.KIND_AUTOROLL_ROLL)

	db := NewDatastoreRollsDB(ctx)
	r, err := NewRecentRolls(ctx, NewDatastoreRollsDB(ctx), "test-roller")
	require.NoError(t, err)

	issue := int64(0)
	const startTs int64 = 1678218051
	now := time.Unix(startTs, 0) // Arbitrary starting point.
	createAndInsertRoll := func(success bool) {
		issue += 1
		now = now.Add(time.Second)
		result := autoroll.ROLL_RESULT_FAILURE
		if success {
			result = autoroll.ROLL_RESULT_SUCCESS
		}
		require.NoError(t, db.Put(ctx, r.roller, &autoroll.AutoRollIssue{
			Closed:     true,
			Committed:  success,
			Created:    now,
			Issue:      issue,
			Modified:   now,
			Patchsets:  []int64{1},
			Result:     result,
			Subject:    "fake roll",
			TryResults: []*autoroll.TryResult(nil),
		}))
	}
	createAndInsertRoll(true)
	for i := 0; i < 2*RecentRollsLength; i++ {
		createAndInsertRoll(false)
	}
	require.NoError(t, r.refreshRecentRolls(ctx))

	require.Equal(t, 2*RecentRollsLength, r.NumFailedRolls())
	require.Equal(t, time.Unix(startTs+int64(1), 0), r.LastSuccessfulRollTime())
}

func TestDatastoreRollsDB_GetRolls(t *testing.T) {
	ctx := context.Background()
	testutil.InitDatastore(t, ds.KIND_AUTOROLL_ROLL)
	db := NewDatastoreRollsDB(ctx)
	roller := uuid.New().String()

	issue := int64(0)
	now := time.Unix(1678218051, 0) // Arbitrary starting point.
	makeRoll := func() *autoroll.AutoRollIssue {
		issue += 1
		now = now.Add(time.Second)
		return &autoroll.AutoRollIssue{
			Closed:     true,
			Committed:  true,
			Created:    now,
			Issue:      issue,
			Modified:   now,
			Patchsets:  []int64{1},
			Result:     autoroll.ROLL_RESULT_SUCCESS,
			Subject:    "fake roll",
			TryResults: []*autoroll.TryResult(nil),
		}
	}

	// Ensure no error when no rolls exist.
	rolls, cursor, err := db.GetRolls(ctx, roller, "")
	require.NoError(t, err)
	require.Equal(t, "", cursor)
	require.Equal(t, 0, len(rolls))

	// Insert a single roll.
	r1 := makeRoll()
	require.NoError(t, db.Put(ctx, roller, r1))
	rolls, cursor, err = db.GetRolls(ctx, roller, "")
	require.NoError(t, err)
	//require.Equal(t, "", cursor)
	require.Equal(t, 1, len(rolls))
	require.Equal(t, rolls[0], r1)

	// Insert enough rolls to necessitate multiple pages.
	for i := 0; i < 3*loadRollsPageSize; i++ {
		require.NoError(t, db.Put(ctx, roller, makeRoll()))
	}
	allRolls := []*autoroll.AutoRollIssue{}
	// Batch 1.
	rolls, cursor, err = db.GetRolls(ctx, roller, "")
	require.NoError(t, err)
	require.NotEqual(t, "", cursor)
	require.Equal(t, loadRollsPageSize, len(rolls))
	allRolls = append(allRolls, rolls...)
	// Batch 2.
	rolls, cursor, err = db.GetRolls(ctx, roller, cursor)
	require.NoError(t, err)
	require.NotEqual(t, "", cursor)
	require.Equal(t, loadRollsPageSize, len(rolls))
	allRolls = append(allRolls, rolls...)
	// Batch 3.
	rolls, cursor, err = db.GetRolls(ctx, roller, cursor)
	require.NoError(t, err)
	require.NotEqual(t, "", cursor)
	require.Equal(t, loadRollsPageSize, len(rolls))
	allRolls = append(allRolls, rolls...)
	// Batch 4. Only one roll left to retrieve. Cursor should be empty.
	rolls, cursor, err = db.GetRolls(ctx, roller, cursor)
	require.NoError(t, err)
	require.Equal(t, "", cursor)
	require.Equal(t, 1, len(rolls))
	allRolls = append(allRolls, rolls...)
	// Ensure that we found all of the rolls we expected.
	require.Equal(t, 76, len(allRolls))
}
