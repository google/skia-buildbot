package cacher

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/isolate_cache"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/syncer"
	"go.skia.org/infra/task_scheduler/go/types"
)

type Cacher struct {
	isolate      *isolate.Client
	isolateCache *isolate_cache.Cache
	s            *syncer.Syncer
	tcc          *specs.TaskCfgCache
}

func New(s *syncer.Syncer, tcc *specs.TaskCfgCache, isolateClient *isolate.Client, isolateCache *isolate_cache.Cache) *Cacher {
	return &Cacher{
		isolate:      isolateClient,
		isolateCache: isolateCache,
		s:            s,
		tcc:          tcc,
	}
}

func (c *Cacher) GetOrCacheRepoState(ctx context.Context, rs types.RepoState) (*specs.TasksCfg, error) {
	ltgr := c.s.LazyTempGitRepo(rs, true)
	defer ltgr.Done()

	sklog.Infof("GetOrCacheRepoState: %s", rs.Revision)

	// Obtain the TasksCfg.
	cv, err := c.tcc.SetIfUnset(ctx, rs, func(ctx context.Context) (*specs.CachedValue, error) {
		var tasksCfg *specs.TasksCfg
		err := ltgr.Do(ctx, func(co *git.TempCheckout) error {
			sklog.Infof("  Reading TasksCfg")
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
		return &specs.CachedValue{
			RepoState: rs,
			Cfg:       tasksCfg,
			Err:       err,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	// Obtain the IsolatedFiles.
	if err := c.isolateCache.SetIfUnset(ctx, rs, func(ctx context.Context) (*isolate_cache.CachedValue, error) {
		isolatedFiles := map[string]*isolate.IsolatedFile{}
		err := ltgr.Do(ctx, func(co *git.TempCheckout) error {
			sklog.Infof("  Isolating tasks.")
			// Isolates may need a .gclient file at the root of the checkout.
			if err := ioutil.WriteFile(path.Join(co.Dir(), "..", ".gclient"), []byte("dummy"), os.ModePerm); err != nil {
				return err
			}

			// Isolate all of the task specs and update their IsolateHashes.
			// TODO(borenet): Batcharchive should do a decent job, but it'd
			// be better to de-duplicate the isolate.Tasks here to save some
			// work.
			isolateFileNames := []string{}
			done := map[string]bool{}
			isolateTasks := []*isolate.Task{}
			for _, taskSpec := range cv.Cfg.Tasks {
				if done[taskSpec.Isolate] {
					continue
				}
				t := &isolate.Task{
					BaseDir:     co.Dir(),
					Blacklist:   isolate.DEFAULT_BLACKLIST,
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
		sklog.Infof("  Isolate got %d files; err: %s", len(isolatedFiles), err)
		if err != nil && !specs.ErrorIsPermanent(err) {
			return nil, err
		}
		errString := ""
		if err != nil {
			errString = err.Error()
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
