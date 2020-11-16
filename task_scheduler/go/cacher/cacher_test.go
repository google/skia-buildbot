package cacher

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	"go.chromium.org/luci/common/isolated"
	"go.skia.org/infra/go/cas/mocks"
	"go.skia.org/infra/go/deepequal/assertdeep"
	depot_tools_testutils "go.skia.org/infra/go/depot_tools/testutils"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/isolate_cache"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/syncer"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	tcc_testutils "go.skia.org/infra/task_scheduler/go/task_cfg_cache/testutils"
	"go.skia.org/infra/task_scheduler/go/types"
)

func setup(t *testing.T) (context.Context, *Cacher, *task_cfg_cache.TaskCfgCache, *isolate_cache.Cache, types.RepoState, func()) {
	ctx, gb, rev, _ := tcc_testutils.SetupTestRepo(t)
	ctx, cancel := context.WithCancel(ctx)

	rs := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: rev,
	}

	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	repos, err := repograph.NewLocalMap(ctx, []string{gb.RepoUrl()}, wd)
	require.NoError(t, err)
	depotTools := depot_tools_testutils.GetDepotTools(t, ctx)
	s := syncer.New(ctx, repos, depotTools, wd, 1)
	btProject, btInstance, btCleanup := tcc_testutils.SetupBigTable(t)
	btCleanupIsolate := isolate_cache.SetupSharedBigTable(t, btProject, btInstance)
	tcc, err := task_cfg_cache.NewTaskCfgCache(ctx, repos, btProject, btInstance, nil)
	require.NoError(t, err)
	isolateClient, err := isolate.NewClient(wd, isolate.ISOLATE_SERVER_URL_FAKE)
	require.NoError(t, err)
	isolateCache, err := isolate_cache.New(ctx, btProject, btInstance, nil)
	require.NoError(t, err)
	cas := &mocks.CAS{}
	c := New(s, tcc, isolateClient, isolateCache, cas)
	return ctx, c, tcc, isolateCache, rs, func() {
		testutils.RemoveAll(t, wd)
		gb.Cleanup()
		btCleanupIsolate()
		btCleanup()
		cancel()
	}
}

func TestGetOrCacheRepoState_AlreadySet(t *testing.T) {
	ctx, c, tcc, isolateCache, _, cleanup := setup(t)
	defer cleanup()

	// This RepoState can't be synced.
	rs := types.RepoState{
		Repo:     "fake/repo.git",
		Revision: "abc123",
	}

	// Verify that GetOrCacheRepoState returns an error when it needs to sync,
	// to ensure that we can trust the below result.
	_, err := c.GetOrCacheRepoState(ctx, rs)
	require.Error(t, err)

	// Insert entries into the task config and isolate caches.
	expect := &specs.TasksCfg{
		Tasks: map[string]*specs.TaskSpec{
			"task": {},
		},
		Jobs: map[string]*specs.JobSpec{
			"job": {},
		},
	}
	require.NoError(t, tcc.Set(ctx, rs, expect, nil))
	require.NoError(t, isolateCache.Set(ctx, rs, &isolate_cache.CachedValue{
		Isolated: map[string]*isolated.Isolated{},
		Error:    "",
	}))

	// Retrieve the cached value. Ensure that there is no error, which implies
	// that we didn't attempt to sync.
	actual, err := c.GetOrCacheRepoState(ctx, rs)
	require.NoError(t, err)
	assertdeep.Equal(t, expect, actual)
}

func TestGetOrCacheRepoState_Unset_Isolate(t *testing.T) {

}

func TestGetOrCacheRepoState_Unset_RBE(t *testing.T) {

}
