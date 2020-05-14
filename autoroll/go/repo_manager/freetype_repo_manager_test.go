package repo_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/recipe_cfg"
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

	cfg := &FreeTypeRepoManagerConfig{
		NoCheckoutDEPSRepoManagerConfig: NoCheckoutDEPSRepoManagerConfig{
			NoCheckoutRepoManagerConfig: NoCheckoutRepoManagerConfig{
				CommonRepoManagerConfig: CommonRepoManagerConfig{
					ChildBranch:  masterBranchTmpl(t),
					ChildPath:    ftChildPath,
					IncludeLog:   true,
					ParentBranch: masterBranchTmpl(t),
					ParentRepo:   parentRepo.RepoUrl(),
				},
			},
			ChildRepo: child.RepoUrl(),
			Gerrit: &codereview.GerritConfig{
				URL:     "https://fake-skia-review.googlesource.com",
				Project: "fake-gerrit-project",
				Config:  codereview.GERRIT_CONFIG_CHROMIUM,
			},
		},
	}
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	rm, err := NewFreeTypeRepoManager(ctx, cfg, setupRegistry(t), wd, g, recipesCfg, "fake.server.com", urlmock.Client(), gerritCR(t, g), false)
	require.NoError(t, err)

	// Mock requests for Update().
	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parentRepo.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)
	mockChild.MockGetCommit(ctx, "master")
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
	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parentRepo.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)
	mockChild.MockGetCommit(ctx, "master")
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
	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parentRepo.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)
	mockChild.MockGetCommit(ctx, "master")
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], childCommits[len(childCommits)-1]))
	for _, hash := range childCommits {
		mockChild.MockGetCommit(ctx, hash)
	}
	// Update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	require.Equal(t, childCommits[0], lastRollRev.Id)

	// Mock the request to retrieve the DEPS file.
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)

	// Mock the request to retrieve the README.chromium file.
	mockParent.MockReadFile(ctx, parent.FtReadmePath, parentMaster)

	// Mock the requests to retrieve the headers to merge.
	for _, h := range parent.FtIncludesToMerge {
		mockParent.MockReadFile(ctx, path.Join(parent.FtIncludeDest, h), parentMaster)
		// No need to mock reading from the child repo; the repo manager
		// actually creates a checkout and uses that.
	}

	// Mock the initial change creation.
	logStr := ""
	childGitRepo := git.GitDir(childRepo.Dir())
	for _, c := range notRolledRevs {
		details, err := childGitRepo.Details(ctx, c.Id)
		require.NoError(t, err)
		ts := details.Timestamp.Format("2006-01-02")
		author := details.Author
		authorSplit := strings.Split(details.Author, "(")
		if len(authorSplit) > 1 {
			author = strings.TrimRight(strings.TrimSpace(authorSplit[1]), ")")
		}
		logStr += fmt.Sprintf("%s %s %s\n", ts, author, details.Subject)
	}
	commitMsg := fmt.Sprintf(`Roll %s %s..%s (%d commits)

%s/+log/%s..%s

git log %s..%s --date=short --first-parent --format='%%ad %%ae %%s'
%s
Created with:
  gclient setdep -r %s@%s

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
fake.server.com
Please CC me@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+doc/master/autoroll/README.md

Bug: None
Tbr: me@google.com`, ftChildPath, lastRollRev.Id[:12], tipRev.Id[:12], len(notRolledRevs), childRepo.RepoUrl(), lastRollRev.Id[:12], tipRev.Id[:12], lastRollRev.Id[:12], tipRev.Id[:12], logStr, ftChildPath, tipRev.Id[:12])
	subject := strings.Split(commitMsg, "\n")[0]
	reqBody := []byte(fmt.Sprintf(`{"project":"%s","subject":"%s","branch":"%s","topic":"","status":"NEW","base_commit":"%s"}`, "fake-gerrit-project", subject, "master", parentMaster))
	ci := gerrit.ChangeInfo{
		ChangeId: "123",
		Id:       "123",
		Issue:    123,
		Revisions: map[string]*gerrit.Revision{
			"ps1": {
				ID:     "ps1",
				Number: 1,
			},
		},
		WorkInProgress: true,
	}
	respBody, err := json.Marshal(ci)
	require.NoError(t, err)
	respBody = append([]byte(")]}'\n"), respBody...)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/", mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, 201))

	// Mock the edit of the change to update the commit message.
	reqBody = []byte(fmt.Sprintf(`{"message":"%s"}`, strings.Replace(commitMsg, "\n", "\\n", -1)))
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
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/detail?o=ALL_REVISIONS", mockhttpclient.MockGetDialogue(respBody))

	// Mock the request to set the change as read for review. This is only
	// done if ChangeInfo.WorkInProgress is true.
	reqBody = []byte(`{}`)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/ready", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	// Mock the request to set the CQ.
	reqBody = []byte(`{"labels":{"Code-Review":1,"Commit-Queue":2},"message":"","reviewers":[{"reviewer":"me@google.com"}]}`)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/revisions/ps1/review", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, []string{"me@google.com"}, "", false)
	require.NoError(t, err)
	require.NotEqual(t, 0, issue)
}
