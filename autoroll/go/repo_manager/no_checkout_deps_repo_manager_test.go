package repo_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
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

func setupNoCheckout(t *testing.T, cfg *NoCheckoutDEPSRepoManagerConfig, strategy string) (context.Context, string, RepoManager, *git_testutils.GitBuilder, *git_testutils.GitBuilder, *gitiles_testutils.MockRepo, *gitiles_testutils.MockRepo, []string, *mockhttpclient.URLMock, func()) {
	unittest.LargeTest(t)

	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	// Create child and parent repos.
	child := git_testutils.GitInit(t, context.Background())
	child.Add(context.Background(), "DEPS", `deps = {
  "child/dep": "grandchild@def4560000def4560000def4560000def4560000",
}`)
	child.Commit(context.Background())
	f := "somefile.txt"
	childCommits := make([]string, 0, 10)
	for i := 0; i < numChildCommits; i++ {
		childCommits = append(childCommits, child.CommitGen(context.Background(), f))
	}

	urlmock := mockhttpclient.NewURLMock()

	mockChild := gitiles_testutils.NewMockRepo(t, child.RepoUrl(), git.GitDir(child.Dir()), urlmock)

	parent := git_testutils.GitInit(t, context.Background())
	parent.Add(context.Background(), "DEPS", fmt.Sprintf(`deps = {
  "%s": "%s@%s",
  "parent/dep": "grandchild@abc1230000abc1230000abc1230000abc1230000",
}`, childPath, child.RepoUrl(), childCommits[0]))
	parent.Commit(context.Background())

	mockParent := gitiles_testutils.NewMockRepo(t, parent.RepoUrl(), git.GitDir(parent.Dir()), urlmock)

	ctx := context.Background()

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

	cfg.ChildRepo = child.RepoUrl()
	cfg.ParentRepo = parent.RepoUrl()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parent.Dir()).RevParse(ctx, "HEAD")
	assert.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], "master"))
	mockChild.MockGetCommit(ctx, childCommits[0])

	rm, err := NewNoCheckoutDEPSRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com", "", urlmock.Client(), gerritCR(t, g), false)
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

func noCheckoutDEPSCfg() *NoCheckoutDEPSRepoManagerConfig {
	return &NoCheckoutDEPSRepoManagerConfig{
		NoCheckoutRepoManagerConfig: NoCheckoutRepoManagerConfig{
			CommonRepoManagerConfig: CommonRepoManagerConfig{
				ChildBranch:  "master",
				ChildPath:    childPath,
				ParentBranch: "master",
			},
		},
		IncludeLog: true,
	}
}

func TestNoCheckoutDEPSRepoManagerUpdate(t *testing.T) {
	cfg := noCheckoutDEPSCfg()
	ctx, _, rm, _, parentRepo, mockChild, mockParent, childCommits, _, cleanup := setupNoCheckout(t, cfg, strategy.ROLL_STRATEGY_BATCH)
	defer cleanup()

	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parentRepo.Dir()).RevParse(ctx, "HEAD")
	assert.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], "master"))
	mockChild.MockGetCommit(ctx, childCommits[0])
	nextRollRev := childCommits[len(childCommits)-1]
	assert.NoError(t, rm.Update(ctx))
	assert.Equal(t, rm.LastRollRev().Id, childCommits[0])
	assert.Equal(t, rm.NextRollRev().Id, nextRollRev)
	assert.Equal(t, len(rm.NotRolledRevisions()), len(childCommits)-1)

	// RolledPast.
	mockChild.MockGetCommit(ctx, childCommits[0])
	currentRev, err := rm.GetRevision(ctx, childCommits[0])
	assert.NoError(t, err)
	assert.Equal(t, childCommits[0], currentRev.Id)
	rp, err := rm.RolledPast(ctx, currentRev)
	assert.NoError(t, err)
	assert.True(t, rp)
	for _, c := range childCommits[1:] {
		mockChild.MockGetCommit(ctx, c)
		rev, err := rm.GetRevision(ctx, c)
		assert.NoError(t, err)
		assert.Equal(t, c, rev.Id)
		mockChild.MockLog(ctx, git.LogFromTo(c, childCommits[0]))
		rp, err := rm.RolledPast(ctx, rev)
		assert.NoError(t, err)
		assert.False(t, rp)
	}
}

func TestNoCheckoutDEPSRepoManagerStrategies(t *testing.T) {
	cfg := noCheckoutDEPSCfg()
	ctx, _, rm, _, parentRepo, mockChild, mockParent, childCommits, _, cleanup := setupNoCheckout(t, cfg, strategy.ROLL_STRATEGY_SINGLE)
	defer cleanup()

	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parentRepo.Dir()).RevParse(ctx, "HEAD")
	assert.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], "master"))
	mockChild.MockGetCommit(ctx, childCommits[0])
	nextRollRev := childCommits[1]
	assert.NoError(t, rm.Update(ctx))
	assert.Equal(t, rm.NextRollRev().Id, nextRollRev)

	// Switch next-roll-rev strategies.
	assert.NoError(t, SetStrategy(ctx, rm, strategy.ROLL_STRATEGY_BATCH))
	mockParent.MockGetCommit(ctx, "master")
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], "master"))
	mockChild.MockGetCommit(ctx, childCommits[0])
	assert.NoError(t, rm.Update(ctx))
	assert.Equal(t, childCommits[len(childCommits)-1], rm.NextRollRev().Id)
	// And back again.
	assert.NoError(t, SetStrategy(ctx, rm, strategy.ROLL_STRATEGY_SINGLE))
	mockParent.MockGetCommit(ctx, "master")
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], "master"))
	mockChild.MockGetCommit(ctx, childCommits[0])
	assert.NoError(t, rm.Update(ctx))
	assert.Equal(t, childCommits[1], rm.NextRollRev().Id)
}

