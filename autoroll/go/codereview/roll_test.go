package codereview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	github_api "github.com/google/go-github/github"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/recent_rolls"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/gerrit"
	gerrit_testutils "go.skia.org/infra/go/gerrit/testutils"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
)

func makeFakeRoll(issueNum int64, from, to string, dryRun, android bool) *gerrit.ChangeInfo {
	// Gerrit API only has millisecond precision.
	now := time.Now().UTC().Round(time.Millisecond)
	description := fmt.Sprintf(`Roll src/third_party/skia/ %s..%s (42 commits).

blah blah
TBR=some-sheriff
`, from[:12], to[:12])
	rev := &gerrit.Revision{
		ID:            "1",
		Number:        1,
		CreatedString: now.Format(gerrit.TIME_FORMAT),
		Created:       now,
	}
	cqLabel := gerrit.COMMITQUEUE_LABEL_SUBMIT
	if dryRun {
		if android {
			cqLabel = gerrit.AUTOSUBMIT_LABEL_NONE
		} else {
			cqLabel = gerrit.COMMITQUEUE_LABEL_DRY_RUN
		}
	}
	roll := &gerrit.ChangeInfo{
		Created:       now,
		CreatedString: now.Format(gerrit.TIME_FORMAT),
		Subject:       description,
		ChangeId:      fmt.Sprintf("%d", issueNum),
		Issue:         issueNum,
		Owner: &gerrit.Owner{
			Email: "fake-deps-roller@chromium.org",
		},
		Project: "skia",
		Revisions: map[string]*gerrit.Revision{
			"1": rev,
		},
		Patchsets:     []*gerrit.Revision{rev},
		Updated:       now,
		UpdatedString: now.Format(gerrit.TIME_FORMAT),
	}
	if android {
		roll.Labels = map[string]*gerrit.LabelEntry{
			gerrit.PRESUBMIT_VERIFIED_LABEL: {
				All: []*gerrit.LabelDetail{},
			},
			gerrit.AUTOSUBMIT_LABEL: {
				All: []*gerrit.LabelDetail{
					{
						Value: gerrit.AUTOSUBMIT_LABEL_SUBMIT,
					},
				},
			},
		}
	} else {
		roll.Labels = map[string]*gerrit.LabelEntry{
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
						Value: cqLabel,
					},
				},
			},
		}
	}
	return roll
}

