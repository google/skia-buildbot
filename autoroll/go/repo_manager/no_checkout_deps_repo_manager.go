package repo_manager

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"

	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/issues"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

var (
	// Use this function to instantiate a RepoManager. This is able to be
	// overridden for testing.
	NewNoCheckoutDEPSRepoManager func(context.Context, *NoCheckoutDEPSRepoManagerConfig, string, gerrit.GerritInterface, string, string, *http.Client) (RepoManager, error) = newNoCheckoutDEPSRepoManager
)

// NoCheckoutDEPSRepoManagerConfig provides configuration for RepoManagers which
// don't use a local checkout.
type NoCheckoutDEPSRepoManagerConfig struct {
	// Branch of the child repo we want to roll.
	ChildBranch string `json:"childBranch"`
	// Path of the child repo within the parent repo.
	ChildPath string `json:"childPath"`
	// URL of the child repo.
	ChildRepo string `json:"childRepo"` // TODO(borenet): Can we just get this from DEPS?
	// Gerrit project for the parent repo.
	GerritProject string `json:"gerritProject"`
	// If false, roll CLs do not include a git log.
	IncludeLog bool `json:"includeLog"`
	// Branch of the parent repo we want to roll into.
	ParentBranch string `json:"parentBranch"`
	// URL of the parent repo.
	ParentRepo string `json:"parentRepo"`
	// Strategy for determining which commit(s) to roll.
	Strategy string `json:"strategy"`

	// Optional fields.

	// Named steps to run before uploading roll CLs.
	PreUploadSteps []string `json:"preUploadSteps"`
}

func (c *NoCheckoutDEPSRepoManagerConfig) Validate() error {
	if c.ChildBranch == "" {
		return errors.New("ChildBranch is required.")
	}
	if c.ChildPath == "" {
		return errors.New("ChildPath is required.")
	}
	if c.ChildRepo == "" {
		return errors.New("ChildRepo is required.")
	}
	if c.GerritProject == "" {
		return errors.New("GerritProject is required.")
	}
	if c.ParentBranch == "" {
		return errors.New("ParentBranch is required.")
	}
	if c.ParentRepo == "" {
		return errors.New("ParentRepo is required.")
	}
	if c.Strategy != ROLL_STRATEGY_BATCH && c.Strategy != ROLL_STRATEGY_SINGLE {
		return fmt.Errorf("Strategy must be either %q or %q.", ROLL_STRATEGY_BATCH, ROLL_STRATEGY_SINGLE)
	}
	for _, s := range c.PreUploadSteps {
		if _, err := GetPreUploadStep(s); err != nil {
			return err
		}
	}
	return nil
}

type noCheckoutDEPSRepoManager struct {
	childBranch      string
	childPath        string
	childRepo        *gitiles.Repo
	childRepoUrl     string
	commitsNotRolled int
	depotTools       string
	g                gerrit.GerritInterface
	gclient          string
	gerritProject    string
	includeLog       bool
	infoMtx          sync.RWMutex
	lastRollRev      string
	nextRollCommits  []*vcsinfo.LongCommit
	nextRollRev      string
	parentBranch     string
	parentRepo       *gitiles.Repo
	parentRepoUrl    string
	preUploadSteps   []PreUploadStep
	serverURL        string
	strategy         NextRollStrategy
	user             string
	workdir          string
}

// newNoCheckoutDEPSRepoManager returns a RepoManager instance which does not use
// a local checkout.
func newNoCheckoutDEPSRepoManager(ctx context.Context, c *NoCheckoutDEPSRepoManagerConfig, workdir string, g gerrit.GerritInterface, recipeCfgFile, serverURL string, client *http.Client) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(workdir, os.ModePerm); err != nil {
		return nil, err
	}
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

	depotTools, err := depot_tools.GetDepotTools(ctx, workdir, recipeCfgFile)
	if err != nil {
		return nil, err
	}

	strat := StrategyGitilesBatch(client, c.ChildRepo, c.ChildBranch)
	if c.Strategy == ROLL_STRATEGY_SINGLE {
		strat = StrategyGitilesSingle(client, c.ChildRepo, c.ChildBranch)
	}

	return &noCheckoutDEPSRepoManager{
		childBranch:    c.ChildBranch,
		childPath:      c.ChildPath,
		childRepo:      gitiles.NewRepo(c.ChildRepo, client),
		childRepoUrl:   c.ChildRepo,
		depotTools:     depotTools,
		g:              g,
		gerritProject:  c.GerritProject,
		gclient:        path.Join(depotTools, GCLIENT),
		parentBranch:   c.ParentBranch,
		parentRepo:     gitiles.NewRepo(c.ParentRepo, client),
		parentRepoUrl:  c.ParentRepo,
		preUploadSteps: preUploadSteps,
		serverURL:      serverURL,
		strategy:       strat,
		user:           user,
		workdir:        workdir,
	}, nil
}

