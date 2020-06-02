package testutils

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.skia.org/infra/go/buildbucket/bb_testutils"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
)

const (
	FakeAccountID  = 101
	FakeGerritURL  = "https://fake-skia-review.googlesource.com"
	FakeGitCookies = ".googlesource.com\tTRUE\t/\tTRUE\t123\to\tgit-user.google.com=abc123"
	FakeUser       = "user@chromium.org"
)

// MockGerrit is a GerritInterface implementation which mocks out requests to
// the server.
type MockGerrit struct {
	bb           *bb_testutils.MockClient
	Gerrit       *gerrit.Gerrit
	Mock         *mockhttpclient.URLMock
	nextChangeID int64
	t            sktest.TestingT
}

// NewGerrit returns a mocked Gerrit instance.
func NewGerrit(t sktest.TestingT, urlmock *mockhttpclient.URLMock) *MockGerrit {
	return NewGerritWithConfig(t, gerrit.CONFIG_CHROMIUM, urlmock)
}

// NewGerritWithConfig returns a mocked Gerrit instance which uses the given
// Config.
func NewGerritWithConfig(t sktest.TestingT, cfg *gerrit.Config, urlmock *mockhttpclient.URLMock) *MockGerrit {
	g, err := gerrit.NewGerritWithConfig(cfg, FakeGerritURL, urlmock.Client())
	require.NoError(t, err)
	bb := bb_testutils.NewMockClient(t)
	g.BuildbucketClient = bb.Client
	return &MockGerrit{
		bb:           bb,
		Gerrit:       g,
		Mock:         urlmock,
		nextChangeID: 123,
		t:            t,
	}
}

func (g *MockGerrit) AssertEmpty() {
	require.True(g.t, g.Mock.Empty())
}

func (g *MockGerrit) MockGetIssueProperties(ci *gerrit.ChangeInfo) {
	url := fmt.Sprintf("%s/a"+gerrit.URL_TMPL_CHANGE, FakeGerritURL, ci.Id)
	serialized, err := json.Marshal(ci)
	require.NoError(g.t, err)
	serialized = append([]byte(")]}'\n"), serialized...)
	g.Mock.MockOnce(url, mockhttpclient.MockGetDialogue(serialized))
}

func (g *MockGerrit) MockGetTrybotResults(ci *gerrit.ChangeInfo, patchset int, results []*buildbucketpb.Build) {
	g.bb.MockGetTrybotsForCL(ci.Issue, int64(patchset), g.Gerrit.Url(ci.Issue), results, nil)
}

func (g *MockGerrit) MakePostRequest(ci *gerrit.ChangeInfo, msg string, labels map[string]int, reviewers []string) (string, []byte) {
	url := fmt.Sprintf("%s/a"+gerrit.URL_TMPL_SET_REVIEW, FakeGerritURL, ci.Id, strconv.Itoa(len(ci.Revisions)))
	if labels == nil {
		labels = map[string]int{}
	}
	var reviewersMaps []map[string]string
	if reviewers != nil {
		for _, reviewer := range reviewers {
			reviewersMaps = append(reviewersMaps, map[string]string{"reviewer": reviewer})
		}
	}
	req := struct {
		Labels    map[string]int      `json:"labels"`
		Message   string              `json:"message"`
		Reviewers []map[string]string `json:"reviewers,omitempty"`
	}{
		Labels:    labels,
		Message:   msg,
		Reviewers: reviewersMaps,
	}
	reqBytes := testutils.MarshalJSON(g.t, req)
	return url, []byte(reqBytes)
}

func (g *MockGerrit) MockPost(ci *gerrit.ChangeInfo, msg string, labels map[string]int, reviewers []string) {
	url, reqBytes := g.MakePostRequest(ci, msg, labels, reviewers)
	g.Mock.MockOnce(url, mockhttpclient.MockPostDialogue("application/json", reqBytes, []byte("")))
}

func (g *MockGerrit) MockAddComment(ci *gerrit.ChangeInfo, msg string) {
	g.MockPost(ci, msg, nil, nil)
}

func (g *MockGerrit) MockSetDryRun(ci *gerrit.ChangeInfo, msg string) {
	g.MockPost(ci, msg, g.Gerrit.Config().SetDryRunLabels, nil)
}

