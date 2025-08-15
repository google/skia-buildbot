package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/go-github/v29/github"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
)

func TestAddComment(t *testing.T) {
	reqType := "application/json"
	reqBody := []byte(`{"body":"test msg"}
`)
	r := chi.NewRouter()
	md := mockhttpclient.MockPostDialogueWithResponseCode(reqType, reqBody, nil, http.StatusCreated)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Post("/repos/kryptonians/krypton/issues/1234/comments", md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient)
	require.NoError(t, err)
	addCommentErr := githubClient.AddComment(1234, "test msg")
	require.NoError(t, addCommentErr)
}

func TestGetAuthenticatedUser(t *testing.T) {
	r := chi.NewRouter()
	md := mockhttpclient.MockGetError("OK", http.StatusOK)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Get("/user", md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient)
	require.NoError(t, err)
	_, getUserErr := githubClient.GetAuthenticatedUser()
	require.NoError(t, getUserErr)
}

func TestListOpenPullRequests(t *testing.T) {
	prNum1 := 1
	prNum2 := 101
	respBody := []byte(testutils.MarshalJSON(t, []*github.PullRequest{
		{Number: &prNum1},
		{Number: &prNum2},
	}))
	r := chi.NewRouter()
	md := mockhttpclient.MockGetDialogue(respBody)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Get("/repos/kryptonians/krypton/pulls", md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient)
	require.NoError(t, err)
	prs, getPullErr := githubClient.ListOpenPullRequests()
	require.NoError(t, getPullErr)
	require.Equal(t, 2, len(prs))
	require.Equal(t, prNum1, prs[0].GetNumber())
	require.Equal(t, prNum2, prs[1].GetNumber())
}

func TestGetPullRequest(t *testing.T) {
	respBody := []byte(testutils.MarshalJSON(t, &github.PullRequest{State: &CLOSED_STATE}))
	r := chi.NewRouter()
	md := mockhttpclient.MockGetDialogue(respBody)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Get("/repos/kryptonians/krypton/pulls/1234", md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient)
	require.NoError(t, err)
	pr, getPullErr := githubClient.GetPullRequest(1234)
	require.NoError(t, getPullErr)
	require.Equal(t, CLOSED_STATE, *pr.State)
}

func TestGetReference(t *testing.T) {
	testSHA := "xyz"
	testRef := "test-branch"
	testRepoOwner := "batman"
	testRepoName := "gotham"
	respBody := []byte(testutils.MarshalJSON(t, &github.Reference{Object: &github.GitObject{SHA: &testSHA}}))
	r := chi.NewRouter()
	md := mockhttpclient.MockGetDialogue(respBody)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Get(fmt.Sprintf("/repos/%s/%s/git/refs/%s", testRepoOwner, testRepoName, url.QueryEscape(testRef)), md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), testRepoOwner, testRepoName, httpClient)
	require.NoError(t, err)
	ref, err := githubClient.GetReference(testRepoOwner, testRepoName, testRef)
	require.NoError(t, err)
	require.Equal(t, testSHA, *ref.Object.SHA)
}

func TestListMatchingReferences(t *testing.T) {
	testSHA1 := "abc"
	testSHA2 := "xyz"
	testRef := "test-branch"
	testRepoOwner := "batman"
	testRepoName := "gotham"
	retRef1 := github.Reference{Object: &github.GitObject{SHA: &testSHA1}}
	retRef2 := github.Reference{Object: &github.GitObject{SHA: &testSHA2}}
	respBody := []byte(testutils.MarshalJSON(t, []github.Reference{retRef1, retRef2}))
	r := chi.NewRouter()
	md := mockhttpclient.MockGetDialogue(respBody)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Get(fmt.Sprintf("/repos/%s/%s/git/refs/%s", testRepoOwner, testRepoName, url.QueryEscape(testRef)), md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), testRepoOwner, testRepoName, httpClient)
	require.NoError(t, err)
	retRefs, err := githubClient.ListMatchingReferences(testRepoOwner, testRepoName, testRef)
	require.NoError(t, err)
	require.Equal(t, 2, len(retRefs))
	require.Equal(t, testSHA1, *retRefs[0].Object.SHA)
	require.Equal(t, testSHA2, *retRefs[1].Object.SHA)
}

