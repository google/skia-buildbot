package gerrit

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	// Flip this boolean to run the below E2E Gerrit tests. Requires a valid
	// ~/.gitcookies file.
	RUN_GERRIT_TESTS = false
)

func skipTestIfRequired(t *testing.T) {
	if !RUN_GERRIT_TESTS {
		t.Skip("Skipping test due to RUN_GERRIT_TESTS=false")
	}
	unittest.LargeTest(t)
}

func TestHasOpenDependency(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerrit(GERRIT_SKIA_URL, DefaultGitCookiesPath(), nil)
	assert.NoError(t, err)

	dep, err := api.HasOpenDependency(context.TODO(), 52160, 1)
	assert.NoError(t, err)
	assert.False(t, dep)

	dep2, err := api.HasOpenDependency(context.TODO(), 52123, 1)
	assert.NoError(t, err)
	assert.True(t, dep2)
}

func TestGerritOwnerModifiedSearch(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerrit(GERRIT_SKIA_URL, DefaultGitCookiesPath(), nil)
	assert.NoError(t, err)

	t_delta := time.Now().Add(-10 * 24 * time.Hour)
	issues, err := api.Search(context.TODO(), 1, SearchModifiedAfter(t_delta), SearchOwner("rmistry@google.com"))
	assert.NoError(t, err)
	assert.True(t, len(issues) > 0)

	for _, issue := range issues {
		details, err := api.GetIssueProperties(context.TODO(), issue.Issue)
		assert.NoError(t, err)
		assert.True(t, details.Updated.After(t_delta))
		assert.True(t, len(details.Patchsets) > 0)
		assert.Equal(t, "rmistry@google.com", details.Owner.Email)
	}

	issues, err = api.Search(context.TODO(), 2, SearchModifiedAfter(time.Now().Add(-time.Hour)))
	assert.NoError(t, err)
}

func TestGerritCommitSearch(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerrit(GERRIT_SKIA_URL, DefaultGitCookiesPath(), nil)
	assert.NoError(t, err)
	api.TurnOnAuthenticatedGets()

	issues, err := api.Search(context.TODO(), 1, SearchCommit("a2eb235a16ed430896cc54989e683cf930319eb7"))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(issues))

	for _, issue := range issues {
		details, err := api.GetIssueProperties(context.TODO(), issue.Issue)
		assert.NoError(t, err)
		assert.Equal(t, 5, len(details.Patchsets))
		assert.Equal(t, "rmistry@google.com", details.Owner.Email)
		assert.Equal(t, "I37876c6f62c85d0532b22dcf8bea8b4e7f4147c0", details.ChangeId)
		assert.True(t, details.Committed)
		assert.Equal(t, "Skia Gerrit 10k!", details.Subject)
	}
}

// It is a little different to test the poller. Enable this to turn on the
// poller and test removing issues from it.
func GerritPollerTest(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerrit(GERRIT_SKIA_URL, DefaultGitCookiesPath(), nil)
	assert.NoError(t, err)

	cache := NewCodeReviewCache(api, 10*time.Second, 3)
	fmt.Println(cache.Get(2386))
	c1 := ChangeInfo{
		Issue: 2386,
	}
	cache.Add(2386, &c1)
	fmt.Println(cache.Get(2386))
	time.Sleep(time.Hour)
}

func TestGetPatch(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerrit(GERRIT_SKIA_URL, DefaultGitCookiesPath(), nil)
	assert.NoError(t, err)

	patch, err := api.GetPatch(context.TODO(), 2370, "current")
	assert.NoError(t, err)

	// Note: The trailing spaces and newlines were added this way
	// because editor plug-ins remove white spaces from the raw string.
	expected := `

diff --git a/whitespace.txt b/whitespace.txt
index c0f0a49..d5733b3 100644
--- a/whitespace.txt
+++ b/whitespace.txt
@@ -1,4 +1,5 @@
 testing
+` + "\n  \n \n \n"

	assert.Equal(t, expected, patch)
}

