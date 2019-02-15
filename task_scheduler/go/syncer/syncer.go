package syncer

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/metrics2"
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
	isolate       *isolate.Client
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

// Close frees up resources used by the TaskCfgCache.
func (s *Syncer) Close() error {
	close(s.queue)
	return nil
}

// TempGitRepo creates a git repository in a temporary directory, gets it into
// the given RepoState, and runs the given function inside the repo dir.
//
// This method uses a worker pool; if all workers are busy, it will block until
// one is free.
func (s *Syncer) TempGitRepo(ctx context.Context, rs types.RepoState, botUpdate bool, fn func(*git.TempCheckout) error) error {
	rvErr := make(chan error)
	s.queue <- func(workerId int) {
		var gr *git.TempCheckout
		var err error
		if botUpdate {
			tmp, err2 := ioutil.TempDir("", "")
			if err2 != nil {
				rvErr <- err2
				return
			}
			defer util.RemoveAll(tmp)
			cacheDir := path.Join(s.workdir, "cache", fmt.Sprintf("%d", workerId))
			gr, err = tempGitRepoBotUpdate(ctx, rs, s.depotToolsDir, cacheDir, tmp)
		} else {
			repo, ok := s.repos[rs.Repo]
			if !ok {
				rvErr <- fmt.Errorf("Unknown repo: %s", rs.Repo)
				return
			}
			gr, err = tempGitRepo(ctx, repo.Repo(), rs)
		}
		if err != nil {
			rvErr <- err
			return
		}
		defer gr.Delete()
		rvErr <- fn(gr)
	}
	return <-rvErr
}

// tempGitRepo creates a git repository in a temporary directory, gets it into
// the given RepoState, and returns its location.
func tempGitRepo(ctx context.Context, repo *git.Repo, rs types.RepoState) (rv *git.TempCheckout, rvErr error) {
	defer metrics2.FuncTimer().Stop()

	if rs.IsTryJob() {
		return nil, fmt.Errorf("tempGitRepo does not apply patches, and should not be called for try jobs.")
	}

	c, err := repo.TempCheckout(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		if rvErr != nil {
			c.Delete()
		}
	}()

	// Check out the correct commit.
	if _, err := c.Git(ctx, "checkout", rs.Revision); err != nil {
		return nil, err
	}

	return c, nil
}

// tempGitRepoBotUpdate creates a git repository in a temporary directory, gets it into
// the given RepoState, and returns its location.
func tempGitRepoBotUpdate(ctx context.Context, rs types.RepoState, depotToolsDir, gitCacheDir, tmp string) (*git.TempCheckout, error) {
	defer metrics2.FuncTimer().Stop()

	// Run bot_update to obtain a checkout of the repo and its DEPS.
	botUpdatePath := path.Join(depotToolsDir, "recipes", "recipe_modules", "bot_update", "resources", "bot_update.py")
	projectName := strings.TrimSuffix(path.Base(rs.Repo), ".git")
	spec := fmt.Sprintf("cache_dir = '%s'\nsolutions = [{'deps_file': '.DEPS.git', 'managed': False, 'name': '%s', 'url': '%s'}]", gitCacheDir, projectName, rs.Repo)
	revMap := map[string]string{
		projectName: "got_revision",
	}

	revisionMappingFile := path.Join(tmp, "revision_mapping")
	revMapBytes, err := json.Marshal(revMap)
	if err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(revisionMappingFile, revMapBytes, os.ModePerm); err != nil {
		return nil, err
	}

	patchRepo := rs.Repo
	patchRepoName := projectName
	if rs.PatchRepo != "" {
		patchRepo = rs.PatchRepo
		patchRepoName = strings.TrimSuffix(path.Base(rs.PatchRepo), ".git")
	}
	outputJson := path.Join(tmp, "output_json")
	cmd := []string{
		"python", "-u", botUpdatePath,
		"--specs", spec,
		"--patch_root", patchRepoName,
		"--revision_mapping_file", revisionMappingFile,
		"--git-cache-dir", gitCacheDir,
		"--output_json", outputJson,
		"--revision", fmt.Sprintf("%s@%s", projectName, rs.Revision),
	}
	if rs.IsTryJob() {
		if strings.Contains(rs.Server, "codereview.chromium") {
			cmd = append(cmd, []string{
				"--issue", rs.Issue,
				"--patchset", rs.Patchset,
			}...)
		} else {
			gerritRef := fmt.Sprintf("refs/changes/%s/%s/%s", rs.Issue[len(rs.Issue)-2:], rs.Issue, rs.Patchset)
			cmd = append(cmd, []string{
				"--patch_ref", fmt.Sprintf("%s@%s", patchRepo, gerritRef),
			}...)
		}
	}
	t := metrics2.NewTimer("bot_update", map[string]string{
		"patchRepo": patchRepo,
	})
	out, err := exec.RunCommand(ctx, &exec.Command{
		Name: cmd[0],
		Args: cmd[1:],
		Dir:  tmp,
		Env: []string{
			fmt.Sprintf("PATH=%s:%s", depotToolsDir, os.Getenv("PATH")),
		},
		InheritEnv: true,
	})
	dur := t.Stop()
	if err != nil {
		sklog.Warningf("bot_update error for %v; output: %s", rs, out)
		return nil, err
	}
	if dur > 5*time.Minute {
		sklog.Warningf("bot_update took %s for %v; output: %s", dur, rs, out)
	}

	// bot_update points the upstream to a local cache. Point back to the
	// "real" upstream, in case the caller cares about the remote URL. Note
	// that this doesn't change the remote URLs for the DEPS.
	co := &git.TempCheckout{
		GitDir: git.GitDir(path.Join(tmp, projectName)),
	}
	if _, err := co.Git(ctx, "remote", "set-url", "origin", rs.Repo); err != nil {
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
