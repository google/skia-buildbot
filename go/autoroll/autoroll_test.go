package autoroll

import (
	"fmt"
	"testing"
	"time"

	github_api "github.com/google/go-github/github"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/comment"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestAutoRollIssueCopy(t *testing.T) {
	unittest.SmallTest(t)
	roll := &AutoRollIssue{
		Closed: true,
		Comments: []*comment.Comment{
			{
				Id:        "123",
				Message:   "hello world",
				Timestamp: time.Now(),
				User:      "me@google.com",
			},
		},
		Committed:      true,
		Created:        time.Now(),
		CqFinished:     true,
		CqSuccess:      true,
		DryRunFinished: true,
		DryRunSuccess:  true,
		IsDryRun:       true,
		Issue:          123,
		Modified:       time.Now(),
		Patchsets:      []int64{1},
		Result:         ROLL_RESULT_SUCCESS,
		RollingFrom:    "abc123",
		RollingTo:      "def456",
		Subject:        "Roll src/third_party/skia abc123..def456 (3 commits).",
		TryResults: []*TryResult{
			{
				Builder:  "build",
				Category: "cats",
				Created:  time.Now(),
				Result:   TRYBOT_RESULT_SUCCESS,
				Status:   TRYBOT_STATUS_COMPLETED,
				Url:      "http://build/cats",
			},
		},
	}
	deepequal.AssertCopy(t, roll, roll.Copy())
}

func TestTrybotResults(t *testing.T) {
	unittest.SmallTest(t)
	// Create a fake roll with one in-progress trybot.
	roll := &AutoRollIssue{
		Closed:    false,
		Committed: false,
		Created:   time.Now(),
		Issue:     123,
		Modified:  time.Now(),
		Patchsets: []int64{1},
		Subject:   "Roll src/third_party/skia abc123..def456 (3 commits).",
	}
	roll.Result = rollResult(roll)

	trybot := &buildbucket.Build{
		Created: time.Now().UTC(),
		Status:  TRYBOT_STATUS_STARTED,
		Parameters: &buildbucket.Parameters{
			BuilderName: "fake-builder",
			Properties: buildbucket.Properties{
				Category: "cq",
			},
		},
	}
	tryResult, err := TryResultFromBuildbucket(trybot)
	assert.NoError(t, err)
	roll.TryResults = []*TryResult{tryResult}
	assert.False(t, roll.AllTrybotsFinished())
	assert.False(t, roll.AllTrybotsSucceeded())

	// Trybot failed.
	tryResult.Status = TRYBOT_STATUS_COMPLETED
	tryResult.Result = TRYBOT_RESULT_FAILURE
	assert.True(t, roll.AllTrybotsFinished())
	assert.False(t, roll.AllTrybotsSucceeded())

	retry := &buildbucket.Build{
		Created: time.Now().UTC(),
		Status:  TRYBOT_STATUS_STARTED,
		Parameters: &buildbucket.Parameters{
			BuilderName: "fake-builder",
			Properties: buildbucket.Properties{
				Category: "cq",
			},
		},
	}
	tryResult, err = TryResultFromBuildbucket(retry)
	assert.NoError(t, err)
	roll.TryResults = append(roll.TryResults, tryResult)
	assert.False(t, roll.AllTrybotsFinished())
	assert.False(t, roll.AllTrybotsSucceeded())

	// The second try result, a retry of the first, succeeded.
	tryResult.Status = TRYBOT_STATUS_COMPLETED
	tryResult.Result = TRYBOT_RESULT_SUCCESS
	assert.True(t, roll.AllTrybotsFinished())
	assert.True(t, roll.AllTrybotsSucceeded())

	// Verify that the ordering of try results does not matter.
	roll.TryResults[0], roll.TryResults[1] = roll.TryResults[1], roll.TryResults[0]
	assert.True(t, roll.AllTrybotsFinished())
	assert.True(t, roll.AllTrybotsSucceeded())

	// Verify that an "experimental" trybot doesn't count against us.
	exp := &buildbucket.Build{
		Created: time.Now().UTC(),
		Result:  TRYBOT_RESULT_SUCCESS,
		Status:  TRYBOT_STATUS_COMPLETED,
		Parameters: &buildbucket.Parameters{
			BuilderName: "fake-builder",
			Properties: buildbucket.Properties{
				Category: "cq-experimental",
			},
		},
	}
	tryResult, err = TryResultFromBuildbucket(exp)
	assert.NoError(t, err)
	roll.TryResults = append(roll.TryResults, tryResult)
	assert.True(t, roll.AllTrybotsFinished())
	assert.True(t, roll.AllTrybotsSucceeded())
}

