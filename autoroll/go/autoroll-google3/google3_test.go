package main

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/recent_rolls"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/jsonutils"
	"go.skia.org/infra/go/mockhttpclient"
)

func setup(t *testing.T) (context.Context, *AutoRoller, *git_testutils.GitBuilder, *gitiles_testutils.MockRepo, func()) {
	ctx := context.Background()
	testutil.InitDatastore(t, ds.KIND_AUTOROLL_ROLL, ds.KIND_AUTOROLL_STATUS)
	gb := git_testutils.GitInit(t, ctx)
	urlmock := mockhttpclient.NewURLMock()
	mockChild := gitiles_testutils.NewMockRepo(t, gb.RepoUrl(), git.GitDir(gb.Dir()), urlmock)
	a, err := NewAutoRoller(ctx, &config.Config{
		ChildDisplayName: "test-child",
		RepoManager: &config.Config_Google3RepoManager{
			Google3RepoManager: &config.Google3RepoManagerConfig{
				ChildBranch: git.MainBranch,
				ChildRepo:   gb.RepoUrl(),
			},
		},
		ParentDisplayName: "test-parent",
		RollerName:        "test-roller",
	}, urlmock.Client(), nil)
	require.NoError(t, err)
	return ctx, a, gb, mockChild, func() {
		gb.Cleanup()
	}
}

func makeIssue(num int64, commit string) *autoroll.AutoRollIssue {
	now := time.Now().UTC()
	return &autoroll.AutoRollIssue{
		Closed:      false,
		Committed:   false,
		Created:     now,
		Issue:       num,
		Modified:    now,
		Patchsets:   nil,
		Result:      autoroll.ROLL_RESULT_IN_PROGRESS,
		RollingFrom: "prevrev",
		RollingTo:   commit,
		Subject:     fmt.Sprintf("%d", num),
		TryResults: []*autoroll.TryResult{
			{
				Builder:  "Test Summary",
				Category: autoroll.TRYBOT_CATEGORY_CQ,
				Created:  now,
				Result:   "",
				Status:   autoroll.TRYBOT_STATUS_STARTED,
				Url:      "http://example.com/",
			},
		},
	}
}

func closeIssue(issue *autoroll.AutoRollIssue, result string) {
	issue.Closed = true
	issue.CqFinished = true
	issue.Modified = time.Now().UTC()
	issue.Result = result
	issue.TryResults[0].Status = autoroll.TRYBOT_STATUS_COMPLETED
	issue.TryResults[0].Result = autoroll.TRYBOT_RESULT_FAILURE
	if result == autoroll.ROLL_RESULT_SUCCESS {
		issue.Committed = true
		issue.CqSuccess = true
		issue.TryResults[0].Result = autoroll.TRYBOT_RESULT_SUCCESS
	}
}

