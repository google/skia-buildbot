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
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/mockhttpclient"
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
		VersionFile: filepath.Join(childPath, "version.sha1"),
		Copies: []parent.CopyEntry{
			{
				SrcRelPath: "somefile.txt",
				DstRelPath: "somefile",
			},
		},
		Gerrit: &codereview.GerritConfig{
			URL:     "https://fake-skia-review.googlesource.com",
			Project: "fake-gerrit-project",
			Config:  codereview.GERRIT_CONFIG_CHROMIUM,
		},
	}
}

func setupCopy(t *testing.T) (context.Context, string, *parentChildRepoManager, *git_testutils.GitBuilder, *git_testutils.GitBuilder, *gitiles_testutils.MockRepo, []string, *mockhttpclient.URLMock, func()) {
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
	parent.Add(ctx, filepath.Join(childPath, "version.sha1"), childCommits[0])
	parent.Commit(ctx)

	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		if strings.Contains(cmd.Name, "git") && cmd.Args[0] == "push" {
			return nil
		}
		return exec.DefaultRun(ctx, cmd)
	})
	ctx = exec.NewContext(ctx, mockRun.Run)

	cfg := copyCfg(t)
	cfg.ChildRepo = child.RepoUrl()
	cfg.ParentRepo = parent.RepoUrl()
	cfg.ChildPath = path.Join(path.Base(parent.RepoUrl()), childPath)
	urlmock := mockhttpclient.NewURLMock()
	g := setupFakeGerrit(t, cfg.Gerrit, urlmock)

	// Mock requests for Update.
	mockChild := gitiles_testutils.NewMockRepo(t, child.RepoUrl(), git.GitDir(child.Dir()), urlmock)
	mockChild.MockGetCommit(ctx, "master")
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], childCommits[len(childCommits)-1]))
	for _, hash := range childCommits {
		mockChild.MockGetCommit(ctx, hash)
	}

	// Create the RepoManager.
	rm, err := NewCopyRepoManager(ctx, cfg, setupRegistry(t), wd, g, "fake.server.com", urlmock.Client(), gerritCR(t, g), false)
	require.NoError(t, err)

	// Update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, childCommits[0], lastRollRev.Id)
	require.Equal(t, childCommits[len(childCommits)-1], tipRev.Id)
	require.Equal(t, len(childCommits)-1, len(notRolledRevs))

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
	}

	return ctx, wd, rm, child, parent, mockChild, childCommits, urlmock, cleanup
}

// TestCopyRepoManager tests all aspects of the CopyRepoManager.
func TestCopyRepoManager(t *testing.T) {
	unittest.LargeTest(t)

	ctx, _, rm, child, _, mockChild, childCommits, _, cleanup := setupCopy(t)
	defer cleanup()

	// New commit landed.
	lastCommit := child.CommitGen(context.Background(), "abc.txt")

	// Mock requests for Update.
	mockChild.MockGetCommit(ctx, lastCommit)
	mockChild.MockGetCommit(ctx, "master")
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], lastCommit))
	for _, hash := range childCommits {
		mockChild.MockGetCommit(ctx, hash)
	}

	// Update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, childCommits[0], lastRollRev.Id)
	require.Equal(t, lastCommit, tipRev.Id)
	require.Equal(t, len(childCommits), len(notRolledRevs))
}

func TestCopyRepoManagerCreateNewRoll(t *testing.T) {
	unittest.LargeTest(t)

	ctx, _, rm, _, _, mockChild, childCommits, urlMock, cleanup := setupCopy(t)
	defer cleanup()

	// Mock requests for Update.
	mockChild.MockGetCommit(ctx, childCommits[len(childCommits)-1])
	mockChild.MockGetCommit(ctx, "master")
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], childCommits[len(childCommits)-1]))
	for _, hash := range childCommits {
		mockChild.MockGetCommit(ctx, hash)
	}

	// Update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, childCommits[len(childCommits)-1], tipRev.Id)

	// Create a roll, assert that it's at tip of tree.
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
	}
	respBody, err := json.Marshal(ci)
	require.NoError(t, err)
	respBody = append([]byte(")]}'\n"), respBody...)
	urlMock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/detail?o=ALL_REVISIONS", mockhttpclient.MockGetDialogue(respBody))

	// Mock the request to set the CQ.
	reqBody := []byte(`{"labels":{"Code-Review":1,"Commit-Queue":2},"message":"","reviewers":[{"reviewer":"reviewer@chromium.org"}]}`)
	urlMock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/revisions/ps1/review", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	// Upload the CL.
	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, false, fakeCommitMsg)
	require.NoError(t, err)
	require.Equal(t, int64(123), issue)
}