func TestUpdateFromGerritChangeInfo(t *testing.T) {
	unittest.SmallTest(t)

	now := time.Now()

	a := &AutoRollIssue{
		Issue:       123,
		RollingFrom: "abc123",
		RollingTo:   "def456",
	}

	// Ensure that we don't overwrite the issue number.
	assert.EqualError(t, a.UpdateFromGerritChangeInfo(&gerrit.ChangeInfo{}, false), "CL ID 0 differs from existing issue number 123!")

	// Normal, in-progress CL.
	rev := &gerrit.Revision{
		ID:            "1",
		Number:        1,
		Created:       now,
		CreatedString: now.Format(gerrit.TIME_FORMAT),
	}
	ci := &gerrit.ChangeInfo{
		Created:       now,
		CreatedString: now.Format(gerrit.TIME_FORMAT),
		Subject:       "roll the deps",
		ChangeId:      fmt.Sprintf("%d", a.Issue),
		Issue:         a.Issue,
		Labels: map[string]*gerrit.LabelEntry{
			gerrit.CODEREVIEW_LABEL: {
				All: []*gerrit.LabelDetail{
					{
						Value: gerrit.CODEREVIEW_LABEL_APPROVE,
					},
				},
			},
			gerrit.COMMITQUEUE_LABEL: {
				All: []*gerrit.LabelDetail{
					{
						Value: gerrit.COMMITQUEUE_LABEL_SUBMIT,
					},
				},
			},
		},
		Owner: &gerrit.Owner{
			Email: "fake@chromium.org",
		},
		Project: "skia",
		Revisions: map[string]*gerrit.Revision{
			rev.ID: rev,
		},
		Patchsets:     []*gerrit.Revision{rev},
		Status:        gerrit.CHANGE_STATUS_NEW,
		Updated:       now,
		UpdatedString: now.Format(gerrit.TIME_FORMAT),
	}
	assert.NoError(t, a.UpdateFromGerritChangeInfo(ci, false))
	expect := &AutoRollIssue{
		Created:     now,
		Issue:       123,
		Modified:    now,
		Patchsets:   []int64{1},
		Result:      ROLL_RESULT_IN_PROGRESS,
		RollingFrom: "abc123",
		RollingTo:   "def456",
		Subject:     "roll the deps",
	}
	deepequal.AssertDeepEqual(t, expect, a)

	// CQ failed.
	delete(ci.Labels, gerrit.COMMITQUEUE_LABEL)
	expect.CqFinished = true
	expect.Result = ROLL_RESULT_FAILURE
	assert.NoError(t, a.UpdateFromGerritChangeInfo(ci, false))
	deepequal.AssertDeepEqual(t, expect, a)

	// CQ succeeded.
	ci.Committed = true
	ci.Status = gerrit.CHANGE_STATUS_MERGED
	expect.Closed = true
	expect.Committed = true
	expect.CqSuccess = true
	expect.Result = ROLL_RESULT_SUCCESS
	assert.NoError(t, a.UpdateFromGerritChangeInfo(ci, false))
	deepequal.AssertDeepEqual(t, expect, a)

	// CL was abandoned while CQ was running.
	ci.Labels[gerrit.COMMITQUEUE_LABEL] = &gerrit.LabelEntry{
		All: []*gerrit.LabelDetail{
			{
				Value: gerrit.COMMITQUEUE_LABEL_SUBMIT,
			},
		},
	}
	ci.Committed = false
	ci.Status = gerrit.CHANGE_STATUS_ABANDONED
	expect.Committed = false
	expect.CqFinished = true // Not really, but the CL is finished.
	expect.CqSuccess = false
	expect.Result = ROLL_RESULT_FAILURE
	assert.NoError(t, a.UpdateFromGerritChangeInfo(ci, false))
	deepequal.AssertDeepEqual(t, expect, a)

	// Dry run active.
	ci.Status = gerrit.CHANGE_STATUS_NEW
	ci.Labels[gerrit.COMMITQUEUE_LABEL].All[0].Value = gerrit.COMMITQUEUE_LABEL_DRY_RUN
	expect.Closed = false
	expect.CqFinished = false
	expect.IsDryRun = true
	expect.Result = ROLL_RESULT_DRY_RUN_IN_PROGRESS
	a.IsDryRun = true
	assert.NoError(t, a.UpdateFromGerritChangeInfo(ci, false))
	deepequal.AssertDeepEqual(t, expect, a)

	// Dry run failed.
	delete(ci.Labels, gerrit.COMMITQUEUE_LABEL)
	expect.DryRunFinished = true
	expect.Result = ROLL_RESULT_DRY_RUN_FAILURE
	expect.TryResults = []*TryResult{
		{
			Builder:  "fake",
			Category: TRYBOT_CATEGORY_CQ,
			Result:   TRYBOT_RESULT_FAILURE,
			Status:   TRYBOT_STATUS_COMPLETED,
		},
	}
	a.TryResults = expect.TryResults
	assert.NoError(t, a.UpdateFromGerritChangeInfo(ci, false))
	deepequal.AssertDeepEqual(t, expect, a)

	// The CL was abandoned while the dry run was running.
	expect.TryResults[0].Result = ""
	expect.TryResults[0].Status = TRYBOT_STATUS_SCHEDULED
	ci.Status = gerrit.CHANGE_STATUS_ABANDONED
	expect.Closed = true
	expect.CqFinished = true
	expect.DryRunFinished = true
	expect.Result = ROLL_RESULT_DRY_RUN_FAILURE
	assert.NoError(t, a.UpdateFromGerritChangeInfo(ci, true))
	deepequal.AssertDeepEqual(t, expect, a)

	// The CL was landed while the dry run was running.
	ci.Committed = true
	ci.Status = gerrit.CHANGE_STATUS_MERGED
	expect.Committed = true
	expect.CqSuccess = true
	expect.DryRunSuccess = true
	expect.Result = ROLL_RESULT_DRY_RUN_SUCCESS
	assert.NoError(t, a.UpdateFromGerritChangeInfo(ci, true))
	deepequal.AssertDeepEqual(t, expect, a)

	// Dry run success.
	ci.Committed = false
	ci.Status = gerrit.CHANGE_STATUS_NEW
	expect.Closed = false
	expect.Committed = false
	expect.CqFinished = false
	expect.CqSuccess = false
	expect.DryRunSuccess = true
	expect.Result = ROLL_RESULT_DRY_RUN_SUCCESS
	expect.TryResults[0].Result = TRYBOT_RESULT_SUCCESS
	expect.TryResults[0].Status = TRYBOT_STATUS_COMPLETED
	assert.NoError(t, a.UpdateFromGerritChangeInfo(ci, false))
	deepequal.AssertDeepEqual(t, expect, a)
}

