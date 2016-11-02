package recent_rolls

import (
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/testutils"
)

// TestRecentRolls verifies that we correctly track mode history.
func TestRecentRolls(t *testing.T) {
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)

	// Create the RecentRolls.
	tmpDir, err := ioutil.TempDir("", "test_autoroll_recent_")
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	}()
	r, err := NewRecentRolls(path.Join(tmpDir, "test.db"))
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, r.Close())
	}()

	// Use this function for checking expectations.
	check := func(current, last *autoroll.AutoRollIssue, history []*autoroll.AutoRollIssue) {
		testutils.AssertDeepEqual(t, current, r.CurrentRoll())
		testutils.AssertDeepEqual(t, last, r.LastRoll())
		testutils.AssertDeepEqual(t, history, r.GetRecentRolls())
	}

	// Add one issue.
	now := time.Now().UTC()
	ari1 := &autoroll.AutoRollIssue{
		Closed:      false,
		Committed:   false,
		CommitQueue: true,
		Created:     now,
		Issue:       1010101,
		Modified:    now,
		Patchsets:   []int64{1},
		Result:      autoroll.ROLL_RESULT_IN_PROGRESS,
		Subject:     "FAKE DEPS ROLL 1",
		TryResults:  []*autoroll.TryResult{},
	}
	expect := []*autoroll.AutoRollIssue{ari1}
	assert.NoError(t, r.Add(ari1))
	check(ari1, nil, expect)

	// Try to add it again. Ensure that we throw an error.
	assert.Error(t, r.Add(ari1))
	check(ari1, nil, expect)

	// Close the issue as successful. Ensure that it's now the last roll
	// instead of the current roll.
	ari1.Closed = true
	ari1.Committed = true
	ari1.CommitQueue = false
	ari1.Result = autoroll.ROLL_RESULT_SUCCESS
	assert.NoError(t, r.Update(ari1))
	check(nil, ari1, expect)

	// Add another issue. Ensure that it's the current roll with the
	// previously-added roll as the last roll.
	now = time.Now().UTC()
	ari2 := &autoroll.AutoRollIssue{
		Closed:      false,
		Committed:   false,
		CommitQueue: true,
		Created:     now,
		Issue:       1010102,
		Modified:    now,
		Patchsets:   []int64{1},
		Result:      autoroll.ROLL_RESULT_IN_PROGRESS,
		Subject:     "FAKE DEPS ROLL 2",
		TryResults:  []*autoroll.TryResult{},
	}
	assert.NoError(t, r.Add(ari2))
	expect = []*autoroll.AutoRollIssue{ari2, ari1}
	check(ari2, ari1, expect)

	// Try to add another active issue. Ensure that the RecentRolls complains.
	now = time.Now().UTC()
	bad1 := &autoroll.AutoRollIssue{
		Closed:      false,
		Committed:   false,
		CommitQueue: true,
		Created:     now,
		Issue:       1010103,
		Modified:    now,
		Patchsets:   []int64{1},
		Result:      autoroll.ROLL_RESULT_IN_PROGRESS,
		Subject:     "FAKE DEPS ROLL 3",
		TryResults:  []*autoroll.TryResult{},
	}
	assert.Error(t, r.Add(bad1))

	// Close the issue as failed. Ensure that it's now the last roll
	// instead of the current roll.
	ari2.Closed = true
	ari2.Committed = false
	ari2.CommitQueue = false
	ari2.Result = autoroll.ROLL_RESULT_FAILURE
	assert.NoError(t, r.Update(ari2))
	check(nil, ari2, expect)

	// Try to add a bogus issue.
	now = time.Now().UTC()
	bad2 := &autoroll.AutoRollIssue{
		Closed:      false,
		Committed:   true,
		CommitQueue: true,
		Created:     now,
		Issue:       1010104,
		Modified:    now,
		Patchsets:   []int64{1},
		Result:      autoroll.ROLL_RESULT_FAILURE,
		Subject:     "FAKE DEPS ROLL 4",
		TryResults:  []*autoroll.TryResult{},
	}
	assert.Error(t, r.Add(bad2))

	// Add one more roll. Ensure that it's the current roll.
	now = time.Now().UTC()
	ari3 := &autoroll.AutoRollIssue{
		Closed:      false,
		Committed:   false,
		CommitQueue: true,
		Created:     now,
		Issue:       1010105,
		Modified:    now,
		Patchsets:   []int64{1},
		Result:      autoroll.ROLL_RESULT_IN_PROGRESS,
		Subject:     "FAKE DEPS ROLL 5",
		TryResults:  []*autoroll.TryResult{},
	}
	assert.NoError(t, r.Add(ari3))
	expect = []*autoroll.AutoRollIssue{ari3, ari2, ari1}
	check(ari3, ari2, expect)
}
