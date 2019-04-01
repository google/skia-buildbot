package repo_manager

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/go/testutils"
)

func copyCfg() *CopyRepoManagerConfig {
	return &CopyRepoManagerConfig{
		DepotToolsRepoManagerConfig: DepotToolsRepoManagerConfig{
			CommonRepoManagerConfig: CommonRepoManagerConfig{
				ChildBranch:  "master",
				ParentBranch: "master",
			},
		},
	}
}

func setupCopy(t *testing.T) (context.Context, string, *git_testutils.GitBuilder, []string, *git_testutils.GitBuilder, *exec.CommandCollector, func()) {
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	// Create child and parent repos.
	ctx := context.Background()
	child := git_testutils.GitInit(t, context.Background())
	f1 := "somefile.txt"
	f2 := "anotherfile.txt"
	childCommits := make([]string, 0, 10)
	childCommits = append(childCommits, child.CommitGen(context.Background(), f2))
	for i := 0; i < numChildCommits-1; i++ {
		childCommits = append(childCommits, child.CommitGen(context.Background(), f1))
	}

	parent := git_testutils.GitInit(t, context.Background())
	parent.Add(ctx, "somefile", "dummy")
	parent.Add(ctx, path.Join(childPath, COPY_VERSION_HASH_FILE), childCommits[0])
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

// TestCopyRepoManager tests all aspects of the CopyRepoManager.
func TestCopyRepoManager(t *testing.T) {
	testutils.LargeTest(t)

	ctx, wd, child, childCommits, parent, _, cleanup := setupCopy(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g := setupFakeGerrit(t, wd)
	cfg := copyCfg()
	cfg.ChildRepo = child.RepoUrl()
	cfg.ParentRepo = parent.RepoUrl()
	cfg.ChildPath = path.Join(path.Base(parent.RepoUrl()), childPath)
	rm, err := NewCopyRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com", nil, gerritCR(t, g), false)
	assert.NoError(t, err)
	assert.NoError(t, SetStrategy(ctx, rm, strategy.ROLL_STRATEGY_BATCH))
	assert.NoError(t, rm.Update(ctx))
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
}

func TestCopyCreateNewDEPSRoll(t *testing.T) {
	testutils.LargeTest(t)

	ctx, wd, child, childCommits, parent, _, cleanup := setupCopy(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	gUrl := "https://fake-skia-review.googlesource.com"
	urlMock := mockhttpclient.NewURLMock()
	serialized, err := json.Marshal(&gerrit.AccountDetails{
		AccountId: 101,
		Name:      mockUser,
		Email:     mockUser,
		UserName:  mockUser,
	})
	assert.NoError(t, err)
	serialized = append([]byte("abcd\n"), serialized...)
	urlMock.MockOnce(gUrl+"/a/accounts/self/detail", mockhttpclient.MockGetDialogue(serialized))
	gitcookies := path.Join(wd, "gitcookies_fake")
	assert.NoError(t, ioutil.WriteFile(gitcookies, []byte(".googlesource.com\tTRUE\t/\tTRUE\t123\to\tgit-user.google.com=abc123"), os.ModePerm))
	g, err := gerrit.NewGerrit(gUrl, gitcookies, urlMock.Client())
	assert.NoError(t, err)

	cfg := copyCfg()
	cfg.ChildRepo = child.RepoUrl()
	cfg.ParentRepo = parent.RepoUrl()
	cfg.ChildPath = path.Join(path.Base(parent.RepoUrl()), childPath)
	rm, err := NewCopyRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com", nil, gerritCR(t, g), false)
	assert.NoError(t, err)
	assert.NoError(t, SetStrategy(ctx, rm, strategy.ROLL_STRATEGY_BATCH))
	assert.NoError(t, rm.Update(ctx))

	// Create a roll, assert that it's at tip of tree.
	ci := gerrit.ChangeInfo{
		ChangeId: "12345",
		Id:       "12345",
		Issue:    12345,
		Revisions: map[string]*gerrit.Revision{
			"ps1": &gerrit.Revision{
				ID:     "ps1",
				Number: 1,
			},
		},
	}
	respBody, err := json.Marshal(ci)
	assert.NoError(t, err)
	respBody = append([]byte(")]}'\n"), respBody...)
	urlMock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/12345/detail?o=ALL_REVISIONS", mockhttpclient.MockGetDialogue(respBody))
	issue, err := rm.CreateNewRoll(ctx, rm.LastRollRev(), rm.NextRollRev(), emails, cqExtraTrybots, false)
	assert.NoError(t, err)
	assert.Equal(t, issueNum, issue)
	msg, err := ioutil.ReadFile(path.Join(rm.(*copyRepoManager).parentDir, ".git", "COMMIT_EDITMSG"))
	assert.NoError(t, err)
	from, to, err := autoroll.RollRev(ctx, strings.Split(string(msg), "\n")[0], func(ctx context.Context, h string) (string, error) {
		return git.GitDir(child.Dir()).RevParse(ctx, h)
	})
	assert.NoError(t, err)
	assert.Equal(t, childCommits[0], from)
	assert.Equal(t, childCommits[len(childCommits)-1], to)
}