func TestUpdateFromGerritChangeInfoAndroid(t *testing.T) {
	unittest.SmallTest(t)

	now := time.Now()

	a := &AutoRollIssue{
		Issue:       123,
		RollingFrom: "abc123",
		RollingTo:   "def456",
	}

	// Ensure that we don't overwrite the issue number.
	assert.EqualError(t, a.UpdateFromGerritChangeInfo(&gerrit.ChangeInfo{}, true), "CL ID 0 differs from existing issue number 123!")

	// Normal, in-progress CL.
	rev := &gerrit.Revision{
		ID:            "1",
		Number:        1,
		Created:       now,
		CreatedString: now.Format(gerrit.TIME_FORMAT),
	}
	ci := &gerrit.ChangeInfo{
		Created:       now,
		CreatedString: now.Format(gerrit.TIME_FORMAT),
		Subject:       "roll the deps",
		ChangeId:      fmt.Sprintf("%d", a.Issue),
		Issue:         a.Issue,
		Labels: map[string]*gerrit.LabelEntry{
			gerrit.CODEREVIEW_LABEL: {
				All: []*gerrit.LabelDetail{
					{
						Value: 2,
					},
				},
			},
			gerrit.PRESUBMIT_READY_LABEL: {
				All: []*gerrit.LabelDetail{
					{
						Value: 1,
					},
				},
			},
			gerrit.AUTOSUBMIT_LABEL: {
				All: []*gerrit.LabelDetail{
					{
						Value: gerrit.AUTOSUBMIT_LABEL_NONE,
					},
				},
			},
		},
		Owner: &gerrit.Owner{
			Email: "fake@chromium.org",
		},
		Project: "skia",
		Revisions: map[string]*gerrit.Revision{
			rev.ID: rev,
		},
		Patchsets:     []*gerrit.Revision{rev},
		Status:        gerrit.CHANGE_STATUS_NEW,
		Updated:       now,
		UpdatedString: now.Format(gerrit.TIME_FORMAT),
	}
	assert.NoError(t, a.UpdateFromGerritChangeInfo(ci, true))
	expect := &AutoRollIssue{
		Created:     now,
		Issue:       123,
		Modified:    now,
		Patchsets:   []int64{1},
		Result:      ROLL_RESULT_IN_PROGRESS,
		RollingFrom: "abc123",
		RollingTo:   "def456",
		Subject:     "roll the deps",
	}
	deepequal.AssertDeepEqual(t, expect, a)

	// CQ failed.
	ci.Labels[gerrit.PRESUBMIT_VERIFIED_LABEL] = &gerrit.LabelEntry{
		All: []*gerrit.LabelDetail{
			{
				Value: gerrit.PRESUBMIT_VERIFIED_LABEL_REJECTED,
			},
		},
	}
	expect.CqFinished = true
	expect.Result = ROLL_RESULT_FAILURE
	assert.NoError(t, a.UpdateFromGerritChangeInfo(ci, true))
	deepequal.AssertDeepEqual(t, expect, a)

	// CQ succeeded.
	ci.Committed = true
	ci.Status = gerrit.CHANGE_STATUS_MERGED
	expect.Closed = true
	expect.Committed = true
	expect.CqSuccess = true
	expect.Result = ROLL_RESULT_SUCCESS
	assert.NoError(t, a.UpdateFromGerritChangeInfo(ci, true))
	deepequal.AssertDeepEqual(t, expect, a)

	// CL was abandoned while CQ was running.
	delete(ci.Labels, gerrit.PRESUBMIT_VERIFIED_LABEL)
	ci.Committed = false
	ci.Status = gerrit.CHANGE_STATUS_ABANDONED
	expect.Committed = false
	expect.CqFinished = true // Not really, but the CL is finished.
	expect.CqSuccess = false
	expect.Result = ROLL_RESULT_FAILURE
	assert.NoError(t, a.UpdateFromGerritChangeInfo(ci, true))
	deepequal.AssertDeepEqual(t, expect, a)

	// Dry run active.
	ci.Status = gerrit.CHANGE_STATUS_NEW
	ci.Labels[gerrit.AUTOSUBMIT_LABEL].All[0].Value = gerrit.AUTOSUBMIT_LABEL_NONE
	expect.Closed = false
	expect.CqFinished = false
	expect.IsDryRun = true
	expect.Result = ROLL_RESULT_DRY_RUN_IN_PROGRESS
	a.IsDryRun = true
	assert.NoError(t, a.UpdateFromGerritChangeInfo(ci, true))
	deepequal.AssertDeepEqual(t, expect, a)

	// Dry run failed.
	ci.Labels[gerrit.PRESUBMIT_VERIFIED_LABEL] = &gerrit.LabelEntry{
		All: []*gerrit.LabelDetail{
			{
				Value: gerrit.PRESUBMIT_VERIFIED_LABEL_REJECTED,
			},
		},
	}
	expect.DryRunFinished = true
	expect.Result = ROLL_RESULT_DRY_RUN_FAILURE
	assert.NoError(t, a.UpdateFromGerritChangeInfo(ci, true))
	deepequal.AssertDeepEqual(t, expect, a)

	// The CL was abandoned while the dry run was running.
	ci.Labels[gerrit.PRESUBMIT_VERIFIED_LABEL].All[0].Value = gerrit.PRESUBMIT_VERIFIED_LABEL_RUNNING
	ci.Status = gerrit.CHANGE_STATUS_ABANDONED
	expect.Closed = true
	expect.CqFinished = true
	expect.DryRunFinished = true
	expect.Result = ROLL_RESULT_DRY_RUN_FAILURE
	assert.NoError(t, a.UpdateFromGerritChangeInfo(ci, true))
	deepequal.AssertDeepEqual(t, expect, a)

	// The CL was landed while the dry run was running.
	ci.Committed = true
	ci.Status = gerrit.CHANGE_STATUS_MERGED
	expect.Committed = true
	expect.CqSuccess = true
	expect.DryRunSuccess = true
	expect.Result = ROLL_RESULT_DRY_RUN_SUCCESS
	assert.NoError(t, a.UpdateFromGerritChangeInfo(ci, true))
	deepequal.AssertDeepEqual(t, expect, a)

	// Dry run success.
	ci.Labels[gerrit.PRESUBMIT_VERIFIED_LABEL] = &gerrit.LabelEntry{
		All: []*gerrit.LabelDetail{
			{
				Value: gerrit.PRESUBMIT_VERIFIED_LABEL_ACCEPTED,
			},
		},
	}
	ci.Committed = false
	ci.Status = gerrit.CHANGE_STATUS_NEW
	expect.Closed = false
	expect.CqFinished = false
	expect.CqSuccess = false
	expect.Committed = false
	expect.DryRunSuccess = true
	expect.Result = ROLL_RESULT_DRY_RUN_SUCCESS
	assert.NoError(t, a.UpdateFromGerritChangeInfo(ci, true))
	deepequal.AssertDeepEqual(t, expect, a)
}

