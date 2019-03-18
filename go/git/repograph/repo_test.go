package repograph

import (
	"context"
	"io/ioutil"
	"testing"

	assert "github.com/stretchr/testify/require"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
)

type repoUpdater struct{}

// No-op, since the repo held by Graph is updated by the Graph during Update.
func (u *repoUpdater) addCommits(...*vcsinfo.LongCommit) {}

// setupRepo performs common setup for git.Repo based Graphs.
func setupRepo(t *testing.T) (context.Context, *git_testutils.GitBuilder, *Graph, updater, func()) {
	ctx, g, cleanup := commonSetup(t)

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	repo, err := NewLocalGraph(ctx, g.Dir(), tmp)
	assert.NoError(t, err)

	return ctx, g, repo, &repoUpdater{}, func() {
		testutils.RemoveAll(t, tmp)
		cleanup()
	}
}

func TestGraphRepo(t *testing.T) {
	testutils.MediumTest(t)
	ctx, g, repo, ud, cleanup := setupRepo(t)
	defer cleanup()
	testGraph(t, ctx, g, repo, ud)
}

func TestRecurseRepo(t *testing.T) {
	testutils.MediumTest(t)
	ctx, g, repo, ud, cleanup := setupRepo(t)
	defer cleanup()
	testRecurse(t, ctx, g, repo, ud)
}

func TestRecurseAllBranchesRepo(t *testing.T) {
	testutils.MediumTest(t)
	ctx, g, repo, ud, cleanup := setupRepo(t)
	defer cleanup()
	testRecurseAllBranches(t, ctx, g, repo, ud)
}

func TestUpdateHistoryChangedRepo(t *testing.T) {
	testutils.MediumTest(t)
	ctx, g, repo, ud, cleanup := setupRepo(t)
	defer cleanup()
	testUpdateHistoryChanged(t, ctx, g, repo, ud)
}

func TestUpdateAndReturnNewCommitsRepo(t *testing.T) {
	testutils.MediumTest(t)
	ctx, g, repo, ud, cleanup := setupRepo(t)
	defer cleanup()
	testUpdateAndReturnNewCommits(t, ctx, g, repo, ud)
}

func TestRevListRepo(t *testing.T) {
	testutils.MediumTest(t)
	ctx, g, repo, ud, cleanup := setupRepo(t)
	defer cleanup()
	testRevList(t, ctx, g, repo, ud)
}
