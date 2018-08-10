package repo_manager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"

	"go.skia.org/infra/autoroll/go/strategy"
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
	NewNoCheckoutDEPSRepoManager func(context.Context, *NoCheckoutDEPSRepoManagerConfig, string, gerrit.GerritInterface, string, string, string, *http.Client) (RepoManager, error) = newNoCheckoutDEPSRepoManager
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
	// If false, roll CLs do not link to bugs from the commits in the child
	// repo.
	IncludeBugs bool `json:"includeBugs"`
	// If false, roll CLs do not include a git log.
	IncludeLog bool `json:"includeLog"`
	// Branch of the parent repo we want to roll into.
	ParentBranch string `json:"parentBranch"`
	// URL of the parent repo.
	ParentRepo string `json:"parentRepo"`

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
	for _, s := range c.PreUploadSteps {
		if _, err := GetPreUploadStep(s); err != nil {
			return err
		}
	}
	return nil
}

type noCheckoutDEPSRepoManager struct {
	baseCommit          string
	childBranch         string
	childPath           string
	childRepo           *gitiles.Repo
	childRepoUrl        string
	commitsNotRolled    int
	depotTools          string
	g                   gerrit.GerritInterface
	gclient             string
	gerritProject       string
	includeBugs         bool
	includeLog          bool
	infoMtx             sync.RWMutex
	lastRollRev         string
	nextRollCommits     []*vcsinfo.LongCommit
	nextRollDEPSContent []byte
	nextRollRev         string
	parentBranch        string
	parentRepo          *gitiles.Repo
	parentRepoUrl       string
	preUploadSteps      []PreUploadStep
	serverURL           string
	strategy            strategy.NextRollStrategy
	strategyMtx         sync.RWMutex
	user                string
	workdir             string
}