func TestGerritRoll(t *testing.T) {
	testutils.LargeTest(t)

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	testutil.InitDatastore(t, ds.KIND_AUTOROLL_ROLL)

	g := gerrit_testutils.NewGerrit(t, tmp, false)
	ctx := context.Background()
	recent, err := recent_rolls.NewRecentRolls(ctx, "test-roller")
	assert.NoError(t, err)

	rollFinishedCalled := map[string]int{}
	rollFinished := func(ctx context.Context, roll RollImpl) error {
		issue := roll.IssueID()
		rollFinishedCalled[issue] = rollFinishedCalled[issue] + 1
		return nil
	}

	// Upload and retrieve the roll.
	sklog.Errorf("normal gerrit roll")
	from := "abcde12345abcde12345abcde12345abcde12345"
	to := "fghij67890fghij67890fghij67890fghij67890"
	fullHash := func(ctx context.Context, hash string) (string, error) {
		if strings.HasPrefix(from, hash) {
			return from, nil
		}
		if strings.HasPrefix(to, hash) {
			return to, nil
		}
		return "", errors.New("Unknown hash")
	}
	roll := makeFakeRoll(123, from, to, false, false)
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, 1, nil)
	gr, err := newGerritRoll(ctx, g.Gerrit, fullHash, recent, roll.Issue, "http://issue/", rollFinished)
	assert.NoError(t, err)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, to, gr.RollingTo())

	// Insert into DB.
	current := recent.CurrentRoll()
	assert.Nil(t, current)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	current = recent.CurrentRoll()
	assert.NotNil(t, current)
	assert.Equal(t, current.Issue, roll.Issue)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, 0, rollFinishedCalled[gr.IssueID()])

	// Add a comment.
	msg := "Here's a comment"
	g.MockAddComment(roll, msg)
	assert.NoError(t, gr.AddComment(msg))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, 0, rollFinishedCalled[gr.IssueID()])

	// Set dry run.
	g.MockSetDryRun(roll, "Mode was changed to dry run")
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, 1, nil)
	assert.NoError(t, gr.SwitchToDryRun(ctx))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, 0, rollFinishedCalled[gr.IssueID()])

	// Set normal.
	g.MockSetCQ(roll, "Mode was changed to normal")
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, 1, nil)
	assert.NoError(t, gr.SwitchToNormal(ctx))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, 0, rollFinishedCalled[gr.IssueID()])

	// Update.
	roll.Status = gerrit.CHANGE_STATUS_MERGED
	// Landing a change adds an empty patchset.
	rev := &gerrit.Revision{
		Number:  int64(len(roll.Revisions) + 1),
		Created: time.Now(),
		Kind:    "",
	}
	roll.Revisions[fmt.Sprintf("%d", rev.Number)] = rev
	roll.Patchsets = append(roll.Patchsets, rev)
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, 1, nil)
	assert.NoError(t, gr.Update(ctx))
	assert.True(t, gr.IsFinished())
	assert.True(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	assert.Nil(t, recent.CurrentRoll())
	assert.Equal(t, 1, rollFinishedCalled[gr.IssueID()])

	// Upload and retrieve another roll, dry run this time.
	sklog.Errorf("Dry run gerrit roll")
	roll = makeFakeRoll(124, from, to, true, false)
	g.MockGetIssueProperties(roll)
	tryjob := &buildbucket.Build{
		Id:      "99999",
		Created: time.Now().UTC().Round(time.Millisecond),
		Status:  autoroll.TRYBOT_STATUS_STARTED,
		Parameters: &buildbucket.Parameters{
			BuilderName: "fake-builder",
			Properties: buildbucket.Properties{
				Category:       "cq",
				GerritPatchset: "1",
			},
		},
	}
	g.MockGetTrybotResults(roll, 1, []*buildbucket.Build{tryjob})
	gr, err = newGerritRoll(ctx, g.Gerrit, fullHash, recent, roll.Issue, "http://issue/", rollFinished)
	assert.NoError(t, err)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, to, gr.RollingTo())
	assert.Equal(t, 0, rollFinishedCalled[gr.IssueID()])

	// Insert into DB.
	current = recent.CurrentRoll()
	assert.Nil(t, current)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	current = recent.CurrentRoll()
	assert.NotNil(t, current)
	assert.Equal(t, current.Issue, roll.Issue)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, 0, rollFinishedCalled[gr.IssueID()])

	// Success.
	tryjob.Status = autoroll.TRYBOT_STATUS_COMPLETED
	tryjob.Result = autoroll.TRYBOT_RESULT_SUCCESS
	roll.Labels[gerrit.COMMITQUEUE_LABEL] = &gerrit.LabelEntry{
		All: []*gerrit.LabelDetail{
			{
				Value: gerrit.COMMITQUEUE_LABEL_NONE,
			},
		},
	}
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, 1, []*buildbucket.Build{tryjob})
	assert.NoError(t, gr.Update(ctx))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.True(t, gr.IsDryRunFinished())
	assert.True(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, 1, rollFinishedCalled[gr.IssueID()])

	// Update again, ensure that we don't call the callback twice.
	tryjob.Status = autoroll.TRYBOT_STATUS_COMPLETED
	tryjob.Result = autoroll.TRYBOT_RESULT_SUCCESS
	roll.Labels[gerrit.COMMITQUEUE_LABEL] = &gerrit.LabelEntry{
		All: []*gerrit.LabelDetail{
			{
				Value: gerrit.COMMITQUEUE_LABEL_NONE,
			},
		},
	}
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, 1, []*buildbucket.Build{tryjob})
	assert.NoError(t, gr.Update(ctx))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.True(t, gr.IsDryRunFinished())
	assert.True(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, 1, rollFinishedCalled[gr.IssueID()])

	// Close for cleanup.
	roll.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, 1, []*buildbucket.Build{tryjob})
	assert.NoError(t, gr.Update(ctx))

	// Verify that all of the mutation functions handle a conflict (eg.
	// someone closed the CL) gracefully.
	sklog.Errorf("Test mutations")

	// 1. SwitchToDryRun.
	roll = makeFakeRoll(125, from, to, false, false)
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, 1, nil)
	gr, err = newGerritRoll(ctx, g.Gerrit, fullHash, recent, roll.Issue, "http://issue/", nil)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	url, reqBytes := g.MakePostRequest(roll, "Mode was changed to dry run", map[string]int{
		gerrit.COMMITQUEUE_LABEL: gerrit.COMMITQUEUE_LABEL_DRY_RUN,
	})
	g.Mock.MockOnce(url, mockhttpclient.MockPostError("application/json", reqBytes, "CONFLICT", http.StatusConflict))
	roll.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, 1, nil)
	assert.NoError(t, gr.SwitchToDryRun(ctx))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()

	// 2. SwitchToNormal
	roll = makeFakeRoll(126, from, to, false, false)
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, 1, nil)
	gr, err = newGerritRoll(ctx, g.Gerrit, fullHash, recent, roll.Issue, "http://issue/", nil)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	url, reqBytes = g.MakePostRequest(roll, "Mode was changed to normal", map[string]int{
		gerrit.COMMITQUEUE_LABEL: gerrit.COMMITQUEUE_LABEL_SUBMIT,
	})
	g.Mock.MockOnce(url, mockhttpclient.MockPostError("application/json", reqBytes, "CONFLICT", http.StatusConflict))
	roll.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, 1, nil)
	assert.NoError(t, gr.SwitchToNormal(ctx))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()

	// 3. Close.
	roll = makeFakeRoll(127, from, to, false, false)
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, 1, nil)
	gr, err = newGerritRoll(ctx, g.Gerrit, fullHash, recent, roll.Issue, "http://issue/", nil)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	url = fmt.Sprintf("%s/a/changes/%d/abandon", gerrit_testutils.FAKE_GERRIT_URL, roll.Issue)
	req := testutils.MarshalJSON(t, &struct {
		Message string `json:"message"`
	}{
		Message: "close it!",
	})
	g.Mock.MockOnce(url, mockhttpclient.MockPostError("application/json", []byte(req), "CONFLICT", http.StatusConflict))
	roll.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, 1, nil)
	assert.NoError(t, gr.Close(ctx, autoroll.ROLL_RESULT_FAILURE, "close it!"))
	assert.True(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()

	// Verify that we set the correct status when abandoning a CL.
	roll = makeFakeRoll(128, from, to, false, false)
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, 1, nil)
	gr, err = newGerritRoll(ctx, g.Gerrit, fullHash, recent, roll.Issue, "http://issue/", nil)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	url = fmt.Sprintf("%s/a/changes/%d/abandon", gerrit_testutils.FAKE_GERRIT_URL, roll.Issue)
	req = testutils.MarshalJSON(t, &struct {
		Message string `json:"message"`
	}{
		Message: "close it!",
	})
	g.Mock.MockOnce(url, mockhttpclient.MockPostDialogue("application/json", []byte(req), nil))
	roll.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, 1, nil)
	assert.NoError(t, gr.Close(ctx, autoroll.ROLL_RESULT_DRY_RUN_SUCCESS, "close it!"))
	g.AssertEmpty()
	issue, err := recent.Get(ctx, 128)
	assert.NoError(t, err)
	assert.Equal(t, issue.Result, autoroll.ROLL_RESULT_DRY_RUN_SUCCESS)
}