func TestStatus(t *testing.T) {
	t.Skip("skbug.com/12357")

	ctx, a, gb, mockChild, cleanup := setup(t)
	defer cleanup()

	commits := []string{gb.CommitGen(ctx, "a.txt")}

	issue1 := makeIssue(1, commits[0])
	require.NoError(t, a.AddOrUpdateIssue(ctx, issue1, http.MethodPost))
	closeIssue(issue1, autoroll.ROLL_RESULT_SUCCESS)
	require.NoError(t, a.AddOrUpdateIssue(ctx, issue1, http.MethodPut))

	// Ensure that repo update occurs when updating status.
	commits = append(commits, gb.CommitGen(ctx, "a.txt"))

	mockChild.MockGetCommit(ctx, git.MainBranch)
	mockChild.MockLog(ctx, git.LogFromTo(commits[0], commits[1]))
	require.NoError(t, a.UpdateStatus(ctx, "", true))
	status := a.status.Get()
	require.Equal(t, 0, status.NumFailedRolls)
	require.Equal(t, 1, status.NumNotRolledCommits)
	require.Equal(t, issue1.RollingTo, status.LastRollRev)
	require.Nil(t, status.CurrentRoll)
	assertdeep.Equal(t, issue1, status.LastRoll)
	assertdeep.Equal(t, []*autoroll.AutoRollIssue{issue1}, status.Recent)

	// Ensure that repo update occurs when adding an issue.
	commits = append(commits, gb.CommitGen(ctx, "a.txt"))

	issue2 := makeIssue(2, commits[2])
	require.NoError(t, a.AddOrUpdateIssue(ctx, issue2, http.MethodPost))
	closeIssue(issue2, autoroll.ROLL_RESULT_FAILURE)
	require.NoError(t, a.AddOrUpdateIssue(ctx, issue2, http.MethodPut))

	issue3 := makeIssue(3, commits[2])
	require.NoError(t, a.AddOrUpdateIssue(ctx, issue3, http.MethodPost))
	closeIssue(issue3, autoroll.ROLL_RESULT_FAILURE)
	require.NoError(t, a.AddOrUpdateIssue(ctx, issue3, http.MethodPut))

	issue4 := makeIssue(4, commits[2])
	require.NoError(t, a.AddOrUpdateIssue(ctx, issue4, http.MethodPost))

	recent := []*autoroll.AutoRollIssue{issue4, issue3, issue2, issue1}
	mockChild.MockGetCommit(ctx, git.MainBranch)
	mockChild.MockLog(ctx, git.LogFromTo(commits[0], commits[2]))
	require.NoError(t, a.UpdateStatus(ctx, "error message", false))
	status = a.status.Get()
	require.Equal(t, 2, status.NumFailedRolls)
	require.Equal(t, 2, status.NumNotRolledCommits)
	require.Equal(t, issue1.RollingTo, status.LastRollRev)
	require.Equal(t, "error message", status.Error)
	assertdeep.Equal(t, issue4, status.CurrentRoll)
	assertdeep.Equal(t, issue3, status.LastRoll)
	assertdeep.Equal(t, recent, status.Recent)

	// Test preserving error.
	mockChild.MockGetCommit(ctx, git.MainBranch)
	mockChild.MockLog(ctx, git.LogFromTo(commits[0], commits[2]))
	require.NoError(t, a.UpdateStatus(ctx, "", true))
	status = a.status.Get()
	require.Equal(t, "error message", status.Error)

	// Overflow recent_rolls.RECENT_ROLLS_LENGTH.
	closeIssue(issue4, autoroll.ROLL_RESULT_FAILURE)
	require.NoError(t, a.AddOrUpdateIssue(ctx, issue4, http.MethodPut))
	recent = []*autoroll.AutoRollIssue{issue4, issue3}
	// Rolls 3 and 4 failed, so we need 5 thru recent_rolls.RECENT_ROLLS_LENGTH + 3 to also fail for
	// overflow.
	for i := int64(5); i < recent_rolls.RECENT_ROLLS_LENGTH+3; i++ {
		issueI := makeIssue(i, commits[2])
		require.NoError(t, a.AddOrUpdateIssue(ctx, issueI, http.MethodPost))
		closeIssue(issueI, autoroll.ROLL_RESULT_FAILURE)
		require.NoError(t, a.AddOrUpdateIssue(ctx, issueI, http.MethodPut))
		recent = append([]*autoroll.AutoRollIssue{issueI}, recent...)
	}
	mockChild.MockGetCommit(ctx, git.MainBranch)
	mockChild.MockLog(ctx, git.LogFromTo(commits[0], commits[2]))
	require.NoError(t, a.UpdateStatus(ctx, "error message", false))
	status = a.status.Get()
	require.Equal(t, recent_rolls.RECENT_ROLLS_LENGTH+1, status.NumFailedRolls)
	require.Equal(t, 2, status.NumNotRolledCommits)
	require.Equal(t, issue1.RollingTo, status.LastRollRev)
	require.Equal(t, "error message", status.Error)
	require.Nil(t, status.CurrentRoll)
	assertdeep.Equal(t, recent[0], status.LastRoll)
	assertdeep.Equal(t, recent, status.Recent)
}

func TestAddOrUpdateIssue(t *testing.T) {
	t.Skip("skbug.com/12357")

	ctx, a, gb, mockChild, cleanup := setup(t)
	defer cleanup()

	commits := []string{gb.CommitGen(ctx, "a.txt"), gb.CommitGen(ctx, "a.txt"), gb.CommitGen(ctx, "a.txt")}

	issue1 := makeIssue(1, commits[0])
	require.NoError(t, a.AddOrUpdateIssue(ctx, issue1, http.MethodPost))
	closeIssue(issue1, autoroll.ROLL_RESULT_SUCCESS)
	require.NoError(t, a.AddOrUpdateIssue(ctx, issue1, http.MethodPut))

	// Test adding an issue that is already closed.
	issue2 := makeIssue(2, commits[1])
	closeIssue(issue2, autoroll.ROLL_RESULT_SUCCESS)
	require.NoError(t, a.AddOrUpdateIssue(ctx, issue2, http.MethodPut))
	mockChild.MockGetCommit(ctx, git.MainBranch)
	mockChild.MockLog(ctx, git.LogFromTo(commits[1], commits[2]))
	require.NoError(t, a.UpdateStatus(ctx, "", true))
	assertdeep.Equal(t, []*autoroll.AutoRollIssue{issue2, issue1}, a.status.Get().Recent)

	// Test adding a two issues without closing the first one.
	issue3 := makeIssue(3, commits[2])
	require.NoError(t, a.AddOrUpdateIssue(ctx, issue3, http.MethodPost))
	issue4 := makeIssue(4, commits[2])
	require.NoError(t, a.AddOrUpdateIssue(ctx, issue4, http.MethodPost))
	mockChild.MockGetCommit(ctx, git.MainBranch)
	mockChild.MockLog(ctx, git.LogFromTo(commits[1], commits[2]))
	require.NoError(t, a.UpdateStatus(ctx, "", true))
	issue3.Closed = true
	issue3.Result = autoroll.ROLL_RESULT_FAILURE
	assertdeep.Equal(t, []*autoroll.AutoRollIssue{issue4, issue3, issue2, issue1}, a.status.Get().Recent)

	// Test both situations at the same time.
	issue5 := makeIssue(5, commits[2])
	closeIssue(issue5, autoroll.ROLL_RESULT_SUCCESS)
	require.NoError(t, a.AddOrUpdateIssue(ctx, issue5, http.MethodPut))
	mockChild.MockGetCommit(ctx, git.MainBranch)
	mockChild.MockLog(ctx, git.LogFromTo(commits[2], commits[2]))
	require.NoError(t, a.UpdateStatus(ctx, "", true))
	issue4.Closed = true
	issue4.Result = autoroll.ROLL_RESULT_FAILURE
	assertdeep.Equal(t, []*autoroll.AutoRollIssue{issue5, issue4, issue3, issue2, issue1}, a.status.Get().Recent)
}