func TestDeleteReference(t *testing.T) {
	testRef := "test-branch"
	testRepoOwner := "batman"
	testRepoName := "gotham"
	r := chi.NewRouter()
	md := mockhttpclient.MockDeleteDialogueWithResponseCode("", nil, nil, http.StatusNoContent)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Delete(fmt.Sprintf("/repos/%s/%s/git/refs/%s", testRepoOwner, testRepoName, testRef), md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), testRepoOwner, testRepoName, httpClient)
	require.NoError(t, err)
	err = githubClient.DeleteReference(testRepoOwner, testRepoName, testRef)
	require.NoError(t, err)
}

func TestCreateReference(t *testing.T) {
	testSHA := "xyz"
	testRef := "test-branch"
	testRepoOwner := "batman"
	testRepoName := "gotham"
	reqBody := []byte(fmt.Sprintf(`{"ref":"refs/%s","sha":"%s"}
`, testRef, testSHA))
	r := chi.NewRouter()
	md := mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, nil, http.StatusCreated)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Post(fmt.Sprintf("/repos/%s/%s/git/refs", testRepoOwner, testRepoName), md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), testRepoOwner, testRepoName, httpClient)
	require.NoError(t, err)
	err = githubClient.CreateReference(testRepoOwner, testRepoName, testRef, testSHA)
	require.NoError(t, err)
}

func TestCreatePullRequest(t *testing.T) {
	reqType := "application/json"
	reqBody := []byte(`{"title":"title","head":"headBranch","base":"baseBranch","body":"testBody"}
`)
	number := 12345
	respBody := []byte(testutils.MarshalJSON(t, &github.PullRequest{Number: &number}))
	r := chi.NewRouter()
	md := mockhttpclient.MockPostDialogueWithResponseCode(reqType, reqBody, respBody, http.StatusCreated)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Post("/repos/kryptonians/krypton/pulls", md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient)
	require.NoError(t, err)
	pullRequest, createPullErr := githubClient.CreatePullRequest("title", "baseBranch", "headBranch", "testBody")
	require.NoError(t, createPullErr)
	require.Equal(t, number, *pullRequest.Number)
}

func TestMergePullRequest(t *testing.T) {
	reqType := "application/json"
	reqBody := []byte(`{"commit_message":"test comment","merge_method":"squash"}
`)
	r := chi.NewRouter()
	md := mockhttpclient.MockPutDialogue(reqType, reqBody, nil)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Put("/repos/kryptonians/krypton/pulls/1234/merge", md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient)
	require.NoError(t, err)
	mergePullErr := githubClient.MergePullRequest(1234, "test comment", "squash")
	require.NoError(t, mergePullErr)
}

func TestClosePullRequest(t *testing.T) {
	respBody := []byte(testutils.MarshalJSON(t, &github.PullRequest{State: &CLOSED_STATE}))
	reqType := "application/json"
	reqBody := []byte(`{"state":"closed"}
`)
	r := chi.NewRouter()
	md := mockhttpclient.MockPatchDialogue(reqType, reqBody, respBody)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Patch("/repos/kryptonians/krypton/pulls/1234", md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient)
	require.NoError(t, err)
	pr, closePullErr := githubClient.ClosePullRequest(1234)
	require.NoError(t, closePullErr)
	require.Equal(t, CLOSED_STATE, *pr.State)
}

func TestGetIssues(t *testing.T) {
	id1 := int64(11)
	id2 := int64(22)
	issue1 := github.Issue{ID: &id1}
	issue2 := github.Issue{ID: &id2}
	respBody := []byte(testutils.MarshalJSON(t, []*github.Issue{&issue1, &issue2}))
	r := chi.NewRouter()
	md := mockhttpclient.MockGetDialogue(respBody)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com"),
		mockhttpclient.QueryMatcher("labels", "label1,label2", "per_page", "123", "state", "open")).
		Get("/repos/kryptonians/krypton/issues", md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient)
	require.NoError(t, err)
	issues, getIssuesErr := githubClient.GetIssues(true, []string{"label1", "label2"}, 123)
	require.NoError(t, getIssuesErr)
	require.Equal(t, 2, len(issues))
	require.Equal(t, id1, issues[0].GetID())
	require.Equal(t, id2, issues[1].GetID())
}

