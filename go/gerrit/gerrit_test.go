package gerrit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/httputils"
)

const (
	// Flip this boolean to run the below E2E Gerrit tests. Requires a valid
	// ~/.gitcookies file.
	RUN_GERRIT_TESTS = false
)

var (
	// http.Client used for testing.
	c = httputils.NewTimeoutClient()
)

func skipTestIfRequired(t *testing.T) {
	if !RUN_GERRIT_TESTS {
		t.Skip("Skipping test due to RUN_GERRIT_TESTS=false")
	}
}

func TestHasOpenDependency(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerritWithConfig(ConfigChromium, GerritSkiaURL, c)
	require.NoError(t, err)

	dep, err := api.HasOpenDependency(context.Background(), 52160, 1)
	require.NoError(t, err)
	require.False(t, dep)

	dep2, err := api.HasOpenDependency(context.Background(), 52123, 1)
	require.NoError(t, err)
	require.True(t, dep2)
}

func TestGerritOwnerModifiedSearch(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerritWithConfig(ConfigChromium, GerritSkiaURL, c)
	require.NoError(t, err)

	tDelta := time.Now().Add(-10 * 24 * time.Hour)
	issues, err := api.Search(context.Background(), 1, true, SearchModifiedAfter(tDelta), SearchOwner("rmistry@google.com"))
	require.NoError(t, err)
	require.True(t, len(issues) > 0)

	for _, issue := range issues {
		details, err := api.GetIssueProperties(context.Background(), issue.Issue)
		require.NoError(t, err)
		require.True(t, details.Updated.After(tDelta))
		require.True(t, len(details.Patchsets) > 0)
		require.Equal(t, "rmistry@google.com", details.Owner.Email)
	}

	issues, err = api.Search(context.Background(), 2, true, SearchModifiedAfter(time.Now().Add(-time.Hour)))
	require.NoError(t, err)
}

func TestGerritCommitSearch(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerritWithConfig(ConfigChromium, GerritSkiaURL, c)
	require.NoError(t, err)

	issues, err := api.Search(context.Background(), 1, true, SearchCommit("a2eb235a16ed430896cc54989e683cf930319eb7"))
	require.NoError(t, err)
	require.Equal(t, 1, len(issues))

	for _, issue := range issues {
		details, err := api.GetIssueProperties(context.Background(), issue.Issue)
		require.NoError(t, err)
		require.Equal(t, 5, len(details.Patchsets))
		require.Equal(t, "rmistry@google.com", details.Owner.Email)
		require.Equal(t, "I37876c6f62c85d0532b22dcf8bea8b4e7f4147c0", details.ChangeId)
		require.True(t, details.Committed)
		require.Equal(t, "Skia Gerrit 10k!", details.Subject)
	}
}

// It is a little different to test the poller. Enable this to turn on the
// poller and test removing issues from it.
func GerritPollerTest(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerritWithConfig(ConfigChromium, GerritSkiaURL, c)
	require.NoError(t, err)

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

	api, err := NewGerritWithConfig(ConfigChromium, GerritSkiaURL, c)
	require.NoError(t, err)

	patch, err := api.GetPatch(context.Background(), 2370, "current")
	require.NoError(t, err)

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

	require.Equal(t, expected, patch)
}

func TestAddCC(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerritWithConfig(ConfigChromium, GerritSkiaURL, c)
	require.NoError(t, err)

	changeInfo, err := api.GetIssueProperties(context.Background(), 2370)
	require.NoError(t, err)
	err = api.AddCC(context.Background(), changeInfo, []string{"test-user1@google.com", "test-user2@google.com"})
	require.NoError(t, err)
}

func TestAddComment(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerritWithConfig(ConfigChromium, GerritSkiaURL, c)
	require.NoError(t, err)

	changeInfo, err := api.GetIssueProperties(context.Background(), 2370)
	require.NoError(t, err)
	err = api.AddComment(context.Background(), changeInfo, "Testing API!!")
	require.NoError(t, err)
}