func TestAddComment(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerrit(GERRIT_SKIA_URL, DefaultGitCookiesPath(), nil)
	assert.NoError(t, err)

	changeInfo, err := api.GetIssueProperties(context.TODO(), 2370)
	assert.NoError(t, err)
	err = api.AddComment(context.TODO(), changeInfo, "Testing API!!")
	assert.NoError(t, err)
}

func TestSendToDryRun(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerrit(GERRIT_SKIA_URL, DefaultGitCookiesPath(), nil)
	assert.NoError(t, err)

	// Send to dry run.
	changeInfo, err := api.GetIssueProperties(context.TODO(), 2370)
	assert.NoError(t, err)
	defaultLabelValue := changeInfo.Labels[COMMITQUEUE_LABEL].DefaultValue
	err = api.SendToDryRun(context.TODO(), changeInfo, "Sending to dry run")
	assert.NoError(t, err)

	// Wait for a second for the above to take place.
	time.Sleep(time.Second)

	// Verify that the change was sent to dry run.
	changeInfo, err = api.GetIssueProperties(context.TODO(), 2370)
	assert.NoError(t, err)
	assert.Equal(t, 1, changeInfo.Labels[COMMITQUEUE_LABEL].All[0].Value)

	// Remove from dry run.
	err = api.RemoveFromCQ(context.TODO(), changeInfo, "")
	assert.NoError(t, err)

	// Verify that the change was removed from dry run.
	changeInfo, err = api.GetIssueProperties(context.TODO(), 2370)
	assert.NoError(t, err)
	assert.Equal(t, defaultLabelValue, changeInfo.Labels[COMMITQUEUE_LABEL].All[0].Value)
}

func TestSendToCQ(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerrit(GERRIT_SKIA_URL, DefaultGitCookiesPath(), nil)
	assert.NoError(t, err)

	// Send to CQ.
	changeInfo, err := api.GetIssueProperties(context.TODO(), 2370)
	assert.NoError(t, err)
	defaultLabelValue := changeInfo.Labels[COMMITQUEUE_LABEL].DefaultValue
	err = api.SendToCQ(context.TODO(), changeInfo, "Sending to CQ")
	assert.NoError(t, err)

	// Wait for a few seconds for the above to take place.
	time.Sleep(5 * time.Second)

	// Verify that the change was sent to CQ.
	changeInfo, err = api.GetIssueProperties(context.TODO(), 2370)
	assert.NoError(t, err)
	assert.Equal(t, 2, changeInfo.Labels[COMMITQUEUE_LABEL].All[0].Value)

	// Remove from CQ.
	err = api.RemoveFromCQ(context.TODO(), changeInfo, "")
	assert.NoError(t, err)

	// Verify that the change was removed from CQ.
	changeInfo, err = api.GetIssueProperties(context.TODO(), 2370)
	assert.NoError(t, err)
	assert.Equal(t, defaultLabelValue, changeInfo.Labels[COMMITQUEUE_LABEL].All[0].Value)
}

func TestApprove(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerrit(GERRIT_SKIA_URL, DefaultGitCookiesPath(), nil)
	assert.NoError(t, err)

	// Approve.
	changeInfo, err := api.GetIssueProperties(context.TODO(), 2370)
	assert.NoError(t, err)
	defaultLabelValue := changeInfo.Labels[CODEREVIEW_LABEL].DefaultValue
	err = api.Approve(context.TODO(), changeInfo, "LGTM")
	assert.NoError(t, err)

	// Wait for a second for the above to take place.
	time.Sleep(time.Second)

	// Verify that the change was approved.
	changeInfo, err = api.GetIssueProperties(context.TODO(), 2370)
	assert.NoError(t, err)
	assert.Equal(t, 1, changeInfo.Labels[CODEREVIEW_LABEL].All[0].Value)

	// Remove approval.
	err = api.NoScore(context.TODO(), changeInfo, "")
	assert.NoError(t, err)

	// Verify that the change has no score.
	changeInfo, err = api.GetIssueProperties(context.TODO(), 2370)
	assert.NoError(t, err)
	assert.Equal(t, defaultLabelValue, changeInfo.Labels[CODEREVIEW_LABEL].All[0].Value)
}

