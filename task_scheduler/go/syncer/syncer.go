package syncer

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	DEFAULT_NUM_WORKERS = 10
)

// Syncer is a struct used for syncing code to particular RepoStates.
type Syncer struct {
	depotToolsDir string
	repos         repograph.Map
	queue         chan func(int)
	workdir       string
}

// New returns a Syncer instance.
func New(ctx context.Context, repos repograph.Map, depotToolsDir, workdir string, numWorkers int) *Syncer {
	queue := make(chan func(int))
	s := &Syncer{
		depotToolsDir: depotToolsDir,
		queue:         queue,
		repos:         repos,
		workdir:       workdir,
	}
	for i := 0; i < numWorkers; i++ {
		go func(i int) {
			for f := range queue {
				f(i)
			}
		}(i)
	}
	return s
}

// Close frees up resources used by the Syncer.
func (s *Syncer) Close() error {
	close(s.queue)
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
	rvErr := make(chan error)
	s.queue <- func(workerId int) {
		tmp, err2 := ioutil.TempDir("", "")
		if err2 != nil {
			rvErr <- err2
			return
		}
		defer util.RemoveAll(tmp)
		cacheDir := path.Join(s.workdir, "cache", fmt.Sprintf("%d", workerId))
		gr, err := tempGitRepoGclient(ctx, rs, s.depotToolsDir, cacheDir, tmp)
		if err != nil {
			rvErr <- err
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
func tempGitRepoGclient(ctx context.Context, rs types.RepoState, depotToolsDir, gitCacheDir, tmp string) (*git.TempCheckout, error) {
	defer metrics2.FuncTimer().Stop()

	// Run gclient to obtain a checkout of the repo and its DEPS.
	gclientPath := path.Join(depotToolsDir, "gclient.py")
	projectName := strings.TrimSuffix(path.Base(rs.Repo), ".git")
	spec := fmt.Sprintf("cache_dir = '%s'\nsolutions = [{'deps_file': '.DEPS.git', 'managed': False, 'name': '%s', 'url': '%s'}]", gitCacheDir, projectName, rs.Repo)
	if _, err := exec.RunCwd(ctx, tmp, "python", "-u", gclientPath, "config", fmt.Sprintf("--spec=%s", spec)); err != nil {
		return nil, skerr.Wrapf(err, "Failed 'gclient config'")
	}

	patchRepo := rs.Repo
	patchRepoName := projectName
	if rs.PatchRepo != "" {
		patchRepo = rs.PatchRepo
		patchRepoName = strings.TrimSuffix(path.Base(rs.PatchRepo), ".git")
	}
	cmd := []string{
		"python", "-u", gclientPath, "sync",
		"--revision", fmt.Sprintf("%s@%s", projectName, rs.Revision),
		"--reset", "--force", "--ignore_locks", "--nohooks", "--noprehooks",
		"-v", "-v", "-v",
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
	if err := util.WithReadFile(filepath.Join(os.Getenv("HOME"), ".gitconfig"), func(r io.Reader) error {
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
		Name: cmd[0],
		Args: cmd[1:],
		Dir:  tmp,
		Env: []string{
			"DEPOT_TOOLS_METRICS=0",
			"DEPOT_TOOLS_UPDATE=0",
			fmt.Sprintf("GIT_CACHE_PATH=%s", gitCacheDir),
			fmt.Sprintf("HOME=%s", tmp),
			fmt.Sprintf("INFRA_GIT_WRAPPER_HOME=%s", tmp),
			fmt.Sprintf("PATH=%s:%s", depotToolsDir, os.Getenv("PATH")),
		},
		InheritEnv: true,
	})
	dur := t.Stop()
	if err != nil {
		return nil, err
	}
	if dur > 5*time.Minute {
		sklog.Warningf("'gclient sync' took %s for %v; output: %s", dur, rs, out)
	}

	// gclient points the upstream to a local cache. Point back to the
	// "real" upstream, in case the caller cares about the remote URL. Note
	// that this doesn't change the remote URLs for the DEPS.
	co := &git.TempCheckout{
		GitDir: git.GitDir(path.Join(tmp, projectName)),
	}
	if _, err := co.Git(ctx, "remote", "set-url", git.DefaultRemote, rs.Repo); err != nil {
		return nil, err
	}

	// Self-check.
	head, err := co.RevParse(ctx, "HEAD")
	if err != nil {
		return nil, err
	}
	if head != rs.Revision {
		return nil, fmt.Errorf("TempGitRepo ended up at the wrong revision. Wanted %q but got %q", rs.Revision, head)
	}

	return co, nil
}