func TestGerritAndroidRoll(t *testing.T) {
	testutils.LargeTest(t)

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	testutil.InitDatastore(t, ds.KIND_AUTOROLL_ROLL)

	g := gerrit_testutils.NewGerrit(t, tmp, true)

	ctx := context.Background()
	recent, err := recent_rolls.NewRecentRolls(ctx, "test-roller")
	assert.NoError(t, err)

	rollFinishedCalled := map[string]int{}
	rollFinished := func(ctx context.Context, roll RollImpl) error {
		issue := roll.IssueID()
		rollFinishedCalled[issue] = rollFinishedCalled[issue] + 1
		return nil
	}

	// Upload and retrieve the roll.
	sklog.Errorf("Normal android roll")
	from := "abcde12345abcde12345abcde12345abcde12345"
	to := "fghij67890fghij67890fghij67890fghij67890"
	fullHash := func(ctx context.Context, hash string) (string, error) {
		if strings.HasPrefix(from, hash) {
			return from, nil
		}
		if strings.HasPrefix(to, hash) {
			return to, nil
		}
		return "", errors.New("Unknown hash")
	}
	roll := makeFakeRoll(123, from, to, false, true)
	g.MockGetIssueProperties(roll)
	gr, err := newGerritAndroidRoll(ctx, g.Gerrit, fullHash, recent, roll.Issue, "http://issue/", rollFinished)
	assert.NoError(t, err)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, to, gr.RollingTo())
	assert.Equal(t, 0, rollFinishedCalled[gr.IssueID()])

	// Insert into DB.
	current := recent.CurrentRoll()
	assert.Nil(t, current)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	current = recent.CurrentRoll()
	assert.NotNil(t, current)
	assert.Equal(t, current.Issue, roll.Issue)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, 0, rollFinishedCalled[gr.IssueID()])

	// Add a comment.
	msg := "Here's a comment"
	g.MockAddComment(roll, msg)
	assert.NoError(t, gr.AddComment(msg))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, 0, rollFinishedCalled[gr.IssueID()])

	// Set dry run.
	g.MockSetDryRun(roll, "Mode was changed to dry run")
	g.MockGetIssueProperties(roll)
	assert.NoError(t, gr.SwitchToDryRun(ctx))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, 0, rollFinishedCalled[gr.IssueID()])

	// Set normal.
	g.MockSetCQ(roll, "Mode was changed to normal")
	g.MockGetIssueProperties(roll)
	assert.NoError(t, gr.SwitchToNormal(ctx))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, 0, rollFinishedCalled[gr.IssueID()])

	// Update.
	roll.Status = gerrit.CHANGE_STATUS_MERGED
	g.MockGetIssueProperties(roll)
	assert.NoError(t, gr.Update(ctx))
	assert.True(t, gr.IsFinished())
	assert.True(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	assert.Nil(t, recent.CurrentRoll())
	assert.Equal(t, 1, rollFinishedCalled[gr.IssueID()])

	// Upload and retrieve another roll, dry run this time.
	sklog.Errorf("Dry run android roll")
	roll = makeFakeRoll(124, from, to, true, true)
	g.MockGetIssueProperties(roll)
	gr, err = newGerritAndroidRoll(ctx, g.Gerrit, fullHash, recent, roll.Issue, "http://issue/", rollFinished)
	assert.NoError(t, err)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, to, gr.RollingTo())
	assert.Equal(t, 0, rollFinishedCalled[gr.IssueID()])

	// Insert into DB.
	current = recent.CurrentRoll()
	assert.Nil(t, current)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	current = recent.CurrentRoll()
	assert.NotNil(t, current)
	assert.Equal(t, current.Issue, roll.Issue)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, 0, rollFinishedCalled[gr.IssueID()])

	// Success.
	roll.Labels[gerrit.PRESUBMIT_VERIFIED_LABEL] = &gerrit.LabelEntry{
		All: []*gerrit.LabelDetail{
			{
				Value: gerrit.PRESUBMIT_VERIFIED_LABEL_ACCEPTED,
			},
		},
	}
	g.MockGetIssueProperties(roll)
	assert.NoError(t, gr.Update(ctx))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.True(t, gr.IsDryRunFinished())
	assert.True(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, 1, rollFinishedCalled[gr.IssueID()])

	// Update again, ensure that we don't call the callback twice.
	roll.Labels[gerrit.COMMITQUEUE_LABEL] = &gerrit.LabelEntry{
		All: []*gerrit.LabelDetail{
			{
				Value: gerrit.PRESUBMIT_VERIFIED_LABEL_ACCEPTED,
			},
		},
	}
	g.MockGetIssueProperties(roll)
	assert.NoError(t, gr.Update(ctx))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.True(t, gr.IsDryRunFinished())
	assert.True(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, 1, rollFinishedCalled[gr.IssueID()])

	// Close for cleanup.
	roll.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(roll)
	assert.NoError(t, gr.Update(ctx))

	// Verify that all of the mutation functions handle a conflict (eg.
	// someone closed the CL) gracefully.
	sklog.Errorf("Android mutation functions")

	// 1. SwitchToDryRun.
	roll = makeFakeRoll(125, from, to, false, true)
	g.MockGetIssueProperties(roll)
	gr, err = newGerritAndroidRoll(ctx, g.Gerrit, fullHash, recent, roll.Issue, "http://issue/", nil)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	url, reqBytes := g.MakePostRequest(roll, "Mode was changed to dry run", map[string]int{
		gerrit.AUTOSUBMIT_LABEL: gerrit.AUTOSUBMIT_LABEL_NONE,
	})
	g.Mock.MockOnce(url, mockhttpclient.MockPostError("application/json", reqBytes, "CONFLICT", http.StatusConflict))
	roll.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(roll)
	assert.NoError(t, gr.SwitchToDryRun(ctx))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()

	// 2. SwitchToNormal
	roll = makeFakeRoll(126, from, to, false, true)
	g.MockGetIssueProperties(roll)
	gr, err = newGerritAndroidRoll(ctx, g.Gerrit, fullHash, recent, roll.Issue, "http://issue/", nil)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	url, reqBytes = g.MakePostRequest(roll, "Mode was changed to normal", map[string]int{
		gerrit.AUTOSUBMIT_LABEL: gerrit.AUTOSUBMIT_LABEL_SUBMIT,
	})
	g.Mock.MockOnce(url, mockhttpclient.MockPostError("application/json", reqBytes, "CONFLICT", http.StatusConflict))
	roll.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(roll)
	assert.NoError(t, gr.SwitchToNormal(ctx))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()

	// 3. Close.
	roll = makeFakeRoll(127, from, to, false, true)
	g.MockGetIssueProperties(roll)
	gr, err = newGerritAndroidRoll(ctx, g.Gerrit, fullHash, recent, roll.Issue, "http://issue/", nil)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	url = fmt.Sprintf("%s/a/changes/%d/abandon", gerrit_testutils.FAKE_GERRIT_URL, roll.Issue)
	req := testutils.MarshalJSON(t, &struct {
		Message string `json:"message"`
	}{
		Message: "close it!",
	})
	g.Mock.MockOnce(url, mockhttpclient.MockPostError("application/json", []byte(req), "CONFLICT", http.StatusConflict))
	roll.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(roll)
	assert.NoError(t, gr.Close(ctx, autoroll.ROLL_RESULT_FAILURE, "close it!"))
	assert.True(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()

	// Verify that we set the correct status when abandoning a CL.
	roll = makeFakeRoll(128, from, to, false, true)
	g.MockGetIssueProperties(roll)
	gr, err = newGerritAndroidRoll(ctx, g.Gerrit, fullHash, recent, roll.Issue, "http://issue/", nil)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	url = fmt.Sprintf("%s/a/changes/%d/abandon", gerrit_testutils.FAKE_GERRIT_URL, roll.Issue)
	req = testutils.MarshalJSON(t, &struct {
		Message string `json:"message"`
	}{
		Message: "close it!",
	})
	g.Mock.MockOnce(url, mockhttpclient.MockPostDialogue("application/json", []byte(req), nil))
	roll.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(roll)
	assert.NoError(t, gr.Close(ctx, autoroll.ROLL_RESULT_DRY_RUN_SUCCESS, "close it!"))
	g.AssertEmpty()
	issue, err := recent.Get(ctx, 128)
	assert.NoError(t, err)
	assert.Equal(t, issue.Result, autoroll.ROLL_RESULT_DRY_RUN_SUCCESS)
}

