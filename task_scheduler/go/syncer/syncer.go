package syncer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/bazel/external/cipd/cpython3"
	"go.skia.org/infra/bazel/external/cipd/vpython"
	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/types"
)

const (
	DefaultNumWorkers = 10

	syncTimeout       = 15 * time.Minute
	metricSyncing     = "task_scheduler_jc_syncing"
	metricSyncTimeout = "task_scheduler_sync_timeout"
	metricWorkerBusy  = "task_scheduler_jc_worker_busy"

	// This is the key used in context.Value to determine whether
	// "--download-topcs" should not be added to "gclient sync".
	SkipDownloadTopicsKey = "skia_infra_skip_download_topics"
)

// Syncer is a struct used for syncing code to particular RepoStates.
type Syncer struct {
	depotToolsDir string
	repos         repograph.Map
	queue         chan func(int)
	workdir       string
	tmpDir        string
}

// New returns a Syncer instance.
func New(ctx context.Context, repos repograph.Map, depotToolsDir, workdir string, numWorkers int) (*Syncer, error) {
	queue := make(chan func(int))
	s := &Syncer{
		depotToolsDir: depotToolsDir,
		queue:         queue,
		repos:         repos,
		workdir:       workdir,
		tmpDir:        filepath.Join(os.TempDir(), "task-scheduler-syncer"),
	}
	if err := s.cleanupTempDirs(ctx); err != nil {
		return nil, skerr.Wrap(err)
	}
	for i := 0; i < numWorkers; i++ {
		go func(i int) {
			for f := range queue {
				f(i)
			}
		}(i)
	}
	return s, nil
}

// Close frees up resources used by the Syncer.
func (s *Syncer) Close() error {
	close(s.queue)
	return nil
}

// cleanupTempDirs cleans up any pre-existing temporary directories in the
// working directory.
func (s *Syncer) cleanupTempDirs(ctx context.Context) error {
	sklog.Infof("Cleaning up temp dir %s", s.tmpDir)
	if err := os.RemoveAll(s.tmpDir); err != nil {
		return skerr.Wrap(err)
	}
	if err := os.MkdirAll(s.tmpDir, os.ModePerm); err != nil {
		return skerr.Wrap(err)
	}
	sklog.Infof("Finished cleaning up temp dir %s", s.tmpDir)
	return nil
}

// TempGitRepo creates a git repository in a subdirectory of a temporary
// directory, gets it into the given RepoState, and runs the given function
// inside the repo dir. It is safe to write into the parent of the repo dir, as
// that is a temporary directory which will be cleaned up.
//
// This method uses a worker pool; if all workers are busy, it will block until
// one is free.
func (s *Syncer) TempGitRepo(ctx context.Context, rs types.RepoState, fn func(*git.TempCheckout) error) error {
	tags := map[string]string{
		"repo":      rs.Repo,
		"revision":  rs.Revision,
		"issue":     rs.Issue,
		"patchset":  rs.Patchset,
		"patchrepo": rs.PatchRepo,
		"server":    rs.Server,
	}
	m := metrics2.NewTimer(metricSyncing, tags)
	m.Start()
	defer func() {
		m.Stop()
	}()
	rvErr := make(chan error)
	s.queue <- func(workerId int) {
		m := metrics2.GetInt64Metric(metricWorkerBusy, map[string]string{
			"worker": strconv.Itoa(workerId),
		}, tags)
		m.Update(1)
		defer func() {
			m.Update(0)
		}()
		tmp, err2 := os.MkdirTemp(s.tmpDir, "")
		if err2 != nil {
			rvErr <- err2
			return
		}
		defer util.RemoveAll(tmp)
		cacheDir := path.Join(s.workdir, "cache", fmt.Sprintf("%d", workerId))
		gr, err := tempGitRepoGclient(ctx, rs, s.depotToolsDir, cacheDir, tmp)
		if err != nil {
			rvErr <- skerr.Wrap(err)
			return
		}
		defer gr.Delete()
		rvErr <- fn(gr)
	}
	return <-rvErr
}

// LazyTempGitRepo is a struct which performs a TempGitRepo only when requested.
// Intended to be used by multiple users which may or may not need the
// TempCheckout. Guaranteed to only call TempGitRepo once. Callers MUST call
// Done() or one of the Syncer's worker goroutines will become permanently
// stuck.
type LazyTempGitRepo struct {
	rs types.RepoState
	s  *Syncer

	// mtx protects queue.
	mtx   sync.Mutex
	queue chan func(*git.TempCheckout, error)
}

