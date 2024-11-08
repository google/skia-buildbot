package cacher

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"

	patch_finder "go.skia.org/infra/bazel/external/cipd/patch"
	"go.skia.org/infra/go/cas"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/syncer"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	"go.skia.org/infra/task_scheduler/go/types"
)

// CachedError
type CachedError struct {
	err error
}

// Error implements error.
func (e *CachedError) Error() string {
	return e.err.Error()
}

// IsCachedError returns true if the given error is a CachedError.
func IsCachedError(err error) bool {
	_, ok := err.(*CachedError)
	return ok
}

type Cacher interface {
	// GetOrCacheRepoState returns the cached value(s) for the given RepoState,
	// performing the sync to obtain and insert the value(s) into the cache(s) if
	// necessary.
	GetOrCacheRepoState(ctx context.Context, rs types.RepoState) (*specs.TasksCfg, error)
}

// CacherImpl is a struct which handles insertion of data for RepoStates into
// various caches used by Task Scheduler. It ensures that we only sync to a
// given RepoState once (barring transient errors).
type CacherImpl struct {
	rbeCas cas.CAS
	s      *syncer.Syncer
	tcc    task_cfg_cache.TaskCfgCache
	gerrit gerrit.GerritInterface
	repos  map[string]gitiles.GitilesRepo
}

// New creates a Cacher instance.
func New(s *syncer.Syncer, tcc task_cfg_cache.TaskCfgCache, rbeCas cas.CAS, repos map[string]gitiles.GitilesRepo, gerrit gerrit.GerritInterface) *CacherImpl {
	return &CacherImpl{
		rbeCas: rbeCas,
		s:      s,
		tcc:    tcc,
		gerrit: gerrit,
		repos:  repos,
	}
}

// getTasksCfgAtRepoState retrieves the tasks.json content at the given
// RepoState, applying the diff from any CL onto the specified commit if
// necessary.
func (c *CacherImpl) getTasksCfgAtRepoState(ctx context.Context, rs types.RepoState) (*specs.TasksCfg, error) {
	repo, ok := c.repos[rs.Repo]
	if !ok {
		return nil, skerr.Fmt("no gitiles repo instance for %s", rs.Repo)
	}

	// Retrieve tasks.json at the requested commit.
	tasksJSONAtTip, err := repo.ReadFileAtRef(ctx, specs.TASKS_CFG_FILE, rs.Revision)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to retrieve tasks.json from %s at %s", rs.Repo, rs.Revision)
	}
	if !rs.IsTryJob() {
		return specs.ParseTasksCfg(string(tasksJSONAtTip))
	}

	// Retrieve the tasks.json diff from the CL, if any.
	// Note: we could just request the diff directly and assume that any error
	// response indicates that tasks.json wasn't changed in this CL, but this
	// approach avoids potential issues where unrelated errors occur at the cost
	// of an additional request.
	issue, err := strconv.ParseInt(rs.Issue, 10, 64)
	if err != nil {
		return nil, skerr.Wrapf(err, "issue ID %q is invalid", rs.Issue)
	}
	files, err := c.gerrit.GetFileNames(ctx, issue, rs.Patchset)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to retrieve changed files for %s at patchset %s", c.gerrit.Url(issue), rs.Patchset)
	}
	tasksJSONChanged := false
	for _, file := range files {
		if file == specs.TASKS_CFG_FILE {
			tasksJSONChanged = true
			break
		}
	}
	if !tasksJSONChanged {
		sklog.Infof("tasks.json not changed; returning checked-in content")
		return specs.ParseTasksCfg(string(tasksJSONAtTip))
	}

	diff, err := c.gerrit.GetPatch(ctx, issue, rs.Patchset, specs.TASKS_CFG_FILE)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to retrieve tasks.json diff for %s at patchset %s", c.gerrit.Url(issue), rs.Patchset)
	}

	patchedTasksJSON, err := patchFile(ctx, specs.TASKS_CFG_FILE, tasksJSONAtTip, []byte(diff))
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to patch tasks.json")
	}

	return specs.ParseTasksCfg(string(patchedTasksJSON))
}

// patchFile applies the given patch to the given file contents by writing the
// old file contents to a temporary directory and shelling out to `patch` to
// apply the patch. We do this because we didn't have much success with third-
// party Go libraries and didn't want to reinvent the wheel and write our own
// implementation.
func patchFile(ctx context.Context, path string, oldContent []byte, patch []byte) ([]byte, error) {
	patchBinary, err := patch_finder.FindPatch()
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer util.RemoveAll(tmp)

	dir, _ := filepath.Split(path)
	if dir != "" {
		if err := os.MkdirAll(filepath.Join(tmp, dir), os.ModePerm); err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	fp := filepath.Join(tmp, path)
	if err := os.WriteFile(fp, oldContent, os.ModePerm); err != nil {
		return nil, skerr.Wrap(err)
	}

	cmd := &exec.Command{
		Name:  patchBinary,
		Args:  []string{"-p", "1"},
		Dir:   tmp,
		Stdin: bytes.NewReader(patch),
	}
	if _, err := exec.RunCommand(ctx, cmd); err != nil {
		return nil, skerr.Wrap(err)
	}
	return os.ReadFile(fp)
}

// GetOrCacheRepoState returns the cached value(s) for the given RepoState,
// performing the sync to obtain and insert the value(s) into the cache(s) if
// necessary.
func (c *CacherImpl) GetOrCacheRepoState(ctx context.Context, rs types.RepoState) (*specs.TasksCfg, error) {
	ltgr := c.s.LazyTempGitRepo(rs)
	defer ltgr.Done()

	// Obtain the TasksCfg.
	cv, err := c.tcc.SetIfUnset(ctx, rs, func(ctx context.Context) (*task_cfg_cache.CachedValue, error) {
		// Retrieve the TasksCfg.
		cfg, err := c.getTasksCfgAtRepoState(ctx, rs)
		if err != nil {
			if !specs.ErrorIsPermanent(err) {
				return nil, skerr.Wrap(err)
			}
			return &task_cfg_cache.CachedValue{
				RepoState: rs,
				Cfg:       nil,
				Err:       err.Error(),
			}, nil
		}

		// We only need to sync if there are CasSpecs that need to be uploaded
		// and resolved to digests.
		needSync := false
		for name, casSpec := range cfg.CasSpecs {
			if casSpec.Digest == "" {
				sklog.Infof("cas spec %s has no digest; need sync", name)
				needSync = true
				break
			}
		}

		if needSync {
			err := ltgr.Do(ctx, func(co *git.TempCheckout) error {
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
				return nil
			})
			if err != nil {
				if !specs.ErrorIsPermanent(err) {
					return nil, skerr.Wrap(err)
				}
				return &task_cfg_cache.CachedValue{
					RepoState: rs,
					Cfg:       cfg,
					Err:       err.Error(),
				}, nil
			}
		}

		// Return the cache entry.
		return &task_cfg_cache.CachedValue{
			RepoState: rs,
			Cfg:       cfg,
			Err:       "",
		}, nil
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if cv.Err != "" {
		return nil, &CachedError{err: skerr.Fmt(cv.Err)}
	}
	return cv.Cfg, nil
}

// Assert that CacherImpl implements Cacher.
var _ Cacher = &CacherImpl{}
