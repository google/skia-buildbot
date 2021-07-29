package repo_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func setupNoCheckout(t *testing.T, cfg *config.ParentChildRepoManagerConfig) (context.Context, string, *parentChildRepoManager, *git_testutils.GitBuilder, *git_testutils.GitBuilder, *gitiles_testutils.MockRepo, *gitiles_testutils.MockRepo, []string, *mockhttpclient.URLMock, func()) {
	unittest.LargeTest(t)

	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	// Create child and parent repos.
	child := git_testutils.GitInit(t, context.Background())
	child.Add(context.Background(), "DEPS", `deps = {
  "child/dep": "https://grandchild-in-child@def4560000def4560000def4560000def4560000",
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
  "parent/dep": "https://grandchild-in-parent@abc1230000abc1230000abc1230000abc1230000",
}`, childPath, child.RepoUrl(), childCommits[0]))
	parent.Commit(context.Background())

	mockParent := gitiles_testutils.NewMockRepo(t, parent.RepoUrl(), git.GitDir(parent.Dir()), urlmock)

	ctx := context.Background()

	parentCfg := cfg.Parent.(*config.ParentChildRepoManagerConfig_GitilesParent).GitilesParent

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
	g, err := gerrit.NewGerritWithConfig(codereview.GerritConfigs[parentCfg.Gerrit.Config], gUrl, urlmock.Client())
	require.NoError(t, err)

	parentCfg.Gitiles.RepoUrl = parent.RepoUrl()
	parentCfg.Dep.Primary.Id = child.RepoUrl()
	childCfg := cfg.Child.(*config.ParentChildRepoManagerConfig_GitilesChild).GitilesChild
	childCfg.Gitiles.RepoUrl = child.RepoUrl()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	// Create the RepoManager.
	rm, err := newParentChildRepoManager(ctx, cfg, setupRegistry(t), wd, "fake-roller", recipesCfg, "fake.server.com", urlmock.Client(), gerritCR(t, g, urlmock.Client()))
	require.NoError(t, err)

	// Mock requests for Update().
	mockParent.MockGetCommit(ctx, git.MasterBranch)
	parentHead, err := git.GitDir(parent.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentHead)
	mockChild.MockGetCommit(ctx, git.MasterBranch)
	if len(parentCfg.Dep.Transitive) > 0 {
		mockChild.MockReadFile(ctx, "DEPS", childCommits[len(childCommits)-1])
	}
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], childCommits[len(childCommits)-1]))
	for _, hash := range childCommits {
		mockChild.MockGetCommit(ctx, hash)
		if len(parentCfg.Dep.Transitive) > 0 {
			mockChild.MockReadFile(ctx, "DEPS", hash)
		}
	}
	// Update.
	_, _, _, err = rm.Update(ctx)
	require.NoError(t, err)
	require.True(t, urlmock.Empty(), strings.Join(urlmock.List(), "\n"))
	cleanup := func() {
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
		require.True(t, urlmock.Empty(), strings.Join(urlmock.List(), "\n"))
	}
	return ctx, wd, rm, child, parent, mockChild, mockParent, childCommits, urlmock, cleanup
}

func noCheckoutDEPSCfg(t *testing.T) *config.ParentChildRepoManagerConfig {
	return &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_GitilesParent{
			GitilesParent: &config.GitilesParentConfig{
				Gitiles: &config.GitilesConfig{
					Branch:  git.MasterBranch,
					RepoUrl: "todo.git",
				},
				Dep: &config.DependencyConfig{
					Primary: &config.VersionFileConfig{
						Id:   "todo.git",
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
		Child: &config.ParentChildRepoManagerConfig_GitilesChild{
			GitilesChild: &config.GitilesChildConfig{
				Gitiles: &config.GitilesConfig{
					Branch:  git.MasterBranch,
					RepoUrl: "todo.git",
				},
			},
		},
	}
}

func TestNoCheckoutDEPSRepoManagerUpdate(t *testing.T) {
	cfg := noCheckoutDEPSCfg(t)
	ctx, _, rm, _, parentRepo, mockChild, mockParent, childCommits, _, cleanup := setupNoCheckout(t, cfg)
	defer cleanup()

	// Mock requests for Update().
	mockParent.MockGetCommit(ctx, git.MasterBranch)
	parentHead, err := git.GitDir(parentRepo.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentHead)
	mockChild.MockGetCommit(ctx, git.MasterBranch)
	parentCfg := cfg.Parent.(*config.ParentChildRepoManagerConfig_GitilesParent).GitilesParent
	if len(parentCfg.Dep.Transitive) > 0 {
		mockChild.MockReadFile(ctx, "DEPS", childCommits[len(childCommits)-1])
	}
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], childCommits[len(childCommits)-1]))
	for _, hash := range childCommits {
		mockChild.MockGetCommit(ctx, hash)
		if len(parentCfg.Dep.Transitive) > 0 {
			mockChild.MockReadFile(ctx, "DEPS", hash)
		}
	}
	// Update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, childCommits[0], lastRollRev.Id)
	require.Equal(t, childCommits[len(childCommits)-1], tipRev.Id)
	require.Equal(t, len(notRolledRevs), len(childCommits)-1)
}

