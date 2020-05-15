package repo_manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	github_api "github.com/google/go-github/v29/github"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/exec"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	githubApiUrl = "https://api.github.com"
)

var (
	githubEmails = []string{"reviewer@chromium.org"}

	mockGithubUser      = "superman"
	mockGithubUserEmail = "superman@krypton.com"
	testPullNumber      = 12345
)

func githubDEPSCfg(t *testing.T) *GithubDEPSRepoManagerConfig {
	return &GithubDEPSRepoManagerConfig{
		DepotToolsRepoManagerConfig: DepotToolsRepoManagerConfig{
			CommonRepoManagerConfig: CommonRepoManagerConfig{
				ChildBranch:  masterBranchTmpl(t),
				ChildPath:    childPath,
				ParentBranch: masterBranchTmpl(t),
			},
		},
	}
}

func TestGithubDEPSRepoManagerConfigValidation(t *testing.T) {
	unittest.SmallTest(t)

	cfg := githubDEPSCfg(t)
	cfg.ChildRepo = "git@github.com:fake/child.git"
	cfg.ParentRepo = "git@github.com:fake/parent.git" // Excluded from githubCfg.
	cfg.ForkRepoURL = "git@github.com:fake/fork.git"
	require.NoError(t, cfg.Validate())

	// The only fields come from the nested Configs, so exclude them and
	// verify that we fail validation.
	cfg = &GithubDEPSRepoManagerConfig{}
	require.Error(t, cfg.Validate())
}

func setupGithubDEPS(t *testing.T, c *GithubDEPSRepoManagerConfig) (context.Context, *parentChildRepoManager, string, *git_testutils.GitBuilder, []string, *git_testutils.GitBuilder, *exec.CommandCollector, *mockhttpclient.URLMock, func()) {
	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	ctx := context.Background()

	// Create child and parent repos.
	child := git_testutils.GitInit(t, ctx)
	child.Add(ctx, "DEPS", `deps = {
  "child/dep": {
    "url": "https://grandchild-in-child@def4560000def4560000def4560000def4560000",
    "condition": "False",
  },
}`)
	child.Commit(ctx)
	f := "somefile.txt"
	childCommits := make([]string, 0, 10)
	for i := 0; i < numChildCommits; i++ {
		childCommits = append(childCommits, child.CommitGen(ctx, f))
	}

	parent := git_testutils.GitInit(t, ctx)
	parent.Add(ctx, "DEPS", fmt.Sprintf(`deps = {
  "%s": "%s@%s",
  "parent/dep": {
    "url": "https://grandchild-in-parent@abc1230000abc1230000abc1230000abc1230000",
    "condition": "False",
  },
}`, childPath, child.RepoUrl(), childCommits[0]))
	parent.Commit(ctx)

	fork := git_testutils.GitInit(t, ctx)
	fork.Git(ctx, "remote", "set-url", "origin", parent.RepoUrl())
	fork.Git(ctx, "fetch", "origin")
	fork.Git(ctx, "checkout", "master")
	fork.Git(ctx, "reset", "--hard", "origin/master")

	c.ChildRepo = child.RepoUrl()
	c.ParentRepo = parent.RepoUrl()
	c.ForkRepoURL = fork.RepoUrl()

	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		// Without this, the mock commands get confused with:
		// "Could not switch upstream branch from refs/remotes/remote/master to refs/remotes/origin/master"
		if strings.Contains(cmd.Name, "gclient") && (cmd.Args[0] == "sync" || cmd.Args[0] == "runhooks") {
			return nil
		}
		return exec.DefaultRun(ctx, cmd)
	})
	ctx = exec.NewContext(ctx, mockRun.Run)

	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g, urlmock := setupFakeGithubDEPS(t, ctx)
	rm, err := NewGithubDEPSRepoManager(ctx, c, setupRegistry(t), wd, "test_roller_name", g, recipesCfg, "fake.server.com", nil, githubCR(t, g), false)
	require.NoError(t, err)

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
	}

	return ctx, rm, wd, child, childCommits, parent, mockRun, urlmock, cleanup
}

func setupFakeGithubDEPS(t *testing.T, ctx context.Context) (*github.GitHub, *mockhttpclient.URLMock) {
	urlMock := mockhttpclient.NewURLMock()

	// Mock /user endpoint.
	serializedUser, err := json.Marshal(&github_api.User{
		Login: &mockGithubUser,
		Email: &mockGithubUserEmail,
	})
	require.NoError(t, err)
	urlMock.MockOnce(githubApiUrl+"/user", mockhttpclient.MockGetDialogue(serializedUser))

	// Mock /issues endpoint for get and patch requests.
	serializedIssue, err := json.Marshal(&github_api.Issue{
		Labels: []github_api.Label{},
	})
	require.NoError(t, err)
	urlMock.MockOnce(githubApiUrl+"/repos/superman/krypton/issues/12345", mockhttpclient.MockGetDialogue(serializedIssue))
	patchRespBody := []byte(testutils.MarshalJSON(t, &github_api.PullRequest{}))
	patchReqType := "application/json"
	patchReqBody := []byte(`{"labels":["waiting for tree to go green"]}
`)
	patchMd := mockhttpclient.MockPatchDialogue(patchReqType, patchReqBody, patchRespBody)
	urlMock.MockOnce(githubApiUrl+"/repos/superman/krypton/issues/12345", patchMd)

	g, err := github.NewGitHub(ctx, "superman", "krypton", urlMock.Client())
	require.NoError(t, err)
	return g, urlMock
}

