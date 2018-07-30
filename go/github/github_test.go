package github

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/go-github/github"
	"github.com/gorilla/mux"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
)

func TestAddComment(t *testing.T) {
	testutils.SmallTest(t)
	reqType := "application/json"
	reqBody := []byte(`{"body":"test msg"}
`)
	r := mux.NewRouter()
	md := mockhttpclient.MockPostDialogueWithResponseCode(reqType, reqBody, nil, http.StatusCreated)
	r.Schemes("https").Host("api.github.com").Methods("POST").Path("/repos/kryptonians/krypton/issues/1234/comments").Handler(md)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient, "")
	assert.NoError(t, err)
	addCommentErr := githubClient.AddComment(1234, "test msg")
	assert.NoError(t, addCommentErr)
}

func TestGetAuthenticatedUser(t *testing.T) {
	testutils.SmallTest(t)
	r := mux.NewRouter()
	md := mockhttpclient.MockGetError("OK", http.StatusOK)
	r.Schemes("https").Host("api.github.com").Methods("GET").Path("/user").Handler(md)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient, "")
	assert.NoError(t, err)
	_, getUserErr := githubClient.GetAuthenticatedUser()
	assert.NoError(t, getUserErr)
}

func TestGetPullRequest(t *testing.T) {
	testutils.SmallTest(t)
	respBody := []byte(testutils.MarshalJSON(t, &github.PullRequest{State: &CLOSED_STATE}))
	r := mux.NewRouter()
	md := mockhttpclient.MockGetDialogue(respBody)
	r.Schemes("https").Host("api.github.com").Methods("GET").Path("/repos/kryptonians/krypton/pulls/1234").Handler(md)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient, "")
	assert.NoError(t, err)
	pr, getPullErr := githubClient.GetPullRequest(1234)
	assert.NoError(t, getPullErr)
	assert.Equal(t, CLOSED_STATE, *pr.State)
}

func TestCreatePullRequest(t *testing.T) {
	testutils.SmallTest(t)
	reqType := "application/json"
	reqBody := []byte(`{"title":"title","head":"headBranch","base":"baseBranch","body":"testBody"}
`)
	number := 12345
	respBody := []byte(testutils.MarshalJSON(t, &github.PullRequest{Number: &number}))
	r := mux.NewRouter()
	md := mockhttpclient.MockPostDialogueWithResponseCode(reqType, reqBody, respBody, http.StatusCreated)
	r.Schemes("https").Host("api.github.com").Methods("POST").Path("/repos/kryptonians/krypton/pulls").Handler(md)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient, "")
	assert.NoError(t, err)
	pullRequest, createPullErr := githubClient.CreatePullRequest("title", "baseBranch", "headBranch", "testBody")
	assert.NoError(t, createPullErr)
	assert.Equal(t, number, *pullRequest.Number)
}

func TestMergePullRequest(t *testing.T) {
	testutils.SmallTest(t)
	reqType := "application/json"
	reqBody := []byte(`{"commit_message":"test comment","merge_method":"squash"}
`)
	r := mux.NewRouter()
	md := mockhttpclient.MockPutDialogue(reqType, reqBody, nil)
	r.Schemes("https").Host("api.github.com").Methods("PUT").Path("/repos/kryptonians/krypton/pulls/1234/merge").Handler(md)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient, "")
	assert.NoError(t, err)
	mergePullErr := githubClient.MergePullRequest(1234, "test comment", "squash")
	assert.NoError(t, mergePullErr)
}

func TestClosePullRequest(t *testing.T) {
	testutils.SmallTest(t)
	respBody := []byte(testutils.MarshalJSON(t, &github.PullRequest{State: &CLOSED_STATE}))
	reqType := "application/json"
	reqBody := []byte(`{"state":"closed"}
`)
	r := mux.NewRouter()
	md := mockhttpclient.MockPatchDialogue(reqType, reqBody, respBody)
	r.Schemes("https").Host("api.github.com").Methods("PATCH").Path("/repos/kryptonians/krypton/pulls/1234").Handler(md)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient, "")
	assert.NoError(t, err)
	pr, closePullErr := githubClient.ClosePullRequest(1234)
	assert.NoError(t, closePullErr)
	assert.Equal(t, CLOSED_STATE, *pr.State)
}

func TestGetLabelsRequest(t *testing.T) {
	testutils.SmallTest(t)
	label1Name := "test1"
	label2Name := "test2"
	label1 := github.Label{Name: &label1Name}
	label2 := github.Label{Name: &label2Name}
	respBody := []byte(testutils.MarshalJSON(t, &github.PullRequest{Labels: []*github.Label{&label1, &label2}}))
	r := mux.NewRouter()
	md := mockhttpclient.MockGetDialogue(respBody)
	r.Schemes("https").Host("api.github.com").Methods("GET").Path("/repos/kryptonians/krypton/issues/1234").Handler(md)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient, "")
	assert.NoError(t, err)
	labels, getLabelsErr := githubClient.GetLabels(1234)
	assert.NoError(t, getLabelsErr)
	assert.Equal(t, []string{label1Name, label2Name}, labels)
}