const (
	githubApiUrl = "https://api.github.com"
)

var (
	githubEmails = []string{"reviewer@chromium.org"}

	mockGithubUser      = "superman"
	mockGithubUserEmail = "superman@krypton.com"
	mockGithubRepo      = "krypton"

	githubConfig = &GithubConfig{
		RepoOwner:      mockGithubUser,
		RepoName:       mockGithubRepo,
		ChecksNum:      0,
		ChecksWaitFor:  []string{"my-check"},
		MergeMethodURL: "",
	}
)

func setupFakeGithub(t *testing.T) (*github.GitHub, *mockhttpclient.URLMock) {
	urlMock := mockhttpclient.NewURLMock()
	g, err := github.NewGitHub(context.Background(), "superman", "krypton", urlMock.Client())
	assert.NoError(t, err)
	return g, urlMock
}

func makeFakeGithubRoll(issue int, from, to string, dryRun bool) *github_api.PullRequest {
	commits := 1
	title := fmt.Sprintf("Roll fake %s..%s (N commits)", from, to)
	cqLabel := github.COMMIT_LABEL
	if dryRun {
		cqLabel = github.DRYRUN_LABEL
	}
	return &github_api.PullRequest{
		Commits: &commits,
		Head: &github_api.PullRequestBranch{
			SHA: &to,
		},
		Labels: []*github_api.Label{
			{
				Name: &cqLabel,
			},
		},
		Number: &issue,
		Title:  &title,
	}
}