func mockGithubDEPSRequests(t *testing.T, urlMock *mockhttpclient.URLMock) {
	// Mock /pulls endpoint.
	serializedPull, err := json.Marshal(&github_api.PullRequest{
		Number: &testPullNumber,
	})
	require.NoError(t, err)
	reqType := "application/json"
	md := mockhttpclient.MockPostDialogueWithResponseCode(reqType, mockhttpclient.DONT_CARE_REQUEST, serializedPull, http.StatusCreated)
	urlMock.MockOnce(githubApiUrl+"/repos/superman/krypton/pulls", md)

	// Mock /comments endpoint.
	reqType = "application/json"
	reqBody := []byte(`{"body":"@reviewer : New roll has been created by fake.server.com"}
`)
	md = mockhttpclient.MockPostDialogueWithResponseCode(reqType, reqBody, nil, http.StatusCreated)
	urlMock.MockOnce(githubApiUrl+"/repos/superman/krypton/issues/12345/comments", md)
}

// TestGithubDEPSRepoManager tests all aspects of the GithubDEPSRepoManager except for CreateNewRoll.
func TestGithubDEPSRepoManager(t *testing.T) {
	unittest.LargeTest(t)

	cfg := githubDEPSCfg(t)
	ctx, rm, _, child, childCommits, _, _, _, cleanup := setupGithubDEPS(t, cfg)
	defer cleanup()

	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, childCommits[0], lastRollRev.Id)
	require.Equal(t, childCommits[len(childCommits)-1], tipRev.Id)
	require.Equal(t, len(childCommits)-1, len(notRolledRevs))

	// Test update.
	lastCommit := child.CommitGen(ctx, "abc.txt")
	_, tipRev, _, err = rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, lastCommit, tipRev.Id)
}

func TestGithubDEPSRepoManagerCreateNewRoll(t *testing.T) {
	unittest.LargeTest(t)

	cfg := githubDEPSCfg(t)
	ctx, rm, _, _, _, _, _, urlMock, cleanup := setupGithubDEPS(t, cfg)
	defer cleanup()

	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Create a roll, assert that it's at tip of tree.
	mockGithubDEPSRequests(t, urlMock)
	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
	require.NoError(t, err)
	require.Equal(t, issueNum, issue)
}

func TestGithubDEPSRepoManagerCreateNewRollTransitive(t *testing.T) {
	unittest.LargeTest(t)

	cfg := githubDEPSCfg(t)
	cfg.TransitiveDeps = []*version_file_common.TransitiveDepConfig{
		{
			Child: &version_file_common.VersionFileConfig{
				ID:   "https://grandchild-in-child",
				Path: "DEPS",
			},
			Parent: &version_file_common.VersionFileConfig{
				ID:   "https://grandchild-in-parent",
				Path: "DEPS",
			},
		},
	}
	ctx, rm, _, _, _, _, _, urlMock, cleanup := setupGithubDEPS(t, cfg)
	defer cleanup()

	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Create a roll, assert that it's at tip of tree.
	mockGithubDEPSRequests(t, urlMock)
	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
	require.NoError(t, err)
	require.Equal(t, issueNum, issue)
}

// Verify that we ran the PreUploadSteps.
func TestGithubDEPSRepoManagerPreUploadSteps(t *testing.T) {
	unittest.LargeTest(t)

	// Create a dummy pre-upload step.
	ran := false
	stepName := parent.AddPreUploadStepForTesting(func(context.Context, []string, *http.Client, string) error {
		ran = true
		return nil
	})
	cfg := githubDEPSCfg(t)
	cfg.PreUploadSteps = []string{stepName}
	ctx, rm, _, _, _, _, _, urlMock, cleanup := setupGithubDEPS(t, cfg)
	defer cleanup()
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Create a roll, assert that we ran the PreUploadSteps.
	mockGithubDEPSRequests(t, urlMock)
	_, createErr := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
	require.NoError(t, createErr)
	require.True(t, ran)
}

// Verify that we fail when a PreUploadStep fails.
func TestGithubDEPSRepoManagerPreUploadStepsError(t *testing.T) {
	unittest.LargeTest(t)

	// Create a dummy pre-upload step.
	ran := false
	expectedErr := errors.New("Expected error")
	stepName := parent.AddPreUploadStepForTesting(func(context.Context, []string, *http.Client, string) error {
		ran = true
		return expectedErr
	})
	cfg := githubDEPSCfg(t)
	cfg.PreUploadSteps = []string{stepName}
	ctx, rm, _, _, _, _, _, urlMock, cleanup := setupGithubDEPS(t, cfg)
	defer cleanup()

	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Create a roll, assert that we ran the PreUploadSteps.
	mockGithubDEPSRequests(t, urlMock)
	_, createErr := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
	require.Error(t, expectedErr, createErr)
	require.True(t, ran)
}
