package repo_manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	github_api "github.com/google/go-github/v29/github"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/exec"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func githubCR(t *testing.T, g *github.GitHub) codereview.CodeReview {
	rv, err := (&codereview.GithubConfig{
		RepoOwner:     "me",
		RepoName:      "my-repo",
		ChecksWaitFor: []string{"a", "b", "c"},
	}).Init(nil, g)
	require.NoError(t, err)
	return rv
}

func githubRmCfg(t *testing.T) *GithubRepoManagerConfig {
	return &GithubRepoManagerConfig{
		CommonRepoManagerConfig: CommonRepoManagerConfig{
			ChildBranch:  masterBranchTmpl(t),
			ChildPath:    "earth",
			ParentBranch: masterBranchTmpl(t),
			ParentRepo:   "git@github.com:jorel/krypton.git",
		},
		Github: &codereview.GithubConfig{
			RepoOwner:     "me",
			RepoName:      "my-repo",
			ChecksWaitFor: []string{"a", "b", "c"},
		},
		ChildRepoName: "earth",
		ChildUserName: "superman",
		RevisionFile:  "dummy-file.txt",
	}
}

func TestGithubRepoManagerConfigValidation(t *testing.T) {
	unittest.SmallTest(t)

	cfg := githubRmCfg(t)
	cfg.ChildRepoURL = "https://github.com/fake/child"
	cfg.ForkRepoURL = "https://github.com/fake/fork"
	cfg.ParentRepo = "https://github.com/fake/parent"
	require.NoError(t, cfg.Validate())

	// The only fields come from the nested Configs, so exclude them and
	// verify that we fail validation.
	cfg = &GithubRepoManagerConfig{}
	require.Error(t, cfg.Validate())
}

func setupGithub(t *testing.T, cfg *GithubRepoManagerConfig) (context.Context, *parentChildRepoManager, string, *git_testutils.GitBuilder, []string, *git_testutils.GitBuilder, *exec.CommandCollector, *mockhttpclient.URLMock, func()) {
	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	ctx := context.Background()

	// Create child and parent repos.
	childPath := filepath.Join(wd, "github_repos", "earth")
	require.NoError(t, os.MkdirAll(childPath, 0755))
	child := git_testutils.GitInitWithDir(t, ctx, childPath)
	f := "somefile.txt"
	childCommits := make([]string, 0, 10)
	for i := 0; i < numChildCommits; i++ {
		childCommits = append(childCommits, child.CommitGen(ctx, f))
	}

	parentPath := filepath.Join(wd, "github_repos", "krypton")
	require.NoError(t, os.MkdirAll(parentPath, 0755))
	parent := git_testutils.GitInitWithDir(t, ctx, parentPath)
	parent.Add(ctx, "dummy-file.txt", fmt.Sprintf(`%s`, childCommits[0]))
	parent.Commit(ctx)

	fork := git_testutils.GitInit(t, ctx)
	fork.Git(ctx, "remote", "set-url", "origin", parent.RepoUrl())
	fork.Git(ctx, "fetch", "origin")
	fork.Git(ctx, "checkout", "master")
	fork.Git(ctx, "reset", "--hard", "origin/master")

	cfg.ChildRepoURL = child.RepoUrl()
	cfg.ForkRepoURL = fork.RepoUrl()
	cfg.ParentRepo = parent.RepoUrl()

	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		if strings.Contains(cmd.Name, "git") {
			if cmd.Args[0] == "clone" || cmd.Args[0] == "fetch" {
				return nil
			}
			if cmd.Args[0] == "checkout" && cmd.Args[1] == "remote/master" {
				// Pretend origin is the remote branch for testing ease.
				cmd.Args[1] = "origin/master"
			}
		}
		return exec.DefaultRun(ctx, cmd)
	})
	ctx = exec.NewContext(ctx, mockRun.Run)

	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g, urlMock := setupFakeGithub(t, ctx, childCommits)
	rm, err := NewGithubRepoManager(ctx, cfg, setupRegistry(t), wd, "rollerName", g, recipesCfg, "fake.server.com", urlMock.Client(), githubCR(t, g), false)
	require.NoError(t, err)

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
	}

	return ctx, rm, wd, child, childCommits, parent, mockRun, urlMock, cleanup
}

