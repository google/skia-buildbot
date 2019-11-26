package repo_manager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/issues"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	DEFAULT_REMOTE = "origin"

	ROLL_BRANCH = "roll_branch"

	gerritRevTmpl = "%s/+/%s"
	githubRevTmpl = "%s/commit/%s"
)

// RepoManager is the interface used by different Autoroller implementations
// to manage checkouts.
type RepoManager interface {
	// Create a new roll attempt.
	CreateNewRoll(context.Context, *revision.Revision, *revision.Revision, []*revision.Revision, []string, string, bool) (int64, error)

	// Update the RepoManager's view of the world. Depending on the
	// implementation, this may sync repos and may take some time. Returns
	// the currently-rolled Revision, the tip-of-tree Revision, and a list
	// of all revisions which have not yet been rolled (ie. those between
	// the current and tip-of-tree, including the latter), in reverse
	// chronological order.
	Update(context.Context) (*revision.Revision, *revision.Revision, []*revision.Revision, error)

	// GetRevision returns a revision.Revision instance from the given
	// revision ID.
	GetRevision(context.Context, string) (*revision.Revision, error)
}

// Start makes the RepoManager begin the periodic update process.
func Start(ctx context.Context, r RepoManager, frequency time.Duration) {
	sklog.Infof("Starting repo_manager")
	lv := metrics2.NewLiveness("last_successful_repo_manager_update")
	cleanup.Repeat(frequency, func(_ context.Context) {
		sklog.Infof("Running repo_manager update.")
		// Explicitly ignore the passed-in context; this allows us to
		// continue updating the RepoManager even if the context is
		// canceled, which helps to prevent errors due to interrupted
		// syncs, etc.
		ctx := context.Background()
		if _, _, _, err := r.Update(ctx); err != nil {
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
	// If false, roll CLs do not link to bugs from the commits in the child
	// repo.
	IncludeBugs bool `json:"includeBugs"`
	// If true, include the "git log" (or other revision details) in the
	// commit message. This should be false for internal -> external rollers
	// to avoid leaking internal commit messages.
	IncludeLog bool `json:"includeLog"`
	// Branch of the parent repo we want to roll into.
	ParentBranch string `json:"parentBranch"`
	// URL of the parent repo.
	ParentRepo string `json:"parentRepo"`

	// Optional fields.

	// ChildRevLinkTmpl is a template used to create links to revisions of
	// the child repo. If not supplied, no links will be created.
	ChildRevLinkTmpl string `json:"childRevLinkTmpl"`
	// CommitMsgTmpl is a template used to build commit messages. See the
	// CommitMsgVars type for more information.
	CommitMsgTmpl string `json:"commitMsgTmpl"`
	// ChildSubdir indicates the subdirectory of the workdir in which
	// the childPath should be rooted. In most cases, this should be empty,
	// but if ChildPath is relative to the parent repo dir (eg. when DEPS
	// specifies use_relative_paths), then this is required.
	ChildSubdir string `json:"childSubdir,omitempty"`
	// Monorail project name associated with the parent repo.
	BugProject string `json:"bugProject,omitempty"`
	// Named steps to run before uploading roll CLs.
	PreUploadSteps []string `json:"preUploadSteps,omitempty"`
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
	if c.ParentRepo == "" {
		return errors.New("ParentRepo is required.")
	}
	if c.IncludeBugs && c.BugProject == "" {
		return errors.New("IncludeBugs is true, but BugProject is empty.")
	}
	if proj := issues.REPO_PROJECT_MAPPING[c.ParentRepo]; proj != "" && c.BugProject != "" && proj != c.BugProject {
		return errors.New("BugProject is non-empty but does not match the entry in issues.REPO_PROJECT_MAPPING.")
	}
	for _, s := range c.PreUploadSteps {
		if _, err := GetPreUploadStep(s); err != nil {
			return err
		}
	}
	return nil
}

// See documentation for RepoManagerConfig interface.
func (r *CommonRepoManagerConfig) DefaultStrategy() string {
	return strategy.ROLL_STRATEGY_BATCH
}

// See documentation for RepoManagerConfig interface.
func (r *CommonRepoManagerConfig) NoCheckout() bool {
	return false
}

// See documentation for RepoManagerConfig interface.
func (r *CommonRepoManagerConfig) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
		strategy.ROLL_STRATEGY_SINGLE,
	}
}

// commonRepoManager is a struct used by the AutoRoller implementations for
// managing checkouts.
type commonRepoManager struct {
	childBranch      string
	childDir         string
	childPath        string
	childRepo        *git.Checkout
	childRevLinkTmpl string
	childSubdir      string
	codereview       codereview.CodeReview
	commitMsgTmpl    *template.Template
	g                gerrit.GerritInterface
	httpClient       *http.Client
	includeBugs      bool
	includeLog       bool
	infoMtx          sync.RWMutex
	local            bool
	bugProject       string
	parentBranch     string
	preUploadSteps   []PreUploadStep
	repoMtx          sync.RWMutex
	serverURL        string
	workdir          string
}

// Returns a commonRepoManager instance.
func newCommonRepoManager(ctx context.Context, c CommonRepoManagerConfig, workdir, serverURL string, g gerrit.GerritInterface, client *http.Client, cr codereview.CodeReview, local bool) (*commonRepoManager, error) {
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

	if _, err := os.Stat(workdir); err == nil {
		if err := deleteGitLockFiles(ctx, workdir); err != nil {
			return nil, err
		}
	}
	preUploadSteps, err := GetPreUploadSteps(c.PreUploadSteps)
	if err != nil {
		return nil, err
	}
	commitMsgTmplStr := TMPL_COMMIT_MSG_DEFAULT
	if c.CommitMsgTmpl != "" {
		commitMsgTmplStr = c.CommitMsgTmpl
	}
	commitMsgTmpl, err := ParseCommitMsgTemplate(commitMsgTmplStr)
	if err != nil {
		return nil, err
	}
	return &commonRepoManager{
		childBranch:      c.ChildBranch,
		childDir:         childDir,
		childPath:        c.ChildPath,
		childRepo:        childRepo,
		childRevLinkTmpl: c.ChildRevLinkTmpl,
		childSubdir:      c.ChildSubdir,
		codereview:       cr,
		commitMsgTmpl:    commitMsgTmpl,
		g:                g,
		httpClient:       client,
		includeBugs:      c.IncludeBugs,
		includeLog:       c.IncludeLog,
		local:            local,
		bugProject:       c.BugProject,
		parentBranch:     c.ParentBranch,
		preUploadSteps:   preUploadSteps,
		serverURL:        serverURL,
		workdir:          workdir,
	}, nil
}

func (r *commonRepoManager) getTipRev(ctx context.Context) (*revision.Revision, error) {
	c, err := r.childRepo.Details(ctx, fmt.Sprintf("origin/%s", r.childBranch))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return revision.FromLongCommit(r.childRevLinkTmpl, c), nil
}

func (r *commonRepoManager) getCommitsNotRolled(ctx context.Context, lastRollRev, tipRev *revision.Revision) ([]*revision.Revision, error) {
	if tipRev.Id == lastRollRev.Id {
		return []*revision.Revision{}, nil
	}
	commits, err := r.childRepo.RevList(ctx, "--first-parent", git.LogFromTo(lastRollRev.Id, tipRev.Id))
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
	return revision.FromLongCommits(r.childRevLinkTmpl, notRolled), nil
}

// See documentation for RepoManager interface.
func (r *commonRepoManager) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	r.repoMtx.RLock()
	defer r.repoMtx.RUnlock()
	details, err := r.childRepo.Details(ctx, id)
	if err != nil {
		return nil, err
	}
	return revision.FromLongCommit(r.childRevLinkTmpl, details), nil
}