// See documentation for RepoManager interface.
func (rm *noCheckoutDEPSRepoManager) CommitsNotRolled() int {
	rm.infoMtx.RLock()
	defer rm.infoMtx.RUnlock()
	return rm.commitsNotRolled
}

// See documentation for RepoManager interface.
func (rm *noCheckoutDEPSRepoManager) CreateNewRoll(ctx context.Context, from, to string, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	rm.infoMtx.RLock()
	defer rm.infoMtx.RUnlock()

	// Build the commit message.
	bugs := []string{}
	monorailProject := issues.REPO_PROJECT_MAPPING[rm.parentRepoUrl]
	if monorailProject == "" {
		sklog.Warningf("Found no entry in issues.REPO_PROJECT_MAPPING for %q", rm.parentRepoUrl)
	}
	logStr := ""
	for _, c := range rm.nextRollCommits {
		date := c.Timestamp.Format("2016-01-02")
		author := strings.Split(c.Author, "@")[0]
		logStr += fmt.Sprintf("%s %s %s\n", date, author, c.Subject)

		// Bugs list.
		if monorailProject != "" {
			b := util.BugsFromCommitMsg(c.Body)
			for _, bug := range b[monorailProject] {
				bugs = append(bugs, fmt.Sprintf("%s:%s", monorailProject, bug))
			}
		}
	}

	commitMsg, err := buildCommitMsg(from, to, rm.childPath, cqExtraTrybots, rm.childRepoUrl, rm.serverURL, logStr, bugs, rm.commitsNotRolled, rm.includeLog)
	if err != nil {
		return 0, err
	}

	// Get the current state of the DEPS file.
	wd, err := ioutil.TempDir("", "")
	if err != nil {
		return 0, err
	}
	defer util.RemoveAll(wd)

	// Download the DEPS file from the parent repo.
	depsFile := path.Join(wd, "DEPS")
	if err := getDEPSFile(rm.parentRepo, rm.parentBranch, depsFile); err != nil {
		return 0, err
	}

	// Run "gclient setdep" to set the new revision.
	args := []string{"setdep", "-r", fmt.Sprintf("%s@%s", rm.childPath, to)}
	sklog.Infof("Running command: gclient %s", strings.Join(args, " "))
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  wd,
		Env:  depot_tools.Env(rm.depotTools),
		Name: rm.gclient,
		Args: args,
	}); err != nil {
		return 0, err
	}

	// Read the updated DEPS file.
	b, err := ioutil.ReadFile(depsFile)
	if err != nil {
		return 0, err
	}

	// Create the change.
	ci, err := gerrit.CreateAndEditChange(rm.g, rm.gerritProject, rm.parentBranch, commitMsg, func(g gerrit.GerritInterface, ci *gerrit.ChangeInfo) error {
		return g.EditFile(ci, "DEPS", string(b))
	})
	if err != nil {
		if ci != nil {
			if err2 := rm.g.Abandon(ci, "Failed to create roll CL"); err != nil {
				return 0, fmt.Errorf("Failed to create roll with: %s\nAnd failed to abandon the change with: %s", err, err2)
			}
		}
		return 0, err
	}

	// Set the CQ bit as appropriate.
	if dryRun {
		err = rm.g.SendToDryRun(ci, "")
	} else {
		err = rm.g.SendToCQ(ci, "")
	}
	if err != nil {
		// TODO(borenet): Should we try to abandon the CL?
		return 0, err
	}

	return ci.Issue, nil
}

// See documentation for RepoManager interface.
func (rm *noCheckoutDEPSRepoManager) FullChildHash(ctx context.Context, ref string) (string, error) {
	c, err := rm.childRepo.GetCommit(ref)
	if err != nil {
		return "", err
	}
	return c.Hash, nil
}

