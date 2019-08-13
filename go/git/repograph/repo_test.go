package repograph

import (
	"context"
	"io/ioutil"
	"path"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
)

type repoRefresher struct{}

// No-op, since the repo held by Graph is updated by the Graph during Update.
func (u *repoRefresher) Refresh(...*vcsinfo.LongCommit) {}

// setupRepo performs common setup for git.Repo based Graphs.
func setupRepo(t sktest.TestingT) (context.Context, *git_testutils.GitBuilder, *Graph, RepoImplRefresher, func()) {
	ctx, g, cleanup := commonSetup(t)

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	repo, err := NewLocalGraph(ctx, g.Dir(), tmp)
	assert.NoError(t, err)

	return ctx, g, repo, &repoRefresher{}, func() {
		testutils.RemoveAll(t, tmp)
		cleanup()
	}
}

func TestGraphRepo(t *testing.T) {
	unittest.MediumTest(t)
	ctx, g, repo, ud, cleanup := setupRepo(t)
	defer cleanup()
	TestGraph(t, ctx, g, repo, ud)
}

func TestRecurseRepo(t *testing.T) {
	unittest.MediumTest(t)
	ctx, g, repo, ud, cleanup := setupRepo(t)
	defer cleanup()
	TestRecurse(t, ctx, g, repo, ud)
}

func TestRecurseAllBranchesRepo(t *testing.T) {
	unittest.MediumTest(t)
	ctx, g, repo, ud, cleanup := setupRepo(t)
	defer cleanup()
	TestRecurseAllBranches(t, ctx, g, repo, ud)
}

func TestUpdateHistoryChangedRepo(t *testing.T) {
	unittest.MediumTest(t)
	ctx, g, repo, ud, cleanup := setupRepo(t)
	defer cleanup()
	TestUpdateHistoryChanged(t, ctx, g, repo, ud)
}

func TestUpdateAndReturnCommitDiffsRepo(t *testing.T) {
	unittest.MediumTest(t)
	ctx, g, repo, ud, cleanup := setupRepo(t)
	defer cleanup()
	TestUpdateAndReturnCommitDiffs(t, ctx, g, repo, ud)
}

func TestRevListRepo(t *testing.T) {
	unittest.MediumTest(t)
	ctx, g, repo, ud, cleanup := setupRepo(t)
	defer cleanup()
	TestRevList(t, ctx, g, repo, ud)
}

func TestSerialize(t *testing.T) {
	unittest.MediumTest(t)
	ctx, g, repo, rf, cleanup := setupRepo(t)
	defer cleanup()
	gitSetup(t, ctx, g, repo, rf)

	wd, err := ioutil.TempDir("", "")
	defer testutils.RemoveAll(t, wd)
	cacheFile := path.Join(wd, CACHE_FILE)
	assert.NoError(t, writeCacheFile(repo, cacheFile))
	repo2 := &Graph{
		repoImpl: repo.repoImpl,
	}
	assert.NoError(t, initFromFile(repo2, cacheFile))
	assert.NoError(t, err)
	deepequal.AssertDeepEqual(t, repo, repo2)
}

func TestFindCommit(t *testing.T) {
	unittest.LargeTest(t)
	ctx1, g1, repo1, rf1, cleanup1 := setupRepo(t)
	defer cleanup1()
	commits1 := gitSetup(t, ctx1, g1, repo1, rf1)
	ctx2, g2, repo2, rf2, cleanup2 := setupRepo(t)
	defer cleanup2()
	commits2 := gitSetup(t, ctx2, g2, repo2, rf2)

	m := Map{
		g1.Dir(): repo1,
		g2.Dir(): repo2,
	}

	tc := []struct {
		hash string
		url  string
		repo *Graph
		err  bool
	}{
		{
			hash: commits1[0].Hash,
			url:  g1.Dir(),
			repo: repo1,
			err:  false,
		},
		{
			hash: commits1[len(commits1)-1].Hash,
			url:  g1.Dir(),
			repo: repo1,
			err:  false,
		},
		{
			hash: commits2[0].Hash,
			url:  g2.Dir(),
			repo: repo2,
			err:  false,
		},
		{
			hash: commits2[len(commits2)-1].Hash,
			url:  g2.Dir(),
			repo: repo2,
			err:  false,
		},
		{
			hash: "",
			err:  true,
		},
		{
			hash: "abcdef",
			err:  true,
		},
	}
	for _, c := range tc {
		commit, url, repo, err := m.FindCommit(c.hash)
		if c.err {
			assert.Error(t, err)
		} else {
			assert.Nil(t, err)
			assert.NotNil(t, commit)
			assert.Equal(t, c.hash, commit.Hash)
			assert.Equal(t, c.url, url)
			assert.Equal(t, c.repo, repo)
		}
	}
}