func TestReadOnlyFailure(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerrit(GERRIT_SKIA_URL, "", nil)
	assert.NoError(t, err)

	// Approve.
	changeInfo, err := api.GetIssueProperties(context.TODO(), 2370)
	assert.NoError(t, err)
	err = api.Approve(context.TODO(), changeInfo, "LGTM")
	assert.Error(t, err)
}

func TestDisApprove(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerrit(GERRIT_SKIA_URL, DefaultGitCookiesPath(), nil)
	assert.NoError(t, err)

	// DisApprove.
	changeInfo, err := api.GetIssueProperties(context.TODO(), 2370)
	assert.NoError(t, err)
	defaultLabelValue := changeInfo.Labels[CODEREVIEW_LABEL].DefaultValue
	err = api.DisApprove(context.TODO(), changeInfo, "not LGTM")
	assert.NoError(t, err)

	// Wait for a second for the above to take place.
	time.Sleep(time.Second)

	// Verify that the change was disapproved.
	changeInfo, err = api.GetIssueProperties(context.TODO(), 2370)
	assert.NoError(t, err)
	assert.Equal(t, -1, changeInfo.Labels[CODEREVIEW_LABEL].All[0].Value)

	// Remove disapproval.
	err = api.NoScore(context.TODO(), changeInfo, "")
	assert.NoError(t, err)

	// Verify that the change has no score.
	changeInfo, err = api.GetIssueProperties(context.TODO(), 2370)
	assert.NoError(t, err)
	assert.Equal(t, defaultLabelValue, changeInfo.Labels[CODEREVIEW_LABEL].All[0].Value)
}

func TestAbandon(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerrit(GERRIT_SKIA_URL, "", nil)
	assert.NoError(t, err)
	c1 := ChangeInfo{
		ChangeId: "Idb96a747c8446126f60fdf1adca361dbc2e539d5",
	}
	err = api.Abandon(context.TODO(), &c1, "Abandoning this CL")
	assert.Error(t, err, "Got status 409 Conflict (409)")
}

func TestFiles(t *testing.T) {
	unittest.SmallTest(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := fmt.Fprintln(w, `)]}'
{
  "/COMMIT_MSG": {
    "status": "A",
    "lines_inserted": 10,
    "size_delta": 353,
    "size": 353
  },
  "BUILD.gn": {
    "lines_inserted": 20,
    "lines_deleted": 5,
    "size_delta": 531,
    "size": 50072
  },
  "include/gpu/vk/GrVkDefines.h": {
    "lines_inserted": 28,
    "lines_deleted": 21,
    "size_delta": 383,
    "size": 1615
  },
  "tools/gpu/vk/GrVulkanDefines.h": {
    "status": "A",
    "lines_inserted": 33,
    "size_delta": 861,
    "size": 861
  }
}`)
		assert.NoError(t, err)
	}))

	defer ts.Close()

	api, err := NewGerrit(ts.URL, "", nil)
	files, err := api.Files(context.TODO(), 12345678, "current")
	assert.NoError(t, err)
	assert.Len(t, files, 4)
	assert.Contains(t, files, "/COMMIT_MSG")
	assert.Equal(t, 353, files["/COMMIT_MSG"].Size)
	assert.Contains(t, files, "tools/gpu/vk/GrVulkanDefines.h")
	assert.Equal(t, 33, files["tools/gpu/vk/GrVulkanDefines.h"].LinesInserted)

	files, err = api.Files(context.TODO(), 12345678, "alert()")
	assert.Error(t, err)
}

