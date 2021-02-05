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
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
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

func githubDEPSCfg(t *testing.T) *config.ParentChildRepoManagerConfig {
	return &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_DepsLocalGithubParent{
			DepsLocalGithubParent: &config.DEPSLocalGitHubParentConfig{
				DepsLocal: &config.DEPSLocalParentConfig{
					GitCheckout: &config.GitCheckoutParentConfig{
						GitCheckout: &config.GitCheckoutConfig{
							Branch:  git.DefaultBranch,
							RepoUrl: "todo.git",
						},
						Dep: &config.DependencyConfig{
							Primary: &config.VersionFileConfig{
								Id:   "todo.git",
								Path: deps_parser.DepsFileName,
							},
						},
					},
					ChildPath: githubCIPDDEPSChildPath,
				},
				Github: &config.GitHubConfig{
					RepoOwner: githubCIPDUser,
					RepoName:  "todo.git",
				},
				ForkRepoUrl: "todo.git",
			},
		},
		Child: &config.ParentChildRepoManagerConfig_GitCheckoutGithubChild{
			GitCheckoutGithubChild: &config.GitCheckoutGitHubChildConfig{
				GitCheckout: &config.GitCheckoutChildConfig{
					GitCheckout: &config.GitCheckoutConfig{
						Branch:  git.DefaultBranch,
						RepoUrl: "todo.git",
					},
				},
				RepoOwner: mockGithubUser,
				RepoName:  "todo.git",
			},
		},
	}
}

func setupGithubDEPS(t *testing.T, c *config.ParentChildRepoManagerConfig) (context.Context, *parentChildRepoManager, string, *git_testutils.GitBuilder, []string, *git_testutils.GitBuilder, *exec.CommandCollector, *mockhttpclient.URLMock, func()) {
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
	fork.Git(ctx, "remote", "set-url", git.DefaultRemote, parent.RepoUrl())
	fork.Git(ctx, "fetch", git.DefaultRemote)
	fork.Git(ctx, "checkout", git.DefaultBranch)
	fork.Git(ctx, "reset", "--hard", git.DefaultRemoteBranch)

	parentCfg := c.Parent.(*config.ParentChildRepoManagerConfig_DepsLocalGithubParent).DepsLocalGithubParent
	parentCfg.DepsLocal.GitCheckout.GitCheckout.RepoUrl = parent.RepoUrl()
	parentCfg.DepsLocal.GitCheckout.Dep.Primary.Id = child.RepoUrl()
	parentCfg.Github.RepoName = parent.RepoUrl()
	parentCfg.ForkRepoUrl = fork.RepoUrl()
	childCfg := c.Child.(*config.ParentChildRepoManagerConfig_GitCheckoutGithubChild).GitCheckoutGithubChild
	childCfg.GitCheckout.GitCheckout.RepoUrl = child.RepoUrl()
	childCfg.RepoName = child.RepoUrl()

	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		// Without this, the mock commands get confused with:
		// "Could not switch upstream branch from refs/remotes/remote/X to refs/remotes/origin/X"
		if strings.Contains(cmd.Name, "gclient") && (cmd.Args[0] == "sync" || cmd.Args[0] == "runhooks") {
			return nil
		}
		return exec.DefaultRun(ctx, cmd)
	})
	ctx = exec.NewContext(ctx, mockRun.Run)

	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g, urlmock := setupFakeGithubDEPS(ctx, t)
	rm, err := newParentChildRepoManager(ctx, c, setupRegistry(t), wd, "test_roller_name", recipesCfg, "fake.server.com", nil, githubCR(t, g))
	require.NoError(t, err)

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
	}

	return ctx, rm, wd, child, childCommits, parent, mockRun, urlmock, cleanup
}

