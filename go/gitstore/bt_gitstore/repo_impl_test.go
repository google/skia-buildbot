package bt_gitstore

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	repograph_shared_tests "go.skia.org/infra/go/git/repograph/shared_tests"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/testutils/unittest"
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
	update := make(map[string]*vcsinfo.LongCommit, len(commits))
	for _, commit := range commits {
		c, err := u.repo.Details(ctx, commit.Hash)
		require.NoError(u.t, err)
		// This is inefficient, but the test repo is small.
		hashes, err := u.repo.RevList(ctx, "--first-parent", c.Hash)
		require.NoError(u.t, err)
		c.Index = len(hashes) - 1
		c.Branches = map[string]bool{}
		update[c.Hash] = c
	}
	branches, err := u.repo.Branches(ctx)
	require.NoError(u.t, err)
	for _, b := range branches {
		hashes, err := u.repo.RevList(ctx, "--first-parent", b.Head)
		require.NoError(u.t, err)
		for _, hash := range hashes {
			c, ok := update[hash]
			if ok {
				c.Branches[b.Name] = true
			}
		}
	}
	putCommits := make([]*vcsinfo.LongCommit, 0, len(update))
	putHashes := make([]string, 0, len(update))
	for _, c := range update {
		putCommits = append(putCommits, c)
		putHashes = append(putHashes, c.Hash)
	}
	require.NoError(u.t, u.gs.Put(ctx, putCommits))
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

	// Wait for GitStore to be up to date.
	for {
		time.Sleep(10 * time.Millisecond)
		actual, err := u.gs.GetBranches(ctx)
		require.NoError(u.t, err)
		allMatch := true
		for _, expectBranch := range branches {
			actualBranch, ok := actual[expectBranch.Name]
			if !ok || actualBranch.Head != expectBranch.Head {
				allMatch = false
				break
			}
		}
		for name := range actual {
			if _, ok := putBranches[name]; name != gitstore.ALL_BRANCHES && !ok {
				allMatch = false
				break
			}
		}
		gotCommits, err := u.gs.Get(ctx, putHashes)
		require.NoError(u.t, err)
		for idx, expect := range putCommits {
			if !deepequal.DeepEqual(expect, gotCommits[idx]) {
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
func setupGitStore(t *testing.T) (context.Context, *git_testutils.GitBuilder, *repograph.Graph, repograph_shared_tests.RepoImplRefresher, func()) {
	unittest.RequiresBigTableEmulator(t)
	ctx, g, cleanup := repograph_shared_tests.CommonSetup(t)

	btConf := &BTConfig{
		ProjectID:  "fake-project",
		InstanceID: fmt.Sprintf("fake-instance-%s", uuid.New()),
		TableID:    "repograph-gitstore",
		AppProfile: "testing",
	}
	require.NoError(t, InitBT(btConf))
	gs, err := New(context.Background(), btConf, g.RepoUrl())
	require.NoError(t, err)
	ud := newGitstoreUpdater(t, gs, g)
	repo, err := gitstore.GetRepoGraph(ctx, gs)
	require.NoError(t, err)
	return ctx, g, repo, ud, cleanup
}

func TestGraphWellFormedBTGitStore(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestGraphWellFormed(t, ctx, g, repo, ud)
}

func TestRecurseBTGitStore(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestRecurse(t, ctx, g, repo, ud)
}

func TestRecurseAllBranchesBTGitStore(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestRecurseAllBranches(t, ctx, g, repo, ud)
}

func TestLogLinearBTGitStore(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestLogLinear(t, ctx, g, repo, ud)
}

func TestUpdateHistoryChangedBTGitStore(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestUpdateHistoryChanged(t, ctx, g, repo, ud)
}

func TestUpdateAndReturnCommitDiffsBTGitStore(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestUpdateAndReturnCommitDiffs(t, ctx, g, repo, ud)
}

func TestRevListBTGitStore(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestRevList(t, ctx, g, repo, ud)
}

func TestBranchMembershipBTGitStore(t *testing.T) {
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestBranchMembership(t, ctx, g, repo, ud)
}
