package repo_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/strategy"
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

func setupFreeType(t *testing.T, strategy string) (context.Context, string, RepoManager, *git_testutils.GitBuilder, *git_testutils.GitBuilder, *gitiles_testutils.MockRepo, *gitiles_testutils.MockRepo, []string, *mockhttpclient.URLMock, func()) {
	unittest.LargeTest(t)

	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	ctx := context.Background()

	// Create child and parent repos.
	child := git_testutils.GitInit(t, ctx)
	childCommits := make([]string, 0, 10)
	for i := 0; i < numChildCommits; i++ {
		for idx, h := range ftIncludesToMerge {
			child.Add(ctx, path.Join(ftIncludeSrc, h), fmt.Sprintf(ftIncludeTmpl, "child", idx, i))
		}
		childCommits = append(childCommits, child.Commit(ctx))
		_, err = git.GitDir(child.Dir()).Git(ctx, "tag", "-a", fmt.Sprintf(ftVersionTmpl, i), "-m", fmt.Sprintf("Version %d", i))
		assert.NoError(t, err)
	}

	urlmock := mockhttpclient.NewURLMock()

	mockChild := gitiles_testutils.NewMockRepo(t, child.RepoUrl(), git.GitDir(child.Dir()), urlmock)

	parent := git_testutils.GitInit(t, ctx)
	parent.Add(ctx, "DEPS", fmt.Sprintf(`deps = {
  "%s": "%s@%s",
}`, ftChildPath, child.RepoUrl(), childCommits[0]))
	parent.Add(ctx, ftReadmePath, fmt.Sprintf(ftReadmeTmpl, fmt.Sprintf(ftVersionTmpl, 0), childCommits[0]))
	for idx, h := range ftIncludesToMerge {
		parent.Add(ctx, path.Join(ftIncludeDest, h), fmt.Sprintf(ftIncludeTmpl, "parent", idx, 0))
	}
	parent.Commit(ctx)

	mockParent := gitiles_testutils.NewMockRepo(t, parent.RepoUrl(), git.GitDir(parent.Dir()), urlmock)

	gUrl := "https://fake-skia-review.googlesource.com"
	gitcookies := path.Join(wd, "gitcookies_fake")
	assert.NoError(t, ioutil.WriteFile(gitcookies, []byte(".googlesource.com\tTRUE\t/\tTRUE\t123\to\tgit-user.google.com=abc123"), os.ModePerm))
	serialized, err := json.Marshal(&gerrit.AccountDetails{
		AccountId: 101,
		Name:      mockUser,
		Email:     mockUser,
		UserName:  mockUser,
	})
	assert.NoError(t, err)
	serialized = append([]byte("abcd\n"), serialized...)
	urlmock.MockOnce(gUrl+"/a/accounts/self/detail", mockhttpclient.MockGetDialogue(serialized))
	g, err := gerrit.NewGerrit(gUrl, gitcookies, urlmock.Client())
	assert.NoError(t, err)

	cfg := &FreeTypeRepoManagerConfig{
		NoCheckoutDEPSRepoManagerConfig: NoCheckoutDEPSRepoManagerConfig{
			NoCheckoutRepoManagerConfig: NoCheckoutRepoManagerConfig{
				CommonRepoManagerConfig: CommonRepoManagerConfig{
					ChildBranch:  "master",
					ChildPath:    ftChildPath,
					ParentBranch: "master",
				},
				ParentRepo: parent.RepoUrl(),
			},
			ChildRepo:  child.RepoUrl(),
			IncludeLog: true,
		},
	}
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parent.Dir()).RevParse(ctx, "HEAD")
	assert.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)
	mockChild.MockLog(ctx, childCommits[0], "master")
	mockChild.MockGetCommit(ctx, childCommits[0])

	rm, err := NewFreeTypeRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com", "", urlmock.Client(), gerritCR(t, g), false)
	assert.NoError(t, err)
	assert.NoError(t, SetStrategy(ctx, rm, strategy))
	assert.NoError(t, rm.Update(ctx))

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
		assert.True(t, urlmock.Empty(), strings.Join(urlmock.List(), "\n"))
	}
	return ctx, wd, rm, child, parent, mockChild, mockParent, childCommits, urlmock, cleanup
}

