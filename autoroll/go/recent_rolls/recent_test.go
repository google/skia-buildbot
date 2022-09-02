package recent_rolls

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
)

// TestRecentRolls verifies that we correctly track mode history.
func TestRecentRolls(t *testing.T) {
	ctx := context.Background()
	testutil.InitDatastore(t, ds.KIND_AUTOROLL_ROLL)

	// Create the RecentRolls.
	r, err := NewRecentRolls(ctx, "test-roller")
	require.NoError(t, err)

	// Use this function for checking expectations.
	check := func(current, last *autoroll.AutoRollIssue, history []*autoroll.AutoRollIssue) {
		assertdeep.Equal(t, current, r.CurrentRoll())
		assertdeep.Equal(t, last, r.LastRoll())
		assertdeep.Equal(t, history, r.GetRecentRolls())
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