func TestNoCheckoutDEPSRepoManagerCreateNewRoll(t *testing.T) {
	cfg := noCheckoutDEPSCfg()
	ctx, _, rm, childRepo, parentRepo, mockChild, mockParent, childCommits, urlmock, cleanup := setupNoCheckout(t, cfg, strategy.ROLL_STRATEGY_BATCH)
	defer cleanup()

	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parentRepo.Dir()).RevParse(ctx, "HEAD")
	assert.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], "master"))
	mockChild.MockGetCommit(ctx, childCommits[0])
	nextRollRev := childCommits[len(childCommits)-1]
	assert.NoError(t, rm.Update(ctx))

	lastRollRev := childCommits[0]

	// Mock the request to retrieve the DEPS file.
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)

	// Mock the initial change creation.
	logStr := ""
	childGitRepo := git.GitDir(childRepo.Dir())
	commitsToRoll, err := childGitRepo.RevList(ctx, git.LogFromTo(lastRollRev, nextRollRev))
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
TBR=me@google.com`, childPath, lastRollRev[:12], nextRollRev[:12], len(rm.NotRolledRevisions()), childRepo.RepoUrl(), lastRollRev[:12], nextRollRev[:12], lastRollRev[:12], nextRollRev[:12], logStr, childPath, nextRollRev[:12], "fake.server.com")
	subject := strings.Split(commitMsg, "\n")[0]
	reqBody := []byte(fmt.Sprintf(`{"project":"%s","subject":"%s","branch":"%s","topic":"","status":"NEW","base_commit":"%s"}`, rm.(*noCheckoutDEPSRepoManager).gerritConfig.Project, subject, cfg.ParentBranch, parentMaster))
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
  "parent/dep": "grandchild@abc1230000abc1230000abc1230000abc1230000",
}`, childPath, childRepo.RepoUrl(), nextRollRev))
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/edit/DEPS", mockhttpclient.MockPutDialogue("", reqBody, []byte("")))

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

func TestNoCheckoutDEPSRepoManagerCreateNewRollTransitive(t *testing.T) {
	cfg := noCheckoutDEPSCfg()
	cfg.TransitiveDeps = map[string]string{
		"child/dep": "parent/dep",
	}
	ctx, _, rm, childRepo, parentRepo, mockChild, mockParent, childCommits, urlmock, cleanup := setupNoCheckout(t, cfg, strategy.ROLL_STRATEGY_BATCH)
	defer cleanup()

	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parentRepo.Dir()).RevParse(ctx, "HEAD")
	assert.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], "master"))
	mockChild.MockGetCommit(ctx, childCommits[0])
	nextRollRev := childCommits[len(childCommits)-1]
	assert.NoError(t, rm.Update(ctx))

	lastRollRev := childCommits[0]

	// Mock the request to retrieve the DEPS file.
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)

	// Mock the request to retrieve the child's DEPS file.
	mockChild.MockReadFile(ctx, "DEPS", rm.NextRollRev().Id)

	// Mock the initial change creation.
	logStr := ""
	childGitRepo := git.GitDir(childRepo.Dir())
	commitsToRoll, err := childGitRepo.RevList(ctx, git.LogFromTo(lastRollRev, nextRollRev))
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
Also rolling transitive DEPS:
  parent/dep abc1230000ab..def4560000de

Created with:
  gclient setdep -r %s@%s

The AutoRoll server is located here: %s

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md

If the roll is causing failures, please contact the current sheriff, who should
be CC'd on the roll, and stop the roller if necessary.


Bug: None
TBR=me@google.com`, childPath, lastRollRev[:12], nextRollRev[:12], len(rm.NotRolledRevisions()), childRepo.RepoUrl(), lastRollRev[:12], nextRollRev[:12], lastRollRev[:12], nextRollRev[:12], logStr, childPath, nextRollRev[:12], "fake.server.com")
	subject := strings.Split(commitMsg, "\n")[0]
	reqBody := []byte(fmt.Sprintf(`{"project":"%s","subject":"%s","branch":"%s","topic":"","status":"NEW","base_commit":"%s"}`, rm.(*noCheckoutDEPSRepoManager).gerritConfig.Project, subject, cfg.ParentBranch, parentMaster))
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
  "parent/dep": "grandchild@def4560000def4560000def4560000def4560000",
}`, childPath, childRepo.RepoUrl(), nextRollRev))
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/edit/DEPS", mockhttpclient.MockPutDialogue("", reqBody, []byte("")))

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
