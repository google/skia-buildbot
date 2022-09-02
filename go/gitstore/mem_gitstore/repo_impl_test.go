package mem_gitstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	repograph_shared_tests "go.skia.org/infra/go/git/repograph/shared_tests"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/vcsinfo"
)

// gitstoreRefresher is an implementation of repograph_shared_tests.RepoImplRefresher
// used for testing a GitStore.
type gitstoreRefresher struct {
	gs   gitstore.GitStore
	repo git.GitDir
	t    *testing.T
}

func newGitstoreUpdater(t *testing.T, gs gitstore.GitStore, gb *git_testutils.GitBuilder) repograph_shared_tests.RepoImplRefresher {
	return &gitstoreRefresher{
		gs:   gs,
		repo: git.GitDir(gb.Dir()),
		t:    t,
	}
}

func (u *gitstoreRefresher) Refresh(commits ...*vcsinfo.LongCommit) {
	ctx := context.Background()
	// Add the commits.
	require.NoError(u.t, u.gs.Put(ctx, commits))
	branches, err := u.repo.Branches(ctx)
	require.NoError(u.t, err)
	putBranches := make(map[string]string, len(branches))
	for _, branch := range branches {
		putBranches[branch.Name] = branch.Head
	}
	oldBranches, err := u.gs.GetBranches(ctx)
	require.NoError(u.t, err)
	for name := range oldBranches {
		if name == gitstore.ALL_BRANCHES {
			continue
		}
		if _, ok := putBranches[name]; !ok {
			putBranches[name] = gitstore.DELETE_BRANCH
		}
	}
	require.NoError(u.t, u.gs.PutBranches(ctx, putBranches))
}

// setupGitStore performs common setup for GitStore based Graphs.
func setupGitStore(t *testing.T) (context.Context, *git_testutils.GitBuilder, *repograph.Graph, repograph_shared_tests.RepoImplRefresher, func()) {
	ctx, g, cleanup := repograph_shared_tests.CommonSetup(t)

	gs := New()
	ud := newGitstoreUpdater(t, gs, g)
	repo, err := gitstore.GetRepoGraph(ctx, gs)
	require.NoError(t, err)
	return ctx, g, repo, ud, cleanup
}

func TestGraphWellFormedGitStore(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestGraphWellFormed(t, ctx, g, repo, ud)
}

func TestRecurseGitStore(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestRecurse(t, ctx, g, repo, ud)
}

func TestRecurseAllBranchesGitStore(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestRecurseAllBranches(t, ctx, g, repo, ud)
}

func TestLogLinearGitStore(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestLogLinear(t, ctx, g, repo, ud)
}

func TestUpdateHistoryChangedGitStore(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestUpdateHistoryChanged(t, ctx, g, repo, ud)
}

func TestUpdateAndReturnCommitDiffsGitStore(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestUpdateAndReturnCommitDiffs(t, ctx, g, repo, ud)
}

func TestRevListGitStore(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestRevList(t, ctx, g, repo, ud)
}

func TestBranchMembershipGitStore(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestBranchMembership(t, ctx, g, repo, ud)
}