func TestFreeTypeRepoManagerUpdate(t *testing.T) {
	ctx, _, rm, _, parentRepo, mockChild, mockParent, childCommits, _, cleanup := setupFreeType(t, strategy.ROLL_STRATEGY_BATCH)
	defer cleanup()

	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parentRepo.Dir()).RevParse(ctx, "HEAD")
	assert.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)
	mockChild.MockLog(ctx, childCommits[0], "master")
	mockChild.MockGetCommit(ctx, childCommits[0])
	nextRollRev := childCommits[len(childCommits)-1]
	assert.NoError(t, rm.Update(ctx))
	assert.Equal(t, rm.LastRollRev().Id, childCommits[0])
	assert.Equal(t, rm.NextRollRev().Id, nextRollRev)
	assert.Equal(t, len(rm.NotRolledRevisions()), len(childCommits)-1)

	// RolledPast.
	currentRev, err := rm.GetRevision(ctx, childCommits[0])
	assert.NoError(t, err)
	assert.Equal(t, childCommits[0], currentRev.Id)
	rp, err := rm.RolledPast(ctx, currentRev)
	assert.NoError(t, err)
	assert.True(t, rp)
	for _, c := range childCommits[1:] {
		rev, err := rm.GetRevision(ctx, c)
		assert.NoError(t, err)
		assert.Equal(t, c, rev.Id)
		mockChild.MockLog(ctx, c, childCommits[0])
		rp, err := rm.RolledPast(ctx, rev)
		assert.NoError(t, err)
		assert.False(t, rp)
	}
}

func TestFreeTypeRepoManagerStrategies(t *testing.T) {
	ctx, _, rm, _, parentRepo, mockChild, mockParent, childCommits, _, cleanup := setupFreeType(t, strategy.ROLL_STRATEGY_SINGLE)
	defer cleanup()

	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parentRepo.Dir()).RevParse(ctx, "HEAD")
	assert.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)
	mockChild.MockLog(ctx, childCommits[0], "master")
	mockChild.MockGetCommit(ctx, childCommits[0])
	nextRollRev := childCommits[1]
	assert.NoError(t, rm.Update(ctx))
	assert.Equal(t, rm.NextRollRev().Id, nextRollRev)

	// Switch next-roll-rev strategies.
	assert.NoError(t, SetStrategy(ctx, rm, strategy.ROLL_STRATEGY_BATCH))
	mockParent.MockGetCommit(ctx, "master")
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)
	mockChild.MockLog(ctx, childCommits[0], "master")
	mockChild.MockGetCommit(ctx, childCommits[0])
	assert.NoError(t, rm.Update(ctx))
	assert.Equal(t, childCommits[len(childCommits)-1], rm.NextRollRev().Id)
	// And back again.
	assert.NoError(t, SetStrategy(ctx, rm, strategy.ROLL_STRATEGY_SINGLE))
	mockParent.MockGetCommit(ctx, "master")
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)
	mockChild.MockLog(ctx, childCommits[0], "master")
	mockChild.MockGetCommit(ctx, childCommits[0])
	assert.NoError(t, rm.Update(ctx))
	assert.Equal(t, childCommits[1], rm.NextRollRev().Id)
}