func TestSendToDryRun(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerritWithConfig(ConfigChromium, GerritSkiaURL, c)
	require.NoError(t, err)

	// Send to dry run.
	changeInfo, err := api.GetIssueProperties(context.Background(), 2370)
	require.NoError(t, err)
	defaultLabelValue := changeInfo.Labels[LabelCommitQueue].DefaultValue
	err = api.SendToDryRun(context.Background(), changeInfo, "Sending to dry run")
	require.NoError(t, err)

	// Wait for a second for the above to take place.
	time.Sleep(time.Second)

	// Verify that the change was sent to dry run.
	changeInfo, err = api.GetIssueProperties(context.Background(), 2370)
	require.NoError(t, err)
	require.Equal(t, 1, changeInfo.Labels[LabelCommitQueue].All[0].Value)

	// Remove from dry run.
	err = api.RemoveFromCQ(context.Background(), changeInfo, "")
	require.NoError(t, err)

	// Verify that the change was removed from dry run.
	changeInfo, err = api.GetIssueProperties(context.Background(), 2370)
	require.NoError(t, err)
	require.Equal(t, defaultLabelValue, changeInfo.Labels[LabelCommitQueue].All[0].Value)
}

func TestSendToCQ(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerritWithConfig(ConfigChromium, GerritSkiaURL, c)
	require.NoError(t, err)

	// Send to CQ.
	changeInfo, err := api.GetIssueProperties(context.Background(), 2370)
	require.NoError(t, err)
	defaultLabelValue := changeInfo.Labels[LabelCommitQueue].DefaultValue
	err = api.SendToCQ(context.Background(), changeInfo, "Sending to CQ")
	require.NoError(t, err)

	// Wait for a few seconds for the above to take place.
	time.Sleep(5 * time.Second)

	// Verify that the change was sent to CQ.
	changeInfo, err = api.GetIssueProperties(context.Background(), 2370)
	require.NoError(t, err)
	require.Equal(t, 2, changeInfo.Labels[LabelCommitQueue].All[0].Value)

	// Remove from CQ.
	err = api.RemoveFromCQ(context.Background(), changeInfo, "")
	require.NoError(t, err)

	// Verify that the change was removed from CQ.
	changeInfo, err = api.GetIssueProperties(context.Background(), 2370)
	require.NoError(t, err)
	require.Equal(t, defaultLabelValue, changeInfo.Labels[LabelCommitQueue].All[0].Value)
}

func TestApprove(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerritWithConfig(ConfigChromium, GerritSkiaURL, c)
	require.NoError(t, err)

	// Approve.
	changeInfo, err := api.GetIssueProperties(context.Background(), 2370)
	require.NoError(t, err)
	defaultLabelValue := changeInfo.Labels[LabelCodeReview].DefaultValue
	err = api.Approve(context.Background(), changeInfo, "LGTM")
	require.NoError(t, err)

	// Wait for a second for the above to take place.
	time.Sleep(time.Second)

	// Verify that the change was approved.
	changeInfo, err = api.GetIssueProperties(context.Background(), 2370)
	require.NoError(t, err)
	require.Equal(t, 1, changeInfo.Labels[LabelCodeReview].All[0].Value)

	// Remove approval.
	err = api.NoScore(context.Background(), changeInfo, "")
	require.NoError(t, err)

	// Verify that the change has no score.
	changeInfo, err = api.GetIssueProperties(context.Background(), 2370)
	require.NoError(t, err)
	require.Equal(t, defaultLabelValue, changeInfo.Labels[LabelCodeReview].All[0].Value)
}

func TestReadOnlyFailure(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerritWithConfig(ConfigChromium, GerritSkiaURL, c)
	require.NoError(t, err)

	// Approve.
	changeInfo, err := api.GetIssueProperties(context.Background(), 2370)
	require.NoError(t, err)
	err = api.Approve(context.Background(), changeInfo, "LGTM")
	require.Error(t, err)
}

func TestDisApprove(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerritWithConfig(ConfigChromium, GerritSkiaURL, c)
	require.NoError(t, err)

	// DisApprove.
	changeInfo, err := api.GetIssueProperties(context.Background(), 2370)
	require.NoError(t, err)
	defaultLabelValue := changeInfo.Labels[LabelCodeReview].DefaultValue
	err = api.Disapprove(context.Background(), changeInfo, "not LGTM")
	require.NoError(t, err)

	// Wait for a second for the above to take place.
	time.Sleep(time.Second)

	// Verify that the change was disapproved.
	changeInfo, err = api.GetIssueProperties(context.Background(), 2370)
	require.NoError(t, err)
	require.Equal(t, -1, changeInfo.Labels[LabelCodeReview].All[0].Value)

	// Remove disapproval.
	err = api.NoScore(context.Background(), changeInfo, "")
	require.NoError(t, err)

	// Verify that the change has no score.
	changeInfo, err = api.GetIssueProperties(context.Background(), 2370)
	require.NoError(t, err)
	require.Equal(t, defaultLabelValue, changeInfo.Labels[LabelCodeReview].All[0].Value)
}