// See documentation for RepoManager interface.
func (rm *noCheckoutDEPSRepoManager) LastRollRev() string {
	rm.infoMtx.RLock()
	defer rm.infoMtx.RUnlock()
	return rm.lastRollRev
}

// See documentation for RepoManager interface.
func (rm *noCheckoutDEPSRepoManager) NextRollRev() string {
	rm.infoMtx.RLock()
	defer rm.infoMtx.RUnlock()
	return rm.nextRollRev
}

// See documentation for RepoManager interface.
func (rm *noCheckoutDEPSRepoManager) PreUploadSteps() []PreUploadStep {
	return rm.preUploadSteps
}

// See documentation for RepoManager interface.
func (rm *noCheckoutDEPSRepoManager) RolledPast(ctx context.Context, hash string) (bool, error) {
	rm.infoMtx.RLock()
	defer rm.infoMtx.RUnlock()
	if hash == rm.lastRollRev {
		return true, nil
	}
	commits, err := rm.childRepo.Log(hash, rm.lastRollRev)
	if err != nil {
		return false, err
	}
	return len(commits) > 0, nil
}

// getDEPSFile downloads the DEPS file at the given ref from the given repo.
func getDEPSFile(repo *gitiles.Repo, ref, dest string) error {
	depsFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	if err := repo.ReadFileAtRef("DEPS", ref, depsFile); err != nil {
		return err
	}
	return depsFile.Close()
}

// getLastRollRev returns the commit hash of the last-completed DEPS roll.
func (rm *noCheckoutDEPSRepoManager) getLastRollRev(ctx context.Context, wd string) (string, error) {
	// Download the DEPS file from the parent repo.
	if err := getDEPSFile(rm.parentRepo, rm.parentBranch, path.Join(wd, "DEPS")); err != nil {
		return "", err
	}

	// Use "gclient getdep" to retrieve the last roll revision.
	output, err := exec.RunCwd(ctx, wd, "python", rm.gclient, "getdep", "-r", rm.childPath)
	if err != nil {
		return "", err
	}
	commit := strings.TrimSpace(output)
	if len(commit) != 40 {
		return "", fmt.Errorf("Got invalid output for `gclient getdep`: %s", output)
	}
	return commit, nil
}

// See documentation for RepoManager interface.
func (rm *noCheckoutDEPSRepoManager) Update(ctx context.Context) error {
	wd, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}
	defer util.RemoveAll(wd)

	// Get the last roll revision.
	lastRollRev, err := rm.getLastRollRev(ctx, wd)
	if err != nil {
		return err
	}

	// Find the not-yet-rolled child repo commits.
	notRolled, err := rm.childRepo.Log(lastRollRev, rm.childBranch)
	if err != nil {
		return err
	}
	notRolledCount := len(notRolled)

	// Get the next roll revision.
	nextRollRev, err := rm.strategy.GetNextRollRev(ctx, nil, lastRollRev)
	if err != nil {
		return err
	}
	nextRollCommits := []*vcsinfo.LongCommit{}
	if nextRollRev != lastRollRev {
		nextRollCommits, err = rm.childRepo.Log(lastRollRev, nextRollRev)
		if err != nil {
			return err
		}
	}

	rm.infoMtx.Lock()
	defer rm.infoMtx.Unlock()
	rm.lastRollRev = lastRollRev
	rm.nextRollRev = nextRollRev
	rm.commitsNotRolled = notRolledCount
	rm.nextRollCommits = nextRollCommits
	return nil
}

// See documentation for RepoManager interface.
func (rm *noCheckoutDEPSRepoManager) User() string {
	// No locking required because rm.user is never changed after rm is created.
	return rm.user
}

// See documentation for RepoManager interface.
func (rm *noCheckoutDEPSRepoManager) GetFullHistoryUrl() string {
	// No locking required because rm.g is never changed after rm is created.
	return rm.g.Url(0) + "/q/owner:" + rm.User()
}

// See documentation for RepoManager interface.
func (rm *noCheckoutDEPSRepoManager) GetIssueUrlBase() string {
	// No locking required because rm.g is never changed after rm is created.
	return rm.g.Url(0) + "/c/"
}