// Helper function for unsetting the WIP bit on a Gerrit CL if necessary.
// Either the change or issueNum parameter is required; if change is not
// provided, it will be loaded from Gerrit. unsetWIP checks for a nil
// GerritInterface, so this is safe to call from RepoManagers which don't
// use Gerrit. If we fail to unset the WIP bit, unsetWIP abandons the change.
func (r *commonRepoManager) unsetWIP(ctx context.Context, change *gerrit.ChangeInfo, issueNum int64) error {
	if r.g != nil {
		if change == nil {
			var err error
			change, err = r.g.GetIssueProperties(ctx, issueNum)
			if err != nil {
				return err
			}
		}
		if change.WorkInProgress {
			if err := r.g.SetReadyForReview(ctx, change); err != nil {
				if err2 := r.g.Abandon(ctx, change, "Failed to set ready for review."); err2 != nil {
					return fmt.Errorf("Failed to set ready for review with: %s\nand failed to abandon with: %s", err, err2)
				}
				return fmt.Errorf("Failed to set ready for review: %s", err)
			}
		}
	}
	return nil
}

// buildCommitMsg executes the commit message template using the given
// CommitMsgVars.
func (r *commonRepoManager) buildCommitMsg(vars *CommitMsgVars) (string, error) {
	// Bugs.
	vars.Bugs = nil
	if r.includeBugs {
		// TODO(borenet): Move this to a util.MakeBugLines utility?
		bugMap := map[string]bool{}
		for _, rev := range vars.Revisions {
			for _, bug := range rev.Bugs[r.bugProject] {
				bugMap[bug] = true
			}
		}
		if len(bugMap) > 0 {
			vars.Bugs = make([]string, 0, len(bugMap))
			for bug := range bugMap {
				bugStr := fmt.Sprintf("%s:%s", r.bugProject, bug)
				if r.bugProject == util.BUG_PROJECT_BUGANIZER {
					bugStr = fmt.Sprintf("b/%s", bug)
				}
				vars.Bugs = append(vars.Bugs, bugStr)
			}
			sort.Strings(vars.Bugs)
		}
	}

	// IncludeLog.
	vars.IncludeLog = r.includeLog

	// Tests.
	vars.Tests = nil
	testsMap := map[string]bool{}
	for _, rev := range vars.Revisions {
		for _, test := range rev.Tests {
			testsMap[test] = true
		}
	}
	if len(testsMap) > 0 {
		vars.Tests = make([]string, 0, len(testsMap))
		for test := range testsMap {
			vars.Tests = append(vars.Tests, test)
		}
		sort.Strings(vars.Tests)
	}

	// Create the commit message.
	var buf bytes.Buffer
	if err := r.commitMsgTmpl.Execute(&buf, vars); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// DepotToolsRepoManagerConfig provides configuration for depotToolsRepoManager.
type DepotToolsRepoManagerConfig struct {
	CommonRepoManagerConfig

	// Optional fields.

	// Override the default gclient spec with this string.
	GClientSpec string `json:"gclientSpec,omitempty"`
}

// depotToolsRepoManager is a struct used by AutoRoller implementations that use
// depot_tools to manage checkouts.
type depotToolsRepoManager struct {
	*commonRepoManager
	depotTools    string
	depotToolsEnv []string
	gclient       string
	gclientSpec   string
	parentDir     string
	parentRepo    string
}

// Return a depotToolsRepoManager instance.
func newDepotToolsRepoManager(ctx context.Context, c DepotToolsRepoManagerConfig, workdir, recipeCfgFile, serverURL string, g gerrit.GerritInterface, client *http.Client, cr codereview.CodeReview, local bool) (*depotToolsRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	crm, err := newCommonRepoManager(ctx, c.CommonRepoManagerConfig, workdir, serverURL, g, client, cr, local)
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
		depotToolsEnv:     append(depot_tools.Env(depotTools), "SKIP_GCE_AUTH_FOR_GIT=1"),
		gclient:           path.Join(depotTools, GCLIENT),
		gclientSpec:       c.GClientSpec,
		parentDir:         parentDir,
		parentRepo:        c.ParentRepo,
	}, nil
}