func TestAbandon(t *testing.T) {
	skipTestIfRequired(t)

	api, err := NewGerritWithConfig(ConfigChromium, GerritSkiaURL, c)
	require.NoError(t, err)
	c1 := ChangeInfo{
		ChangeId: "Idb96a747c8446126f60fdf1adca361dbc2e539d5",
	}
	err = api.Abandon(context.Background(), &c1, "Abandoning this CL")
	require.Error(t, err, "Got status 409 Conflict (409)")
}

func TestGetAbandonReason(t *testing.T) {

	// Messages that will be used by tests.
	msgWithAbandonFromUser := ChangeInfoMessage{
		Tag:     "something",
		Message: "Abandoned",
	}
	msgWithAbandonAutogenerated := ChangeInfoMessage{
		Tag:     "autogenerated:gerrit:abandon",
		Message: "Abandoned",
	}
	msgWithAbandonPrefixAndMsgAutogenerated := ChangeInfoMessage{
		Tag:     "autogenerated:gerrit:abandon",
		Message: "Abandoned\n\nTesting testing",
	}
	msgWithNoPrefixAndMsgAutogenerated := ChangeInfoMessage{
		Tag:     "autogenerated:gerrit:abandon",
		Message: "Something",
	}

	tests := []struct {
		status                string
		messages              []ChangeInfoMessage
		expectedAbandonReason string
		failureMessage        string
	}{
		{
			status:                ChangeStatusAbandoned,
			messages:              []ChangeInfoMessage{msgWithAbandonFromUser, msgWithAbandonPrefixAndMsgAutogenerated},
			expectedAbandonReason: "Testing testing",
			failureMessage:        "autogenerated abandon messages with prefix should return message without prefix",
		},
		{
			status:                ChangeStatusNew,
			messages:              []ChangeInfoMessage{msgWithAbandonFromUser, msgWithAbandonPrefixAndMsgAutogenerated},
			expectedAbandonReason: "",
			failureMessage:        "non-abandoned changes have no abandon message"},
		{
			status:                ChangeStatusAbandoned,
			messages:              []ChangeInfoMessage{msgWithAbandonFromUser, msgWithAbandonAutogenerated},
			expectedAbandonReason: "",
			failureMessage:        "autogenerated abandon messages with only prefix should return an empty message"},
		{
			status:                ChangeStatusAbandoned,
			messages:              []ChangeInfoMessage{msgWithAbandonFromUser, msgWithNoPrefixAndMsgAutogenerated},
			expectedAbandonReason: "Something",
			failureMessage:        "autogenerated abandon messages with no prefix should return the specified message"},
		{
			status:                ChangeStatusAbandoned,
			messages:              []ChangeInfoMessage{msgWithAbandonFromUser},
			expectedAbandonReason: "",
			failureMessage:        "abandon messages from users are ignored"},
	}
	for _, test := range tests {
		changeInfo := ChangeInfo{
			Status:   test.status,
			Messages: test.messages,
		}
		require.Equal(t, test.expectedAbandonReason, changeInfo.GetAbandonReason(context.Background()), test.failureMessage)
	}
}

func TestFiles(t *testing.T) {
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
		require.NoError(t, err)
	}))

	defer ts.Close()

	api, err := NewGerritWithConfig(ConfigChromium, ts.URL, c)
	files, err := api.Files(context.Background(), 12345678, "current")
	require.NoError(t, err)
	require.Len(t, files, 4)
	require.Contains(t, files, "/COMMIT_MSG")
	require.Equal(t, 353, files["/COMMIT_MSG"].Size)
	require.Contains(t, files, "tools/gpu/vk/GrVulkanDefines.h")
	require.Equal(t, 33, files["tools/gpu/vk/GrVulkanDefines.h"].LinesInserted)

	files, err = api.Files(context.Background(), 12345678, "alert()")
	require.Error(t, err)
}

