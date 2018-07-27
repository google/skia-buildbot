package repo_manager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	COMMIT_MSG_FOOTER_TMPL = `
The AutoRoll server is located here: %s

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md

If the roll is causing failures, please contact the current sheriff, who should
be CC'd on the roll, and stop the roller if necessary.

`

	DEFAULT_REMOTE = "origin"

	ROLL_BRANCH = "roll_branch"
)

// RepoManager is the interface used by different Autoroller implementations
// to manage checkouts.
type RepoManager interface {
	// Return the number of commits which have not yet been rolled.
	CommitsNotRolled() int

	// Create a new roll attempt.
	CreateNewRoll(context.Context, string, string, []string, string, bool) (int64, error)

	// Return the full git commit hash for the given short hash or ref in
	// the child repo.
	FullChildHash(context.Context, string) (string, error)

	// Return the last-rolled child revision.
	LastRollRev() string

	// Return the next child revision to be rolled.
	NextRollRev() string

	// PreUploadSteps returns a slice of functions which should be run after the
	// roll is performed but before a CL is uploaded for it.
	PreUploadSteps() []PreUploadStep

	// Return true iff the roller has rolled up through or past the given
	// commit.
	RolledPast(context.Context, string) (bool, error)

	// Update the RepoManager's view of the world. Depending on
	// implementation, this may sync repos and may take some time.
	Update(context.Context) error

	// Return the name of the user who owns the uploaded rolls.
	User() string

	// GetFullHistoryUrl returns a url that contains all changes uploaded by the
	// user.
	GetFullHistoryUrl() string

	// Return the base URL used for building the URLs of uploaded rolls.
	GetIssueUrlBase() string

	// Create a new NextRollRevStrategy from the given name.
	CreateNextRollStrategy(context.Context, string) (strategy.NextRollStrategy, error)

	// Set the RepoManager's NextRollRevStrategy.
	SetStrategy(strategy.NextRollStrategy)

	// Return the default NextRollStrategy name.
	DefaultStrategy() string

	// Return the list of valid strategy names for this RepoManager.
	ValidStrategies() []string
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

// CommonRepoManagerConfig provides configuration for commonRepoManager.
type CommonRepoManagerConfig struct {
	// Required fields.

	// Branch of the child repo we want to roll.
	ChildBranch string `json:"childBranch"`
	// Path of the child repo within the parent repo.
	ChildPath string `json:"childPath"`
	// Branch of the parent repo we want to roll into.
	ParentBranch string `json:"parentBranch"`

	// Optional fields.

	// ChildSubdir indicates the subdirectory of the workdir in which
	// the childPath should be rooted. In most cases, this should be empty,
	// but if ChildPath is relative to the parent repo dir (eg. when DEPS
	// specifies use_relative_paths), then this is required.
	ChildSubdir string `json:"childSubdir"`
	// Named steps to run before uploading roll CLs.
	PreUploadSteps []string `json:"preUploadSteps"`
}

// Validate the config.
func (c *CommonRepoManagerConfig) Validate() error {
	if c.ChildBranch == "" {
		return errors.New("ChildBranch is required.")
	}
	if c.ChildPath == "" {
		return errors.New("ChildPath is required.")
	}
	if c.ParentBranch == "" {
		return errors.New("ParentBranch is required.")
	}
	for _, s := range c.PreUploadSteps {
		if _, err := GetPreUploadStep(s); err != nil {
			return err
		}
	}
	return nil
}

// commonRepoManager is a struct used by the AutoRoller implementations for
// managing checkouts.
type commonRepoManager struct {
	childBranch      string
	childDir         string
	childPath        string
	childRepo        *git.Checkout
	childSubdir      string
	commitsNotRolled int
	g                gerrit.GerritInterface
	infoMtx          sync.RWMutex
	lastRollRev      string
	nextRollRev      string
	parentBranch     string
	preUploadSteps   []PreUploadStep
	repoMtx          sync.RWMutex
	serverURL        string
	strategy         strategy.NextRollStrategy
	strategyMtx      sync.RWMutex
	user             string
	workdir          string
}

// Returns a commonRepoManager instance.
func newCommonRepoManager(c CommonRepoManagerConfig, workdir, serverURL string, g gerrit.GerritInterface) (*commonRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(workdir, os.ModePerm); err != nil {
		return nil, err
	}
	childDir := path.Join(workdir, c.ChildPath)
	if c.ChildSubdir != "" {
		childDir = path.Join(workdir, c.ChildSubdir, c.ChildPath)
	}
	childRepo := &git.Checkout{GitDir: git.GitDir(childDir)}
	preUploadSteps, err := GetPreUploadSteps(c.PreUploadSteps)
	if err != nil {
		return nil, err
	}
	user := ""
	if g.Initialized() {
		user, err = g.GetUserEmail()
		if err != nil {
			return nil, fmt.Errorf("Failed to determine Gerrit user: %s", err)
		}
	}
	sklog.Infof("Repo Manager user: %s", user)
	return &commonRepoManager{
		childBranch:    c.ChildBranch,
		childDir:       childDir,
		childPath:      c.ChildPath,
		childRepo:      childRepo,
		childSubdir:    c.ChildSubdir,
		g:              g,
		parentBranch:   c.ParentBranch,
		preUploadSteps: preUploadSteps,
		serverURL:      serverURL,
		user:           user,
		workdir:        workdir,
	}, nil
}

// See documentation for RepoManager interface.
func (r *commonRepoManager) FullChildHash(ctx context.Context, shortHash string) (string, error) {
	r.repoMtx.RLock()
	defer r.repoMtx.RUnlock()
	return r.childRepo.FullHash(ctx, shortHash)
}

// See documentation for RepoManager interface.
func (r *commonRepoManager) LastRollRev() string {
	r.infoMtx.RLock()
	defer r.infoMtx.RUnlock()
	return r.lastRollRev
}

// See documentation for RepoManager interface.
func (r *commonRepoManager) RolledPast(ctx context.Context, hash string) (bool, error) {
	r.repoMtx.RLock()
	defer r.repoMtx.RUnlock()
	return r.childRepo.IsAncestor(ctx, hash, r.lastRollRev)
}

// See documentation for RepoManager interface.
func (r *commonRepoManager) NextRollRev() string {
	r.infoMtx.RLock()
	defer r.infoMtx.RUnlock()
	return r.nextRollRev
}

// See documentation for RepoManager interface.
func (r *commonRepoManager) PreUploadSteps() []PreUploadStep {
	return r.preUploadSteps
}

// See documentation for RepoManager interface.
func (r *commonRepoManager) GetFullHistoryUrl() string {
	return r.g.Url(0) + "/q/owner:" + r.User()
}

// See documentation for RepoManager interface.
func (r *commonRepoManager) GetIssueUrlBase() string {
	return r.g.Url(0) + "/c/"
}

// See documentation for RepoManager interface.
func (r *commonRepoManager) User() string {
	return r.user
}

// See documentation for RepoManager interface.
func (r *commonRepoManager) CommitsNotRolled() int {
	return r.commitsNotRolled
}

// See documentation for RepoManger interface.
func (r *commonRepoManager) CreateNextRollStrategy(ctx context.Context, s string) (strategy.NextRollStrategy, error) {
	return strategy.GetNextRollStrategy(ctx, s, r.childBranch, DEFAULT_REMOTE, r.childRepo, nil)
}

// See documentation for RepoManager interface.
func (r *commonRepoManager) SetStrategy(s strategy.NextRollStrategy) {
	r.strategyMtx.Lock()
	defer r.strategyMtx.Unlock()
	r.strategy = s
}

// Set the given strategy on the RepoManager.
func SetStrategy(ctx context.Context, r RepoManager, s string) error {
	valid := r.ValidStrategies()
	if !util.In(s, valid) {
		return fmt.Errorf("Invalid strategy %q; valid: %v", s, valid)
	}
	strat, err := r.CreateNextRollStrategy(ctx, s)
	if err != nil {
		return err
	}
	r.SetStrategy(strat)
	return nil
}

func (r *commonRepoManager) getNextRollRev(ctx context.Context, notRolled []*vcsinfo.LongCommit, lastRollRev string) (string, error) {
	r.strategyMtx.RLock()
	defer r.strategyMtx.RUnlock()
	nextRollRev, err := r.strategy.GetNextRollRev(ctx, notRolled)
	if err != nil {
		return "", err
	}
	if nextRollRev == "" {
		nextRollRev = lastRollRev
	}
	return nextRollRev, nil
}

// See documentation for RepoManager interface.
func (r *commonRepoManager) DefaultStrategy() string {
	return strategy.ROLL_STRATEGY_BATCH
}

// See documentation for RepoManager interface.
func (r *commonRepoManager) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
		strategy.ROLL_STRATEGY_SINGLE,
	}
}

