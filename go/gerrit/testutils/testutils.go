package testutils

import (
	"encoding/json"
	"fmt"

	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.skia.org/infra/go/buildbucket/bb_testutils"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
)

const (
	// FakeGerritURL is a fake Gerrit URL.
	FakeGerritURL = "https://fake-skia-review.googlesource.com"
	// FakeGitCookies are fake .gitcookies contents.
	FakeGitCookies = ".googlesource.com\tTRUE\t/\tTRUE\t123\to\tgit-user.google.com=abc123"
	// FakeChangeId is the Change ID used for changes uploaded to MockGerrit.
	FakeChangeId = "123"
)

// MockGerrit is a GerritInterface implementation which mocks out requests to
// the server.
type MockGerrit struct {
	bb     *bb_testutils.MockClient
	Gerrit *gerrit.Gerrit
	Mock   *mockhttpclient.URLMock
	t      sktest.TestingT
}

// NewGerrit returns a mocked Gerrit instance.
func NewGerrit(t sktest.TestingT) *MockGerrit {
	return NewGerritWithConfig(t, gerrit.ConfigChromium)
}

// NewGerritWithConfig returns a mocked Gerrit instance which uses the given
// Config.
func NewGerritWithConfig(t sktest.TestingT, cfg *gerrit.Config) *MockGerrit {
	mock := mockhttpclient.NewURLMock()
	g, err := gerrit.NewGerritWithConfig(cfg, FakeGerritURL, mock.Client())
	require.NoError(t, err)
	bb := bb_testutils.NewMockClient(t)
	g.BuildbucketClient = bb.Client
	return &MockGerrit{
		bb:     bb,
		Gerrit: g,
		Mock:   mock,
		t:      t,
	}
}

// AssertEmpty asserts that the URLMock instance is empty.
func (g *MockGerrit) AssertEmpty() {
	require.True(g.t, g.Mock.Empty())
}

// MockGetUserEmail mocks the requests for GetUserEmail.
func (g *MockGerrit) MockGetUserEmail(acc *gerrit.AccountDetails) {
	url := FakeGerritURL + "/a/accounts/self/detail"
	serialized, err := json.Marshal(acc)
	require.NoError(g.t, err)
	serialized = append([]byte(")]}'\n"), serialized...)
	g.Mock.MockOnce(url, mockhttpclient.MockGetDialogue(serialized))
}

// MockGetIssueProperties mocks the requests for GetIssueProperties.
func (g *MockGerrit) MockGetIssueProperties(ci *gerrit.ChangeInfo) {
	url := FakeGerritURL + "/a" + fmt.Sprintf(gerrit.URLTmplChange, fmt.Sprintf("%d", ci.Issue))
	serialized, err := json.Marshal(ci)
	require.NoError(g.t, err)
	serialized = append([]byte(")]}'\n"), serialized...)
	g.Mock.MockOnce(url, mockhttpclient.MockGetDialogue(serialized))
}

// MockGetTrybotResults mocks the requests for GetIssueProperties.
func (g *MockGerrit) MockGetTrybotResults(ci *gerrit.ChangeInfo, patchset int, results []*buildbucketpb.Build) {
	g.bb.MockGetTrybotsForCL(ci.Issue, int64(patchset), g.Gerrit.Url(ci.Issue), results, nil)
}

// MakePostRequest creates a POST request to Gerrit for mocking.
func (g *MockGerrit) MakePostRequest(ci *gerrit.ChangeInfo, msg string, labels map[string]int, reviewers []string) (string, []byte) {
	url := fmt.Sprintf("%s/a/changes/%s~%s~%s/revisions/%d/review", FakeGerritURL, ci.Project, ci.Branch, ci.ChangeId, len(ci.Revisions))
	if labels == nil {
		labels = map[string]int{}
	}
	req := struct {
		Labels    map[string]int     `json:"labels"`
		Message   string             `json:"message"`
		Reviewers []*gerrit.Reviewer `json:"reviewers"`
	}{
		Labels:    labels,
		Message:   msg,
		Reviewers: []*gerrit.Reviewer{},
	}
	for _, reviewer := range reviewers {
		req.Reviewers = append(req.Reviewers, &gerrit.Reviewer{
			Reviewer: reviewer,
		})
	}
	reqBytes := testutils.MarshalJSON(g.t, req)
	return url, []byte(reqBytes)
}

// MockPost mocks a POST request to the given change with the given message and
// labels.
func (g *MockGerrit) MockPost(ci *gerrit.ChangeInfo, msg string, labels map[string]int, reviewers []string) {
	url, reqBytes := g.MakePostRequest(ci, msg, labels, reviewers)
	g.Mock.MockOnce(url, mockhttpclient.MockPostDialogue("application/json", reqBytes, []byte("")))
}

// MockAddComment mocks addition of a comment to the change.
func (g *MockGerrit) MockAddComment(ci *gerrit.ChangeInfo, msg string, reviewers []string) {
	g.MockPost(ci, msg, nil, reviewers)
}

// MockSetDryRun mocks the setting of the dry run labels.
func (g *MockGerrit) MockSetDryRun(ci *gerrit.ChangeInfo, msg string, reviewers []string) {
	g.MockPost(ci, msg, gerrit.MergeLabels(g.Gerrit.Config().SetDryRunLabels, g.Gerrit.Config().SelfApproveLabels), reviewers)
}

// MockSetCQ mocks the setting of the commit queue labels.
func (g *MockGerrit) MockSetCQ(ci *gerrit.ChangeInfo, msg string, reviewers []string) {
	g.MockPost(ci, msg, gerrit.MergeLabels(g.Gerrit.Config().SetCqLabels, g.Gerrit.Config().SelfApproveLabels), reviewers)
}

// Abandon mocks the request to abandon the change.
func (g *MockGerrit) Abandon(ci *gerrit.ChangeInfo, msg string) {
	url := fmt.Sprintf("%s/a/changes/%d/abandon", FakeGerritURL, ci.Issue)
	req := struct {
		Message string `json:"message"`
	}{
		Message: msg,
	}
	reqBytes := testutils.MarshalJSON(g.t, req)
	g.Mock.MockOnce(url, mockhttpclient.MockPostDialogue("application/json", []byte(reqBytes), []byte("")))
}

// MockDownloadCommitMsgHook mocks the request to download the commit message
// hook.
func (g *MockGerrit) MockDownloadCommitMsgHook() {
	url := fmt.Sprintf("%s/a/tools/hooks/commit-msg", FakeGerritURL)
	respBody := []byte(fmt.Sprintf(`#!/bin/sh
git interpret-trailers --trailer "Change-Id: %s" >> $1
`, FakeChangeId))
	g.Mock.MockOnce(url, mockhttpclient.MockGetDialogue(respBody))
}