func TestGetFileNames(t *testing.T) {
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
		require.NoError(t, err)
	}))

	defer ts.Close()

	api, err := NewGerritWithConfig(ConfigChromium, ts.URL, c)
	files, err := api.GetFileNames(context.Background(), 12345678, "current")
	require.NoError(t, err)
	require.Len(t, files, 4)
	require.Contains(t, files, "/COMMIT_MSG")
	require.Contains(t, files, "tools/gpu/vk/GrVulkanDefines.h")
}

func TestGetFilesToContent(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/a/changes/123/revisions/current/files" {
			w.Header().Set("Content-Type", "application/json")
			_, err := fmt.Fprintln(w, `)]}'
{
  "dir1/file1": {},
  "file2": {},
  "file3": {}
}`)
			require.NoError(t, err)
		} else if r.URL.Path == "/a/changes/123/revisions/current/files/dir1/file1/content" {
			w.Header().Set("Content-Type", "text/plain")
			_, err := fmt.Fprintln(w, base64.StdEncoding.EncodeToString([]byte("xyz")))
			require.NoError(t, err)
		} else if r.URL.Path == "/a/changes/123/revisions/current/files/file2/content" {
			w.Header().Set("Content-Type", "text/plain")
			_, err := fmt.Fprintln(w, base64.StdEncoding.EncodeToString([]byte("xyz abc")))
			require.NoError(t, err)
		} else if r.URL.Path == "/a/changes/123/revisions/current/files/file3/content" {
			http.Error(w, "404 Not Found", http.StatusNotFound)
		}
	}))
	defer ts.Close()

	api, err := NewGerritWithConfig(ConfigChromium, ts.URL, c)
	ci := &ChangeInfo{Issue: int64(123)}
	filesToContent, err := api.GetFilesToContent(context.Background(), ci.Issue, "current")
	require.NoError(t, err)
	require.Len(t, filesToContent, 3)
	require.Equal(t, "xyz", filesToContent["dir1/file1"])
	require.Equal(t, "xyz abc", filesToContent["file2"])
	require.Equal(t, "", filesToContent["file3"])
}

func TestSubmittedTogether(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := fmt.Fprintln(w, `)]}'
{
	"changes": [
		{
			"id": "change1"
		},
		{
			"id": "change2"
		}
	],
	"non_visible_changes": 1
}`)
		require.NoError(t, err)
	}))
	defer ts.Close()

	api, err := NewGerritWithConfig(ConfigChromium, ts.URL, c)
	ci := &ChangeInfo{Issue: int64(123)}
	submittedTogether, nonVisible, err := api.SubmittedTogether(context.Background(), ci)
	require.NoError(t, err)
	require.Len(t, submittedTogether, 2)
	require.Equal(t, "change1", submittedTogether[0].Id)
	require.Equal(t, "change2", submittedTogether[1].Id)
	require.Equal(t, 1, nonVisible)
}

func TestIsBinaryPatch(t *testing.T) {

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
		require.NoError(t, err)
	}))
	defer tsNoBinary.Close()
	api, err := NewGerritWithConfig(ConfigChromium, tsNoBinary.URL, c)
	require.NoError(t, err)
	isBinaryPatch, err := api.IsBinaryPatch(context.Background(), 4649, "3")
	require.NoError(t, err)
	require.False(t, isBinaryPatch)

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
		require.NoError(t, err)
	}))
	defer tsBinary.Close()
	api, err = NewGerritWithConfig(ConfigChromium, tsBinary.URL, c)
	require.NoError(t, err)
	isBinaryPatch, err = api.IsBinaryPatch(context.Background(), 2370, "5")
	require.NoError(t, err)
	require.True(t, isBinaryPatch)
}

func TestExtractIssueFromCommit(t *testing.T) {
	cmtMsg := `
   	Author: John Doe <jdoe@example.com>
		Date:   Mon Feb 5 10:51:20 2018 -0500

    Some change

    Change-Id: I26c4fd0e1414ab2385e8590cd729bc70c66ef37e
    Reviewed-on: https://skia-review.googlesource.com/549319
    Commit-Queue: John Doe <jdoe@example.com>
	`
	api, err := NewGerritWithConfig(ConfigChromium, GerritSkiaURL, c)
	require.NoError(t, err)
	issueID, err := api.ExtractIssueFromCommit(cmtMsg)
	require.NoError(t, err)
	require.Equal(t, int64(549319), issueID)
	_, err = api.ExtractIssueFromCommit("")
	require.Error(t, err)
}

