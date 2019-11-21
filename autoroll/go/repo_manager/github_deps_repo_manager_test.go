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

	github_api "github.com/google/go-github/github"
	"github.com/stretchr/testify/require"
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

func githubDEPSCfg() *GithubDEPSRepoManagerConfig {
	return &GithubDEPSRepoManagerConfig{
		DepotToolsRepoManagerConfig: DepotToolsRepoManagerConfig{
			CommonRepoManagerConfig: CommonRepoManagerConfig{
				ChildBranch:  "master",
				ChildPath:    childPath,
				ParentBranch: "master",
			},
		},
	}
}

func TestGithubDEPSConfigValidation(t *testing.T) {
	unittest.SmallTest(t)

	cfg := githubDEPSCfg()
	cfg.ParentRepo = "repo" // Excluded from githubCfg.
	require.NoError(t, cfg.Validate())

	// The only fields come from the nested Configs, so exclude them and
	// verify that we fail validation.
	cfg = &GithubDEPSRepoManagerConfig{}
	require.Error(t, cfg.Validate())
}

func setupGithubDEPS(t *testing.T) (context.Context, string, *git_testutils.GitBuilder, []string, *git_testutils.GitBuilder, *exec.CommandCollector, func()) {
	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	// Create child and parent repos.
	grandchild := git_testutils.GitInit(t, context.Background())
	f := "somefile.txt"
	grandchildA := grandchild.CommitGen(context.Background(), f)
	grandchildB := grandchild.CommitGen(context.Background(), f)

	child := git_testutils.GitInit(t, context.Background())
	child.Add(context.Background(), "DEPS", fmt.Sprintf(`deps = {
  "child/dep": "%s@%s"
}`, grandchild.RepoUrl(), grandchildB))
	child.Commit(context.Background())
	childCommits := make([]string, 0, 10)
	for i := 0; i < numChildCommits; i++ {
		childCommits = append(childCommits, child.CommitGen(context.Background(), f))
	}

	parent := git_testutils.GitInit(t, context.Background())
	parent.Add(context.Background(), "DEPS", fmt.Sprintf(`deps = {
  "%s": "%s@%s",
  "parent/dep": "%s@%s",
}`, childPath, child.RepoUrl(), childCommits[0], grandchild.RepoUrl(), grandchildA))
	parent.Commit(context.Background())

	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		// Without this, the mock commands get confused with:
		// "Could not switch upstream branch from refs/remotes/remote/master to refs/remotes/origin/master"
		if strings.Contains(cmd.Name, "gclient") && (cmd.Args[0] == "sync" || cmd.Args[0] == "runhooks") {
			return nil
		}
		return exec.DefaultRun(ctx, cmd)
	})
	ctx := exec.NewContext(context.Background(), mockRun.Run)

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
	}

	return ctx, wd, child, childCommits, parent, mockRun, cleanup
}

func setupFakeGithubDEPS(t *testing.T) (*github.GitHub, *mockhttpclient.URLMock) {
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
	patchReqBody := []byte(`{"labels":["autoroller: commit"]}
`)
	patchMd := mockhttpclient.MockPatchDialogue(patchReqType, patchReqBody, patchRespBody)
	urlMock.MockOnce(githubApiUrl+"/repos/superman/krypton/issues/12345", patchMd)

	g, err := github.NewGitHub(context.Background(), "superman", "krypton", urlMock.Client())
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

	ctx, wd, child, childCommits, parent, _, cleanup := setupGithubDEPS(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g, _ := setupFakeGithubDEPS(t)
	cfg := githubDEPSCfg()
	cfg.ParentRepo = parent.RepoUrl()
	rm, err := NewGithubDEPSRepoManager(ctx, cfg, wd, "test_roller_name", g, recipesCfg, "fake.server.com", nil, githubCR(t, g), false)
	require.NoError(t, err)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, childCommits[0], lastRollRev.Id)
	require.Equal(t, childCommits[len(childCommits)-1], tipRev.Id)
	require.Equal(t, len(childCommits)-1, len(notRolledRevs))

	// Test update.
	lastCommit := child.CommitGen(context.Background(), "abc.txt")
	_, tipRev, _, err = rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, lastCommit, tipRev.Id)
}