// Do checks out a TempGitRepo and runs the given function. Returns any error
// encountered while performing the checkout or the error returned by the
// passed-in func. The passed-in func is run after all of the checkout work is
// complete; if the func runs, then the checkout is guaranteed to have completed
// successfully. Similarly, if the passed-in func returns no error and Do()
// returns an error, it is because of a failure during checkout. It is safe to
// write into the parent of the repo dir, as that is a temporary directory which
// will be cleaned up.
func (r *LazyTempGitRepo) Do(ctx context.Context, fn func(*git.TempCheckout) error) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	// We'll pass the result of the passed-in func along this channel, which
	// lets us know when it's finished.
	rvErr := make(chan error)

	// If this is the first call to Do(), spin up the goroutine to sync the
	// RepoState and run the funcs.
	if r.queue == nil {
		r.queue = make(chan func(*git.TempCheckout, error))
		go func() {
			// Sync the RepoState and run all of the queued funcs.
			err := r.s.TempGitRepo(ctx, r.rs, func(co *git.TempCheckout) error {
				for fn := range r.queue {
					fn(co, nil)
				}
				return nil
			})

			// The above call to TempGitRepo only returns an error
			// if the checkout itself failed. Therefore, if an error
			// is returned, none of the funcs in r.queue have been
			// consumed. We have to do so now.
			if err != nil {
				// Consume all of the funcs in r.queue, passing
				// the sync error to the func so that it can be
				// passed back to the caller.
				for fn := range r.queue {
					fn(nil, err)
				}
			}
		}()
	}

	// Enqueue a wrapper func which handles the case where we failed to
	// sync the given RepoState and therefore cannot run the passed-in
	// func.
	r.queue <- func(co *git.TempCheckout, err error) {
		if err != nil {
			// We have a sync error. Return it without running the
			// passed-in func.
			rvErr <- err
		} else {
			// Run the passed-in func, returning any error.
			rvErr <- fn(co)
		}
	}
	// Wait for our passed-in func to run (or not, if there was an error).
	return <-rvErr
}

// Done frees up the worker goroutine used by this LazyTempGitRepo. Done must be
// called exactly once per LazyTempGitRepo instance, after all calls to Do().
func (r *LazyTempGitRepo) Done() {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if r.queue != nil {
		close(r.queue)
	}
}

// LazyTempGitRepo returns a LazyTempGitRepo instance. The caller must call
// Done() exactly once on the LazyTempGitRepo, after all calls to Do(), in order
// to free up the worker goroutine.
func (s *Syncer) LazyTempGitRepo(rs types.RepoState) *LazyTempGitRepo {
	return &LazyTempGitRepo{
		rs: rs,
		s:  s,
	}
}