func githubPullURLBase(pr *github_api.PullRequest) string {
	return githubApiUrl + fmt.Sprintf("/repos/%s/%s/pulls/%d", mockGithubUser, mockGithubRepo, *pr.Number)
}

func githubIssueURLBase(pr *github_api.PullRequest) string {
	return githubApiUrl + fmt.Sprintf("/repos/%s/%s/issues/%d", mockGithubUser, mockGithubRepo, *pr.Number)
}

func githubChecksURL(pr *github_api.PullRequest) string {
	return githubApiUrl + fmt.Sprintf("/repos/%s/%s/commits/%s/status", mockGithubUser, mockGithubRepo, *pr.Head.SHA)
}

func mockGetLabels(t *testing.T, urlMock *mockhttpclient.URLMock, pr *github_api.PullRequest) {
	resp, err := json.Marshal(pr)
	assert.NoError(t, err)
	urlMock.MockOnce(githubIssueURLBase(pr), mockhttpclient.MockGetDialogue(resp))
}

func mockGetGithubIssueProperties(t *testing.T, urlMock *mockhttpclient.URLMock, pr *github_api.PullRequest, checks []github_api.RepoStatus) {
	resp, err := json.Marshal(pr)
	assert.NoError(t, err)
	urlMock.MockOnce(githubPullURLBase(pr), mockhttpclient.MockGetDialogue(resp))

	mockGetLabels(t, urlMock, pr)

	combinedStatus := github_api.CombinedStatus{
		Statuses: checks,
	}
	resp, err = json.Marshal(combinedStatus)
	assert.NoError(t, err)
	urlMock.MockOnce(githubChecksURL(pr), mockhttpclient.MockGetDialogue(resp))
}