func testNoCheckoutDEPSRepoManagerCreateNewRoll(t *testing.T, cfg *config.ParentChildRepoManagerConfig) {
	ctx, _, rm, childRepo, parentRepo, mockChild, mockParent, childCommits, urlmock, cleanup := setupNoCheckout(t, cfg)
	defer cleanup()

	// Mock requests for Update().
	mockParent.MockGetCommit(ctx, git.MasterBranch)
	parentHead, err := git.GitDir(parentRepo.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentHead)
	mockChild.MockGetCommit(ctx, git.MasterBranch)
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], childCommits[len(childCommits)-1]))
	for _, hash := range childCommits {
		mockChild.MockGetCommit(ctx, hash)
	}
	// Update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, childCommits[0], lastRollRev.Id)
	require.Equal(t, childCommits[len(childCommits)-1], tipRev.Id)

	// Mock the request to retrieve the DEPS file.
	mockParent.MockReadFile(ctx, "DEPS", parentHead)

	// Mock the initial change creation.
	subject := strings.Split(fakeCommitMsg, "\n")[0]
	reqBody := []byte(fmt.Sprintf(`{"project":"%s","subject":"%s","branch":"%s","topic":"","status":"NEW","base_commit":"%s"}`, "fake-gerrit-project", subject, git.MasterBranch, parentHead))
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
  "parent/dep": "https://grandchild-in-parent@abc1230000abc1230000abc1230000abc1230000",
}`, childPath, childRepo.RepoUrl(), tipRev.Id))
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/edit/DEPS", mockhttpclient.MockPutDialogue("", reqBody, []byte("")))

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
	parentCfg := cfg.Parent.(*config.ParentChildRepoManagerConfig_GitilesParent).GitilesParent
	gerritCfg := codereview.GerritConfigs[parentCfg.Gerrit.Config]
	if gerritCfg.HasCq {
		reqBody = []byte(`{"labels":{"Code-Review":1,"Commit-Queue":2},"message":"","reviewers":[{"reviewer":"me@google.com"}]}`)
	} else {
		reqBody = []byte(`{"labels":{"Code-Review":1},"message":"","reviewers":[{"reviewer":"me@google.com"}]}`)
	}
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/test-project~test-branch~123/revisions/ps2/review", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))
	if !gerritCfg.HasCq {
		urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/submit", mockhttpclient.MockPostDialogue("application/json", []byte("{}"), []byte("")))
	}

	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, []string{"me@google.com"}, false, fakeCommitMsg)
	require.NoError(t, err)
	require.NotEqual(t, 0, issue)
}

func TestNoCheckoutDEPSRepoManagerCreateNewRoll(t *testing.T) {
	cfg := noCheckoutDEPSCfg(t)
	testNoCheckoutDEPSRepoManagerCreateNewRoll(t, cfg)
}

func TestNoCheckoutDEPSRepoManagerCreateNewRollNoCQ(t *testing.T) {
	cfg := noCheckoutDEPSCfg(t)
	parentCfg := cfg.Parent.(*config.ParentChildRepoManagerConfig_GitilesParent).GitilesParent
	parentCfg.Gerrit.Config = config.GerritConfig_CHROMIUM_NO_CQ
	testNoCheckoutDEPSRepoManagerCreateNewRoll(t, cfg)
}

func TestNoCheckoutDEPSRepoManagerCreateNewRollTransitive(t *testing.T) {
	cfg := noCheckoutDEPSCfg(t)
	parentCfg := cfg.Parent.(*config.ParentChildRepoManagerConfig_GitilesParent).GitilesParent
	parentCfg.Dep.Transitive = []*config.TransitiveDepConfig{
		{
			Child: &config.VersionFileConfig{
				Id:   "https://grandchild-in-child",
				Path: "DEPS",
			},
			Parent: &config.VersionFileConfig{
				Id:   "https://grandchild-in-parent",
				Path: "DEPS",
			},
		},
	}
	childCfg := cfg.Child.(*config.ParentChildRepoManagerConfig_GitilesChild).GitilesChild
	childCfg.Gitiles.Dependencies = []*config.VersionFileConfig{
		{
			Id:   "https://grandchild-in-child",
			Path: "DEPS",
		},
	}
	ctx, _, rm, childRepo, parentRepo, mockChild, mockParent, childCommits, urlmock, cleanup := setupNoCheckout(t, cfg)
	defer cleanup()

	// Mock requests for Update().
	mockParent.MockGetCommit(ctx, git.MasterBranch)
	parentHead, err := git.GitDir(parentRepo.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentHead)
	mockChild.MockGetCommit(ctx, git.MasterBranch)
	mockChild.MockReadFile(ctx, "DEPS", childCommits[len(childCommits)-1])
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], childCommits[len(childCommits)-1]))
	for _, hash := range childCommits {
		mockChild.MockGetCommit(ctx, hash)
		mockChild.MockReadFile(ctx, "DEPS", hash)
	}
	// Update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.True(t, urlmock.Empty())
	require.Equal(t, childCommits[0], lastRollRev.Id)
	require.Equal(t, childCommits[len(childCommits)-1], tipRev.Id)

	// Mock the request to retrieve the DEPS file.
	mockParent.MockReadFile(ctx, "DEPS", parentHead)

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
	subject := strings.Split(fakeCommitMsg, "\n")[0]
	reqBody := []byte(fmt.Sprintf(`{"project":"%s","subject":"%s","branch":"%s","topic":"","status":"NEW","base_commit":"%s"}`, "fake-gerrit-project", subject, git.MasterBranch, parentHead))
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
  "parent/dep": "https://grandchild-in-parent@def4560000def4560000def4560000def4560000",
}`, childPath, childRepo.RepoUrl(), tipRev.Id))
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/edit/DEPS", mockhttpclient.MockPutDialogue("", reqBody, []byte("")))

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