func TestCreateNewGithubDEPSRoll(t *testing.T) {
	unittest.LargeTest(t)

	ctx, wd, _, _, parent, _, cleanup := setupGithubDEPS(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g, urlMock := setupFakeGithubDEPS(t)
	cfg := githubDEPSCfg()
	cfg.ParentRepo = parent.RepoUrl()
	rm, err := NewGithubDEPSRepoManager(ctx, cfg, wd, "test_roller_name", g, recipesCfg, "fake.server.com", nil, githubCR(t, g), false)
	require.NoError(t, err)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Create a roll, assert that it's at tip of tree.
	mockGithubDEPSRequests(t, urlMock)
	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
	require.NoError(t, err)
	require.Equal(t, issueNum, issue)
}

func TestCreateNewGithubDEPSRollTransitive(t *testing.T) {
	unittest.LargeTest(t)

	ctx, wd, _, _, parent, _, cleanup := setupGithubDEPS(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g, urlMock := setupFakeGithubDEPS(t)
	cfg := githubDEPSCfg()
	cfg.ParentRepo = parent.RepoUrl()
	cfg.TransitiveDeps = map[string]string{
		"child/dep": "parent/dep",
	}
	rm, err := NewGithubDEPSRepoManager(ctx, cfg, wd, "test_roller_name", g, recipesCfg, "fake.server.com", nil, githubCR(t, g), false)
	require.NoError(t, err)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Create a roll, assert that it's at tip of tree.
	mockGithubDEPSRequests(t, urlMock)
	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
	require.NoError(t, err)
	require.Equal(t, issueNum, issue)
}

// Verify that we ran the PreUploadSteps.
func TestRanPreUploadStepsGithubDEPS(t *testing.T) {
	unittest.LargeTest(t)

	ctx, wd, _, _, parent, _, cleanup := setupGithubDEPS(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g, urlMock := setupFakeGithubDEPS(t)
	cfg := githubDEPSCfg()
	cfg.ParentRepo = parent.RepoUrl()
	rm, err := NewGithubDEPSRepoManager(ctx, cfg, wd, "test_roller_name", g, recipesCfg, "fake.server.com", nil, githubCR(t, g), false)
	require.NoError(t, err)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	ran := false
	rm.(*githubDEPSRepoManager).preUploadSteps = []PreUploadStep{
		func(context.Context, []string, *http.Client, string) error {
			ran = true
			return nil
		},
	}

	// Create a roll, assert that we ran the PreUploadSteps.
	mockGithubDEPSRequests(t, urlMock)
	_, createErr := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
	require.NoError(t, createErr)
	require.True(t, ran)
}

// Verify that we fail when a PreUploadStep fails.
func TestErrorPreUploadStepsGithubDEPS(t *testing.T) {
	unittest.LargeTest(t)

	ctx, wd, _, _, parent, _, cleanup := setupGithubDEPS(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g, urlMock := setupFakeGithubDEPS(t)
	cfg := githubDEPSCfg()
	cfg.ParentRepo = parent.RepoUrl()
	rm, err := NewGithubDEPSRepoManager(ctx, cfg, wd, "test_roller_name", g, recipesCfg, "fake.server.com", nil, githubCR(t, g), false)
	require.NoError(t, err)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	ran := false
	expectedErr := errors.New("Expected error")
	rm.(*githubDEPSRepoManager).preUploadSteps = []PreUploadStep{
		func(context.Context, []string, *http.Client, string) error {
			ran = true
			return expectedErr
		},
	}

	// Create a roll, assert that we ran the PreUploadSteps.
	mockGithubDEPSRequests(t, urlMock)
	_, createErr := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
	require.Error(t, expectedErr, createErr)
	require.True(t, ran)
}