func mockAddGithubComment(urlMock *mockhttpclient.URLMock, pr *github_api.PullRequest, comment string) {
	reqType := "application/json"
	reqBody := []byte(fmt.Sprintf(`{"body":"%s"}
`, comment))
	md := mockhttpclient.MockPostDialogueWithResponseCode(reqType, reqBody, nil, http.StatusCreated)
	urlMock.MockOnce(githubIssueURLBase(pr)+"/comments", md)
}

func mockSetGithubDryRun(t *testing.T, urlMock *mockhttpclient.URLMock, pr *github_api.PullRequest, comment string) {
	mockGetLabels(t, urlMock, pr)
	reqType := "application/json"
	reqBody := []byte(fmt.Sprintf(`{"labels":["%s"]}
`, github.DRYRUN_LABEL))
	md := mockhttpclient.MockPatchDialogue(reqType, reqBody, nil)
	urlMock.MockOnce(githubIssueURLBase(pr), md)
	for _, label := range pr.Labels {
		if *label.Name == github.COMMIT_LABEL {
			*label.Name = github.DRYRUN_LABEL
		}
	}
}

func mockSetGithubCQ(t *testing.T, urlMock *mockhttpclient.URLMock, pr *github_api.PullRequest, comment string) {
	mockGetLabels(t, urlMock, pr)
	reqType := "application/json"
	reqBody := []byte(fmt.Sprintf(`{"labels":["%s"]}
`, github.COMMIT_LABEL))
	md := mockhttpclient.MockPatchDialogue(reqType, reqBody, nil)
	urlMock.MockOnce(githubIssueURLBase(pr), md)
	for _, label := range pr.Labels {
		if *label.Name == github.DRYRUN_LABEL {
			*label.Name = github.COMMIT_LABEL
		}
	}
}