// cleanParent forces the parent checkout into a clean state.
func (r *depotToolsRepoManager) cleanParent(ctx context.Context) error {
	return r.cleanParentWithRemoteAndBranch(ctx, "origin", ROLL_BRANCH, r.parentBranch)
}

func (r *depotToolsRepoManager) cleanParentWithRemoteAndBranch(ctx context.Context, remote, localBranch, remoteBranch string) error {
	if _, err := git.GitDir(r.parentDir).Git(ctx, "clean", "-d", "-f", "-f"); err != nil {
		return err
	}
	_, _ = git.GitDir(r.parentDir).Git(ctx, "rebase", "--abort")
	if _, err := git.GitDir(r.parentDir).Git(ctx, "checkout", fmt.Sprintf("%s/%s", remote, remoteBranch), "-f"); err != nil {
		return err
	}
	_, _ = git.GitDir(r.parentDir).Git(ctx, "branch", "-D", localBranch)
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  r.workdir,
		Env:  r.depotToolsEnv,
		Name: "python",
		Args: []string{r.gclient, "revert", "--nohooks"},
	}); err != nil {
		return err
	}
	return nil
}

func (r *depotToolsRepoManager) createAndSyncParent(ctx context.Context) error {
	return r.createAndSyncParentWithRemoteAndBranch(ctx, "origin", ROLL_BRANCH, r.parentBranch)
}