// tempGitRepoGclient creates a git repository in subdirectory of a temporary
// directory, gets it into the given RepoState, and returns a git.TempCheckout.
// If ctx.SkipDownloadTopicsKey is true then gclient sync is not called with
// --download-topics. This check is primarily for tests to avoid the network
// call in gclient that would fail for test repos.
func tempGitRepoGclient(ctx context.Context, rs types.RepoState, depotToolsDir, gitCacheDir, tmp string) (*git.TempCheckout, error) {
	defer metrics2.FuncTimer().Stop()

	// Prepend git binary to PATH.
	gitPath, err := git.Executable(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	paths := strings.Split(os.Getenv("PATH"), ":")
	paths = append([]string{filepath.Dir(gitPath)}, paths...)

	// Prepend depotToolsDir to PATH.
	paths = append([]string{depotToolsDir}, paths...)

	// Run gclient to obtain a checkout of the repo and its DEPS.
	gclientPath := path.Join(depotToolsDir, "gclient.py")
	projectName := strings.TrimSuffix(path.Base(rs.Repo), ".git")
	// gclient requires the use of vpython3 to bring in needed dependencies.
	vpythonBinary := "vpython3"
	if bazel.InBazelTest() {
		var err error
		vpythonBinary, err = vpython.FindVPython3()
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to find vpython3 binary from CIPD")
		}
		pythonBinary, err := cpython3.FindPythonBinary()
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to find python3.8 binary from CIPD")
		}
		pythonBinaryDir := filepath.Dir(pythonBinary)
		paths = append([]string{pythonBinaryDir}, paths...)
	}

	env := []string{
		"DEPOT_TOOLS_METRICS=0",
		"DEPOT_TOOLS_UPDATE=0",
		fmt.Sprintf("GIT_CACHE_PATH=%s", gitCacheDir),
		fmt.Sprintf("HOME=%s", tmp),
		fmt.Sprintf("INFRA_GIT_WRAPPER_HOME=%s", tmp),
		fmt.Sprintf("PATH=%s", strings.Join(paths, ":")),
		// Incase we need to download topics.
		"SKIP_GCE_AUTH_FOR_GIT=1",
	}

	spec := fmt.Sprintf("solutions = [{'deps_file': '.DEPS.git', 'managed': False, 'name': '%s', 'url': '%s'}]", projectName, rs.Repo)
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Name: vpythonBinary,
		Args: []string{"-u", gclientPath, "config", fmt.Sprintf("--spec=%s", spec)},
		Dir:  tmp,
		Env:  env,
	}); err != nil {
		return nil, skerr.Wrapf(err, "Failed 'gclient config'")
	}

	patchRepo := rs.Repo
	patchRepoName := projectName
	if rs.PatchRepo != "" {
		patchRepo = rs.PatchRepo
		patchRepoName = strings.TrimSuffix(path.Base(rs.PatchRepo), ".git")
	}
	cmd := []string{
		vpythonBinary, "-u", gclientPath, "sync",
		"--revision", fmt.Sprintf("%s@%s", projectName, rs.Revision),
		"--reset", "--force", "--ignore_locks", "--nohooks", "--noprehooks",
		"--shallow",
		"-v", "-v", "-v", // Delete this if/when logs are too verbose.
	}
	if ctx.Value(SkipDownloadTopicsKey) == nil {
		cmd = append(cmd, "--download-topics")
	}
	if rs.IsTryJob() {
		gerritRef := fmt.Sprintf("refs/changes/%s/%s/%s", rs.Issue[len(rs.Issue)-2:], rs.Issue, rs.Patchset)
		cmd = append(cmd, "--patch-ref", fmt.Sprintf("%s@%s:%s", patchRepoName, rs.Revision, gerritRef))
	} else {
		cmd = append(cmd, "--revision", fmt.Sprintf("%s@%s", projectName, rs.Revision))
	}

	// Copy the global git config file into the temporary directory.
	// bot_update modifies this file, which causes problems when there are
	// multiple instances running at once.
	gitconfig, err := git.ConfigFilePath(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to find .gitconfig")
	}
	if err := util.WithReadFile(gitconfig, func(r io.Reader) error {
		return util.WithWriteFile(filepath.Join(tmp, ".gitconfig"), func(w io.Writer) error {
			_, err := io.Copy(w, r)
			return err
		})
	}); err != nil {
		if os.IsNotExist(skerr.Unwrap(err)) {
			sklog.Warningf("Failed to copy git config: %s", err)
		} else {
			return nil, skerr.Wrapf(err, "Failed to copy git config")
		}
	}
	t := metrics2.NewTimer("gclient_sync", map[string]string{
		"patchRepo": patchRepo,
	})
	sklog.Infof("Executing: %s", strings.Join(cmd, " "))
	out, err := exec.RunCommand(ctx, &exec.Command{
		Name:       cmd[0],
		Args:       cmd[1:],
		Dir:        tmp,
		Env:        env,
		InheritEnv: true,
		Timeout:    syncTimeout,
	})
	dur := t.Stop()
	if err != nil {
		if strings.Contains(err.Error(), exec.TIMEOUT_ERROR_PREFIX) {
			metrics2.GetInt64Metric(metricSyncTimeout, map[string]string{
				"issue":    rs.Issue,
				"patchset": rs.Patchset,
				"revision": rs.Revision,
				"repo":     rs.Repo,
			}).Update(1)
		}
		return nil, skerr.Wrapf(err, "syncing to %s", cmd[len(cmd)-1])
	}
	if dur > 5*time.Minute {
		sklog.Warningf("'gclient sync' took %s for %v; output: %s", dur, rs, out)
	}

	// gclient points the upstream to a local cache. Point back to the
	// "real" upstream, in case the caller cares about the remote URL. Note
	// that this doesn't change the remote URLs for the DEPS.
	co := &git.TempCheckout{
		Checkout: git.CheckoutDir(filepath.Join(tmp, projectName)),
	}
	if _, err := co.Git(ctx, "remote", "set-url", git.DefaultRemote, rs.Repo); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Self-check.
	head, err := co.RevParse(ctx, "HEAD")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if head != rs.Revision {
		return nil, skerr.Fmt("TempGitRepo ended up at the wrong revision. Wanted %q but got %q", rs.Revision, head)
	}

	return co, nil
}
