package repo_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	ftChildPath   = "src/third_party/freetype/src"
	ftIncludeTmpl = `%s header %d


commit %d
`
	ftReadmeTmpl = `Fake README.chromium
blah blah
Version: %s
Revision: %s
blah blah`
	ftVersionTmpl = "v0.0.%d"
)

func setupFreeType(t *testing.T) (context.Context, string, RepoManager, *git_testutils.GitBuilder, *git_testutils.GitBuilder, *gitiles_testutils.MockRepo, *gitiles_testutils.MockRepo, []string, *mockhttpclient.URLMock, func()) {
	unittest.LargeTest(t)

	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	ctx := context.Background()

	// Create child and parent repos.
	child := git_testutils.GitInit(t, ctx)
	childCommits := make([]string, 0, 10)
	for i := 0; i < numChildCommits; i++ {
		for idx, h := range parent.FtIncludesToMerge {
			child.Add(ctx, path.Join(parent.FtIncludeSrc, h), fmt.Sprintf(ftIncludeTmpl, "child", idx, i))
		}
		childCommits = append(childCommits, child.Commit(ctx))
		_, err = git.GitDir(child.Dir()).Git(ctx, "tag", "-a", fmt.Sprintf(ftVersionTmpl, i), "-m", fmt.Sprintf("Version %d", i))
		require.NoError(t, err)
	}

	urlmock := mockhttpclient.NewURLMock()

	mockChild := gitiles_testutils.NewMockRepo(t, child.RepoUrl(), git.GitDir(child.Dir()), urlmock)

	parentRepo := git_testutils.GitInit(t, ctx)
	parentRepo.Add(ctx, "DEPS", fmt.Sprintf(`deps = {
  "%s": "%s@%s",
}`, ftChildPath, child.RepoUrl(), childCommits[0]))
	parentRepo.Add(ctx, parent.FtReadmePath, fmt.Sprintf(ftReadmeTmpl, fmt.Sprintf(ftVersionTmpl, 0), childCommits[0]))
	for idx, h := range parent.FtIncludesToMerge {
		parentRepo.Add(ctx, path.Join(parent.FtIncludeDest, h), fmt.Sprintf(ftIncludeTmpl, "parent", idx, 0))
	}
	parentRepo.Commit(ctx)

	mockParent := gitiles_testutils.NewMockRepo(t, parentRepo.RepoUrl(), git.GitDir(parentRepo.Dir()), urlmock)

	gUrl := "https://fake-skia-review.googlesource.com"
	serialized, err := json.Marshal(&gerrit.AccountDetails{
		AccountId: 101,
		Name:      mockUser,
		Email:     mockUser,
		UserName:  mockUser,
	})
	require.NoError(t, err)
	serialized = append([]byte("abcd\n"), serialized...)
	urlmock.MockOnce(gUrl+"/a/accounts/self/detail", mockhttpclient.MockGetDialogue(serialized))
	g, err := gerrit.NewGerrit(gUrl, urlmock.Client())
	require.NoError(t, err)

	cfg := &config.FreeTypeRepoManagerConfig{
		Parent: &config.FreeTypeParentConfig{
			Gitiles: &config.GitilesParentConfig{
				Gitiles: &config.GitilesConfig{
					Branch:  git.MainBranch,
					RepoUrl: parentRepo.RepoUrl(),
				},
				Dep: &config.DependencyConfig{
					Primary: &config.VersionFileConfig{
						Id:   child.RepoUrl(),
						Path: deps_parser.DepsFileName,
					},
				},
				Gerrit: &config.GerritConfig{
					Url:     "https://fake-skia-review.googlesource.com",
					Project: "fake-gerrit-project",
					Config:  config.GerritConfig_CHROMIUM,
				},
			},
		},
		Child: &config.GitilesChildConfig{
			Gitiles: &config.GitilesConfig{
				Branch:  git.MainBranch,
				RepoUrl: child.RepoUrl(),
			},
		},
	}

	rm, err := NewFreeTypeRepoManager(ctx, cfg, setupRegistry(t), wd, "fake.server.com", urlmock.Client(), gerritCR(t, g, urlmock.Client()), false)
	require.NoError(t, err)

	// Mock requests for Update().
	mockParent.MockGetCommit(ctx, git.MainBranch)
	parentHead, err := git.GitDir(parentRepo.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentHead)
	mockChild.MockGetCommit(ctx, git.MainBranch)
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], childCommits[len(childCommits)-1]))
	for _, hash := range childCommits {
		mockChild.MockGetCommit(ctx, hash)
	}
	// Update.
	_, _, _, err = rm.Update(ctx)
	require.NoError(t, err)

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parentRepo.Cleanup()
		require.True(t, urlmock.Empty(), strings.Join(urlmock.List(), "\n"))
	}
	return ctx, wd, rm, child, parentRepo, mockChild, mockParent, childCommits, urlmock, cleanup
}

