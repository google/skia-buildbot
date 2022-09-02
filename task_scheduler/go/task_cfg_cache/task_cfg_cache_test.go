package task_cfg_cache

import (
	"context"
	"errors"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/atomic_miss_cache"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/testutils"
	tu "go.skia.org/infra/task_scheduler/go/task_cfg_cache/testutils"
	"go.skia.org/infra/task_scheduler/go/types"
)

func TestTaskSpecs(t *testing.T) {

	ctx, gb, c1, c2 := tu.SetupTestRepo(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repo, err := repograph.NewLocalGraph(ctx, gb.RepoUrl(), tmp)
	require.NoError(t, err)
	repos := repograph.Map{
		gb.RepoUrl(): repo,
	}
	require.NoError(t, repos.Update(ctx))

	project, instance, cleanup := tu.SetupBigTable(t)
	defer cleanup()
	cache, err := NewTaskCfgCache(ctx, repos, project, instance, nil)
	require.NoError(t, err)
	defer testutils.AssertCloses(t, cache)

	rs1 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c1,
	}
	rs2 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c2,
	}
	require.NoError(t, cache.Set(ctx, rs1, tu.TasksCfg1, nil))
	require.NoError(t, cache.Set(ctx, rs2, tu.TasksCfg2, nil))
	specs, err := cache.getTaskSpecsForRepoStates(ctx, []types.RepoState{rs1, rs2})
	require.NoError(t, err)
	// c1 has a Build and Test task, c2 has a Build, Test, and Perf task.
	total, countC1, countC2, countBuild, countTest, countPerf := 0, 0, 0, 0, 0, 0
	for rs, byName := range specs {
		for name := range byName {
			total++
			if rs.Revision == c1 {
				countC1++
			} else if rs.Revision == c2 {
				countC2++
			} else {
				t.Fatalf("Unknown commit: %q", rs.Revision)
			}
			if strings.HasPrefix(name, "Build") {
				countBuild++
			} else if strings.HasPrefix(name, "Test") {
				countTest++
			} else if strings.HasPrefix(name, "Perf") {
				countPerf++
			} else {
				t.Fatalf("Unknown task spec name: %q", name)
			}
		}
	}
	require.Equal(t, 2, countC1)
	require.Equal(t, 3, countC2)
	require.Equal(t, 2, countBuild)
	require.Equal(t, 2, countTest)
	require.Equal(t, 1, countPerf)
	require.Equal(t, 5, total)
}

func cacheLen(c *atomic_miss_cache.AtomicMissCache) int {
	length := 0
	c.ForEach(context.Background(), func(_ context.Context, _ string, _ atomic_miss_cache.Value) {
		length++
	})
	return length
}

func assertCacheLen(t *testing.T, c *atomic_miss_cache.AtomicMissCache, expect int) {
	require.Equal(t, expect, cacheLen(c))
}

func TestTaskCfgCacheCleanup(t *testing.T) {

	ctx, gb, c1, c2 := tu.SetupTestRepo(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repo, err := repograph.NewLocalGraph(ctx, gb.RepoUrl(), tmp)
	require.NoError(t, err)
	repos := repograph.Map{
		gb.RepoUrl(): repo,
	}
	require.NoError(t, repos.Update(ctx))
	project, instance, cleanup := tu.SetupBigTable(t)
	defer cleanup()
	cache, err := NewTaskCfgCache(ctx, repos, project, instance, nil)
	require.NoError(t, err)
	defer testutils.AssertCloses(t, cache)

	// Load configs into the cache.
	rs1 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c1,
	}
	rs2 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c2,
	}
	require.NoError(t, cache.Set(ctx, rs1, tu.TasksCfg1, nil))
	require.NoError(t, cache.Set(ctx, rs2, tu.TasksCfg2, nil))
	_, err = cache.getTaskSpecsForRepoStates(ctx, []types.RepoState{rs1, rs2})
	require.NoError(t, err)
	assertCacheLen(t, cache.cache, 2)

	// Cleanup, with a period intentionally designed to remove c1 but not c2.
	r, err := git.NewRepo(ctx, gb.RepoUrl(), tmp)
	require.NoError(t, err)
	d1, err := r.Details(ctx, c1)
	require.NoError(t, err)
	d2, err := r.Details(ctx, c2)
	diff := d2.Timestamp.Sub(d1.Timestamp)
	now := time.Now()
	period := now.Sub(d2.Timestamp) + (diff / 2)
	require.NoError(t, cache.Cleanup(ctx, period))
	assertCacheLen(t, cache.cache, 1)
}