func TestGetCommit(t *testing.T) {
	skipTestIfRequired(t)

	// Fetch the parent for the given issueID and revision.
	issueID := int64(52160)
	revision := "91740d74af689d53b9fa4d172544e0d5620de9bd"
	expectedParent := "aaab3c73575d5502ae345dd71cf8748c2070ffda"

	api, err := NewGerritWithConfig(ConfigChromium, GerritSkiaURL, c)
	require.NoError(t, err)

	commitInfo, err := api.GetCommit(context.Background(), issueID, revision)
	require.NoError(t, err)
	require.Equal(t, expectedParent, commitInfo.Parents[0].Commit)
}

func TestParseChangeId(t *testing.T) {

	expect := "Ie00a12db04350ab0f8c754b3674eaa5a0a556b63"
	actual, err := ParseChangeId(`commit 96c2eb6258aef6146d947648db12b6470de8197a (origin/master, origin/HEAD, master)
Author: Eric Boren <borenet@google.com>
Date:   Mon Mar 2 14:53:04 2020 -0500

    [recipes] Move nanobench flags logic into gen_tasks_logic/nanobench_flags.go

    Change-Id: Ie00a12db04350ab0f8c754b3674eaa5a0a556b63
    Reviewed-on: https://skia-review.googlesource.com/c/skia/+/274596
    Commit-Queue: Eric Boren <borenet@google.com>
    Reviewed-by: Ben Wagner aka dogben <benjaminwagner@google.com>
`)
	require.NoError(t, err)
	require.Equal(t, expect, actual)
}

func TestFullChangeId(t *testing.T) {

	ci := &ChangeInfo{
		Project:  "skia",
		Branch:   "main",
		ChangeId: "abc",
		Issue:    123,
	}
	require.Equal(t, "skia~123", FullChangeId(ci))

	// Test project with "/" in the name.
	ci.Project = "chromium/src"
	require.Equal(t, "chromium%2Fsrc~123", FullChangeId(ci))

	// Test branch with "/" in the name.
	ci.Branch = "chrome/m90"
	require.Equal(t, "chromium%2Fsrc~123", FullChangeId(ci))

	// Test branch with "refs/heads/" prefix.
	ci.Branch = "refs/heads/chrome/m90"
	require.Equal(t, "chromium%2Fsrc~123", FullChangeId(ci))
}

func TestCreateChange_WithCommitAndChangeID_ReturnsError(t *testing.T) {
	const (
		commit   = "b85141fe76d3dc598407865b07778d164204726e"
		changeID = "Icc898ef6bb4eeb8e93fa8c5d1195364d55ca2a4c"
		project  = "skia"
		branch   = "main"
		subject  = "Test subject"
	)
	ctx := context.Background()
	g := Gerrit{}

	_, err := g.CreateChange(ctx, project, branch, subject, commit, changeID)
	require.Equal(t, ErrBothChangeAndCommitID, err)
}

func TestCreateChange_WithBaseCommit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var data createChangePostData
		require.NoError(t, json.Unmarshal(body, &data))

		require.Equal(t, "myProject", data.Project)
		require.Equal(t, "master", data.Branch)
		require.Equal(t, "Let's support 100% Gerrit workflow direct in browser", data.Subject)
		require.Equal(t, "NEW", data.Status)
		require.Equal(t, "1234567890123456789012345678901234567890", data.BaseCommit)
		require.Equal(t, "", data.BaseChange)
		response := `)]}'
{
  "id": "myProject~master~I8473b95934b5732ac55d26311a706c9c2bde9941",
  "project": "myProject",
  "branch": "master",
  "topic": "create-change-in-browser",
  "change_id": "I8473b95934b5732ac55d26311a706c9c2bde9941",
  "subject": "Let's support 100% Gerrit workflow direct in browser",
  "status": "NEW",
  "created": "2014-05-05 07:15:44.639000000",
  "updated": "2014-05-05 07:15:44.639000000",
  "mergeable": true,
  "insertions": 0,
  "deletions": 0,
  "_number": 4711,
  "owner": {
    "name": "John Doe"
  }
}`
		_, err = fmt.Fprintln(w, response)
		require.NoError(t, err)
	}))

	defer ts.Close()

	api, err := NewGerritWithConfig(ConfigChromium, ts.URL, c)
	require.NoError(t, err)
	ci, err := api.CreateChange(
		context.Background(),
		"myProject",
		"master",
		"Let's support 100% Gerrit workflow direct in browser",
		"1234567890123456789012345678901234567890",
		"",
	)
	require.NoError(t, err)
	require.Equal(t, "master", ci.Branch)
	require.Equal(t, "myProject~master~I8473b95934b5732ac55d26311a706c9c2bde9941", ci.Id)
	require.Equal(t, "I8473b95934b5732ac55d26311a706c9c2bde9941", ci.ChangeId)
}