func TestFreeTypeRepoManagerUpdate(t *testing.T) {
	ctx, _, rm, _, parentRepo, mockChild, mockParent, childCommits, _, cleanup := setupFreeType(t)
	defer cleanup()

	// Mock requests for Update().
	mockParent.MockGetCommit(ctx, git.MainBranch)
	parentHead, err := git.GitDir(parentRepo.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentHead)
	mockChild.MockGetCommit(ctx, git.MainBranch)
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], childCommits[len(childCommits)-1]))
	for _, hash := range childCommits {
		mockChild.MockGetCommit(ctx, hash)
	}
	// Update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, lastRollRev.Id, childCommits[0])
	require.Equal(t, tipRev.Id, childCommits[len(childCommits)-1])
	require.Equal(t, len(notRolledRevs), len(childCommits)-1)
}

func TestFreeTypeRepoManagerCreateNewRoll(t *testing.T) {
	ctx, _, rm, childRepo, parentRepo, mockChild, mockParent, childCommits, urlmock, cleanup := setupFreeType(t)
	defer cleanup()

	// Mock requests for Update().
	mockParent.MockGetCommit(ctx, git.MainBranch)
	parentHead, err := git.GitDir(parentRepo.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentHead)
	mockChild.MockGetCommit(ctx, git.MainBranch)
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], childCommits[len(childCommits)-1]))
	for _, hash := range childCommits {
		mockChild.MockGetCommit(ctx, hash)
	}
	// Update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	require.Equal(t, childCommits[0], lastRollRev.Id)

	// Mock the request to retrieve the DEPS file.
	mockParent.MockGetCommit(ctx, parentHead)
	mockParent.MockReadFile(ctx, "DEPS", parentHead)

	// Mock the request to retrieve the README.chromium file.
	mockParent.MockReadFile(ctx, parent.FtReadmePath, parentHead)

	// Mock the requests to retrieve the headers to merge.
	for _, h := range parent.FtIncludesToMerge {
		mockParent.MockReadFile(ctx, path.Join(parent.FtIncludeDest, h), parentHead)
		// No need to mock reading from the child repo; the repo manager
		// actually creates a checkout and uses that.
	}

	subject := strings.Split(fakeCommitMsg, "\n")[0]
	reqBody := []byte(fmt.Sprintf(`{"project":"%s","subject":"%s","branch":"%s","topic":"","status":"NEW","base_commit":"%s"}`, "fake-gerrit-project", subject, git.MainBranch, parentHead))
	ci := gerrit.ChangeInfo{
		ChangeId: "123",
		Project:  "test-project",
		Branch:   "test-branch",
		Id:       "123",
		Issue:    123,
		Revisions: map[string]*gerrit.Revision{
			"ps1": {
				ID:     "ps1",
				Number: 1,
			},
			"ps2": {
				ID:     "ps2",
				Number: 2,
			},
		},
		WorkInProgress: true,
	}
	respBody, err := json.Marshal(ci)
	require.NoError(t, err)
	respBody = append([]byte(")]}'\n"), respBody...)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/", mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, 201))

	// Mock the edit of the change to update the commit message.
	reqBody = []byte(fmt.Sprintf(`{"message":"%s"}`, strings.Replace(fakeCommitMsgMock, "\n", "\\n", -1)))
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/edit:message", mockhttpclient.MockPutDialogue("application/json", reqBody, []byte("")))

	// Mock the request to modify the DEPS file.
	reqBody = []byte(fmt.Sprintf(`deps = {
  "%s": "%s@%s",
}`, ftChildPath, childRepo.RepoUrl(), tipRev.Id))
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/edit/DEPS", mockhttpclient.MockPutDialogue("", reqBody, []byte("")))

	// Mock the request to modify the README.chromium file.
	reqBody = []byte(fmt.Sprintf(ftReadmeTmpl, fmt.Sprintf("v0.0.9-0-g%s", tipRev.Id[:7]), tipRev.Id))
	urlmock.MockOnce(fmt.Sprintf("https://fake-skia-review.googlesource.com/a/changes/123/edit/%s", url.QueryEscape(parent.FtReadmePath)), mockhttpclient.MockPutDialogue("", reqBody, []byte("")))

	// Mock the requests to modify the header files.
	for idx, h := range parent.FtIncludesToMerge {
		reqBody = []byte(fmt.Sprintf(ftIncludeTmpl, "parent", idx, 9))
		urlmock.MockOnce(fmt.Sprintf("https://fake-skia-review.googlesource.com/a/changes/123/edit/%s", url.QueryEscape(path.Join(parent.FtIncludeDest, h))), mockhttpclient.MockPutDialogue("", reqBody, []byte("")))
	}

	// Mock the request to publish the change edit.
	reqBody = []byte(`{"notify":"ALL"}`)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/edit:publish", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	// Mock the request to load the updated change.
	respBody, err = json.Marshal(ci)
	require.NoError(t, err)
	respBody = append([]byte(")]}'\n"), respBody...)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/detail?o=ALL_REVISIONS&o=SUBMITTABLE", mockhttpclient.MockGetDialogue(respBody))

	// Mock the request to set the change as read for review. This is only
	// done if ChangeInfo.WorkInProgress is true.
	reqBody = []byte(`{}`)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/ready", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	// Mock the request to set the CQ.
	reqBody = []byte(`{"labels":{"Code-Review":1,"Commit-Queue":2},"message":"","reviewers":[{"reviewer":"me@google.com"}]}`)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/test-project~test-branch~123/revisions/ps2/review", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, []string{"me@google.com"}, false, fakeCommitMsg)
	require.NoError(t, err)
	require.NotEqual(t, 0, issue)
}