func setupFakeGithub(t *testing.T, ctx context.Context, childCommits []string) (*github.GitHub, *mockhttpclient.URLMock) {
	urlMock := mockhttpclient.NewURLMock()

	// Mock /user endpoint.
	serializedUser, err := json.Marshal(&github_api.User{
		Login: &mockGithubUser,
		Email: &mockGithubUserEmail,
	})
	require.NoError(t, err)
	urlMock.MockOnce(githubApiUrl+"/user", mockhttpclient.MockGetDialogue(serializedUser))

	if childCommits != nil && len(childCommits) > 0 {
		// Mock getRawFile.
		urlMock.MockOnce("https://raw.githubusercontent.com/superman/krypton/master/dummy-file.txt", mockhttpclient.MockGetDialogue([]byte(childCommits[0])))
	}

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

func mockGithubRequests(t *testing.T, urlMock *mockhttpclient.URLMock) {
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

// TestGithubRepoManager tests all aspects of the Github RepoManager except for CreateNewRoll.
func TestGithubRepoManager(t *testing.T) {
	unittest.LargeTest(t)

	cfg := githubRmCfg(t)
	ctx, rm, _, _, childCommits, _, _, _, cleanup := setupGithub(t, cfg)
	defer cleanup()

	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, childCommits[0], lastRollRev.Id)
	require.Equal(t, childCommits[len(childCommits)-1], tipRev.Id)
	require.Equal(t, len(childCommits)-1, len(notRolledRevs))
}

func TestGithubRepoManagerCreateNewRoll(t *testing.T) {
	unittest.LargeTest(t)

	cfg := githubRmCfg(t)
	ctx, rm, _, _, _, _, _, urlMock, cleanup := setupGithub(t, cfg)
	defer cleanup()

	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Create a roll.
	mockGithubRequests(t, urlMock)
	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, false, fakeCommitMsg)
	require.NoError(t, err)
	require.Equal(t, issueNum, issue)
}

// Verify that we ran the PreUploadSteps.
func TestGithubRepoManagerPreUploadSteps(t *testing.T) {
	unittest.LargeTest(t)

	cfg := githubRmCfg(t)
	// Create a dummy pre-upload step.
	ran := false
	stepName := parent.AddPreUploadStepForTesting(func(context.Context, []string, *http.Client, string) error {
		ran = true
		return nil
	})
	cfg.PreUploadSteps = []string{stepName}
	ctx, rm, _, _, _, _, _, urlMock, cleanup := setupGithub(t, cfg)
	defer cleanup()

	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Create a roll, assert that we ran the PreUploadSteps.
	mockGithubRequests(t, urlMock)
	_, createErr := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, false, fakeCommitMsg)
	require.NoError(t, createErr)
	require.True(t, ran)
}

// Verify that we fail when a PreUploadStep fails.
func TestGithubRepoManagerPreUploadStepsError(t *testing.T) {
	unittest.LargeTest(t)

	cfg := githubRmCfg(t)
	// Create a dummy pre-upload step.
	ran := false
	expectedErr := errors.New("Expected error")
	stepName := parent.AddPreUploadStepForTesting(func(context.Context, []string, *http.Client, string) error {
		ran = true
		return expectedErr
	})
	cfg.PreUploadSteps = []string{stepName}

	ctx, rm, _, _, _, _, _, urlMock, cleanup := setupGithub(t, cfg)
	defer cleanup()

	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Create a roll, assert that we ran the PreUploadSteps.
	mockGithubRequests(t, urlMock)
	_, createErr := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, false, fakeCommitMsg)
	require.Error(t, expectedErr, createErr)
	require.True(t, ran)
}