func TestGetLabelsRequest(t *testing.T) {
	label1Name := "test1"
	label2Name := "test2"
	label1 := github.Label{Name: &label1Name}
	label2 := github.Label{Name: &label2Name}
	respBody := []byte(testutils.MarshalJSON(t, &github.PullRequest{Labels: []*github.Label{&label1, &label2}}))
	r := chi.NewRouter()
	md := mockhttpclient.MockGetDialogue(respBody)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Get("/repos/kryptonians/krypton/issues/1234", md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient)
	require.NoError(t, err)
	labels, getLabelsErr := githubClient.GetLabels(1234)
	require.NoError(t, getLabelsErr)
	require.Equal(t, []string{label1Name, label2Name}, labels)
}

func TestAddLabelRequest(t *testing.T) {
	label1Name := "test1"
	label2Name := "test2"
	label1 := github.Label{Name: &label1Name}
	label2 := github.Label{Name: &label2Name}
	respBody := []byte(testutils.MarshalJSON(t, &github.PullRequest{Labels: []*github.Label{&label1, &label2}}))
	r := chi.NewRouter()
	md := mockhttpclient.MockGetDialogue(respBody)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Get("/repos/kryptonians/krypton/issues/1234", md.ServeHTTP)

	patchRespBody := []byte(testutils.MarshalJSON(t, &github.PullRequest{}))
	patchReqType := "application/json"
	patchReqBody := []byte(`{"labels":["test1","test2","test3"]}
`)
	patchMd := mockhttpclient.MockPatchDialogue(patchReqType, patchReqBody, patchRespBody)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Patch("/repos/kryptonians/krypton/issues/1234", patchMd.ServeHTTP)

	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient)
	require.NoError(t, err)
	addLabelErr := githubClient.AddLabel(1234, "test3")
	require.NoError(t, addLabelErr)
}

func TestRemoveLabelRequest(t *testing.T) {
	label1Name := "test1"
	label2Name := "test2"
	label1 := github.Label{Name: &label1Name}
	label2 := github.Label{Name: &label2Name}
	respBody := []byte(testutils.MarshalJSON(t, &github.PullRequest{Labels: []*github.Label{&label1, &label2}}))
	r := chi.NewRouter()
	md := mockhttpclient.MockGetDialogue(respBody)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Get("/repos/kryptonians/krypton/issues/1234", md.ServeHTTP)

	patchRespBody := []byte(testutils.MarshalJSON(t, &github.PullRequest{}))
	patchReqType := "application/json"
	patchReqBody := []byte(`{"labels":["test1"]}
`)
	patchMd := mockhttpclient.MockPatchDialogue(patchReqType, patchReqBody, patchRespBody)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Patch("/repos/kryptonians/krypton/issues/1234", patchMd.ServeHTTP)

	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient)
	require.NoError(t, err)
	removeLabelErr1 := githubClient.RemoveLabel(1234, "test2")
	require.NoError(t, removeLabelErr1)
}

func TestReplaceLabelRequest(t *testing.T) {
	label1Name := "test1"
	label2Name := "test2"
	label1 := github.Label{Name: &label1Name}
	label2 := github.Label{Name: &label2Name}
	respBody := []byte(testutils.MarshalJSON(t, &github.PullRequest{Labels: []*github.Label{&label1, &label2}}))
	r := chi.NewRouter()
	md := mockhttpclient.MockGetDialogue(respBody)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Get("/repos/kryptonians/krypton/issues/1234", md.ServeHTTP)

	patchRespBody := []byte(testutils.MarshalJSON(t, &github.PullRequest{}))
	patchReqType := "application/json"
	patchReqBody := []byte(`{"labels":["test2","test3"]}
`)
	patchMd := mockhttpclient.MockPatchDialogue(patchReqType, patchReqBody, patchRespBody)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Patch("/repos/kryptonians/krypton/issues/1234", patchMd.ServeHTTP)

	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient)
	require.NoError(t, err)
	removeLabelErr := githubClient.ReplaceLabel(1234, "test1", "test3")
	require.NoError(t, removeLabelErr)
}