func TestGithubRoll(t *testing.T) {
	testutils.LargeTest(t)

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	testutil.InitDatastore(t, ds.KIND_AUTOROLL_ROLL)

	g, urlMock := setupFakeGithub(t)

	ctx := context.Background()
	recent, err := recent_rolls.NewRecentRolls(ctx, "test-roller")
	assert.NoError(t, err)

	rollFinishedCalled := map[string]int{}
	rollFinished := func(ctx context.Context, roll RollImpl) error {
		issue := roll.IssueID()
		rollFinishedCalled[issue] = rollFinishedCalled[issue] + 1
		return nil
	}

	// Upload and retrieve the roll.
	sklog.Errorf("Normal github roll")
	from := "abcde12345abcde12345abcde12345abcde12345"
	to := "fghij67890fghij67890fghij67890fghij67890"
	fullHash := func(ctx context.Context, hash string) (string, error) {
		if strings.HasPrefix(from, hash) {
			return from, nil
		}
		if strings.HasPrefix(to, hash) {
			return to, nil
		}
		return "", errors.New("Unknown hash")
	}
	roll := makeFakeGithubRoll(123, from, to, false)
	checkId := int64(10123)
	checkState := github.CHECK_STATE_PENDING
	checkTargetURL := "http://blahblah"
	check := github_api.RepoStatus{
		ID:        &checkId,
		State:     &checkState,
		Context:   &githubConfig.ChecksWaitFor[0],
		TargetURL: &checkTargetURL,
	}
	checks := []github_api.RepoStatus{check}
	mockGetGithubIssueProperties(t, urlMock, roll, checks)
	gr, err := newGithubRoll(ctx, g, fullHash, recent, int64(*roll.Number), "http://issue/", githubConfig, rollFinished)
	assert.NoError(t, err)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	assert.True(t, urlMock.Empty())
	assert.Equal(t, to, gr.RollingTo())
	assert.Equal(t, 0, rollFinishedCalled[gr.IssueID()])

	// Insert into DB.
	current := recent.CurrentRoll()
	assert.Nil(t, current)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	current = recent.CurrentRoll()
	assert.NotNil(t, current)
	assert.Equal(t, current.Issue, int64(*roll.Number))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	assert.True(t, urlMock.Empty())
	assert.Equal(t, 0, rollFinishedCalled[gr.IssueID()])

	// Add a comment.
	msg := "Here's a comment"
	mockAddGithubComment(urlMock, roll, msg)
	assert.NoError(t, gr.AddComment(msg))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	assert.True(t, urlMock.Empty())
	assert.Equal(t, 0, rollFinishedCalled[gr.IssueID()])

	// Set dry run.
	mockSetGithubDryRun(t, urlMock, roll, "Mode was changed to dry run")
	mockGetGithubIssueProperties(t, urlMock, roll, checks)
	assert.NoError(t, gr.SwitchToDryRun(ctx))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	assert.True(t, urlMock.Empty())
	assert.Equal(t, 0, rollFinishedCalled[gr.IssueID()])

	// Set normal.
	mockSetGithubCQ(t, urlMock, roll, "Mode was changed to normal")
	mockGetGithubIssueProperties(t, urlMock, roll, checks)
	assert.NoError(t, gr.SwitchToNormal(ctx))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	assert.True(t, urlMock.Empty())
	assert.Equal(t, 0, rollFinishedCalled[gr.IssueID()])

	// Update.
	merged := true
	roll.Merged = &merged
	mockGetGithubIssueProperties(t, urlMock, roll, checks)
	assert.NoError(t, gr.Update(ctx))
	assert.True(t, gr.IsFinished())
	assert.True(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	assert.Nil(t, recent.CurrentRoll())
	assert.Equal(t, 1, rollFinishedCalled[gr.IssueID()])

	// Upload and retrieve another roll, dry run this time.
	sklog.Errorf("Dry run github roll")
	roll = makeFakeGithubRoll(124, from, to, true)
	mockGetGithubIssueProperties(t, urlMock, roll, checks)
	gr, err = newGithubRoll(ctx, g, fullHash, recent, int64(*roll.Number), "http://issue/", githubConfig, rollFinished)
	assert.NoError(t, err)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	assert.True(t, urlMock.Empty())
	assert.Equal(t, to, gr.RollingTo())
	assert.Equal(t, 0, rollFinishedCalled[gr.IssueID()])

	// Insert into DB.
	current = recent.CurrentRoll()
	assert.Nil(t, current)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	current = recent.CurrentRoll()
	assert.NotNil(t, current)
	assert.Equal(t, current.Issue, int64(*roll.Number))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	assert.True(t, urlMock.Empty())
	assert.Equal(t, 0, rollFinishedCalled[gr.IssueID()])

	// Success.
	/*roll.Labels[gerrit.PRESUBMIT_VERIFIED_LABEL] = &gerrit.LabelEntry{
		All: []*gerrit.LabelDetail{
			{
				Value: gerrit.PRESUBMIT_VERIFIED_LABEL_ACCEPTED,
			},
		},
	}*/
	mockGetGithubIssueProperties(t, urlMock, roll, checks)
	assert.NoError(t, gr.Update(ctx))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.True(t, gr.IsDryRunFinished())
	assert.True(t, gr.IsDryRunSuccess())
	assert.True(t, urlMock.Empty())
	assert.Equal(t, 1, rollFinishedCalled[gr.IssueID()])

	// Update again, ensure that we don't call the callback twice.
	/*roll.Labels[gerrit.COMMITQUEUE_LABEL] = &gerrit.LabelEntry{
		All: []*gerrit.LabelDetail{
			{
				Value: gerrit.PRESUBMIT_VERIFIED_LABEL_ACCEPTED,
			},
		},
	}*/
	mockGetGithubIssueProperties(t, urlMock, roll, checks)
	assert.NoError(t, gr.Update(ctx))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.True(t, gr.IsDryRunFinished())
	assert.True(t, gr.IsDryRunSuccess())
	assert.True(t, urlMock.Empty())
	assert.Equal(t, 1, rollFinishedCalled[gr.IssueID()])

	// Close for cleanup.
	// How?
	mockGetGithubIssueProperties(t, urlMock, roll, checks)
	assert.NoError(t, gr.Update(ctx))

	// Verify that all of the mutation functions handle a conflict (eg.
	// someone closed the CL) gracefully.
	sklog.Errorf("Github mutation functions")

	// 1. SwitchToDryRun.
	roll = makeFakeGithubRoll(125, from, to, false)
	mockGetGithubIssueProperties(t, urlMock, roll, checks)
	gr, err = newGithubRoll(ctx, g, fullHash, recent, int64(*roll.Number), "http://issue/", githubConfig, rollFinished)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	/*url, reqBytes := g.MakePostRequest(roll, "Mode was changed to dry run", map[string]int{
		gerrit.AUTOSUBMIT_LABEL: gerrit.AUTOSUBMIT_LABEL_NONE,
	})
	urlMock.MockOnce(url, mockhttpclient.MockPostError("application/json", reqBytes, "CONFLICT", http.StatusConflict))*/
	//roll.Status = gerrit.CHANGE_STATUS_ABANDONED
	mockGetGithubIssueProperties(t, urlMock, roll, checks)
	assert.NoError(t, gr.SwitchToDryRun(ctx))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	assert.True(t, urlMock.Empty())

	// 2. SwitchToNormal
	roll = makeFakeGithubRoll(126, from, to, false)
	mockGetGithubIssueProperties(t, urlMock, roll, checks)
	gr, err = newGithubRoll(ctx, g, fullHash, recent, int64(*roll.Number), "http://issue/", githubConfig, rollFinished)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	/*url, reqBytes = g.MakePostRequest(roll, "Mode was changed to normal", map[string]int{
		gerrit.AUTOSUBMIT_LABEL: gerrit.AUTOSUBMIT_LABEL_SUBMIT,
	})
	urlMock.MockOnce(url, mockhttpclient.MockPostError("application/json", reqBytes, "CONFLICT", http.StatusConflict))*/
	//roll.Status = gerrit.CHANGE_STATUS_ABANDONED
	mockGetGithubIssueProperties(t, urlMock, roll, checks)
	assert.NoError(t, gr.SwitchToNormal(ctx))
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	assert.True(t, urlMock.Empty())

	// 3. Close.
	roll = makeFakeGithubRoll(127, from, to, false)
	mockGetGithubIssueProperties(t, urlMock, roll, checks)
	gr, err = newGithubRoll(ctx, g, fullHash, recent, int64(*roll.Number), "http://issue/", githubConfig, rollFinished)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	/*url = fmt.Sprintf("%s/a/changes/%d/abandon", gerrit_testutils.FAKE_GERRIT_URL, roll.Issue)
	req := testutils.MarshalJSON(t, &struct {
		Message string `json:"message"`
	}{
		Message: "close it!",
	})
	urlMock.MockOnce(url, mockhttpclient.MockPostError("application/json", []byte(req), "CONFLICT", http.StatusConflict))*/
	//roll.Status = gerrit.CHANGE_STATUS_ABANDONED
	mockGetGithubIssueProperties(t, urlMock, roll, checks)
	assert.NoError(t, gr.Close(ctx, autoroll.ROLL_RESULT_FAILURE, "close it!"))
	assert.True(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	assert.True(t, urlMock.Empty())

	// Verify that we set the correct status when abandoning a CL.
	roll = makeFakeGithubRoll(128, from, to, false)
	mockGetGithubIssueProperties(t, urlMock, roll, checks)
	gr, err = newGithubRoll(ctx, g, fullHash, recent, int64(*roll.Number), "http://issue/", githubConfig, rollFinished)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	/*url = fmt.Sprintf("%s/a/changes/%d/abandon", gerrit_testutils.FAKE_GERRIT_URL, roll.Issue)
	req = testutils.MarshalJSON(t, &struct {
		Message string `json:"message"`
	}{
		Message: "close it!",
	})
	urlMock.MockOnce(url, mockhttpclient.MockPostDialogue("application/json", []byte(req), nil))*/
	//roll.Status = gerrit.CHANGE_STATUS_ABANDONED
	mockGetGithubIssueProperties(t, urlMock, roll, checks)
	assert.NoError(t, gr.Close(ctx, autoroll.ROLL_RESULT_DRY_RUN_SUCCESS, "close it!"))
	assert.True(t, urlMock.Empty())
	issue, err := recent.Get(ctx, 128)
	assert.NoError(t, err)
	assert.Equal(t, issue.Result, autoroll.ROLL_RESULT_DRY_RUN_SUCCESS)
}