func TestTaskCfgCacheError(t *testing.T) {

	// Verify that we properly cache merge errors.
	ctx, gb, c1, c2 := tu.SetupTestRepo(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repo, err := repograph.NewLocalGraph(ctx, gb.RepoUrl(), tmp)
	require.NoError(t, err)
	repos := repograph.Map{
		gb.RepoUrl(): repo,
	}
	require.NoError(t, repos.Update(ctx))
	project, instance, cleanup := tu.SetupBigTable(t)
	defer cleanup()
	cache, err := NewTaskCfgCache(ctx, repos, project, instance, nil)
	require.NoError(t, err)
	defer testutils.AssertCloses(t, cache)

	// Load configs into the cache.
	rs1 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c1,
	}
	rs2 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c2,
	}
	require.NoError(t, cache.Set(ctx, rs1, tu.TasksCfg1, nil))
	require.NoError(t, cache.Set(ctx, rs2, tu.TasksCfg2, nil))
	_, err = cache.getTaskSpecsForRepoStates(ctx, []types.RepoState{rs1, rs2})
	require.NoError(t, err)
	assertCacheLen(t, cache.cache, 2)

	rs3 := types.RepoState{
		Repo:     rs1.Repo,
		Revision: rs1.Revision,
		Patch: types.Patch{
			Server:   "my-server",
			Issue:    "12345",
			Patchset: "1",
		},
	}
	repoStates := []types.RepoState{rs3}

	// This is a permanent error. It shouldn't be returned from
	// getTaskSpecsForRepoStates, since that would block scheduling
	// permanently.
	storedErr := errors.New("error: Failed to merge in the changes.; Stdout+Stderr:\n")
	require.NoError(t, cache.Set(ctx, rs3, nil, storedErr))
	_, err = cache.getTaskSpecsForRepoStates(ctx, repoStates)
	require.NoError(t, err)
	_, cachedErr, err := cache.Get(ctx, rs3)
	require.EqualError(t, cachedErr, storedErr.Error())
	require.Nil(t, err)

	// Create a new cache, assert that we get the same error.
	cache2, err := NewTaskCfgCache(ctx, repos, project, instance, nil)
	require.NoError(t, err)
	defer testutils.AssertCloses(t, cache2)
	_, err = cache2.getTaskSpecsForRepoStates(ctx, repoStates)
	require.NoError(t, err)
	_, cachedErr, err = cache2.Get(ctx, rs3)
	require.EqualError(t, cachedErr, storedErr.Error())
	require.Nil(t, err)
}

func TestTaskCfgCacheStorage(t *testing.T) {

	ctx, gb, r1, _ := tu.SetupTestRepo(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repos, err := repograph.NewLocalMap(ctx, []string{gb.RepoUrl()}, tmp)
	require.NoError(t, err)
	require.NoError(t, repos.Update(ctx))

	project, instance, cleanup := tu.SetupBigTable(t)
	defer cleanup()
	c, err := NewTaskCfgCache(ctx, repos, project, instance, nil)
	require.NoError(t, err)
	defer testutils.AssertCloses(t, c)

	check := func(rs ...types.RepoState) {
		c2, err := NewTaskCfgCache(ctx, repos, project, instance, nil)
		require.NoError(t, err)
		defer testutils.AssertCloses(t, c2)
		for _, r := range rs {
			cfg, cachedErr, err := c2.Get(ctx, r)
			require.NoError(t, err)
			require.NoError(t, cachedErr)
			require.NotNil(t, cfg)
		}

		// Verify that the caches are updated as expected.
		require.Equal(t, cacheLen(c.cache), cacheLen(c2.cache))
		c.cache.ForEach(ctx, func(ctx context.Context, key string, value1 atomic_miss_cache.Value) {
			value2, err := c2.cache.Get(ctx, key)
			require.NoError(t, err)
			v1 := value1.(*CachedValue)
			v2 := value2.(*CachedValue)
			require.Equal(t, v1.Err, v2.Err)
			assertdeep.Equal(t, v1.Cfg, v2.Cfg)
			assertdeep.Equal(t, v1.RepoState, v2.RepoState)
		})
	}

	// Empty cache.
	check()

	// No entries.
	rs1 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: r1,
	}
	cfg, cachedErr, err := c.Get(ctx, rs1)
	require.Equal(t, ErrNoSuchEntry, err)
	require.Nil(t, cachedErr)
	require.Nil(t, cfg)
	assertCacheLen(t, c.cache, 0)
	taskSpecs, err := c.getTaskSpecsForRepoStates(ctx, []types.RepoState{rs1})
	require.NoError(t, err)
	require.Equal(t, 0, len(taskSpecs))

	// One entry.
	require.NoError(t, c.Set(ctx, rs1, tu.TasksCfg1, nil))
	check(rs1)

	// Cleanup() the cache to remove the entries.
	require.NoError(t, c.Cleanup(ctx, time.Duration(0)))
	assertCacheLen(t, c.cache, 0)
	check()

	// Add two commits with identical tasks.json hash and check serialization.
	r3 := gb.CommitGen(ctx, "otherfile.txt")
	rs3 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: r3,
	}
	r4 := gb.CommitGen(ctx, "otherfile.txt")
	rs4 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: r4,
	}
	require.NoError(t, repos.Update(ctx))
	require.NoError(t, c.Set(ctx, rs3, tu.TasksCfg2, nil))
	require.NoError(t, c.Set(ctx, rs4, tu.TasksCfg2, nil))
	_, err = c.getTaskSpecsForRepoStates(ctx, []types.RepoState{rs3, rs4})
	require.NoError(t, err)
	assertCacheLen(t, c.cache, 2)
	check(rs3, rs4)
}
