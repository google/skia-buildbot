package roller

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/recent_rolls"
	repo_manager_testutils "go.skia.org/infra/autoroll/go/repo_manager/testutils"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/gerrit"
	gerrit_testutils "go.skia.org/infra/go/gerrit/testutils"
	"go.skia.org/infra/go/jsonutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
)

func TestGerritRoll(t *testing.T) {
	testutils.LargeTest(t)

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	testutil.InitDatastore(t, ds.KIND_AUTOROLL_ROLL)

	g := gerrit_testutils.NewGerrit(t, tmp, false)
	rm := repo_manager_testutils.NewRepoManager(t, false)
	ctx := context.Background()
	recent, err := recent_rolls.NewRecentRolls(ctx, "test-roller", path.Join(tmp, "recent.bdb"))
	assert.NoError(t, err)

	// Upload and retrieve the roll.
	from := "abcde12345abcde12345abcde12345abcde12345"
	to := "fghij67890fghij67890fghij67890fghij67890"
	rm.MockFullChildHash(from[:12], from)
	rm.MockFullChildHash(to[:12], to)
	roll := rm.RollerWillUpload(123, from, to, false)
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, nil)
	gr, err := newGerritRoll(ctx, g.Gerrit, rm, recent, 123, nil)
	assert.NoError(t, err)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	g.AssertEmpty()
	assert.Equal(t, to, gr.RollingTo())

	// Insert into DB.
	current := recent.CurrentRoll()
	assert.Nil(t, current)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	current = recent.CurrentRoll()
	assert.NotNil(t, current)
	assert.Equal(t, current.Issue, roll.Issue)
	g.AssertEmpty()

	// Add a comment.
	msg := "Here's a comment"
	g.MockAddComment(roll, msg)
	assert.NoError(t, gr.AddComment(msg))
	g.AssertEmpty()

	// Set dry run.
	g.MockSetDryRun(roll, "Mode was changed to dry run")
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, nil)
	assert.NoError(t, gr.SwitchToDryRun(ctx))
	g.AssertEmpty()

	// Set normal.
	g.MockSetCQ(roll, "Mode was changed to normal")
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, nil)
	assert.NoError(t, gr.SwitchToNormal(ctx))
	g.AssertEmpty()

	// Update.
	roll.Status = gerrit.CHANGE_STATUS_MERGED
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, nil)
	assert.NoError(t, gr.Update(ctx))
	assert.True(t, gr.IsFinished())
	assert.True(t, gr.IsSuccess())
	assert.Nil(t, recent.CurrentRoll())

	// Upload and retrieve another roll, dry run this time.
	roll = rm.RollerWillUpload(124, from, to, true)
	g.MockGetIssueProperties(roll)
	tryjob := &buildbucket.Build{
		Created:        jsonutils.Time(time.Now().UTC().Round(time.Millisecond)),
		Status:         autoroll.TRYBOT_STATUS_STARTED,
		ParametersJson: "{\"builder_name\":\"fake-builder\",\"properties\":{\"category\":\"cq\"}}",
	}
	g.MockGetTrybotResults(roll, []*buildbucket.Build{tryjob})
	gr, err = newGerritRoll(ctx, g.Gerrit, rm, recent, 124, nil)
	assert.NoError(t, err)
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, to, gr.RollingTo())

	// Insert into DB.
	current = recent.CurrentRoll()
	assert.Nil(t, current)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	current = recent.CurrentRoll()
	assert.NotNil(t, current)
	assert.Equal(t, current.Issue, roll.Issue)
	g.AssertEmpty()

	// Success.
	tryjob.Status = autoroll.TRYBOT_STATUS_COMPLETED
	tryjob.Result = autoroll.TRYBOT_RESULT_SUCCESS
	roll.Labels[gerrit.COMMITQUEUE_LABEL] = &gerrit.LabelEntry{
		All: []*gerrit.LabelDetail{
			&gerrit.LabelDetail{
				Value: gerrit.COMMITQUEUE_LABEL_NONE,
			},
		},
	}
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, []*buildbucket.Build{tryjob})
	assert.NoError(t, gr.Update(ctx))
	assert.True(t, gr.IsDryRunFinished())
	assert.True(t, gr.IsDryRunSuccess())
	g.AssertEmpty()

	// Close for cleanup.
	roll.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, []*buildbucket.Build{tryjob})
	assert.NoError(t, gr.Update(ctx))

	// Verify that all of the mutation functions handle a conflict (eg.
	// someone closed the CL) gracefully.

	// 1. SwitchToDryRun.
	roll = rm.RollerWillUpload(125, from, to, false)
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, nil)
	gr, err = newGerritRoll(ctx, g.Gerrit, rm, recent, 125, nil)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	url, reqBytes := g.MakePostRequest(roll, "Mode was changed to dry run", map[string]int{
		gerrit.COMMITQUEUE_LABEL: gerrit.COMMITQUEUE_LABEL_DRY_RUN,
	})
	g.Mock.MockOnce(url, mockhttpclient.MockPostError("application/json", reqBytes, "CONFLICT", http.StatusConflict))
	roll.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, nil)
	assert.NoError(t, gr.SwitchToDryRun(ctx))
	g.AssertEmpty()

	// 2. SwitchToNormal
	roll = rm.RollerWillUpload(126, from, to, false)
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, nil)
	gr, err = newGerritRoll(ctx, g.Gerrit, rm, recent, 126, nil)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	url, reqBytes = g.MakePostRequest(roll, "Mode was changed to normal", map[string]int{
		gerrit.COMMITQUEUE_LABEL: gerrit.COMMITQUEUE_LABEL_SUBMIT,
	})
	g.Mock.MockOnce(url, mockhttpclient.MockPostError("application/json", reqBytes, "CONFLICT", http.StatusConflict))
	roll.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, nil)
	assert.NoError(t, gr.SwitchToNormal(ctx))
	g.AssertEmpty()

	// 3. Close.
	roll = rm.RollerWillUpload(127, from, to, false)
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, nil)
	gr, err = newGerritRoll(ctx, g.Gerrit, rm, recent, 127, nil)
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
	g.MockGetTrybotResults(roll, nil)
	assert.NoError(t, gr.Close(ctx, autoroll.ROLL_RESULT_FAILURE, "close it!"))
	g.AssertEmpty()

	// Verify that we set the correct status when abandoning a CL.
	roll = rm.RollerWillUpload(128, from, to, true)
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, nil)
	gr, err = newGerritRoll(ctx, g.Gerrit, rm, recent, 128, nil)
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
	g.MockGetTrybotResults(roll, nil)
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
	rm := repo_manager_testutils.NewRepoManager(t, true)

	ctx := context.Background()
	recent, err := recent_rolls.NewRecentRolls(ctx, "test-roller", path.Join(tmp, "recent.bdb"))
	assert.NoError(t, err)

	// Upload and retrieve the roll.
	from := "abcde12345abcde12345abcde12345abcde12345"
	to := "fghij67890fghij67890fghij67890fghij67890"
	rm.MockFullChildHash(from[:12], from)
	rm.MockFullChildHash(to[:12], to)
	roll := rm.RollerWillUpload(123, from, to, false)
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, nil)
	gr, err := newGerritAndroidRoll(ctx, g.Gerrit, rm, recent, 123, nil)
	assert.NoError(t, err)
	assert.False(t, gr.IsFinished())
	assert.False(t, gr.IsSuccess())
	g.AssertEmpty()
	assert.Equal(t, to, gr.RollingTo())

	// Insert into DB.
	current := recent.CurrentRoll()
	assert.Nil(t, current)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	current = recent.CurrentRoll()
	assert.NotNil(t, current)
	assert.Equal(t, current.Issue, roll.Issue)
	g.AssertEmpty()

	// Add a comment.
	msg := "Here's a comment"
	g.MockAddComment(roll, msg)
	assert.NoError(t, gr.AddComment(msg))
	g.AssertEmpty()

	// Set dry run.
	g.MockSetDryRun(roll, "Mode was changed to dry run")
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, nil)
	assert.NoError(t, gr.SwitchToDryRun(ctx))
	g.AssertEmpty()

	// Set normal.
	g.MockSetCQ(roll, "Mode was changed to normal")
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, nil)
	assert.NoError(t, gr.SwitchToNormal(ctx))
	g.AssertEmpty()

	// Update.
	roll.Status = gerrit.CHANGE_STATUS_MERGED
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, nil)
	assert.NoError(t, gr.Update(ctx))
	assert.True(t, gr.IsFinished())
	assert.True(t, gr.IsSuccess())
	assert.Nil(t, recent.CurrentRoll())

	// Upload and retrieve another roll, dry run this time.
	roll = rm.RollerWillUpload(124, from, to, true)
	g.MockGetIssueProperties(roll)
	tryjob := &buildbucket.Build{
		Created:        jsonutils.Time(time.Now().UTC().Round(time.Millisecond)),
		Status:         autoroll.TRYBOT_STATUS_STARTED,
		ParametersJson: "{\"builder_name\":\"fake-builder\",\"properties\":{\"category\":\"cq\"}}",
	}
	g.MockGetTrybotResults(roll, []*buildbucket.Build{tryjob})
	gr, err = newGerritAndroidRoll(ctx, g.Gerrit, rm, recent, 124, nil)
	assert.NoError(t, err)
	assert.False(t, gr.IsDryRunFinished())
	assert.False(t, gr.IsDryRunSuccess())
	g.AssertEmpty()
	assert.Equal(t, to, gr.RollingTo())

	// Insert into DB.
	current = recent.CurrentRoll()
	assert.Nil(t, current)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	current = recent.CurrentRoll()
	assert.NotNil(t, current)
	assert.Equal(t, current.Issue, roll.Issue)
	g.AssertEmpty()

	// Success.
	tryjob.Status = autoroll.TRYBOT_STATUS_COMPLETED
	tryjob.Result = autoroll.TRYBOT_RESULT_SUCCESS
	roll.Labels[gerrit.PRESUBMIT_VERIFIED_LABEL] = &gerrit.LabelEntry{
		All: []*gerrit.LabelDetail{
			&gerrit.LabelDetail{
				Value: gerrit.PRESUBMIT_VERIFIED_LABEL_ACCEPTED,
			},
		},
	}
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, []*buildbucket.Build{tryjob})
	assert.NoError(t, gr.Update(ctx))
	assert.True(t, gr.IsDryRunFinished())
	assert.True(t, gr.IsDryRunSuccess())
	g.AssertEmpty()

	// Close for cleanup.
	roll.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, []*buildbucket.Build{tryjob})
	assert.NoError(t, gr.Update(ctx))

	// Verify that all of the mutation functions handle a conflict (eg.
	// someone closed the CL) gracefully.

	// 1. SwitchToDryRun.
	roll = rm.RollerWillUpload(125, from, to, true)
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, nil)
	gr, err = newGerritAndroidRoll(ctx, g.Gerrit, rm, recent, 125, nil)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	url, reqBytes := g.MakePostRequest(roll, "Mode was changed to dry run", map[string]int{
		gerrit.AUTOSUBMIT_LABEL: gerrit.AUTOSUBMIT_LABEL_NONE,
	})
	g.Mock.MockOnce(url, mockhttpclient.MockPostError("application/json", reqBytes, "CONFLICT", http.StatusConflict))
	roll.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, nil)
	assert.NoError(t, gr.SwitchToDryRun(ctx))
	g.AssertEmpty()

	// 2. SwitchToNormal
	roll = rm.RollerWillUpload(126, from, to, true)
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, nil)
	gr, err = newGerritAndroidRoll(ctx, g.Gerrit, rm, recent, 126, nil)
	assert.NoError(t, err)
	assert.NoError(t, gr.InsertIntoDB(ctx))
	url, reqBytes = g.MakePostRequest(roll, "Mode was changed to normal", map[string]int{
		gerrit.AUTOSUBMIT_LABEL: gerrit.AUTOSUBMIT_LABEL_SUBMIT,
	})
	g.Mock.MockOnce(url, mockhttpclient.MockPostError("application/json", reqBytes, "CONFLICT", http.StatusConflict))
	roll.Status = gerrit.CHANGE_STATUS_ABANDONED
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, nil)
	assert.NoError(t, gr.SwitchToNormal(ctx))
	g.AssertEmpty()

	// 3. Close.
	roll = rm.RollerWillUpload(127, from, to, true)
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, nil)
	gr, err = newGerritAndroidRoll(ctx, g.Gerrit, rm, recent, 127, nil)
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
	g.MockGetTrybotResults(roll, nil)
	assert.NoError(t, gr.Close(ctx, autoroll.ROLL_RESULT_FAILURE, "close it!"))
	g.AssertEmpty()

	// Verify that we set the correct status when abandoning a CL.
	roll = rm.RollerWillUpload(128, from, to, true)
	g.MockGetIssueProperties(roll)
	g.MockGetTrybotResults(roll, nil)
	gr, err = newGerritAndroidRoll(ctx, g.Gerrit, rm, recent, 128, nil)
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
	g.MockGetTrybotResults(roll, nil)
	assert.NoError(t, gr.Close(ctx, autoroll.ROLL_RESULT_DRY_RUN_SUCCESS, "close it!"))
	g.AssertEmpty()
	issue, err := recent.Get(ctx, 128)
	assert.NoError(t, err)
	assert.Equal(t, issue.Result, autoroll.ROLL_RESULT_DRY_RUN_SUCCESS)
}
