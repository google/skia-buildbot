package cacher

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"go.skia.org/infra/go/cas"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
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
}

// New creates a Cacher instance.
func New(s *syncer.Syncer, tcc task_cfg_cache.TaskCfgCache, rbeCas cas.CAS, g gerrit.GerritInterface) *CacherImpl {
	return &CacherImpl{
		rbeCas: rbeCas,
		s:      s,
		tcc:    tcc,
		gerrit: g,
	}
}

// GetOrCacheRepoState returns the cached value(s) for the given RepoState,
// performing the sync to obtain and insert the value(s) into the cache(s) if
// necessary.
func (c *CacherImpl) GetOrCacheRepoState(ctx context.Context, rs types.RepoState) (*specs.TasksCfg, error) {
	ltgr := c.s.LazyTempGitRepo(rs)
	defer ltgr.Done()

	// Obtain the TasksCfg.
	cv, err := c.tcc.SetIfUnset(ctx, rs, func(ctx context.Context) (*task_cfg_cache.CachedValue, error) {
		if rs.IsTryJob() {
			issue, err := strconv.ParseInt(rs.Issue, 10, 64)
			if err != nil {
				return nil, skerr.Wrapf(err, "issue number isn't parseable as integer")
			}
			m, err := c.gerrit.GetMergeable(ctx, issue, rs.Patchset)
			if err != nil {
				return nil, skerr.Wrapf(err, "failed to determine if CL is mergeable")
			}
			if !m.Mergeable {
				return nil, skerr.Fmt("CL is not mergeable (issue %s patchset %s)", rs.Issue, rs.Patchset)
			}
		}

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
		if err != nil && !errorIsPermanent(err) {
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
		return nil, &CachedError{err: skerr.Fmt("%s", cv.Err)}
	}
	return cv.Cfg, nil
}

// Assert that CacherImpl implements Cacher.
var _ Cacher = &CacherImpl{}

// errorIsPermanent returns true if the given error cannot be recovered by
// retrying. In this case, we will never be able to process the TasksCfg,
// so we might as well cancel the jobs.
func errorIsPermanent(err error) bool {
	errMsg := skerr.Unwrap(err).Error()

	any := func(msgs ...string) bool {
		for _, msg := range msgs {
			if strings.Contains(errMsg, msg) {
				return true
			}
		}
		return false
	}

	// These indicate that the error is flaky.
	if any("failed to refresh auth token") {
		return false
	}

	// These indicate that the error is not flaky.
	return any(
		"error: Failed to merge in the changes.",
		"Failed to apply patch",
		"Failed to read tasks cfg: could not parse file:",
		"Invalid TasksCfg",
		"The \"gclient_gn_args_from\" value must be in recursedeps",
		// This repo was moved, so attempts to sync it will always fail.
		"https://skia.googlesource.com/third_party/libjpeg-turbo.git",
		"no such file or directory",
	)
}
