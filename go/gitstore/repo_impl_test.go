package gitstore

import (
	"context"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	repograph_shared_tests "go.skia.org/infra/go/git/repograph/shared_tests"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
)

// testGitStore is an in-memory GitStore implementation used for testing
// gitStoreRepoImpl. It is incomplete and does not support indexed commits
// at all.
type testGitStore struct {
	// commits maps commit hashes to LongCommits.
	commits map[string]*vcsinfo.LongCommit
	// branches maps branch names to the commit hashes of their HEADs.
	branches map[string]string
}

// See documentation for GitStore interface.
func (gs *testGitStore) Put(ctx context.Context, commits []*vcsinfo.LongCommit) error {
	for _, c := range commits {
		gs.commits[c.Hash] = c
	}
	return nil
}

// See documentation for GitStore interface.
func (gs *testGitStore) Get(ctx context.Context, hashes []string) ([]*vcsinfo.LongCommit, error) {
	rv := make([]*vcsinfo.LongCommit, len(hashes))
	for idx, h := range hashes {
		rv[idx] = gs.commits[h]
	}
	return rv, nil
}

// See documentation for GitStore interface.
func (gs *testGitStore) PutBranches(ctx context.Context, branches map[string]string) error {
	for name, hash := range branches {
		if hash == DELETE_BRANCH {
			delete(gs.branches, name)
		} else {
			gs.branches[name] = hash
		}
	}
	return nil
}

// See documentation for GitStore interface.
func (gs *testGitStore) GetBranches(ctx context.Context) (map[string]*BranchPointer, error) {
	rv := make(map[string]*BranchPointer, len(gs.branches))
	for name, hash := range gs.branches {
		rv[name] = &BranchPointer{
			Head: hash,
		}
	}
	return rv, nil
}

// See documentation for GitStore interface.
func (gs *testGitStore) RangeN(ctx context.Context, startIndex, endIndex int, branch string) ([]*vcsinfo.IndexCommit, error) {
	return nil, skerr.Fmt("RangeN not implemented for testGitStore.")
}

// See documentation for GitStore interface.
func (gs *testGitStore) RangeByTime(ctx context.Context, start, end time.Time, branch string) ([]*vcsinfo.IndexCommit, error) {
	if branch != ALL_BRANCHES {
		return nil, skerr.Fmt("RangeByTime not implemented for single branches in testGitStore.")
	}
	rv := make([]*vcsinfo.IndexCommit, 0, len(gs.commits))
	for _, c := range gs.commits {
		if c.Timestamp.Before(end) && !c.Timestamp.Before(start) {
			rv = append(rv, &vcsinfo.IndexCommit{
				Hash:      c.Hash,
				Timestamp: c.Timestamp,
			})
		}
	}
	return rv, nil
}

// gitstoreRefresher is an implementation of repograph_shared_tests.RepoImplRefresher
// used for testing a GitStore.
type gitstoreRefresher struct {
	gs   GitStore
	repo git.GitDir
	t    *testing.T
}

func newGitstoreUpdater(t *testing.T, gs GitStore, gb *git_testutils.GitBuilder) repograph_shared_tests.RepoImplRefresher {
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
		putBranches[branch.Name] = branch.Head
	}
	oldBranches, err := u.gs.GetBranches(ctx)
	assert.NoError(u.t, err)
	for name := range oldBranches {
		if name == ALL_BRANCHES {
			continue
		}
		if _, ok := putBranches[name]; !ok {
			putBranches[name] = DELETE_BRANCH
		}
	}
	assert.NoError(u.t, u.gs.PutBranches(ctx, putBranches))
}

// setupGitStore performs common setup for GitStore based Graphs.
func setupGitStore(t *testing.T) (context.Context, *git_testutils.GitBuilder, *repograph.Graph, repograph_shared_tests.RepoImplRefresher, func()) {
	ctx, g, cleanup := repograph_shared_tests.CommonSetup(t)

	gs := &testGitStore{
		commits:  map[string]*vcsinfo.LongCommit{},
		branches: map[string]string{},
	}
	ud := newGitstoreUpdater(t, gs, g)
	repo, err := GetRepoGraph(ctx, gs)
	assert.NoError(t, err)
	return ctx, g, repo, ud, cleanup
}

func TestGraphWellFormedGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestGraphWellFormed(t, ctx, g, repo, ud)
}

func TestRecurseGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestRecurse(t, ctx, g, repo, ud)
}

func TestRecurseAllBranchesGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestRecurseAllBranches(t, ctx, g, repo, ud)
}

func TestLogLinearGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestLogLinear(t, ctx, g, repo, ud)
}

func TestUpdateHistoryChangedGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestUpdateHistoryChanged(t, ctx, g, repo, ud)
}

func TestUpdateAndReturnCommitDiffsGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestUpdateAndReturnCommitDiffs(t, ctx, g, repo, ud)
}

func TestRevListGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestRevList(t, ctx, g, repo, ud)
}

func TestBranchMembershipGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestBranchMembership(t, ctx, g, repo, ud)
}
