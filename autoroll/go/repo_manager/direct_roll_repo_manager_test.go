package repo_manager

import (
	"context"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/go/testutils"
)

func directCfg() *DirectRepoManagerConfig {
	return &DirectRepoManagerConfig{
		DepotToolsRepoManagerConfig: DepotToolsRepoManagerConfig{
			CommonRepoManagerConfig: CommonRepoManagerConfig{
				ChildBranch:  "master",
				ParentBranch: "master",
				Strategy:     ROLL_STRATEGY_BATCH,
			},
		},
	}
}

func setupDirect(t *testing.T) (context.Context, string, *git_testutils.GitBuilder, []string, *git_testutils.GitBuilder, *exec.CommandCollector, func()) {
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	// Create child and parent repos.
	ctx := context.Background()
	child := git_testutils.GitInit(t, context.Background())
	f := "somefile.txt"
	childCommits := make([]string, 0, 10)
	for i := 0; i < numChildCommits; i++ {
		childCommits = append(childCommits, child.CommitGen(context.Background(), f))
	}

	parent := git_testutils.GitInit(t, context.Background())
	parent.Add(ctx, "somefile", "dummy")
	parent.Add(ctx, path.Join(childPath, DIRECT_VERSION_HASH_FILE), childCommits[0])
	parent.Commit(ctx)

	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(cmd *exec.Command) error {
		if cmd.Name == "git" && cmd.Args[0] == "cl" {
			if cmd.Args[1] == "upload" {
				return nil
			} else if cmd.Args[1] == "issue" {
				json := testutils.MarshalJSON(t, &issueJson{
					Issue:    issueNum,
					IssueUrl: "???",
				})
				f := strings.Split(cmd.Args[2], "=")[1]
				testutils.WriteFile(t, f, json)
				return nil
			}
		}
		return exec.DefaultRun(cmd)
	})
	ctx = exec.NewContext(ctx, mockRun.Run)

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
	}

	return ctx, wd, child, childCommits, parent, mockRun, cleanup
}

// TestDirectRepoManager tests all aspects of the DirectRepoManager.
func TestDirectRepoManager(t *testing.T) {
	testutils.LargeTest(t)

	ctx, wd, child, childCommits, parent, _, cleanup := setupDirect(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g := setupFakeGerrit(t, wd)
	cfg := directCfg()
	cfg.ChildRepo = child.RepoUrl()
	cfg.ParentRepo = parent.RepoUrl()
	cfg.ChildPath = path.Join(path.Base(parent.RepoUrl()), childPath)
	rm, err := NewDirectRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com")
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

	// User, name only.
	assert.Equal(t, mockUser, rm.User())
}

func TestDirectCreateNewDEPSRoll(t *testing.T) {
	testutils.LargeTest(t)

	ctx, wd, child, childCommits, parent, _, cleanup := setupDirect(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g := setupFakeGerrit(t, wd)
	cfg := directCfg()
	cfg.ChildRepo = child.RepoUrl()
	cfg.ParentRepo = parent.RepoUrl()
	cfg.ChildPath = path.Join(path.Base(parent.RepoUrl()), childPath)
	rm, err := NewDirectRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com")
	assert.NoError(t, err)

	// Create a roll, assert that it's at tip of tree.
	issue, err := rm.CreateNewRoll(ctx, rm.LastRollRev(), rm.NextRollRev(), emails, cqExtraTrybots, false)
	assert.NoError(t, err)
	assert.Equal(t, issueNum, issue)
	msg, err := ioutil.ReadFile(path.Join(rm.(*directRepoManager).parentDir, ".git", "COMMIT_EDITMSG"))
	assert.NoError(t, err)
	from, to, err := autoroll.RollRev(strings.Split(string(msg), "\n")[0], func(h string) (string, error) {
		return git.GitDir(child.Dir()).RevParse(ctx, h)
	})
	assert.NoError(t, err)
	assert.Equal(t, childCommits[0], from)
	assert.Equal(t, childCommits[len(childCommits)-1], to)
}