func TestFreeTypeRepoManagerCreateNewRoll(t *testing.T) {
	ctx, _, rm, childRepo, parentRepo, mockChild, mockParent, childCommits, urlmock, cleanup := setupFreeType(t, strategy.ROLL_STRATEGY_BATCH)
	defer cleanup()

	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parentRepo.Dir()).RevParse(ctx, "HEAD")
	assert.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)
	mockChild.MockLog(ctx, childCommits[0], "master")
	mockChild.MockGetCommit(ctx, childCommits[0])
	nextRollRev := childCommits[len(childCommits)-1]
	assert.NoError(t, rm.Update(ctx))

	lastRollRev := childCommits[0]

	// Mock the request to retrieve the DEPS file.
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)

	// Mock the request to retrieve the README.chromium file.
	mockParent.MockReadFile(ctx, ftReadmePath, parentMaster)

	// Mock the requests to retrieve the headers to merge.
	for _, h := range ftIncludesToMerge {
		mockParent.MockReadFile(ctx, path.Join(ftIncludeDest, h), parentMaster)
		// No need to mock reading from the child repo; the repo manager
		// actually creates a checkout and uses that.
	}

	// Mock the initial change creation.
	logStr := ""
	childGitRepo := git.GitDir(childRepo.Dir())
	commitsToRoll, err := childGitRepo.RevList(ctx, fmt.Sprintf("%s..%s", lastRollRev, nextRollRev))
	assert.NoError(t, err)
	for _, c := range commitsToRoll {
		mockChild.MockGetCommit(ctx, c)
		details, err := childGitRepo.Details(ctx, c)
		assert.NoError(t, err)
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

git log %s..%s --date=short --no-merges --format='%%ad %%ae %%s'
%s
Created with:
  gclient setdep -r %s@%s

The AutoRoll server is located here: %s

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md

If the roll is causing failures, please contact the current sheriff, who should
be CC'd on the roll, and stop the roller if necessary.


Bug: None
TBR=me@google.com`, ftChildPath, lastRollRev[:12], nextRollRev[:12], len(rm.NotRolledRevisions()), childRepo.RepoUrl(), lastRollRev[:12], nextRollRev[:12], lastRollRev[:12], nextRollRev[:12], logStr, ftChildPath, nextRollRev[:12], "fake.server.com")
	subject := strings.Split(commitMsg, "\n")[0]
	reqBody := []byte(fmt.Sprintf(`{"project":"%s","subject":"%s","branch":"%s","topic":"","status":"NEW","base_commit":"%s"}`, rm.(*freetypeRepoManager).gerritConfig.Project, subject, "master", parentMaster))
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
	assert.NoError(t, err)
	respBody = append([]byte(")]}'\n"), respBody...)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/", mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, 201))

	// Mock the edit of the change to update the commit message.
	reqBody = []byte(fmt.Sprintf(`{"message":"%s"}`, strings.Replace(commitMsg, "\n", "\\n", -1)))
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/edit:message", mockhttpclient.MockPutDialogue("application/json", reqBody, []byte("")))

	// Mock the request to modify the DEPS file.
	reqBody = []byte(fmt.Sprintf(`deps = {
  "%s": "%s@%s",
}`, ftChildPath, childRepo.RepoUrl(), nextRollRev))
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/edit/DEPS", mockhttpclient.MockPutDialogue("", reqBody, []byte("")))

	// Mock the request to modify the README.chromium file.
	reqBody = []byte(fmt.Sprintf(ftReadmeTmpl, fmt.Sprintf("v0.0.9-0-g%s", nextRollRev[:7]), nextRollRev))
	urlmock.MockOnce(fmt.Sprintf("https://fake-skia-review.googlesource.com/a/changes/123/edit/%s", url.QueryEscape(ftReadmePath)), mockhttpclient.MockPutDialogue("", reqBody, []byte("")))

	// Mock the requests to modify the header files.
	for idx, h := range ftIncludesToMerge {
		reqBody = []byte(fmt.Sprintf(ftIncludeTmpl, "parent", idx, 9))
		urlmock.MockOnce(fmt.Sprintf("https://fake-skia-review.googlesource.com/a/changes/123/edit/%s", url.QueryEscape(path.Join(ftIncludeDest, h))), mockhttpclient.MockPutDialogue("", reqBody, []byte("")))
	}

	// Mock the request to publish the change edit.
	reqBody = []byte(`{"notify":"ALL"}`)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/edit:publish", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	// Mock the request to load the updated change.
	respBody, err = json.Marshal(ci)
	assert.NoError(t, err)
	respBody = append([]byte(")]}'\n"), respBody...)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/detail?o=ALL_REVISIONS", mockhttpclient.MockGetDialogue(respBody))

	// Mock the request to set the change as read for review. This is only
	// done if ChangeInfo.WorkInProgress is true.
	reqBody = []byte(`{}`)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/ready", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	// Mock the request to set the CQ.
	reqBody = []byte(`{"labels":{"Code-Review":1,"Commit-Queue":2},"message":"","reviewers":[{"reviewer":"me@google.com"}]}`)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/revisions/ps1/review", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	issue, err := rm.CreateNewRoll(ctx, rm.LastRollRev(), rm.NextRollRev(), []string{"me@google.com"}, "", false)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, issue)
}