func TestUpdateFromGitHubPullRequest(t *testing.T) {
	unittest.SmallTest(t)

	now := time.Now()

	intPtr := func(v int) *int {
		return &v
	}
	stringPtr := func(v string) *string {
		return &v
	}
	boolPtr := func(v bool) *bool {
		return &v
	}

	a := &AutoRollIssue{
		Issue:       123,
		RollingFrom: "abc123",
		RollingTo:   "def456",
	}

	// Ensure that we don't overwrite the issue number.
	assert.EqualError(t, a.UpdateFromGitHubPullRequest(&github_api.PullRequest{}), "Pull request number 0 differs from existing issue number 123!")

	// Normal, in-progress CL.
	pr := &github_api.PullRequest{
		Number:    intPtr(int(a.Issue)),
		State:     stringPtr(""),
		Commits:   intPtr(1),
		Title:     stringPtr("roll the deps"),
		CreatedAt: &now,
		UpdatedAt: &now,
	}
	assert.NoError(t, a.UpdateFromGitHubPullRequest(pr))
	expect := &AutoRollIssue{
		Created:     now,
		Issue:       123,
		Modified:    now,
		Patchsets:   []int64{1},
		Result:      ROLL_RESULT_IN_PROGRESS,
		RollingFrom: "abc123",
		RollingTo:   "def456",
		Subject:     "roll the deps",
	}
	deepequal.AssertDeepEqual(t, expect, a)

	// CQ failed.
	pr.State = &github.CLOSED_STATE
	expect.Closed = true // if the CQ fails, we close the PR.
	expect.CqFinished = true
	expect.Result = ROLL_RESULT_FAILURE
	assert.NoError(t, a.UpdateFromGitHubPullRequest(pr))
	deepequal.AssertDeepEqual(t, expect, a)

	// CQ succeeded.
	pr.Merged = boolPtr(true)
	expect.Closed = true
	expect.Committed = true
	expect.CqSuccess = true
	expect.Result = ROLL_RESULT_SUCCESS
	assert.NoError(t, a.UpdateFromGitHubPullRequest(pr))
	deepequal.AssertDeepEqual(t, expect, a)

	// CL was abandoned while CQ was running.
	// (the above includes this case)

	// Dry run active.
	pr.Merged = boolPtr(false)
	pr.State = stringPtr("")
	expect.TryResults = []*TryResult{
		{
			Builder:  "fake",
			Category: TRYBOT_CATEGORY_CQ,
			Status:   TRYBOT_STATUS_SCHEDULED,
		},
	}
	expect.Closed = false
	expect.Committed = false
	expect.CqFinished = false
	expect.CqSuccess = false
	expect.IsDryRun = true
	expect.Result = ROLL_RESULT_DRY_RUN_IN_PROGRESS
	a.IsDryRun = true
	a.TryResults = expect.TryResults
	assert.NoError(t, a.UpdateFromGitHubPullRequest(pr))
	deepequal.AssertDeepEqual(t, expect, a)

	// Dry run failed.
	expect.DryRunFinished = true
	expect.Result = ROLL_RESULT_DRY_RUN_FAILURE
	expect.TryResults[0].Result = TRYBOT_RESULT_FAILURE
	expect.TryResults[0].Status = TRYBOT_STATUS_COMPLETED
	a.TryResults = expect.TryResults
	assert.NoError(t, a.UpdateFromGitHubPullRequest(pr))
	deepequal.AssertDeepEqual(t, expect, a)

	// CL was abandoned while dry run was still running.
	expect.TryResults[0].Result = ""
	expect.TryResults[0].Status = TRYBOT_STATUS_SCHEDULED
	pr.State = &github.CLOSED_STATE
	expect.Closed = true
	expect.CqFinished = true
	assert.NoError(t, a.UpdateFromGitHubPullRequest(pr))
	deepequal.AssertDeepEqual(t, expect, a)

	// CL was landed while dry run was still running.
	pr.Merged = boolPtr(true)
	expect.Committed = true
	expect.CqSuccess = true
	expect.DryRunSuccess = true
	expect.Result = ROLL_RESULT_DRY_RUN_SUCCESS
	assert.NoError(t, a.UpdateFromGitHubPullRequest(pr))
	deepequal.AssertDeepEqual(t, expect, a)

	// Dry run success.
	pr.Merged = boolPtr(false)
	pr.State = stringPtr("")
	expect.Closed = false
	expect.Committed = false
	expect.CqFinished = false
	expect.CqSuccess = false
	expect.DryRunSuccess = true
	expect.Result = ROLL_RESULT_DRY_RUN_SUCCESS
	expect.TryResults[0].Result = TRYBOT_RESULT_SUCCESS
	expect.TryResults[0].Status = TRYBOT_STATUS_COMPLETED
	assert.NoError(t, a.UpdateFromGitHubPullRequest(pr))
	deepequal.AssertDeepEqual(t, expect, a)
}
