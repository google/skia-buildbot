package bt_gitstore

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
)

type gitstoreRefresher struct {
	gs   gitstore.GitStore
	repo git.GitDir
	t    *testing.T
}

func newGitstoreRefresher(t *testing.T, gs gitstore.GitStore, gb *git_testutils.GitBuilder) repograph.RepoImplRefresher {
	return &gitstoreRefresher{
		gs:   gs,
		repo: git.GitDir(gb.Dir()),
		t:    t,
	}
}

func (u *gitstoreRefresher) Refresh(commits ...*vcsinfo.LongCommit) {
	ctx := context.Background()
	// Add the commits.
	assert.NoError(u.t, u.gs.Put(ctx, commits))
	branches, err := u.repo.Branches(ctx)
	assert.NoError(u.t, err)
	putBranches := make(map[string]string, len(branches))
	for _, branch := range branches {
		sklog.Infof("Put branch %s", branch.Name)
		putBranches[branch.Name] = branch.Head
	}
	assert.NoError(u.t, u.gs.PutBranches(ctx, putBranches))

	// Wait for GitStore to be up to date.
	for {
		time.Sleep(10 * time.Millisecond)
		actual, err := u.gs.GetBranches(ctx)
		assert.NoError(u.t, err)
		allMatch := true
		for _, expectBranch := range branches {
			actualBranch, ok := actual[expectBranch.Name]
			if !ok || actualBranch.Head != expectBranch.Head {
				allMatch = false
				break
			}
		}
		if allMatch {
			break
		}
	}
}

// setupGraph performs common setup for GitStore based Graphs.
func setupGraph(t *testing.T) (context.Context, *git_testutils.GitBuilder, *repograph.Graph, repograph.RepoImplRefresher, func()) {
	ctx := context.Background()
	g := git_testutils.GitInit(t, ctx)

	btConf := &BTConfig{
		ProjectID:  "fake-project",
		InstanceID: fmt.Sprintf("fake-instance-%s", uuid.New()),
		TableID:    "repograph-gitstore",
	}
	assert.NoError(t, InitBT(btConf))
	gs, err := New(context.Background(), btConf, g.RepoUrl())
	assert.NoError(t, err)
	ud := newGitstoreRefresher(t, gs, g)
	repo, err := gitstore.GetRepoGraph(ctx, gs)
	assert.NoError(t, err)
	return ctx, g, repo, ud, g.Cleanup
}

func TestGraphGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGraph(t)
	defer cleanup()
	repograph.TestGraph(t, ctx, g, repo, ud)
}

func TestRecurseGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGraph(t)
	defer cleanup()
	repograph.TestRecurse(t, ctx, g, repo, ud)
}

func TestRecurseAllBranchesGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGraph(t)
	defer cleanup()
	repograph.TestRecurseAllBranches(t, ctx, g, repo, ud)
}

/*
TODO(borenet): This test is disabled because GitStore doesn't support deleting
branches.
func TestUpdateHistoryChangedGitStore(t *testing.T) {
        unittest.LargeTest(t)
        ctx, g, repo, ud, cleanup := setupGraph(t)
        defer cleanup()
        repograph.TestUpdateHistoryChanged(t, ctx, g, repo, ud)
}*/

func TestUpdateAndReturnCommitDiffsGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGraph(t)
	defer cleanup()
	repograph.TestUpdateAndReturnCommitDiffs(t, ctx, g, repo, ud)
}

func TestRevListGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGraph(t)
	defer cleanup()
	repograph.TestRevList(t, ctx, g, repo, ud)
}