// DepotToolsRepoManagerConfig provides configuration for depotToolsRepoManager.
type DepotToolsRepoManagerConfig struct {
	CommonRepoManagerConfig

	// Required fields.

	// URL of the parent repo.
	ParentRepo string `json:"parentRepo"`

	// Optional fields.

	// Override the default gclient spec with this string.
	GClientSpec string `json:"gclientSpec"`
}

// Validate the config.
func (c *DepotToolsRepoManagerConfig) Validate() error {
	if c.ParentRepo == "" {
		return errors.New("ParentRepo is required.")
	}
	// TODO(borenet): Should we validate c.GClientSpec?
	return c.CommonRepoManagerConfig.Validate()
}

// depotToolsRepoManager is a struct used by AutoRoller implementations that use
// depot_tools to manage checkouts.
type depotToolsRepoManager struct {
	*commonRepoManager
	depotTools  string
	gclient     string
	gclientSpec string
	parentDir   string
	parentRepo  string
}

// Return a depotToolsRepoManager instance.
func newDepotToolsRepoManager(ctx context.Context, c DepotToolsRepoManagerConfig, workdir, recipeCfgFile, serverURL string, g *gerrit.Gerrit) (*depotToolsRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	crm, err := newCommonRepoManager(c.CommonRepoManagerConfig, workdir, serverURL, g)
	if err != nil {
		return nil, err
	}
	depotTools, err := depot_tools.GetDepotTools(ctx, workdir, recipeCfgFile)
	if err != nil {
		return nil, err
	}
	parentBase := strings.TrimSuffix(path.Base(c.ParentRepo), ".git")
	parentDir := path.Join(workdir, parentBase)
	return &depotToolsRepoManager{
		commonRepoManager: crm,
		depotTools:        depotTools,
		gclient:           path.Join(depotTools, GCLIENT),
		gclientSpec:       c.GClientSpec,
		parentDir:         parentDir,
		parentRepo:        c.ParentRepo,
	}, nil
}

