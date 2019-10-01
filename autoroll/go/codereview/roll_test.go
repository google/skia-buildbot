package codereview

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	assert "github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.skia.org/infra/autoroll/go/recent_rolls"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/gerrit"
	gerrit_testutils "go.skia.org/infra/go/gerrit/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func makeFakeRoll(issueNum int64, from, to string, dryRun, android bool) (*gerrit.ChangeInfo, *autoroll.AutoRollIssue) {
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
	if android {
		cqLabel = gerrit.AUTOSUBMIT_LABEL_SUBMIT
	}
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
						Value: cqLabel,
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
	return roll, &autoroll.AutoRollIssue{
		IsDryRun:    dryRun,
		Issue:       issueNum,
		RollingFrom: from,
		RollingTo:   to,
	}
}

func TestGerritRoll(t *testing.T) {
	unittest.LargeTest(t)

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	testutil.InitDatastore(t, ds.KIND_AUTOROLL_ROLL)

	g := gerrit_testutils.NewGerrit(t, tmp, false)
	ctx := context.Background()
	recent, err := recent_rolls.NewRecentRolls(ctx, "test-roller")
	assert.NoError(t, err)

	// Upload and retrieve the roll.
	from := "abcde12345abcde12345abcde12345abcde12345"
	to := "fghij67890fghij67890fghij67890fghij67890"
	toRev := &revision.Revision{
		Id:          to,
		Description: "rolling to fghi",
	}
	ci, issue := makeFakeRoll(123, from, to, false, false)
	g.MockGetIssueProperties(ci)
	g.MockGetTrybotResults(ci, 1, nil)
	gr, err := newGerritRoll(ctx, issue, g.Gerrit, recent, "http://issue/", toRev, nil)
	assert.NoError(t, err)
	assert.False(t, issue.IsDryRun)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, toRev, gr.RollingTo())

	// Insert into DB.
	current := recent.CurrentRoll()
	assert.Nil(t, current)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	current = recent.CurrentRoll()
	assert.NotNil(t, current)
	assert.Equal(t, current.Issue, ci.Issue)
	g.AssertEmpty()

	// Add a comment.
	msg := "Here's a comment"
	g.MockAddComment(ci, msg)
	assert.NoError(t, gr.AddComment(ctx, msg))
	g.AssertEmpty()
	assert.False(t, issue.IsDryRun)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())

	// Set dry run.
	g.MockSetDryRun(ci, "Mode was changed to dry run")
	ci.Labels[gerrit.COMMITQUEUE_LABEL] = &gerrit.LabelEntry{
		All: []*gerrit.LabelDetail{
			{
				Value: gerrit.COMMITQUEUE_LABEL_DRY_RUN,
			},
		},
	}
	g.MockGetIssueProperties(ci)
	g.MockGetTrybotResults(ci, 1, nil)
	assert.NoError(t, gr.SwitchToDryRun(ctx))
	g.AssertEmpty()
	assert.True(t, issue.IsDryRun)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())

	// Set normal.
	g.MockSetCQ(ci, "Mode was changed to normal")
	ci.Labels[gerrit.COMMITQUEUE_LABEL] = &gerrit.LabelEntry{
		All: []*gerrit.LabelDetail{
			{
				Value: gerrit.COMMITQUEUE_LABEL_SUBMIT,
			},
		},
	}
	g.MockGetIssueProperties(ci)
	g.MockGetTrybotResults(ci, 1, nil)
	assert.NoError(t, gr.SwitchToNormal(ctx))
	g.AssertEmpty()
	assert.False(t, issue.IsDryRun)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())

	// Update.
	ci.Status = gerrit.CHANGE_STATUS_MERGED
	// Landing a change adds an empty patchset.
	rev := &gerrit.Revision{
		Number:  int64(len(ci.Revisions) + 1),
		Created: time.Now(),
		Kind:    "",
	}
	ci.Revisions[fmt.Sprintf("%d", rev.Number)] = rev
	ci.Patchsets = append(ci.Patchsets, rev)
	g.MockGetIssueProperties(ci)
	g.MockGetTrybotResults(ci, 1, nil)
	assert.NoError(t, gr.Update(ctx))
	assert.False(t, issue.IsDryRun)
	assert.True(t, gr.IsFinished())
	assert.True(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	assert.Nil(t, recent.CurrentRoll())

	// Upload and retrieve another roll, dry run this time.
	ts, err := ptypes.TimestampProto(time.Now().UTC().Round(time.Millisecond))
	assert.NoError(t, err)
	ci, issue = makeFakeRoll(124, from, to, true, false)
	g.MockGetIssueProperties(ci)
	tryjob := &buildbucketpb.Build{
		Builder: &buildbucketpb.BuilderID{
			Project: "skia",
			Bucket:  "fake",
			Builder: "fake-builder",
		},
		Id:         99999,
		CreateTime: ts,
		Status:     buildbucketpb.Status_STARTED,
		Tags: []*buildbucketpb.StringPair{
			{
				Key:   "user_agent",
				Value: "cq",
			},
			{
				Key:   "cq_experimental",
				Value: "false",
			},
		},
	}
	g.MockGetTrybotResults(ci, 1, []*buildbucketpb.Build{tryjob})
	gr, err = newGerritRoll(ctx, issue, g.Gerrit, recent, "http://issue/", toRev, nil)
	assert.NoError(t, err)
	assert.True(t, issue.IsDryRun)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, toRev, gr.RollingTo())

	// Insert into DB.
	current = recent.CurrentRoll()
	assert.Nil(t, current)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	current = recent.CurrentRoll()
	assert.NotNil(t, current)
	assert.Equal(t, current.Issue, ci.Issue)
	g.AssertEmpty()

	// Success.
	tryjob.Status = buildbucketpb.Status_SUCCESS
	ci.Labels[gerrit.COMMITQUEUE_LABEL] = &gerrit.LabelEntry{
		All: []*gerrit.LabelDetail{
			{
				Value: gerrit.COMMITQUEUE_LABEL_NONE,
			},
		},
	}
	g.MockGetIssueProperties(ci)
	g.MockGetTrybotResults(ci, 1, []*buildbucketpb.Build{tryjob})
	assert.NoError(t, gr.Update(ctx))
	assert.True(t, issue.IsDryRun)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.True(t, gr.IsDryRunFinished())
	assert.True(t, gr.IsDryRunSuccess())
	g.AssertEmpty()

	// Close for cleanup.
	ci.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(ci)
	g.MockGetTrybotResults(ci, 1, []*buildbucketpb.Build{tryjob})
	assert.NoError(t, gr.Update(ctx))

	// Verify that all of the mutation functions handle a conflict (eg.
	// someone closed the CL) gracefully.

	// 1. SwitchToDryRun.
	ci, issue = makeFakeRoll(125, from, to, false, false)
	g.MockGetIssueProperties(ci)
	g.MockGetTrybotResults(ci, 1, nil)
	gr, err = newGerritRoll(ctx, issue, g.Gerrit, recent, "http://issue/", toRev, nil)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	url, reqBytes := g.MakePostRequest(ci, "Mode was changed to dry run", map[string]int{
		gerrit.COMMITQUEUE_LABEL: gerrit.COMMITQUEUE_LABEL_DRY_RUN,
	})
	g.Mock.MockOnce(url, mockhttpclient.MockPostError("application/json", reqBytes, "CONFLICT", http.StatusConflict))
	ci.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(ci)
	g.MockGetTrybotResults(ci, 1, nil)
	assert.NoError(t, gr.SwitchToDryRun(ctx))
	g.AssertEmpty()

	// 2. SwitchToNormal
	ci, issue = makeFakeRoll(126, from, to, false, false)
	g.MockGetIssueProperties(ci)
	g.MockGetTrybotResults(ci, 1, nil)
	gr, err = newGerritRoll(ctx, issue, g.Gerrit, recent, "http://issue/", toRev, nil)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	url, reqBytes = g.MakePostRequest(ci, "Mode was changed to normal", map[string]int{
		gerrit.COMMITQUEUE_LABEL: gerrit.COMMITQUEUE_LABEL_SUBMIT,
	})
	g.Mock.MockOnce(url, mockhttpclient.MockPostError("application/json", reqBytes, "CONFLICT", http.StatusConflict))
	ci.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(ci)
	g.MockGetTrybotResults(ci, 1, nil)
	assert.NoError(t, gr.SwitchToNormal(ctx))
	g.AssertEmpty()

	// 3. Close.
	ci, issue = makeFakeRoll(127, from, to, false, false)
	g.MockGetIssueProperties(ci)
	g.MockGetTrybotResults(ci, 1, nil)
	gr, err = newGerritRoll(ctx, issue, g.Gerrit, recent, "http://issue/", toRev, nil)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	url = fmt.Sprintf("%s/a/changes/%d/abandon", gerrit_testutils.FAKE_GERRIT_URL, ci.Issue)
	req := testutils.MarshalJSON(t, &struct {
		Message string `json:"message"`
	}{
		Message: "close it!",
	})
	g.Mock.MockOnce(url, mockhttpclient.MockPostError("application/json", []byte(req), "CONFLICT", http.StatusConflict))
	ci.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(ci)
	g.MockGetTrybotResults(ci, 1, nil)
	assert.NoError(t, gr.Close(ctx, autoroll.ROLL_RESULT_FAILURE, "close it!"))
	g.AssertEmpty()

	// Verify that we set the correct status when abandoning a CL.
	ci, issue = makeFakeRoll(128, from, to, false, false)
	g.MockGetIssueProperties(ci)
	g.MockGetTrybotResults(ci, 1, nil)
	gr, err = newGerritRoll(ctx, issue, g.Gerrit, recent, "http://issue/", toRev, nil)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	url = fmt.Sprintf("%s/a/changes/%d/abandon", gerrit_testutils.FAKE_GERRIT_URL, ci.Issue)
	req = testutils.MarshalJSON(t, &struct {
		Message string `json:"message"`
	}{
		Message: "close it!",
	})
	g.Mock.MockOnce(url, mockhttpclient.MockPostDialogue("application/json", []byte(req), nil))
	ci.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(ci)
	g.MockGetTrybotResults(ci, 1, nil)
	assert.NoError(t, gr.Close(ctx, autoroll.ROLL_RESULT_DRY_RUN_SUCCESS, "close it!"))
	g.AssertEmpty()
	issue, err = recent.Get(ctx, 128)
	assert.NoError(t, err)
	assert.Equal(t, issue.Result, autoroll.ROLL_RESULT_DRY_RUN_SUCCESS)
}

