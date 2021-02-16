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
func NewGerrit(t sktest.TestingT, workdir string) *MockGerrit {
	return NewGerritWithConfig(t, gerrit.ConfigChromium, workdir)
}

// NewGerritWithConfig returns a mocked Gerrit instance which uses the given
// Config.
func NewGerritWithConfig(t sktest.TestingT, cfg *gerrit.Config, workdir string) *MockGerrit {
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
func (g *MockGerrit) MakePostRequest(ci *gerrit.ChangeInfo, msg string, labels map[string]int) (string, []byte) {
	url := fmt.Sprintf("%s/a/changes/%d/revisions/%d/review", FakeGerritURL, ci.Issue, len(ci.Revisions))
	if labels == nil {
		labels = map[string]int{}
	}
	req := struct {
		Labels  map[string]int `json:"labels"`
		Message string         `json:"message"`
	}{
		Labels:  labels,
		Message: msg,
	}
	reqBytes := testutils.MarshalJSON(g.t, req)
	return url, []byte(reqBytes)
}

// MockPost mocks a POST request to the given change with the given message and
// labels.
func (g *MockGerrit) MockPost(ci *gerrit.ChangeInfo, msg string, labels map[string]int) {
	url, reqBytes := g.MakePostRequest(ci, msg, labels)
	g.Mock.MockOnce(url, mockhttpclient.MockPostDialogue("application/json", reqBytes, []byte("")))
}

// MockAddComment mocks addition of a comment to the change.
func (g *MockGerrit) MockAddComment(ci *gerrit.ChangeInfo, msg string) {
	g.MockPost(ci, msg, nil)
}

// MockSetDryRun mocks the setting of the dry run labels.
func (g *MockGerrit) MockSetDryRun(ci *gerrit.ChangeInfo, msg string) {
	g.MockPost(ci, msg, g.Gerrit.Config().SetDryRunLabels)
}

// MockSetCQ mocks the setting of the commit queue labels.
func (g *MockGerrit) MockSetCQ(ci *gerrit.ChangeInfo, msg string) {
	g.MockPost(ci, msg, g.Gerrit.Config().SetCqLabels)
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
