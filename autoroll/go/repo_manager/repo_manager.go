package repo_manager

import (
	"context"
	"fmt"
	"os"
	"path"
	"regexp"
	"sync"
	"time"

	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

const (
	COMMIT_MSG_FOOTER_TMPL = `
The AutoRoll server is located here: %s

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md

If the roll is causing failures, please contact the current sheriff, who should
be CC'd on the roll, and stop the roller if necessary.

`
	ROLL_BRANCH = "roll_branch"
)

// RepoManager is the interface used by different Autoroller implementations
// to manage checkouts.
type RepoManager interface {
	ChildRevList(context.Context, ...string) ([]string, error)
	CommitsNotRolled() int
	CreateNewRoll(context.Context, string, string, []string, string, bool) (int64, error)
	FullChildHash(context.Context, string) (string, error)
	GetCommitsNotRolled(context.Context, string) (int, error)
	LastRollRev() string
	NextRollRev() string
	PreUploadSteps() []PreUploadStep
	RolledPast(context.Context, string) (bool, error)
	Update(context.Context) error
	User() string
}

// commonRepoManager is a struct used by the AutoRoller implementations for
// managing checkouts.
type commonRepoManager struct {
	childBranch      string
	childDir         string
	childPath        string
	childRepo        *git.Checkout
	commitsNotRolled int
	g                gerrit.GerritInterface
	infoMtx          sync.RWMutex
	lastRollRev      string
	nextRollRev      string
	parentBranch     string
	preUploadSteps   []PreUploadStep
	repoMtx          sync.RWMutex
	serverURL        string
	strategy         NextRollStrategy
	user             string
	workdir          string
}

// ChildRevList returns a slice of commit hashes from the child repo.
func (r *commonRepoManager) ChildRevList(ctx context.Context, args ...string) ([]string, error) {
	r.repoMtx.RLock()
	defer r.repoMtx.RUnlock()
	return r.childRepo.RevList(ctx, args...)
}

// FullChildHash returns the full hash of the given short hash or ref in the
// child repo.
func (r *commonRepoManager) FullChildHash(ctx context.Context, shortHash string) (string, error) {
	r.repoMtx.RLock()
	defer r.repoMtx.RUnlock()
	return r.childRepo.FullHash(ctx, shortHash)
}

// LastRollRev returns the last-rolled child commit.
func (r *commonRepoManager) LastRollRev() string {
	r.infoMtx.RLock()
	defer r.infoMtx.RUnlock()
	return r.lastRollRev
}

// RolledPast determines whether the repo has rolled past the given commit.
func (r *commonRepoManager) RolledPast(ctx context.Context, hash string) (bool, error) {
	r.repoMtx.RLock()
	defer r.repoMtx.RUnlock()
	return r.childRepo.IsAncestor(ctx, hash, r.lastRollRev)
}

// NextRollRev returns the revision of the next roll.
func (r *commonRepoManager) NextRollRev() string {
	r.infoMtx.RLock()
	defer r.infoMtx.RUnlock()
	return r.nextRollRev
}

// PreUploadSteps returns a slice of functions which should be run after the
// roll is performed but before a CL is uploaded for it.
func (r *commonRepoManager) PreUploadSteps() []PreUploadStep {
	return r.preUploadSteps
}

// Start makes the RepoManager begin the periodic update process.
func Start(ctx context.Context, r RepoManager, frequency time.Duration) {
	sklog.Infof("Starting repo_manager")
	lv := metrics2.NewLiveness("last_successful_repo_manager_update")
	cleanup.Repeat(frequency, func() {
		sklog.Infof("Running repo_manager update.")
		if err := r.Update(ctx); err != nil {
			sklog.Errorf("Failed to update repo manager: %s", err)
		} else {
			lv.Reset()
		}
	}, nil)
}

func (r *commonRepoManager) User() string {
	return r.user
}

func (r *commonRepoManager) IsRollSubject(line string) (bool, error) {
	rollSubjectRegex := fmt.Sprintf("^Roll %s [a-zA-Z0-9]+..[a-zA-Z0-9]+ \\([0-9]+ commits\\)$", r.childPath)
	return regexp.MatchString(rollSubjectRegex, line)
}

// CommitsNotRolled returns the number of commits in the child repo which have
// not been rolled into the parent repo.
func (r *commonRepoManager) CommitsNotRolled() int {
	return r.commitsNotRolled
}

// depotToolsRepoManager is a struct used by AutoRoller implementations that use
// depot_tools to manage checkouts.
type depotToolsRepoManager struct {
	*commonRepoManager
	depsCustomVars []string
	depot_tools    string
	gclient        string
	parentDir      string
	parentRepo     string
}

// GetEnvForDepotTools returns the environment used for depot_tools commands.
func (r *depotToolsRepoManager) GetEnvForDepotTools() []string {
	return []string{
		fmt.Sprintf("PATH=%s:%s", r.depot_tools, os.Getenv("PATH")),
		fmt.Sprintf("HOME=%s", os.Getenv("HOME")),
		fmt.Sprintf("SKIP_GCE_AUTH_FOR_GIT=1"),
	}
}

// cleanParent forces the parent checkout into a clean state.
func (r *depotToolsRepoManager) cleanParent(ctx context.Context) error {
	if _, err := exec.RunCwd(ctx, r.parentDir, "git", "clean", "-d", "-f", "-f"); err != nil {
		return err
	}
	_, _ = exec.RunCwd(ctx, r.parentDir, "git", "rebase", "--abort")
	if _, err := exec.RunCwd(ctx, r.parentDir, "git", "checkout", fmt.Sprintf("origin/%s", r.parentBranch), "-f"); err != nil {
		return err
	}
	_, _ = exec.RunCwd(ctx, r.parentDir, "git", "branch", "-D", ROLL_BRANCH)
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  r.workdir,
		Env:  r.GetEnvForDepotTools(),
		Name: r.gclient,
		Args: []string{"revert", "--nohooks"},
	}); err != nil {
		return err
	}
	return nil
}

