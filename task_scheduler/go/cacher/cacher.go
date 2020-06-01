package cacher

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"go.chromium.org/luci/common/isolated"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/task_scheduler/go/isolate_cache"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/syncer"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	"go.skia.org/infra/task_scheduler/go/types"
)

// Cacher is a struct which handles insertion of data for RepoStates into
// various caches used by Task Scheduler. It ensures that we only sync to a
// given RepoState once (barring transient errors).
type Cacher struct {
	isolate      *isolate.Client
	isolateCache *isolate_cache.Cache
	s            *syncer.Syncer
	tcc          *task_cfg_cache.TaskCfgCache
}

// New creates a Cacher instance.
func New(s *syncer.Syncer, tcc *task_cfg_cache.TaskCfgCache, isolateClient *isolate.Client, isolateCache *isolate_cache.Cache) *Cacher {
	return &Cacher{
		isolate:      isolateClient,
		isolateCache: isolateCache,
		s:            s,
		tcc:          tcc,
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
				return err
			}
			tasksCfg = cfg
			return nil
		})
		if err != nil && !specs.ErrorIsPermanent(err) {
			return nil, err
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
		return nil, err
	}
	if cv.Err != "" {
		return nil, skerr.Fmt(cv.Err)
	}

	// Obtain the IsolatedFiles.
	if err := c.isolateCache.SetIfUnset(ctx, rs, func(ctx context.Context) (*isolate_cache.CachedValue, error) {
		isolatedFiles := map[string]*isolated.Isolated{}
		err := ltgr.Do(ctx, func(co *git.TempCheckout) error {
			// Isolates may need a .gclient file at the root of the checkout.
			if err := ioutil.WriteFile(path.Join(co.Dir(), "..", ".gclient"), []byte("dummy"), os.ModePerm); err != nil {
				return err
			}

			// Isolate all of the task specs and update their IsolateHashes.
			isolateFileNames := []string{}
			done := map[string]bool{}
			isolateTasks := []*isolate.Task{}
			for _, taskSpec := range cv.Cfg.Tasks {
				if done[taskSpec.Isolate] {
					continue
				}
				t := &isolate.Task{
					BaseDir:     co.Dir(),
					IsolateFile: path.Join(co.Dir(), "infra", "bots", taskSpec.Isolate),
					OsType:      "linux", // Unused by our isolates.
				}
				isolateFileNames = append(isolateFileNames, taskSpec.Isolate)
				isolateTasks = append(isolateTasks, t)
				done[taskSpec.Isolate] = true
			}
			// Now, isolate all of the tasks.
			if len(isolateTasks) > 0 {
				_, isolatedFilesSlice, err := c.isolate.IsolateTasks(ctx, isolateTasks)
				if err != nil {
					return err
				}
				if len(isolatedFilesSlice) != len(isolateTasks) {
					return fmt.Errorf("IsolateTasks returned incorrect number of isolated files (%d but wanted %d)", len(isolatedFiles), len(isolateTasks))
				}
				for idx, name := range isolateFileNames {
					isolatedFiles[name] = isolatedFilesSlice[idx]
				}
			}
			return nil
		})
		if err != nil && !specs.ErrorIsPermanent(err) {
			return nil, err
		}
		errString := ""
		if err != nil {
			errString = err.Error()
			isolatedFiles = nil
		}
		return &isolate_cache.CachedValue{
			Isolated: isolatedFiles,
			Error:    errString,
		}, nil
	}); err != nil {
		return nil, err
	}

	return cv.Cfg, nil
}