func makeRoll(now time.Time) Roll {
	return Roll{
		ChangeListNumber: 1,
		Closed:           false,
		Created:          jsonutils.Time(now),
		Modified:         jsonutils.Time(now),
		Result:           autoroll.ROLL_RESULT_IN_PROGRESS,
		RollingTo:        "rev",
		RollingFrom:      "prevrev",
		Subject:          "1",
		Submitted:        false,
		TestSummaryUrl:   "http://example.com/",
	}
}

func TestRollAsIssue(t *testing.T) {

	expected := makeIssue(1, "rev")
	now := expected.Created
	roll := makeRoll(now)

	actual, err := roll.AsIssue()
	require.NoError(t, err)
	assertdeep.Equal(t, expected, actual)

	roll.TestSummaryUrl = ""
	savedTryResults := expected.TryResults
	expected.TryResults = []*autoroll.TryResult{}
	actual, err = roll.AsIssue()
	require.NoError(t, err)
	assertdeep.Equal(t, expected, actual)

	roll.Closed = true
	expected.Closed = true
	expected.CqFinished = true
	roll.Result = autoroll.ROLL_RESULT_FAILURE
	expected.Result = autoroll.ROLL_RESULT_FAILURE
	roll.TestSummaryUrl = "http://example.com/"
	expected.TryResults = savedTryResults
	expected.TryResults[0].Result = autoroll.TRYBOT_RESULT_FAILURE
	expected.TryResults[0].Status = autoroll.TRYBOT_STATUS_COMPLETED
	actual, err = roll.AsIssue()
	require.NoError(t, err)
	assertdeep.Equal(t, expected, actual)

	roll.Submitted = true
	roll.Result = autoroll.ROLL_RESULT_SUCCESS
	expected.Committed = true
	expected.CqSuccess = true
	expected.Result = autoroll.ROLL_RESULT_SUCCESS
	expected.TryResults[0].Result = autoroll.TRYBOT_RESULT_SUCCESS
	actual, err = roll.AsIssue()
	require.NoError(t, err)
	assertdeep.Equal(t, expected, actual)

	roll = makeRoll(now)
	roll.Created = jsonutils.Time{}
	_, err = roll.AsIssue()
	require.EqualError(t, err, "Missing parameter.")

	roll = makeRoll(now)
	roll.RollingFrom = ""
	_, err = roll.AsIssue()
	require.EqualError(t, err, "Missing parameter.")

	roll = makeRoll(now)
	roll.RollingTo = ""
	_, err = roll.AsIssue()
	require.EqualError(t, err, "Missing parameter.")

	roll = makeRoll(now)
	roll.Closed = true
	_, err = roll.AsIssue()
	require.EqualError(t, err, "Inconsistent parameters: result must be set.")

	roll = makeRoll(now)
	roll.Submitted = true
	_, err = roll.AsIssue()
	require.EqualError(t, err, "Inconsistent parameters: submitted but not closed.")

	roll = makeRoll(now)
	roll.Result = ""
	_, err = roll.AsIssue()
	require.EqualError(t, err, "Unsupported value for result.")

	roll = makeRoll(now)
	roll.TestSummaryUrl = ":http//example.com"
	_, err = roll.AsIssue()
	require.EqualError(t, err, "Invalid testSummaryUrl parameter.")
}
