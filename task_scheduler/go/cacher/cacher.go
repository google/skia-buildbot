package cacher

import (
	"context"
	"path/filepath"

	"go.skia.org/infra/go/cas"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/syncer"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	"go.skia.org/infra/task_scheduler/go/types"
)

// Cacher is a struct which handles insertion of data for RepoStates into
// various caches used by Task Scheduler. It ensures that we only sync to a
// given RepoState once (barring transient errors).
type Cacher struct {
	rbeCas cas.CAS
	s      *syncer.Syncer
	tcc    *task_cfg_cache.TaskCfgCache
}

// New creates a Cacher instance.
func New(s *syncer.Syncer, tcc *task_cfg_cache.TaskCfgCache, rbeCas cas.CAS) *Cacher {
	return &Cacher{
		rbeCas: rbeCas,
		s:      s,
		tcc:    tcc,
	}
}

// GetOrCacheRepoState returns the cached value(s) for the given RepoState,
// performing the sync to obtain and insert the value(s) into the cache(s) if
// necessary.
func (c *Cacher) GetOrCacheRepoState(ctx context.Context, rs types.RepoState) (*specs.TasksCfg, error) {
	ltgr := c.s.LazyTempGitRepo(rs)
	defer ltgr.Done()

	// Obtain the TasksCfg.
	cv, err := c.tcc.SetIfUnset(ctx, rs, func(ctx context.Context) (*task_cfg_cache.CachedValue, error) {
		var tasksCfg *specs.TasksCfg
		err := ltgr.Do(ctx, func(co *git.TempCheckout) error {
			cfg, err := specs.ReadTasksCfg(co.Dir())
			if err != nil {
				return skerr.Wrap(err)
			}
			for _, casSpec := range cfg.CasSpecs {
				if casSpec.Digest == "" {
					root := filepath.Join(co.Dir(), casSpec.Root)
					digest, err := c.rbeCas.Upload(ctx, root, casSpec.Paths, casSpec.Excludes)
					if err != nil {
						return skerr.Wrap(err)
					}
					casSpec.Digest = digest
				}
			}
			tasksCfg = cfg
			return nil
		})
		if err != nil && !specs.ErrorIsPermanent(err) {
			return nil, skerr.Wrap(err)
		}
		errString := ""
		if err != nil {
			errString = err.Error()
		}
		return &task_cfg_cache.CachedValue{
			RepoState: rs,
			Cfg:       tasksCfg,
			Err:       errString,
		}, nil
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if cv.Err != "" {
		return nil, skerr.Fmt(cv.Err)
	}
	return cv.Cfg, nil
}
