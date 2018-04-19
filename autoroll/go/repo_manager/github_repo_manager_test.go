package repo_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"testing"

	github_api "github.com/google/go-github/github"
	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/go/testutils"
)

var (
	githubEmails = []string{"reviewer@chromium.org"}

	mockGithubUser = "superman"
	testPullNumber = 12345
)

func githubCfg() *GithubRepoManagerConfig {
	return &GithubRepoManagerConfig{
		DepotToolsRepoManagerConfig: DepotToolsRepoManagerConfig{
			CommonRepoManagerConfig: CommonRepoManagerConfig{
				ChildBranch:  "master",
				ChildPath:    childPath,
				ParentBranch: "master",
				Strategy:     ROLL_STRATEGY_BATCH,
			},
		},
	}
}

func TestGithubConfigValidation(t *testing.T) {
	testutils.SmallTest(t)

	cfg := githubCfg()
	cfg.ParentRepo = "repo" // Excluded from githubCfg.
	assert.NoError(t, cfg.Validate())

	// The only fields come from the nested Configs, so exclude them and
	// verify that we fail validation.
	cfg = &GithubRepoManagerConfig{}
	assert.Error(t, cfg.Validate())
}

func setupGithub(t *testing.T) (context.Context, string, *git_testutils.GitBuilder, []string, *git_testutils.GitBuilder, *exec.CommandCollector, func()) {
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	// Create child and parent repos.
	child := git_testutils.GitInit(t, context.Background())
	f := "somefile.txt"
	childCommits := make([]string, 0, 10)
	for i := 0; i < numChildCommits; i++ {
		childCommits = append(childCommits, child.CommitGen(context.Background(), f))
	}

	parent := git_testutils.GitInit(t, context.Background())
	parent.Add(context.Background(), "DEPS", fmt.Sprintf(`deps = {
  "%s": "%s@%s",
}`, childPath, child.RepoUrl(), childCommits[0]))
	parent.Commit(context.Background())

	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(cmd *exec.Command) error {
		// Without this, the mock commands get confused with:
		// "Could not switch upstream branch from refs/remotes/remote/master to refs/remotes/origin/master"
		if strings.Contains(cmd.Name, "gclient") && (cmd.Args[0] == "sync" || cmd.Args[0] == "runhooks") {
			return nil
		}
		return exec.DefaultRun(cmd)
	})
	ctx := exec.NewContext(context.Background(), mockRun.Run)

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
	}

	return ctx, wd, child, childCommits, parent, mockRun, cleanup
}

func setupFakeGithub(t *testing.T, wd string, childCommits []string) *github.GitHub {
	gUrl := "https://api.github.com"
	urlMock := mockhttpclient.NewURLMock()

	// Mock /user endpoint.
	serializedUser, err := json.Marshal(&github_api.User{
		Login: &mockGithubUser,
	})
	assert.NoError(t, err)
	urlMock.MockOnce(gUrl+"/user", mockhttpclient.MockGetDialogue(serializedUser))

	// Mock /pulls endpoint.
	serializedPull, err := json.Marshal(&github_api.PullRequest{
		Number: &testPullNumber,
	})
	assert.NoError(t, err)
	reqType := "application/json"

	issueTitle := fmt.Sprintf("Roll path/to/child/ %s..%s (%d commits)", childCommits[0][:9], childCommits[len(childCommits)-1][:9], len(childCommits)-1)
	headBranch := fmt.Sprintf("%s:%s", mockGithubUser, ROLL_BRANCH)
	baseBranch := "master"
	reqBody := []byte(`{"title":"` + issueTitle + `","head":"` + headBranch + `","base":"` + baseBranch + `"}
`)
	md := mockhttpclient.MockPostDialogueWithResponseCode(reqType, reqBody, serializedPull, http.StatusCreated)
	urlMock.MockOnce(gUrl+"/repos/superman/krypton/pulls", md)

	g, err := github.NewGitHub(context.Background(), "superman", "krypton", urlMock.Client(), "")
	assert.NoError(t, err)
	return g
}

// TestGithubRepoManager tests all aspects of the GithubRepoManager except for CreateNewRoll.
func TestGithubRepoManager(t *testing.T) {
	testutils.LargeTest(t)

	ctx, wd, child, childCommits, parent, _, cleanup := setupGithub(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g := setupFakeGithub(t, wd, childCommits)
	cfg := githubCfg()
	cfg.ParentRepo = parent.RepoUrl()
	rm, err := NewGithubRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com")
	assert.NoError(t, err)
	assert.Equal(t, childCommits[0], rm.LastRollRev())
	assert.Equal(t, childCommits[len(childCommits)-1], rm.NextRollRev())

	// Test FullChildHash.
	for _, c := range childCommits {
		h, err := rm.FullChildHash(ctx, c[:12])
		assert.NoError(t, err)
		assert.Equal(t, c, h)
	}

	// Test update.
	lastCommit := child.CommitGen(context.Background(), "abc.txt")
	assert.NoError(t, rm.Update(ctx))
	assert.Equal(t, lastCommit, rm.NextRollRev())

	// RolledPast.
	rp, err := rm.RolledPast(ctx, childCommits[0])
	assert.NoError(t, err)
	assert.True(t, rp)
	for _, c := range childCommits[1:] {
		rp, err := rm.RolledPast(ctx, c)
		assert.NoError(t, err)
		assert.False(t, rp)
	}
	assert.Equal(t, mockGithubUser, rm.User())
}

func TestCreateNewGithubRoll(t *testing.T) {
	testutils.LargeTest(t)

	ctx, wd, child, childCommits, parent, _, cleanup := setupGithub(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g := setupFakeGithub(t, wd, childCommits)
	cfg := githubCfg()
	cfg.ParentRepo = parent.RepoUrl()
	rm, err := NewGithubRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com")
	assert.NoError(t, err)

	// Create a roll, assert that it's at tip of tree.
	issue, err := rm.CreateNewRoll(ctx, rm.LastRollRev(), rm.NextRollRev(), githubEmails, cqExtraTrybots, false)
	assert.NoError(t, err)
	assert.Equal(t, issueNum, issue)
	msg, err := ioutil.ReadFile(path.Join(rm.(*githubRepoManager).parentDir, ".git", "COMMIT_EDITMSG"))
	assert.NoError(t, err)
	from, to, err := autoroll.RollRev(strings.Split(string(msg), "\n")[0], func(h string) (string, error) {
		return git.GitDir(child.Dir()).RevParse(ctx, h)
	})
	assert.NoError(t, err)
	assert.Equal(t, childCommits[0], from)
	assert.Equal(t, childCommits[numChildCommits-1], to)
}

// Verify that we ran the PreUploadSteps.
func TestRanPreUploadStepsGithub(t *testing.T) {
	testutils.LargeTest(t)

	ctx, wd, _, childCommits, parent, _, cleanup := setupGithub(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g := setupFakeGithub(t, wd, childCommits)
	cfg := githubCfg()
	cfg.ParentRepo = parent.RepoUrl()
	rm, err := NewGithubRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com")
	assert.NoError(t, err)
	ran := false
	rm.(*githubRepoManager).preUploadSteps = []PreUploadStep{
		func(context.Context, string) error {
			ran = true
			return nil
		},
	}

	// Create a roll, assert that we ran the PreUploadSteps.
	_, createErr := rm.CreateNewRoll(ctx, rm.LastRollRev(), rm.NextRollRev(), githubEmails, cqExtraTrybots, false)
	assert.NoError(t, createErr)
	assert.True(t, ran)
}