func TestGetFileNames(t *testing.T) {
	unittest.SmallTest(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := fmt.Fprintln(w, `)]}'
{
  "/COMMIT_MSG": {
    "status": "A",
    "lines_inserted": 10,
    "size_delta": 353,
    "size": 353
  },
  "BUILD.gn": {
    "lines_inserted": 20,
    "lines_deleted": 5,
    "size_delta": 531,
    "size": 50072
  },
  "include/gpu/vk/GrVkDefines.h": {
    "lines_inserted": 28,
    "lines_deleted": 21,
    "size_delta": 383,
    "size": 1615
  },
  "tools/gpu/vk/GrVulkanDefines.h": {
    "status": "A",
    "lines_inserted": 33,
    "size_delta": 861,
    "size": 861
  }
}`)
		assert.NoError(t, err)
	}))

	defer ts.Close()

	api, err := NewGerrit(ts.URL, "", nil)
	files, err := api.GetFileNames(context.TODO(), 12345678, "current")
	assert.NoError(t, err)
	assert.Len(t, files, 4)
	assert.Contains(t, files, "/COMMIT_MSG")
	assert.Contains(t, files, "tools/gpu/vk/GrVulkanDefines.h")
}

func TestIsBinaryPatch(t *testing.T) {
	unittest.SmallTest(t)

	tsNoBinary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := fmt.Fprintln(w, `)]}'
{
  "/COMMIT_MSG": {
    "status": "A",
    "lines_inserted": 10,
    "size_delta": 353,
    "size": 353
  }
}`)
		assert.NoError(t, err)
	}))
	defer tsNoBinary.Close()
	api, err := NewGerrit(tsNoBinary.URL, "", nil)
	assert.NoError(t, err)
	isBinaryPatch, err := api.IsBinaryPatch(context.TODO(), 4649, "3")
	assert.NoError(t, err)
	assert.False(t, isBinaryPatch)

	tsBinary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := fmt.Fprintln(w, `)]}'
{
  "/COMMIT_MSG": {
    "status": "A",
    "lines_inserted": 15,
    "size_delta": 433,
    "size": 433
  },
  "site/dev/design/Test.png": {
    "status": "A",
    "binary": true,
    "size_delta": 49030,
    "size": 49030
  }
}`)
		assert.NoError(t, err)
	}))
	defer tsBinary.Close()
	api, err = NewGerrit(tsBinary.URL, "", nil)
	assert.NoError(t, err)
	isBinaryPatch, err = api.IsBinaryPatch(context.TODO(), 2370, "5")
	assert.NoError(t, err)
	assert.True(t, isBinaryPatch)
}

func TestExtractIssueFromCommit(t *testing.T) {
	unittest.SmallTest(t)
	cmtMsg := `
   	Author: John Doe <jdoe@example.com>
		Date:   Mon Feb 5 10:51:20 2018 -0500

    Some change

    Change-Id: I26c4fd0e1414ab2385e8590cd729bc70c66ef37e
    Reviewed-on: https://skia-review.googlesource.com/549319
    Commit-Queue: John Doe <jdoe@example.com>
	`
	api, err := NewGerrit(GERRIT_SKIA_URL, "", nil)
	assert.NoError(t, err)
	issueID, err := api.ExtractIssueFromCommit(cmtMsg)
	assert.NoError(t, err)
	assert.Equal(t, int64(549319), issueID)
	_, err = api.ExtractIssueFromCommit("")
	assert.Error(t, err)
}

func TestGetCommit(t *testing.T) {
	skipTestIfRequired(t)

	// Fetch the parent for the given issueID and revision.
	issueID := int64(52160)
	revision := "91740d74af689d53b9fa4d172544e0d5620de9bd"
	expectedParent := "aaab3c73575d5502ae345dd71cf8748c2070ffda"

	api, err := NewGerrit(GERRIT_SKIA_URL, DefaultGitCookiesPath(), nil)
	assert.NoError(t, err)

	commitInfo, err := api.GetCommit(context.TODO(), issueID, revision)
	assert.NoError(t, err)
	assert.Equal(t, expectedParent, commitInfo.Parents[0].Commit)
}