// newNoCheckoutDEPSRepoManager returns a RepoManager instance which does not use
// a local checkout.
func newNoCheckoutDEPSRepoManager(ctx context.Context, c *NoCheckoutDEPSRepoManagerConfig, workdir string, g gerrit.GerritInterface, recipeCfgFile, serverURL, gitcookiesPath string, client *http.Client) (RepoManager, error) {
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

	rm := &noCheckoutDEPSRepoManager{
		childBranch:    c.ChildBranch,
		childPath:      c.ChildPath,
		childRepo:      gitiles.NewRepo(c.ChildRepo, gitcookiesPath, client),
		childRepoUrl:   c.ChildRepo,
		depotTools:     depotTools,
		g:              g,
		gerritProject:  c.GerritProject,
		gclient:        path.Join(depotTools, GCLIENT),
		includeBugs:    c.IncludeBugs,
		includeLog:     c.IncludeLog,
		parentBranch:   c.ParentBranch,
		parentRepo:     gitiles.NewRepo(c.ParentRepo, gitcookiesPath, client),
		parentRepoUrl:  c.ParentRepo,
		preUploadSteps: preUploadSteps,
		serverURL:      serverURL,
		user:           user,
		workdir:        workdir,
	}
	return rm, nil
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
		date := c.Timestamp.Format("2006-01-02")
		author := c.Author
		authorSplit := strings.Split(c.Author, "(")
		if len(authorSplit) > 1 {
			author = strings.TrimRight(strings.TrimSpace(authorSplit[1]), ")")
		}
		logStr += fmt.Sprintf("%s %s %s\n", date, author, c.Subject)

		// Bugs list.
		if rm.includeBugs && monorailProject != "" {
			b := util.BugsFromCommitMsg(c.Body)
			for _, bug := range b[monorailProject] {
				bugs = append(bugs, fmt.Sprintf("%s:%s", monorailProject, bug))
			}
		}
	}

	commitMsg, err := buildCommitMsg(from, to, rm.childPath, cqExtraTrybots, rm.childRepoUrl, rm.serverURL, logStr, bugs, len(rm.nextRollCommits), rm.includeLog)
	if err != nil {
		return 0, fmt.Errorf("Failed to build commit msg: %s", err)
	}
	commitMsg += "TBR=" + strings.Join(emails, ",")

	// Create the change.
	ci, err := gerrit.CreateAndEditChange(rm.g, rm.gerritProject, rm.parentBranch, commitMsg, rm.baseCommit, func(g gerrit.GerritInterface, ci *gerrit.ChangeInfo) error {
		if err := g.EditFile(ci, "DEPS", string(rm.nextRollDEPSContent)); err != nil {
			return fmt.Errorf("Failed to edit DEPS file: %s", err)
		}
		return nil
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
	cq := gerrit.COMMITQUEUE_LABEL_SUBMIT
	if dryRun {
		cq = gerrit.COMMITQUEUE_LABEL_DRY_RUN
	}
	if err = rm.g.SetReview(ci, "", map[string]interface{}{
		gerrit.CODEREVIEW_LABEL:  gerrit.CODEREVIEW_LABEL_APPROVE,
		gerrit.COMMITQUEUE_LABEL: cq,
	}, emails); err != nil {
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

func (rm *noCheckoutDEPSRepoManager) getNextRollRev(ctx context.Context, notRolled []*vcsinfo.LongCommit, lastRollRev string) (string, error) {
	rm.strategyMtx.RLock()
	defer rm.strategyMtx.RUnlock()
	nextRollRev, err := rm.strategy.GetNextRollRev(ctx, notRolled)
	if err != nil {
		return "", err
	}
	if nextRollRev == "" {
		nextRollRev = lastRollRev
	}
	return nextRollRev, nil
}

// See documentation for RepoManager interface.
func (rm *noCheckoutDEPSRepoManager) Update(ctx context.Context) error {
	wd, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}
	defer util.RemoveAll(wd)

	// Find HEAD of the desired parent branch. We make sure to provide the
	// base commit of our change, to avoid clobbering other changes to the
	// DEPS file.
	baseCommit, err := rm.parentRepo.GetCommit(rm.parentBranch)
	if err != nil {
		return err
	}

	// Download the DEPS file from the parent repo.
	buf := bytes.NewBuffer([]byte{})
	if err := rm.parentRepo.ReadFileAtRef("DEPS", baseCommit.Hash, buf); err != nil {
		return err
	}

	// Use "gclient getdep" to retrieve the last roll revision.
	depsFile := path.Join(wd, "DEPS")
	if err := ioutil.WriteFile(depsFile, buf.Bytes(), os.ModePerm); err != nil {
		return err
	}
	output, err := exec.RunCwd(ctx, wd, "python", rm.gclient, "getdep", "-r", rm.childPath)
	if err != nil {
		return err
	}
	lastRollRev := strings.TrimSpace(output)
	if len(lastRollRev) != 40 {
		return fmt.Errorf("Got invalid output for `gclient getdep`: %s", output)
	}

	// Find the not-yet-rolled child repo commits.
	// Only consider commits on the "main" branch as roll candidates.
	notRolled, err := rm.childRepo.LogLinear(lastRollRev, rm.childBranch)
	if err != nil {
		return err
	}
	notRolledCount := len(notRolled)

	// Get the next roll revision.
	nextRollRev, err := rm.getNextRollRev(ctx, notRolled, lastRollRev)
	if err != nil {
		return err
	}
	nextRollCommits := make([]*vcsinfo.LongCommit, 0, notRolledCount)
	found := false
	if nextRollRev != lastRollRev {
		for _, c := range notRolled {
			if c.Hash == nextRollRev {
				found = true
			}
			if found {
				nextRollCommits = append(nextRollCommits, c)
			}
		}
	}

	// Go ahead and write the new DEPS content, while we have the file on
	// disk.
	args := []string{"setdep", "-r", fmt.Sprintf("%s@%s", rm.childPath, nextRollRev)}
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  wd,
		Env:  depot_tools.Env(rm.depotTools),
		Name: rm.gclient,
		Args: args,
	}); err != nil {
		return err
	}

	// Read the updated DEPS content.
	newDEPSContent, err := ioutil.ReadFile(depsFile)
	if err != nil {
		return err
	}

	rm.infoMtx.Lock()
	defer rm.infoMtx.Unlock()
	rm.baseCommit = baseCommit.Hash
	rm.lastRollRev = lastRollRev
	rm.nextRollRev = nextRollRev
	rm.commitsNotRolled = notRolledCount
	rm.nextRollCommits = nextRollCommits
	rm.nextRollDEPSContent = newDEPSContent
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

// See documentation for RepoManager interface.
func (r *noCheckoutDEPSRepoManager) CreateNextRollStrategy(ctx context.Context, s string) (strategy.NextRollStrategy, error) {
	return strategy.GetNextRollStrategy(ctx, s, r.childBranch, DEFAULT_REMOTE, "", []string{}, nil, nil)
}

// See documentation for RepoManager interface.
func (r *noCheckoutDEPSRepoManager) SetStrategy(s strategy.NextRollStrategy) {
	r.strategyMtx.Lock()
	defer r.strategyMtx.Unlock()
	r.strategy = s
}

// See documentation for RepoManager interface.
func (r *noCheckoutDEPSRepoManager) DefaultStrategy() string {
	return strategy.ROLL_STRATEGY_BATCH
}

// See documentation for RepoManager interface.
func (r *noCheckoutDEPSRepoManager) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
		strategy.ROLL_STRATEGY_SINGLE,
	}
}