func (r *depotToolsRepoManager) createAndSyncParent(ctx context.Context) error {
	// Create the working directory if needed.
	if _, err := os.Stat(r.workdir); err != nil {
		if err := os.MkdirAll(r.workdir, 0755); err != nil {
			return err
		}
	}

	if _, err := os.Stat(path.Join(r.parentDir, ".git")); err == nil {
		if err := r.cleanParent(ctx); err != nil {
			return err
		}
		// Update the repo.
		if _, err := exec.RunCwd(ctx, r.parentDir, "git", "fetch"); err != nil {
			return err
		}
		if _, err := exec.RunCwd(ctx, r.parentDir, "git", "reset", "--hard", fmt.Sprintf("origin/%s", r.parentBranch)); err != nil {
			return err
		}
	}

	args := []string{"config", r.parentRepo}
	for _, v := range r.depsCustomVars {
		args = append(args, "--custom-var", v)
	}
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  r.workdir,
		Env:  r.GetEnvForDepotTools(),
		Name: r.gclient,
		Args: args,
	}); err != nil {
		return err
	}
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  r.workdir,
		Env:  r.GetEnvForDepotTools(),
		Name: r.gclient,
		Args: []string{"sync", "--nohooks"},
	}); err != nil {
		return err
	}
	return nil
}

func (r *depotToolsRepoManager) GetCommitsNotRolled(ctx context.Context, lastRollRev string) (int, error) {
	head, err := r.childRepo.FullHash(ctx, fmt.Sprintf("origin/%s", r.childBranch))
	if err != nil {
		return -1, err
	}
	notRolled := 0
	if head != lastRollRev {
		commits, err := r.childRepo.RevList(ctx, fmt.Sprintf("%s..%s", lastRollRev, head))
		if err != nil {
			return -1, err
		}
		notRolled = len(commits)
	}
	return notRolled, nil
}