func TestGetChecksRequest(t *testing.T) {

	// Mock out check-runs call.
	checkID1 := int64(100)
	checkName1 := "check1"
	check1 := github.CheckRun{ID: &checkID1, Name: &checkName1, StartedAt: &github.Timestamp{Time: time.Now()}}
	respBody := []byte(testutils.MarshalJSON(t, &github.ListCheckRunsResults{CheckRuns: []*github.CheckRun{&check1}}))
	r := chi.NewRouter()
	md := mockhttpclient.MockGetDialogue(respBody)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Get("/repos/kryptonians/krypton/commits/abcd/check-runs", md.ServeHTTP)

	// Mock out status call.
	checkID2 := int64(200)
	checkName2 := "check2"
	pendingState := "pending"
	repoStatusCheck2 := github.RepoStatus{ID: &checkID2, Context: &checkName2, State: &pendingState}
	statusRespBody := []byte(testutils.MarshalJSON(t, &github.CombinedStatus{Statuses: []github.RepoStatus{repoStatusCheck2}}))
	mdStatus := mockhttpclient.MockGetDialogue(statusRespBody)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Get("/repos/kryptonians/krypton/commits/abcd/status", mdStatus.ServeHTTP)

	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient)
	require.NoError(t, err)
	checks, getChecksErr := githubClient.GetChecks("abcd")
	require.NoError(t, getChecksErr)
	require.Equal(t, 2, len(checks))
	require.Equal(t, checkID1, checks[0].ID)
	require.Equal(t, checkName1, checks[0].Name)
	require.Equal(t, checkID2, checks[1].ID)
	require.Equal(t, checkName2, checks[1].Name)
}

func TestReRequestLatestCheckSuite(t *testing.T) {

	// Mock out list check suites call.
	totalResults := int(1)
	checkSuiteID := int64(100)
	checkSuiteStatus := "failed"
	checkSuite := github.CheckSuite{ID: &checkSuiteID, Status: &checkSuiteStatus}
	respBody := []byte(testutils.MarshalJSON(t, &github.ListCheckSuiteResults{Total: &totalResults, CheckSuites: []*github.CheckSuite{&checkSuite}}))
	r := chi.NewRouter()
	listmd := mockhttpclient.MockGetDialogue(respBody)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Get("/repos/kryptonians/krypton/commits/abcd/check-suites", listmd.ServeHTTP)

	// Mock out rerequest check suite call.
	rerequestmd := mockhttpclient.MockPostDialogueWithResponseCode("application/json", nil, nil, http.StatusCreated)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Post("/repos/kryptonians/krypton/check-suites/100/rerequest", rerequestmd.ServeHTTP)

	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient)
	require.NoError(t, err)
	rerequestErr := githubClient.ReRequestLatestCheckSuite("abcd")
	require.NoError(t, rerequestErr)
}

func TestGetDescription(t *testing.T) {
	body := "test test test"
	respBody := []byte(testutils.MarshalJSON(t, &github.Issue{Body: &body}))
	r := chi.NewRouter()
	md := mockhttpclient.MockGetDialogue(respBody)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api.github.com")).
		Get("/repos/kryptonians/krypton/issues/12345", md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient)
	require.NoError(t, err)
	desc, err := githubClient.GetDescription(12345)
	require.NoError(t, err)
	require.Equal(t, body, desc)
}

func TestReadRawFileRequest(t *testing.T) {
	respBody := []byte(`abcd`)
	r := chi.NewRouter()
	md := mockhttpclient.MockGetDialogue(respBody)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("raw.githubusercontent.com")).
		Get("/kryptonians/krypton/main//dummy/path/to/this.txt", md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient)
	require.NoError(t, err)
	contents, readRawErr := githubClient.ReadRawFile(git.MainBranch, "/dummy/path/to/this.txt")
	require.NoError(t, readRawErr)
	require.Equal(t, "abcd", contents)
}

func TestGetFullHistoryUrl(t *testing.T) {
	httpClient := mockhttpclient.NewMuxClient(chi.NewRouter())
	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient)
	require.NoError(t, err)
	fullHistoryUrl := githubClient.GetFullHistoryUrl("superman@krypton.com")
	require.Equal(t, "https://github.com/kryptonians/krypton/pulls/superman", fullHistoryUrl)
}

func TestGetPullRequestUrlBase(t *testing.T) {
	httpClient := mockhttpclient.NewMuxClient(chi.NewRouter())
	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient)
	require.NoError(t, err)
	pullRequestUrlBase := githubClient.GetPullRequestUrlBase()
	require.Equal(t, "https://github.com/kryptonians/krypton/pull/", pullRequestUrlBase)
}

