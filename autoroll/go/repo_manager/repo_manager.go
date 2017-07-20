package repo_manager

import (
	"context"
	"fmt"
	"os"
	"path"
	"regexp"
	"sync"
	"time"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	ROLL_STRATEGY_BATCH  = "batch"
	ROLL_STRATEGY_SINGLE = "single"
	ROLL_BRANCH          = "roll_branch"
)

// RepoManager is the interface used by different Autoroller implementations
// to manage checkouts.
type RepoManager interface {
	Update() error
	FullChildHash(string) (string, error)
	LastRollRev() string
	NextRollRev() string
	RolledPast(string) (bool, error)
	PreUploadSteps() []PreUploadStep
	CreateNewRoll(string, string, []string, string, bool) (int64, error)
	User() string
	SendToGerritCQ(*gerrit.ChangeInfo, string) error
	SendToGerritDryRun(*gerrit.ChangeInfo, string) error
}

// commonRepoManager is a struct used by the AutoRoller implementations for
// managing checkouts.
type commonRepoManager struct {
	infoMtx        sync.RWMutex
	lastRollRev    string
	nextRollRev    string
	repoMtx        sync.RWMutex
	parentBranch   string
	childDir       string
	childPath      string
	childRepo      *git.Checkout
	childBranch    string
	preUploadSteps []PreUploadStep
	strategy       string
	user           string
	workdir        string
	g              gerrit.GerritInterface
}

// FullChildHash returns the full hash of the given short hash or ref in the
// child repo.
func (r *commonRepoManager) FullChildHash(shortHash string) (string, error) {
	r.repoMtx.RLock()
	defer r.repoMtx.RUnlock()
	return r.childRepo.FullHash(shortHash)
}

// LastRollRev returns the last-rolled child commit.
func (r *commonRepoManager) LastRollRev() string {
	r.infoMtx.RLock()
	defer r.infoMtx.RUnlock()
	return r.lastRollRev
}

// RolledPast determines whether the repo has rolled past the given commit.
func (r *commonRepoManager) RolledPast(hash string) (bool, error) {
	r.repoMtx.RLock()
	defer r.repoMtx.RUnlock()
	return git.GitDir(r.childDir).IsAncestor(hash, r.lastRollRev)
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
func Start(r RepoManager, frequency time.Duration, ctx context.Context) {
	sklog.Infof("Starting repo_manager")
	lv := metrics2.NewLiveness("last-successful-repo-manager-update")
	go util.RepeatCtx(frequency, ctx, func() {
		sklog.Infof("Running repo_manager update.")
		if err := r.Update(); err != nil {
			sklog.Errorf("Failed to update repo manager: %s", err)
		} else {
			lv.Reset()
		}
	})
}

func (r *commonRepoManager) User() string {
	return r.user
}

func (r *commonRepoManager) IsRollSubject(line string) (bool, error) {
	rollSubjectRegex := fmt.Sprintf("^Roll %s [a-zA-Z0-9]+..[a-zA-Z0-9]+ \\([0-9]+ commits\\)$", r.childPath)
	return regexp.MatchString(rollSubjectRegex, line)
}

// depotToolsRepoManager is a struct used by AutoRoller implementations that use
// depot_tools to manage checkouts.
type depotToolsRepoManager struct {
	*commonRepoManager
	depot_tools string
	gclient     string
	parentDir   string
	parentRepo  string
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
func (r *depotToolsRepoManager) cleanParent() error {
	if _, err := exec.RunCwd(r.parentDir, "git", "clean", "-d", "-f", "-f"); err != nil {
		return err
	}
	_, _ = exec.RunCwd(r.parentDir, "git", "rebase", "--abort")
	if _, err := exec.RunCwd(r.parentDir, "git", "checkout", fmt.Sprintf("origin/%s", r.parentBranch), "-f"); err != nil {
		return err
	}
	_, _ = exec.RunCwd(r.parentDir, "git", "branch", "-D", ROLL_BRANCH)
	if _, err := exec.RunCommand(&exec.Command{
		Dir:  r.workdir,
		Env:  r.GetEnvForDepotTools(),
		Name: r.gclient,
		Args: []string{"revert", "--nohooks"},
	}); err != nil {
		return err
	}
	return nil
}

func (r *depotToolsRepoManager) createAndSyncParent() error {
	// Create the working directory if needed.
	if _, err := os.Stat(r.workdir); err != nil {
		if err := os.MkdirAll(r.workdir, 0755); err != nil {
			return err
		}
	}

	if _, err := os.Stat(path.Join(r.parentDir, ".git")); err == nil {
		if err := r.cleanParent(); err != nil {
			return err
		}
		// Update the repo.
		if _, err := exec.RunCwd(r.parentDir, "git", "fetch"); err != nil {
			return err
		}
		if _, err := exec.RunCwd(r.parentDir, "git", "reset", "--hard", fmt.Sprintf("origin/%s", r.parentBranch)); err != nil {
			return err
		}
	}

	if _, err := exec.RunCommand(&exec.Command{
		Dir:  r.workdir,
		Env:  r.GetEnvForDepotTools(),
		Name: r.gclient,
		Args: []string{"config", r.parentRepo},
	}); err != nil {
		return err
	}
	if _, err := exec.RunCommand(&exec.Command{
		Dir:  r.workdir,
		Env:  r.GetEnvForDepotTools(),
		Name: r.gclient,
		Args: []string{"sync", "--nohooks"},
	}); err != nil {
		return err
	}
	return nil
}
