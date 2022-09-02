package repograph_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/git/repograph/shared_tests"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/vcsinfo"
)

// memRefresher is a RepoImplRefresher backed by a local git repo.
type memRefresher struct {
	repo *git.Repo
	ri   *repograph.MemCacheRepoImpl
	t    *testing.T
}

func (u *memRefresher) Refresh(commits ...*vcsinfo.LongCommit) {
	for _, c := range commits {
		u.ri.Commits[c.Hash] = c
	}
	branches, err := u.repo.Branches(context.Background())
	require.NoError(u.t, err)
	u.ri.BranchList = branches
}

// setupMem performs common setup for git.Repo based Graphs.
func setupMem(t *testing.T) (context.Context, *git_testutils.GitBuilder, *repograph.Graph, shared_tests.RepoImplRefresher, func()) {
	ctx, g, cleanup := shared_tests.CommonSetup(t)
	repo := &git.Repo{GitDir: git.GitDir(g.Dir())}
	ri := repograph.NewMemCacheRepoImpl(map[string]*vcsinfo.LongCommit{}, []*git.Branch{})
	graph, err := repograph.NewWithRepoImpl(ctx, ri)
	require.NoError(t, err)
	return ctx, g, graph, &memRefresher{
		repo: repo,
		ri:   ri,
		t:    t,
	}, cleanup
}

func TestGraphWellFormedMem(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupMem(t)
	defer cleanup()
	shared_tests.TestGraphWellFormed(t, ctx, g, repo, ud)
}

func TestRecurseMem(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupMem(t)
	defer cleanup()
	shared_tests.TestRecurse(t, ctx, g, repo, ud)
}

func TestRecurseAllBranchesMem(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupMem(t)
	defer cleanup()
	shared_tests.TestRecurseAllBranches(t, ctx, g, repo, ud)
}

func TestLogLinearMem(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupMem(t)
	defer cleanup()
	shared_tests.TestLogLinear(t, ctx, g, repo, ud)
}

func TestUpdateHistoryChangedMem(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupMem(t)
	defer cleanup()
	shared_tests.TestUpdateHistoryChanged(t, ctx, g, repo, ud)
}

func TestUpdateAndReturnCommitDiffsMem(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupMem(t)
	defer cleanup()
	shared_tests.TestUpdateAndReturnCommitDiffs(t, ctx, g, repo, ud)
}

func TestRevListMem(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupMem(t)
	defer cleanup()
	shared_tests.TestRevList(t, ctx, g, repo, ud)
}

func TestBranchMembershipMem(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupMem(t)
	defer cleanup()
	shared_tests.TestBranchMembership(t, ctx, g, repo, ud)
}
