package repo_manager

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func copyCfg(t *testing.T) *CopyRepoManagerConfig {
	return &CopyRepoManagerConfig{
		DepotToolsRepoManagerConfig: DepotToolsRepoManagerConfig{
			CommonRepoManagerConfig: CommonRepoManagerConfig{
				ChildBranch:  masterBranchTmpl(t),
				ParentBranch: masterBranchTmpl(t),
			},
		},
	}
}

func setupCopy(t *testing.T) (context.Context, *config_vars.Registry, string, *git_testutils.GitBuilder, []string, *git_testutils.GitBuilder, *exec.CommandCollector, func()) {
	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)

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
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		if strings.Contains(cmd.Name, "git") && cmd.Args[0] == "cl" {
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
		return exec.DefaultRun(ctx, cmd)
	})
	ctx = exec.NewContext(ctx, mockRun.Run)

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
	}

	return ctx, setupRegistry(t), wd, child, childCommits, parent, mockRun, cleanup
}

// TestCopyRepoManager tests all aspects of the CopyRepoManager.
func TestCopyRepoManager(t *testing.T) {
	unittest.LargeTest(t)

	ctx, reg, wd, child, childCommits, parent, _, cleanup := setupCopy(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g := setupFakeGerrit(t, wd)
	cfg := copyCfg(t)
	cfg.ChildRepo = child.RepoUrl()
	cfg.ParentRepo = parent.RepoUrl()
	cfg.ChildPath = path.Join(path.Base(parent.RepoUrl()), childPath)
	rm, err := NewCopyRepoManager(ctx, cfg, reg, wd, g, recipesCfg, "fake.server.com", nil, gerritCR(t, g), false)
	require.NoError(t, err)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, childCommits[0], lastRollRev.Id)
	require.Equal(t, childCommits[len(childCommits)-1], tipRev.Id)
	require.Equal(t, len(childCommits)-1, len(notRolledRevs))

	// Test update.
	lastCommit := child.CommitGen(context.Background(), "abc.txt")
	lastRollRev, tipRev, notRolledRevs, err = rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, lastCommit, tipRev.Id)
}

func TestCopyCreateNewDEPSRoll(t *testing.T) {
	unittest.LargeTest(t)

	ctx, reg, wd, child, _, parent, _, cleanup := setupCopy(t)
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
	require.NoError(t, err)
	serialized = append([]byte("abcd\n"), serialized...)
	urlMock.MockOnce(gUrl+"/a/accounts/self/detail", mockhttpclient.MockGetDialogue(serialized))
	g, err := gerrit.NewGerrit(gUrl, urlMock.Client())
	require.NoError(t, err)

	cfg := copyCfg(t)
	cfg.ChildRepo = child.RepoUrl()
	cfg.ParentRepo = parent.RepoUrl()
	cfg.ChildPath = path.Join(path.Base(parent.RepoUrl()), childPath)
	rm, err := NewCopyRepoManager(ctx, cfg, reg, wd, g, recipesCfg, "fake.server.com", nil, gerritCR(t, g), false)
	require.NoError(t, err)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Create a roll, assert that it's at tip of tree.
	ci := gerrit.ChangeInfo{
		ChangeId: "12345",
		Id:       "12345",
		Issue:    12345,
		Revisions: map[string]*gerrit.Revision{
			"ps1": {
				ID:     "ps1",
				Number: 1,
			},
		},
	}
	respBody, err := json.Marshal(ci)
	require.NoError(t, err)
	respBody = append([]byte(")]}'\n"), respBody...)
	urlMock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/12345/detail?o=ALL_REVISIONS", mockhttpclient.MockGetDialogue(respBody))
	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
	require.NoError(t, err)
	require.Equal(t, issueNum, issue)
}