// cleanParent forces the parent checkout into a clean state.
func (r *depotToolsRepoManager) cleanParent(ctx context.Context) error {
	return r.cleanParentWithRemote(ctx, "origin")
}

func (r *depotToolsRepoManager) cleanParentWithRemote(ctx context.Context, remote string) error {
	if _, err := exec.RunCwd(ctx, r.parentDir, "git", "clean", "-d", "-f", "-f"); err != nil {
		return err
	}
	_, _ = exec.RunCwd(ctx, r.parentDir, "git", "rebase", "--abort")
	if _, err := exec.RunCwd(ctx, r.parentDir, "git", "checkout", fmt.Sprintf("%s/%s", remote, r.parentBranch), "-f"); err != nil {
		return err
	}
	_, _ = exec.RunCwd(ctx, r.parentDir, "git", "branch", "-D", ROLL_BRANCH)
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  r.workdir,
		Env:  depot_tools.Env(r.depotTools),
		Name: "python",
		Args: []string{r.gclient, "revert", "--nohooks"},
	}); err != nil {
		return err
	}
	return nil
}

func (r *depotToolsRepoManager) createAndSyncParent(ctx context.Context) error {
	return r.createAndSyncParentWithRemote(ctx, "origin")
}

func (r *depotToolsRepoManager) createAndSyncParentWithRemote(ctx context.Context, remote string) error {
	// Create the working directory if needed.
	if _, err := os.Stat(r.workdir); err != nil {
		if err := os.MkdirAll(r.workdir, 0755); err != nil {
			return err
		}
	}

	if _, err := os.Stat(path.Join(r.parentDir, ".git")); err == nil {
		if err := r.cleanParentWithRemote(ctx, remote); err != nil {
			return err
		}
		// Update the repo.
		if _, err := exec.RunCwd(ctx, r.parentDir, "git", "fetch", remote); err != nil {
			return err
		}
		if _, err := exec.RunCwd(ctx, r.parentDir, "git", "reset", "--hard", fmt.Sprintf("%s/%s", remote, r.parentBranch)); err != nil {
			return err
		}
	}

	args := []string{r.gclient, "config"}
	if r.gclientSpec != "" {
		args = append(args, fmt.Sprintf("--spec=%s", r.gclientSpec))
	} else {
		args = append(args, r.parentRepo, "--unmanaged")
	}
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  r.workdir,
		Env:  depot_tools.Env(r.depotTools),
		Name: "python",
		Args: args,
	}); err != nil {
		return err
	}
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  r.workdir,
		Env:  depot_tools.Env(r.depotTools),
		Name: "python",
		Args: []string{r.gclient, "sync", "--nohooks"},
	}); err != nil {
		return err
	}
	return nil
}

func (r *depotToolsRepoManager) getCommitsNotRolled(ctx context.Context, lastRollRev string) ([]*vcsinfo.LongCommit, error) {
	head, err := r.childRepo.FullHash(ctx, fmt.Sprintf("origin/%s", r.childBranch))
	if err != nil {
		return nil, err
	}
	if head == lastRollRev {
		return []*vcsinfo.LongCommit{}, nil
	}
	// Only consider commits on the "main" branch as roll candidates.
	commits, err := r.childRepo.RevList(ctx, "--ancestry-path", "--first-parent", fmt.Sprintf("%s..%s", lastRollRev, head))
	if err != nil {
		return nil, err
	}
	notRolled := make([]*vcsinfo.LongCommit, 0, len(commits))
	for _, c := range commits {
		detail, err := r.childRepo.Details(ctx, c)
		if err != nil {
			return nil, err
		}
		notRolled = append(notRolled, detail)
	}
	return notRolled, nil
}