func TestGerritAndroidRoll(t *testing.T) {
	unittest.LargeTest(t)

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	testutil.InitDatastore(t, ds.KIND_AUTOROLL_ROLL)

	g := gerrit_testutils.NewGerrit(t, tmp, true)

	ctx := context.Background()
	recent, err := recent_rolls.NewRecentRolls(ctx, "test-roller")
	assert.NoError(t, err)

	// Upload and retrieve the roll.
	from := "abcde12345abcde12345abcde12345abcde12345"
	to := "fghij67890fghij67890fghij67890fghij67890"
	toRev := &revision.Revision{
		Id:          to,
		Description: "rolling to fghi",
	}
	ci, issue := makeFakeRoll(123, from, to, false, true)
	g.MockGetIssueProperties(ci)
	gr, err := newGerritAndroidRoll(ctx, issue, g.Gerrit, recent, "http://issue/", toRev, nil)
	assert.NoError(t, err)
	assert.False(t, issue.IsDryRun)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, toRev, gr.RollingTo())

	// Insert into DB.
	current := recent.CurrentRoll()
	assert.Nil(t, current)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	current = recent.CurrentRoll()
	assert.NotNil(t, current)
	assert.Equal(t, current.Issue, ci.Issue)
	g.AssertEmpty()

	// Add a comment.
	msg := "Here's a comment"
	g.MockAddComment(ci, msg)
	assert.NoError(t, gr.AddComment(ctx, msg))
	g.AssertEmpty()
	assert.False(t, issue.IsDryRun)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())

	// Set dry run.
	g.MockSetDryRun(ci, "Mode was changed to dry run")
	g.MockGetIssueProperties(ci)
	assert.NoError(t, gr.SwitchToDryRun(ctx))
	g.AssertEmpty()
	assert.True(t, issue.IsDryRun)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())

	// Set normal.
	g.MockSetCQ(ci, "Mode was changed to normal")
	g.MockGetIssueProperties(ci)
	assert.NoError(t, gr.SwitchToNormal(ctx))
	g.AssertEmpty()
	assert.False(t, issue.IsDryRun)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())

	// Update.
	ci.Status = gerrit.CHANGE_STATUS_MERGED
	g.MockGetIssueProperties(ci)
	assert.NoError(t, gr.Update(ctx))
	assert.False(t, issue.IsDryRun)
	assert.True(t, gr.IsFinished())
	assert.True(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	assert.Nil(t, recent.CurrentRoll())

	// Upload and retrieve another roll, dry run this time.
	ci, issue = makeFakeRoll(124, from, to, true, true)
	g.MockGetIssueProperties(ci)
	gr, err = newGerritAndroidRoll(ctx, issue, g.Gerrit, recent, "http://issue/", toRev, nil)
	assert.NoError(t, err)
	assert.True(t, issue.IsDryRun)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, toRev, gr.RollingTo())

	// Insert into DB.
	current = recent.CurrentRoll()
	assert.Nil(t, current)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	current = recent.CurrentRoll()
	assert.NotNil(t, current)
	assert.Equal(t, current.Issue, ci.Issue)
	g.AssertEmpty()

	// Success.
	ci.Labels[gerrit.PRESUBMIT_VERIFIED_LABEL] = &gerrit.LabelEntry{
		All: []*gerrit.LabelDetail{
			{
				Value: gerrit.PRESUBMIT_VERIFIED_LABEL_ACCEPTED,
			},
		},
	}
	g.MockGetIssueProperties(ci)
	assert.NoError(t, gr.Update(ctx))
	assert.True(t, issue.IsDryRun)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	assert.True(t, gr.IsDryRunFinished())
	assert.True(t, gr.IsDryRunSuccess())
	g.AssertEmpty()

	// Close for cleanup.
	ci.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(ci)
	assert.NoError(t, gr.Update(ctx))

	// Verify that all of the mutation functions handle a conflict (eg.
	// someone closed the CL) gracefully.

	// 1. SwitchToDryRun.
	ci, issue = makeFakeRoll(125, from, to, false, true)
	g.MockGetIssueProperties(ci)
	gr, err = newGerritAndroidRoll(ctx, issue, g.Gerrit, recent, "http://issue/", toRev, nil)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	url, reqBytes := g.MakePostRequest(ci, "Mode was changed to dry run", map[string]int{
		gerrit.AUTOSUBMIT_LABEL: gerrit.AUTOSUBMIT_LABEL_NONE,
	})
	g.Mock.MockOnce(url, mockhttpclient.MockPostError("application/json", reqBytes, "CONFLICT", http.StatusConflict))
	ci.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(ci)
	assert.NoError(t, gr.SwitchToDryRun(ctx))
	g.AssertEmpty()

	// 2. SwitchToNormal
	ci, issue = makeFakeRoll(126, from, to, false, true)
	g.MockGetIssueProperties(ci)
	gr, err = newGerritAndroidRoll(ctx, issue, g.Gerrit, recent, "http://issue/", toRev, nil)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	url, reqBytes = g.MakePostRequest(ci, "Mode was changed to normal", map[string]int{
		gerrit.AUTOSUBMIT_LABEL: gerrit.AUTOSUBMIT_LABEL_SUBMIT,
	})
	g.Mock.MockOnce(url, mockhttpclient.MockPostError("application/json", reqBytes, "CONFLICT", http.StatusConflict))
	ci.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(ci)
	assert.NoError(t, gr.SwitchToNormal(ctx))
	g.AssertEmpty()

	// 3. Close.
	ci, issue = makeFakeRoll(127, from, to, false, true)
	g.MockGetIssueProperties(ci)
	gr, err = newGerritAndroidRoll(ctx, issue, g.Gerrit, recent, "http://issue/", toRev, nil)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	url = fmt.Sprintf("%s/a/changes/%d/abandon", gerrit_testutils.FAKE_GERRIT_URL, ci.Issue)
	req := testutils.MarshalJSON(t, &struct {
		Message string `json:"message"`
	}{
		Message: "close it!",
	})
	g.Mock.MockOnce(url, mockhttpclient.MockPostError("application/json", []byte(req), "CONFLICT", http.StatusConflict))
	ci.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(ci)
	assert.NoError(t, gr.Close(ctx, autoroll.ROLL_RESULT_FAILURE, "close it!"))
	g.AssertEmpty()

	// Verify that we set the correct status when abandoning a CL.
	ci, issue = makeFakeRoll(128, from, to, false, true)
	g.MockGetIssueProperties(ci)
	gr, err = newGerritAndroidRoll(ctx, issue, g.Gerrit, recent, "http://issue/", toRev, nil)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	url = fmt.Sprintf("%s/a/changes/%d/abandon", gerrit_testutils.FAKE_GERRIT_URL, ci.Issue)
	req = testutils.MarshalJSON(t, &struct {
		Message string `json:"message"`
	}{
		Message: "close it!",
	})
	g.Mock.MockOnce(url, mockhttpclient.MockPostDialogue("application/json", []byte(req), nil))
	ci.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(ci)
	assert.NoError(t, gr.Close(ctx, autoroll.ROLL_RESULT_DRY_RUN_SUCCESS, "close it!"))
	g.AssertEmpty()
	issue, err = recent.Get(ctx, 128)
	assert.NoError(t, err)
	assert.Equal(t, issue.Result, autoroll.ROLL_RESULT_DRY_RUN_SUCCESS)
}