func TestGetIssueUrlBase(t *testing.T) {
	httpClient := mockhttpclient.NewMuxClient(chi.NewRouter())
	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient)
	require.NoError(t, err)
	issueUrlBase := githubClient.GetIssueUrlBase()
	require.Equal(t, "https://github.com/kryptonians/krypton/issues/", issueUrlBase)
}

func TestParseRepoOwnerAndName(t *testing.T) {
	testCases := []struct {
		url         string
		repoOwner   string
		repoName    string
		expectErr   bool
		errContains string
		description string
	}{
		{
			url:         "https://github.com/google/skia",
			repoOwner:   "google",
			repoName:    "skia",
			expectErr:   false,
			description: "https URL",
		},
		{
			url:         "git@github.com:google/skia.git",
			repoOwner:   "google",
			repoName:    "skia",
			expectErr:   false,
			description: "git URL",
		},
		{
			url:         "https://github.com/some-user/some-repo.git",
			repoOwner:   "some-user",
			repoName:    "some-repo",
			expectErr:   false,
			description: "hyphenated user and repo",
		},
		{
			url:         "https://github.com/some_user/some_repo.git",
			repoOwner:   "some_user",
			repoName:    "some_repo",
			expectErr:   false,
			description: "underscored user and repo",
		},
		{
			url:         "https://github.com/some.user/some.repo.git",
			repoOwner:   "some.user",
			repoName:    "some.repo",
			expectErr:   false,
			description: "dotted user and repo",
		},
		{
			url:         "https://not-github.com/google/skia",
			expectErr:   true,
			errContains: "failed to parse repo URL",
			description: "invalid URL",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			repoOwner, repoName, err := ParseRepoOwnerAndName(tc.url)
			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errContains)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.repoOwner, repoOwner)
				require.Equal(t, tc.repoName, repoName)
			}
		})
	}
}

func TestCommitToLongCommit(t *testing.T) {
	ts := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	testCases := []struct {
		description string
		commit      *github.RepositoryCommit
		expected    *vcsinfo.LongCommit
	}{
		{
			description: "Simple commit",
			commit: &github.RepositoryCommit{
				SHA: github.String("abcdef123456"),
				Commit: &github.Commit{
					Author: &github.CommitAuthor{
						Name:  github.String("Test User"),
						Email: github.String("test@google.com"),
					},
					Committer: &github.CommitAuthor{
						Date: &ts,
					},
					Message: github.String("Subject"),
				},
			},
			expected: &vcsinfo.LongCommit{
				ShortCommit: &vcsinfo.ShortCommit{
					Hash:    "abcdef123456",
					Author:  "Test User (test@google.com)",
					Subject: "Subject",
				},
				Parents:   []string{},
				Body:      "",
				Timestamp: ts,
			},
		},
		{
			description: "Commit with body",
			commit: &github.RepositoryCommit{
				SHA: github.String("abcdef123456"),
				Commit: &github.Commit{
					Author: &github.CommitAuthor{
						Name:  github.String("Test User"),
						Email: github.String("test@google.com"),
					},
					Committer: &github.CommitAuthor{
						Date: &ts,
					},
					Message: github.String("Subject\n\nThis is the body."),
				},
			},
			expected: &vcsinfo.LongCommit{
				ShortCommit: &vcsinfo.ShortCommit{
					Hash:    "abcdef123456",
					Author:  "Test User (test@google.com)",
					Subject: "Subject",
				},
				Parents:   []string{},
				Body:      "This is the body.",
				Timestamp: ts,
			},
		},
		{
			description: "Commit with parents",
			commit: &github.RepositoryCommit{
				SHA: github.String("abcdef123456"),
				Commit: &github.Commit{
					Author: &github.CommitAuthor{
						Name:  github.String("Test User"),
						Email: github.String("test@google.com"),
					},
					Committer: &github.CommitAuthor{
						Date: &ts,
					},
					Message: github.String("Subject"),
				},
				Parents: []github.Commit{
					{
						SHA: github.String("parent1"),
					},
					{
						SHA: github.String("parent2"),
					},
				},
			},
			expected: &vcsinfo.LongCommit{
				ShortCommit: &vcsinfo.ShortCommit{
					Hash:    "abcdef123456",
					Author:  "Test User (test@google.com)",
					Subject: "Subject",
				},
				Parents:   []string{"parent1", "parent2"},
				Body:      "",
				Timestamp: ts,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			actual := CommitToLongCommit(tc.commit)
			require.Equal(t, tc.expected, actual)
		})
	}
}
