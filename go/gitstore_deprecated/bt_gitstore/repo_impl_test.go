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
	repograph_shared_tests "go.skia.org/infra/go/git/repograph/shared_tests"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/gitstore_deprecated"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
)

// gitstoreRefresher is an implementation of repograph_shared_tests.RepoImplRefresher
// used for testing a GitStore.
type gitstoreRefresher struct {
	gs   gitstore_deprecated.GitStore
	repo git.GitDir
	t    *testing.T
}

func newGitstoreUpdater(t *testing.T, gs gitstore_deprecated.GitStore, gb *git_testutils.GitBuilder) repograph_shared_tests.RepoImplRefresher {
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
		if name == "" {
			continue
		}
		if _, ok := putBranches[name]; !ok {
			putBranches[name] = gitstore_deprecated.DELETE_BRANCH
		}
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
		for name := range actual {
			if _, ok := putBranches[name]; name != "" && !ok {
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
	ctx, g, cleanup := repograph_shared_tests.CommonSetup(t)

	btConf := &BTConfig{
		ProjectID:  "fake-project",
		InstanceID: fmt.Sprintf("fake-instance-%s", uuid.New()),
		TableID:    TESTING_TABLE_ID,
	}
	assert.NoError(t, InitBT(btConf))
	gs, err := New(context.Background(), btConf, g.RepoUrl())
	assert.NoError(t, err)
	ud := newGitstoreUpdater(t, gs, g)
	repo, err := gitstore_deprecated.GetRepoGraph(ctx, gs)
	assert.NoError(t, err)
	return ctx, g, repo, ud, cleanup
}

func TestGraphWellFormedBTGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestGraphWellFormed(t, ctx, g, repo, ud)
}

func TestRecurseBTGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestRecurse(t, ctx, g, repo, ud)
}

func TestRecurseAllBranchesBTGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestRecurseAllBranches(t, ctx, g, repo, ud)
}

func TestLogLinearBTGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestLogLinear(t, ctx, g, repo, ud)
}

func TestUpdateHistoryChangedBTGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestUpdateHistoryChanged(t, ctx, g, repo, ud)
}

func TestUpdateAndReturnCommitDiffsBTGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestUpdateAndReturnCommitDiffs(t, ctx, g, repo, ud)
}

func TestRevListBTGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestRevList(t, ctx, g, repo, ud)
}

func TestBranchMembershipBTGitStore(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitStore(t)
	defer cleanup()
	repograph_shared_tests.TestBranchMembership(t, ctx, g, repo, ud)
}