func TestCreateChange_WithBaseChangeID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var data createChangePostData
		require.NoError(t, json.Unmarshal(body, &data))

		require.Equal(t, "myProject", data.Project)
		require.Equal(t, "master", data.Branch)
		require.Equal(t, "Let's support 100% Gerrit workflow direct in browser", data.Subject)
		require.Equal(t, "NEW", data.Status)
		require.Equal(t, "", data.BaseCommit)
		require.Equal(t, "I1234567890123456789012345678901234567890", data.BaseChange)
		response := `)]}'
{
  "id": "myProject~master~I8473b95934b5732ac55d26311a706c9c2bde9941",
  "project": "myProject",
  "branch": "master",
  "topic": "create-change-in-browser",
  "change_id": "I8473b95934b5732ac55d26311a706c9c2bde9941",
  "subject": "Let's support 100% Gerrit workflow direct in browser",
  "status": "NEW",
  "created": "2014-05-05 07:15:44.639000000",
  "updated": "2014-05-05 07:15:44.639000000",
  "mergeable": true,
  "insertions": 0,
  "deletions": 0,
  "_number": 4711,
  "owner": {
    "name": "John Doe"
  }
}`
		_, err = fmt.Fprintln(w, response)
		require.NoError(t, err)
	}))

	defer ts.Close()

	api, err := NewGerritWithConfig(ConfigChromium, ts.URL, c)
	require.NoError(t, err)
	ci, err := api.CreateChange(
		context.Background(),
		"myProject",
		"master",
		"Let's support 100% Gerrit workflow direct in browser",
		"",
		"I1234567890123456789012345678901234567890",
	)
	require.NoError(t, err)
	require.Equal(t, "master", ci.Branch)
	require.Equal(t, "myProject~master~I8473b95934b5732ac55d26311a706c9c2bde9941", ci.Id)
	require.Equal(t, "I8473b95934b5732ac55d26311a706c9c2bde9941", ci.ChangeId)
}

func TestCreateCherryPickChange(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t,
			"/a/changes/myProject~release~I8473b95934b5732ac55d26311a706c9c2bde9940/revisions/674ac754f91e64a0efb8087e59a176484bd534d1/cherrypick",
			r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var data cherryPickPostData
		require.NoError(t, json.Unmarshal(body, &data))

		require.Equal(t, "main", data.Destination)
		require.Equal(t, "Picking into main", data.Message)
		format := `)]}'
{
  "id": "myProject~release~I8473b95934b5732ac55d26311a706c9c2bde9941",
  "project": "myProject",
  "branch": "%s",
  "change_id": "I8473b95934b5732ac55d26311a706c9c2bde9941",
  "subject": "%s",
  "status": "NEW",
  "created": "2013-02-01 09:59:32.126000000",
  "updated": "2013-02-21 11:16:36.775000000",
  "mergeable": true,
  "insertions": 12,
  "deletions": 11,
  "_number": 3965,
  "owner": {
    "name": "John Doe"
  }
}` + "\n"
		_, err = fmt.Fprintf(w, format, data.Destination, data.Message)
		require.NoError(t, err)
	}))

	defer ts.Close()

	api, err := NewGerritWithConfig(ConfigChromium, ts.URL, c)
	require.NoError(t, err)
	ci, err := api.CreateCherryPickChange(context.Background(),
		"myProject~release~I8473b95934b5732ac55d26311a706c9c2bde9940",
		"674ac754f91e64a0efb8087e59a176484bd534d1",
		"Picking into main",
		"main")
	require.NoError(t, err)
	require.Equal(t, "main", ci.Branch)
	require.Equal(t, "myProject~release~I8473b95934b5732ac55d26311a706c9c2bde9941", ci.Id)
	require.Equal(t, "I8473b95934b5732ac55d26311a706c9c2bde9941", ci.ChangeId)
}