func (g *MockGerrit) MockSetCQ(ci *gerrit.ChangeInfo, msg string) {
	g.MockPost(ci, msg, g.Gerrit.Config().SetCqLabels, nil)
}

func (g *MockGerrit) MockSubmit(ci *gerrit.ChangeInfo) {
	g.Mock.MockOnce(fmt.Sprintf("%s/a"+gerrit.URL_TMPL_SUBMIT, FakeGerritURL, ci.Id), mockhttpclient.MockPostDialogue("application/json", []byte("{}"), []byte("")))
}

func (g *MockGerrit) MockAbandon(ci *gerrit.ChangeInfo, msg string) {
	url := fmt.Sprintf("%s/a"+gerrit.URL_TMPL_ABANDON, FakeGerritURL, ci.Id)
	req := struct {
		Message string `json:"message"`
	}{
		Message: msg,
	}
	reqBytes := testutils.MarshalJSON(g.t, req)
	g.Mock.MockOnce(url, mockhttpclient.MockPostDialogue("application/json", []byte(reqBytes), []byte("")))
}

func (g *MockGerrit) MockGetUserEmail() {
	serialized, err := json.Marshal(&gerrit.AccountDetails{
		AccountId: FakeAccountID,
		Name:      FakeUser,
		Email:     FakeUser,
		UserName:  FakeUser,
	})
	require.NoError(g.t, err)
	serialized = append([]byte("abcd\n"), serialized...)
	g.Mock.MockOnce(fmt.Sprintf("%s/a"+gerrit.URL_SELF_DETAIL, FakeGerritURL), mockhttpclient.MockGetDialogue(serialized))
}

func (g *MockGerrit) MockCreateChange(commitMsg, targetBranch, baseCommit string, changes map[string]string) *gerrit.ChangeInfo {
	changeID := g.nextChangeID
	g.nextChangeID++

	// Mock the initial change creation.
	subject := strings.Split(commitMsg, "\n")[0]
	reqBody := []byte(fmt.Sprintf(`{"project":"%s","subject":"%s","branch":"%s","topic":"","status":"NEW","base_commit":"%s"}`, "fake-gerrit-project", subject, targetBranch, baseCommit))
	changeIDStr := fmt.Sprintf("%d", changeID)
	ci := &gerrit.ChangeInfo{
		ChangeId: changeIDStr,
		Id:       changeIDStr,
		Issue:    changeID,
		Revisions: map[string]*gerrit.Revision{
			"ps1": {
				ID:     "ps1",
				Number: 1,
			},
		},
	}
	respBody, err := json.Marshal(ci)
	require.NoError(g.t, err)
	respBody = append([]byte(")]}'\n"), respBody...)
	g.Mock.MockOnce(fmt.Sprintf("%s/a"+gerrit.URL_CREATE_CHANGE, FakeGerritURL), mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, 201))

	// Mock the edit of the change to update the commit message.
	reqBody = []byte(fmt.Sprintf(`{"message":"%s"}`, strings.Replace(commitMsg, "\n", "\\n", -1)))
	g.Mock.MockOnce(fmt.Sprintf("%s/a"+gerrit.URL_TMPL_CHANGE_EDIT_MESSAGE, FakeGerritURL, changeIDStr), mockhttpclient.MockPutDialogue("application/json", reqBody, []byte("")))

	// Mock the requests to modify the files.
	for file, contents := range changes {
		url := fmt.Sprintf("%s/a"+gerrit.URL_TMPL_CHANGE_EDIT_FILE, FakeGerritURL, changeIDStr, url.QueryEscape(file))
		g.Mock.MockOnce(url, mockhttpclient.MockPutDialogue("", []byte(contents), []byte("")))
	}

	// Mock the request to publish the change edit.
	reqBody = []byte(`{"notify":"ALL"}`)
	g.Mock.MockOnce(fmt.Sprintf("%s/a"+gerrit.URL_TMPL_CHANGE_EDIT_PUBLISH, FakeGerritURL, changeIDStr), mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	// Mock the request to load the updated change.
	g.MockGetIssueProperties(ci)
	sklog.Errorf("%#v", g.Mock.List())
	return ci
}
