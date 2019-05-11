package repograph

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
)

type gitstoreRefresher struct {
	gs   gitstore.GitStore
	repo git.GitDir
	t    *testing.T
}

func newGitstoreUpdater(t *testing.T, gs gitstore.GitStore, gb *git_testutils.GitBuilder) refresher {
	return &gitstoreRefresher{
		gs:   gs,
		repo: git.GitDir(gb.Dir()),
		t:    t,
	}
}

func (u *gitstoreRefresher) refresh(commits ...*vcsinfo.LongCommit) {
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

// setupGitStore performs common setup for GitStore based Graphs.
func setupGitStore(t *testing.T) (context.Context, *git_testutils.GitBuilder, *Graph, refresher, func()) {
	ctx, g, cleanup := commonSetup(t)

	btConf := &bt_gitstore.BTConfig{
		ProjectID:  "fake-project",
		InstanceID: fmt.Sprintf("fake-instance-%s", uuid.New()),
		TableID:    "repograph-gitstore",
	}
	assert.NoError(t, bt_gitstore.InitBT(btConf))
	gs, err := bt_gitstore.New(context.Background(), btConf, g.RepoUrl())
	assert.NoError(t, err)
	ud := newGitstoreUpdater(t, gs, g)
	repo, err := NewGitStoreGraph(ctx, gs)
	assert.NoError(t, err)
	return ctx, g, repo, ud, cleanup
}

func TestGraphGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	testGraph(t, ctx, g, repo, ud)
}

func TestRecurseGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	testRecurse(t, ctx, g, repo, ud)
}

func TestRecurseAllBranchesGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	testRecurseAllBranches(t, ctx, g, repo, ud)
}

/*
TODO(borenet): This test is disabled because GitStore doesn't support deleting
branches.
func TestUpdateHistoryChangedGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	testUpdateHistoryChanged(t, ctx, g, repo, ud)
}*/

func TestUpdateAndReturnCommitDiffsGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	testUpdateAndReturnCommitDiffs(t, ctx, g, repo, ud)
}

func TestRevListGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	testRevList(t, ctx, g, repo, ud)
}