func TestAddLabelRequest(t *testing.T) {
	testutils.SmallTest(t)
	label1Name := "test1"
	label2Name := "test2"
	label1 := github.Label{Name: &label1Name}
	label2 := github.Label{Name: &label2Name}
	respBody := []byte(testutils.MarshalJSON(t, &github.PullRequest{Labels: []*github.Label{&label1, &label2}}))
	r := mux.NewRouter()
	md := mockhttpclient.MockGetDialogue(respBody)
	r.Schemes("https").Host("api.github.com").Methods("GET").Path("/repos/kryptonians/krypton/issues/1234").Handler(md)

	patchRespBody := []byte(testutils.MarshalJSON(t, &github.PullRequest{}))
	patchReqType := "application/json"
	patchReqBody := []byte(`{"labels":["test1","test2","test3"]}
`)
	patchMd := mockhttpclient.MockPatchDialogue(patchReqType, patchReqBody, patchRespBody)
	r.Schemes("https").Host("api.github.com").Methods("PATCH").Path("/repos/kryptonians/krypton/issues/1234").Handler(patchMd)

	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient, "")
	assert.NoError(t, err)
	addLabelErr := githubClient.AddLabel(1234, "test3")
	assert.NoError(t, addLabelErr)
}

func TestReplaceLabelRequest(t *testing.T) {
	testutils.SmallTest(t)
	label1Name := "test1"
	label2Name := "test2"
	label1 := github.Label{Name: &label1Name}
	label2 := github.Label{Name: &label2Name}
	respBody := []byte(testutils.MarshalJSON(t, &github.PullRequest{Labels: []*github.Label{&label1, &label2}}))
	r := mux.NewRouter()
	md := mockhttpclient.MockGetDialogue(respBody)
	r.Schemes("https").Host("api.github.com").Methods("GET").Path("/repos/kryptonians/krypton/issues/1234").Handler(md)

	patchRespBody := []byte(testutils.MarshalJSON(t, &github.PullRequest{}))
	patchReqType := "application/json"
	patchReqBody := []byte(`{"labels":["test2","test3"]}
`)
	patchMd := mockhttpclient.MockPatchDialogue(patchReqType, patchReqBody, patchRespBody)
	r.Schemes("https").Host("api.github.com").Methods("PATCH").Path("/repos/kryptonians/krypton/issues/1234").Handler(patchMd)

	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient, "")
	assert.NoError(t, err)
	removeLabelErr := githubClient.ReplaceLabel(1234, "test1", "test3")
	assert.NoError(t, removeLabelErr)
}

func TestGetChecksRequest(t *testing.T) {
	testutils.SmallTest(t)
	statusID1 := int64(100)
	statusID2 := int64(200)
	repoStatus1 := github.RepoStatus{ID: &statusID1}
	repoStatus2 := github.RepoStatus{ID: &statusID2}
	respBody := []byte(testutils.MarshalJSON(t, &github.CombinedStatus{Statuses: []github.RepoStatus{repoStatus1, repoStatus2}}))
	r := mux.NewRouter()
	md := mockhttpclient.MockGetDialogue(respBody)
	r.Schemes("https").Host("api.github.com").Methods("GET").Path("/repos/kryptonians/krypton/commits/abcd/status").Handler(md)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient, "")
	assert.NoError(t, err)
	checks, getChecksErr := githubClient.GetChecks("abcd")
	assert.NoError(t, getChecksErr)
	assert.Equal(t, 2, len(checks))
	assert.Equal(t, statusID1, *checks[0].ID)
	assert.Equal(t, statusID2, *checks[1].ID)
}

func TestReadRawFileRequest(t *testing.T) {
	testutils.SmallTest(t)
	respBody := []byte(`abcd`)
	r := mux.NewRouter()
	md := mockhttpclient.MockGetDialogue(respBody)
	r.Schemes("https").Host("raw.githubusercontent.com").Methods("GET").Path("/kryptonians/krypton/master/dummy/path/to/this.txt").Handler(md)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient, "")
	assert.NoError(t, err)
	contents, readRawErr := githubClient.ReadRawFile("master", "/dummy/path/to/this.txt")
	assert.NoError(t, readRawErr)
	assert.Equal(t, "abcd", contents)
}

func TestGetFullHistoryUrl(t *testing.T) {
	testutils.SmallTest(t)
	httpClient := mockhttpclient.NewMuxClient(mux.NewRouter())
	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient, "")
	assert.NoError(t, err)
	fullHistoryUrl := githubClient.GetFullHistoryUrl("superman@krypton.com")
	assert.Equal(t, "https://github.com/kryptonians/krypton/pulls/superman", fullHistoryUrl)
}

func TestGetIssueUrlBase(t *testing.T) {
	testutils.SmallTest(t)
	httpClient := mockhttpclient.NewMuxClient(mux.NewRouter())
	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", httpClient, "")
	assert.NoError(t, err)
	issueUrlBase := githubClient.GetIssueUrlBase()
	assert.Equal(t, "https://github.com/kryptonians/krypton/pull/", issueUrlBase)
}
