package task_scheduler

import (
	"path"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

func TestTaskSpecs(t *testing.T) {
	testutils.SkipIfShort(t)

	tr := util.NewTempRepo()
	defer tr.Cleanup()

	repos := gitinfo.NewRepoMap(tr.Dir)
	cache := newTaskCfgCache(repos)

	c1 := "b993cfa023855f4e27f0280465d477b0e0969708"
	c2 := "21e4383ee704174c6ca715645181e076c4a30bdd"
	repo := "skia.git"
	specs, err := cache.GetTaskSpecsForCommits(map[string][]string{
		repo: []string{c1, c2},
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(specs[repo]))

	// c1 has a Build and Test task.
	assert.Equal(t, 2, len(specs[repo][c1]))

	// c2 adds a Perf task.
	assert.Equal(t, 3, len(specs[repo][c2]))
}

func TestTaskCfgCacheCleanup(t *testing.T) {
	testutils.SkipIfShort(t)

	tr := util.NewTempRepo()
	defer tr.Cleanup()

	repos := gitinfo.NewRepoMap(tr.Dir)
	cache := newTaskCfgCache(repos)

	// Load configs into the cache.
	c1 := "b993cfa023855f4e27f0280465d477b0e0969708"
	c2 := "21e4383ee704174c6ca715645181e076c4a30bdd"
	repo := "skia.git"
	_, err := cache.GetTaskSpecsForCommits(map[string][]string{
		repo: []string{c1, c2},
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(cache.cache[repo]))

	// Cleanup, with a period intentionally designed to remove c1 but not c2.
	r, err := gitinfo.NewGitInfo(path.Join(tr.Dir, repo), false, false)
	assert.NoError(t, err)
	d1, err := r.Details(c1, false)
	assert.NoError(t, err)
	// c1 and c2 are about 1 minute apart.
	period := time.Now().Sub(d1.Timestamp) - 25*time.Second
	assert.NoError(t, cache.Cleanup(period))
	assert.Equal(t, 1, len(cache.cache[repo]))
}