func setupFakeGithubDEPS(ctx context.Context, t *testing.T) (*github.GitHub, *mockhttpclient.URLMock) {
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

func mockGithubRefRequests(t *testing.T, urlMock *mockhttpclient.URLMock, forkRepoURL string) {
	// Mock /refs endpoints.
	forkRepoMatches := parent.REGitHubForkRepoURL.FindStringSubmatch(forkRepoURL)
	forkRepoOwner := forkRepoMatches[2]
	forkRepoName := forkRepoMatches[3]
	testSHA := "xyz"
	serializedRef, err := json.Marshal(&github_api.Reference{
		Object: &github_api.GitObject{
			SHA: &testSHA,
		},
	})
	require.NoError(t, err)
	urlMock.MockOnce(fmt.Sprintf("%s/repos/%s/%s/git/refs/%s", githubApiUrl, forkRepoOwner, forkRepoName, "heads%2F"+git.DefaultBranch), mockhttpclient.MockGetDialogue(serializedRef))
	md := mockhttpclient.MockPostDialogueWithResponseCode("application/json", mockhttpclient.DONT_CARE_REQUEST, nil, http.StatusCreated)
	urlMock.MockOnce(fmt.Sprintf("%s/repos/%s/%s/git/refs", githubApiUrl, forkRepoOwner, forkRepoName), md)
	require.NoError(t, err)
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
	mockGithubRefRequests(t, urlMock, cfg.GetDepsLocalGithubParent().ForkRepoUrl)
	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, false, fakeCommitMsg)
	require.NoError(t, err)
	require.Equal(t, issueNum, issue)
}

func TestGithubDEPSRepoManagerCreateNewRollTransitive(t *testing.T) {
	unittest.LargeTest(t)

	cfg := githubDEPSCfg(t)
	parentCfg := cfg.Parent.(*config.ParentChildRepoManagerConfig_DepsLocalGithubParent).DepsLocalGithubParent
	parentCfg.DepsLocal.GitCheckout.Dep.Transitive = []*config.TransitiveDepConfig{
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
	childCfg := cfg.Child.(*config.ParentChildRepoManagerConfig_GitCheckoutGithubChild).GitCheckoutGithubChild
	childCfg.GitCheckout.GitCheckout.Dependencies = []*config.VersionFileConfig{
		{
			Id:   "https://grandchild-in-child",
			Path: "DEPS",
		},
	}
	ctx, rm, _, _, _, _, _, urlMock, cleanup := setupGithubDEPS(t, cfg)
	defer cleanup()

	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Create a roll, assert that it's at tip of tree.
	mockGithubDEPSRequests(t, urlMock)
	mockGithubRefRequests(t, urlMock, cfg.GetDepsLocalGithubParent().ForkRepoUrl)
	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, false, fakeCommitMsg)
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
	parentCfg := cfg.Parent.(*config.ParentChildRepoManagerConfig_DepsLocalGithubParent).DepsLocalGithubParent
	parentCfg.DepsLocal.PreUploadSteps = []config.PreUploadStep{stepName}
	ctx, rm, _, _, _, _, _, urlMock, cleanup := setupGithubDEPS(t, cfg)
	defer cleanup()
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Create a roll, assert that we ran the PreUploadSteps.
	mockGithubDEPSRequests(t, urlMock)
	mockGithubRefRequests(t, urlMock, parentCfg.ForkRepoUrl)
	_, createErr := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, false, fakeCommitMsg)
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
	parentCfg := cfg.Parent.(*config.ParentChildRepoManagerConfig_DepsLocalGithubParent).DepsLocalGithubParent
	parentCfg.DepsLocal.PreUploadSteps = []config.PreUploadStep{stepName}
	ctx, rm, _, _, _, _, _, urlMock, cleanup := setupGithubDEPS(t, cfg)
	defer cleanup()

	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Create a roll, assert that we ran the PreUploadSteps.
	mockGithubDEPSRequests(t, urlMock)
	_, createErr := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, false, fakeCommitMsg)
	require.Error(t, expectedErr, createErr)
	require.True(t, ran)
}