func (r *depotToolsRepoManager) createAndSyncParentWithRemoteAndBranch(ctx context.Context, remote, localBranch, remoteBranch string) error {
	// Create the working directory if needed.
	if _, err := os.Stat(r.workdir); err != nil {
		if err := os.MkdirAll(r.workdir, 0755); err != nil {
			return err
		}
	}

	// Run "gclient config".
	args := []string{r.gclient, "config"}
	if r.gclientSpec != "" {
		args = append(args, fmt.Sprintf("--spec=%s", r.gclientSpec))
	} else {
		args = append(args, r.parentRepo, "--unmanaged")
	}
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  r.workdir,
		Env:  r.depotToolsEnv,
		Name: "python",
		Args: args,
	}); err != nil {
		return err
	}

	// Clean/reset the parent and child checkouts.
	if _, err := os.Stat(path.Join(r.parentDir, ".git")); err == nil {
		if err := r.cleanParentWithRemoteAndBranch(ctx, remote, localBranch, remoteBranch); err != nil {
			return err
		}
		// Update the repo.
		if _, err := git.GitDir(r.parentDir).Git(ctx, "fetch", remote); err != nil {
			return err
		}
		if _, err := git.GitDir(r.parentDir).Git(ctx, "reset", "--hard", fmt.Sprintf("%s/%s", remote, remoteBranch)); err != nil {
			return err
		}
	}
	if _, err := os.Stat(path.Join(r.childDir, ".git")); err == nil {
		if _, err := r.childRepo.Git(ctx, "fetch"); err != nil {
			return err
		}
	}

	// Run "gclient sync".
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  r.workdir,
		Env:  r.depotToolsEnv,
		Name: "python",
		Args: []string{r.gclient, "sync", "--nohooks"},
	}); err != nil {
		return err
	}
	return nil
}

// deleteGitLockFiles finds and deletes Git lock files within the given workdir.
func deleteGitLockFiles(ctx context.Context, workdir string) error {
	sklog.Infof("Looking for git lockfiles in %s", workdir)
	output, err := exec.RunCwd(ctx, workdir, "find", ".", "-name", "index.lock")
	if err != nil {
		return err
	}
	output = strings.TrimSpace(output)
	if output == "" {
		sklog.Info("No lockfiles found.")
		return nil
	}
	lockfiles := strings.Split(output, "\n")
	for _, f := range lockfiles {
		fp := filepath.Join(workdir, f)
		sklog.Warningf("Removing git lockfile: %s", fp)
		if err := os.Remove(fp); err != nil {
			return err
		}
	}
	return nil
}
