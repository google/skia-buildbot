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

func TestCreatePullRequest(t *testing.T) {
	testutils.SmallTest(t)
	reqType := "application/json"
	reqBody := []byte(`{"title":"title","head":"headBranch","base":"baseBranch"}
`)
	r := mux.NewRouter()
	md := mockhttpclient.MockPostError(reqType, reqBody, "OK", http.StatusCreated)
	r.Schemes("https").Host("api.github.com").Methods("POST").Path("/repos/kryptonians/krypton/pulls").Handler(md)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", "dummy-access-token")
	assert.NoError(t, err)
	githubClient.client = github.NewClient(httpClient)
	_, createPullErr := githubClient.CreatePullRequest("title", "baseBranch", "headBranch")
	assert.NoError(t, createPullErr)
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

	githubClient, err := NewGitHub(context.Background(), "kryptonians", "krypton", "dummy-access-token")
	assert.NoError(t, err)
	githubClient.client = github.NewClient(httpClient)
	_, closePullErr := githubClient.ClosePullRequest(1234)
	assert.NoError(t, closePullErr)
}
