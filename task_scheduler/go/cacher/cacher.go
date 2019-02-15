package cacher

import (
	"context"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/syncer"
	"go.skia.org/infra/task_scheduler/go/types"
)

type Cacher struct {
	isolate *isolate.Client
	s       *syncer.Syncer
	tcc     *specs.TaskCfgCache
}

func New(s *syncer.Syncer, tcc *specs.TaskCfgCache, isolateClient *isolate.Client) *Cacher {
	return &Cacher{
		isolate: isolateClient,
		s:       s,
		tcc:     tcc,
	}
}

func (c *Cacher) GetOrCacheRepoState(ctx context.Context, rs types.RepoState) (*specs.TasksCfg, error) {
	var cfg *specs.TasksCfg
	var err error
	err = c.tcc.WithLockedEntry(ctx, rs, func(e *specs.CacheEntry) error {
		cfg, err = e.Get(ctx)
		if err != nil {
			return err
		}
		if cfg != nil {
			return nil
		}
		_ = c.s.TempGitRepo(ctx, rs, true, func(co *git.TempCheckout) error {
			cfg, err = specs.ReadTasksCfg(co.Dir())
			if err != nil {
				return err
			}

			// TODO(borenet): Also isolate the tasks.
			return nil
		})
		if err != nil && !specs.ErrorIsPermanent(err) {
			return err
		}
		if setErr := e.Set(ctx, cfg, err); setErr != nil {
			return setErr
		}
		return err
	})
	return cfg, err
}
