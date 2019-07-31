package repo_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/issues"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	GCLIENT = "gclient.py"
)

var (
	// Use this function to instantiate a RepoManager. This is able to be
	// overridden for testing.
	NewDEPSRepoManager func(context.Context, *DEPSRepoManagerConfig, string, *gerrit.Gerrit, string, string, *http.Client, codereview.CodeReview, bool) (RepoManager, error) = newDEPSRepoManager
)

// issueJson is the structure of "git cl issue --json"
type issueJson struct {
	Issue    int64  `json:"issue"`
	IssueUrl string `json:"issue_url"`
}

// depsRepoManager is a struct used by DEPs AutoRoller for managing checkouts.
type depsRepoManager struct {
	*depotToolsRepoManager
	childRepoUrl string
	includeBugs  bool
	includeLog   bool
}

// DEPSRepoManagerConfig provides configuration for the DEPS RepoManager.
type DEPSRepoManagerConfig struct {
	DepotToolsRepoManagerConfig

	// If false, roll CLs do not link to bugs from the commits in the child
	// repo.
	IncludeBugs bool `json:"includeBugs"`

	// If false, roll CLs do not include a git log.
	IncludeLog bool `json:"includeLog"`
}

// Validate the config.
func (c *DEPSRepoManagerConfig) Validate() error {
	return c.DepotToolsRepoManagerConfig.Validate()
}

// newDEPSRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func newDEPSRepoManager(ctx context.Context, c *DEPSRepoManagerConfig, workdir string, g *gerrit.Gerrit, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	drm, err := newDepotToolsRepoManager(ctx, c.DepotToolsRepoManagerConfig, path.Join(workdir, "repo_manager"), recipeCfgFile, serverURL, g, client, cr, local)
	if err != nil {
		return nil, err
	}
	dr := &depsRepoManager{
		depotToolsRepoManager: drm,
		includeBugs:           c.IncludeBugs,
		includeLog:            c.IncludeLog,
	}

	return dr, nil
}

// See documentation for RepoManager interface.
func (dr *depsRepoManager) Update(ctx context.Context) error {
	// Sync the projects.
	dr.repoMtx.Lock()
	defer dr.repoMtx.Unlock()

	if err := dr.createAndSyncParent(ctx); err != nil {
		return fmt.Errorf("Could not create and sync parent repo: %s", err)
	}

	// Get the last roll revision.
	lastRollRev, err := dr.getLastRollRev(ctx)
	if err != nil {
		return err
	}

	// Find the not-rolled child repo commits.
	notRolledRevs, err := dr.getCommitsNotRolled(ctx, lastRollRev)
	if err != nil {
		return err
	}

	// Get the next roll revision.
	nextRollRev, err := dr.getNextRollRev(ctx, notRolledRevs, lastRollRev)
	if err != nil {
		return err
	}

	dr.infoMtx.Lock()
	defer dr.infoMtx.Unlock()

	if dr.childRepoUrl == "" {
		childRepo, err := exec.RunCwd(ctx, dr.childDir, "git", "remote", "get-url", "origin")
		if err != nil {
			return err
		}
		dr.childRepoUrl = strings.TrimSpace(childRepo)
	}

	dr.lastRollRev = lastRollRev
	dr.nextRollRev = nextRollRev
	dr.notRolledRevs = notRolledRevs
	return nil
}

func (dr *depsRepoManager) getLastRollRev(ctx context.Context) (*revision.Revision, error) {
	output, err := exec.RunCwd(ctx, dr.parentDir, "python", dr.gclient, "getdep", "-r", dr.childPath)
	if err != nil {
		return nil, err
	}
	commit := strings.TrimSpace(output)
	if len(commit) != 40 {
		return nil, fmt.Errorf("Got invalid output for `gclient getdep`: %s", output)
	}
	details, err := dr.childRepo.Details(ctx, commit)
	if err != nil {
		return nil, err
	}
	return revision.FromLongCommit(dr.childRevLinkTmpl, details), nil
}

// See documentation for RepoManager interface.
func (dr *depsRepoManager) CreateNewRoll(ctx context.Context, from, to *revision.Revision, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	dr.repoMtx.Lock()
	defer dr.repoMtx.Unlock()

	// Clean the checkout, get onto a fresh branch.
	if err := dr.cleanParent(ctx); err != nil {
		return 0, err
	}
	if _, err := exec.RunCwd(ctx, dr.parentDir, "git", "checkout", "-b", ROLL_BRANCH, "-t", fmt.Sprintf("origin/%s", dr.parentBranch), "-f"); err != nil {
		return 0, err
	}

	// Defer some more cleanup.
	defer func() {
		util.LogErr(dr.cleanParent(ctx))
	}()

	// Create the roll CL.
	cr := dr.childRepo
	commits, err := cr.RevList(ctx, fmt.Sprintf("%s..%s", from.Id, to.Id))
	if err != nil {
		return 0, fmt.Errorf("Failed to list revisions: %s", err)
	}
	details := make([]*vcsinfo.LongCommit, 0, len(commits))
	for _, c := range commits {
		d, err := cr.Details(ctx, c)
		if err != nil {
			return 0, fmt.Errorf("Failed to get commit details: %s", err)
		}
		details = append(details, d)
	}
	revs := revision.FromLongCommits(dr.childRevLinkTmpl, details)

	if !dr.local {
		if _, err := exec.RunCwd(ctx, dr.parentDir, "git", "config", "user.name", dr.codereview.UserName()); err != nil {
			return 0, err
		}
		if _, err := exec.RunCwd(ctx, dr.parentDir, "git", "config", "user.email", dr.codereview.UserEmail()); err != nil {
			return 0, err
		}
	}

	// Find relevant bugs.
	bugs := []string{}
	if dr.includeBugs {
		monorailProject := issues.REPO_PROJECT_MAPPING[dr.parentRepo]
		if monorailProject == "" {
			sklog.Warningf("Found no entry in issues.REPO_PROJECT_MAPPING for %q", dr.parentRepo)
		} else {
			for _, d := range details {
				b := util.BugsFromCommitMsg(d.Body)
				for _, bug := range b[monorailProject] {
					bugs = append(bugs, fmt.Sprintf("%s:%s", monorailProject, bug))
				}
			}
		}
	}

	// Run "gclient setdep".
	args := []string{"setdep", "-r", fmt.Sprintf("%s@%s", dr.childPath, to.Id)}
	sklog.Infof("Running command: gclient %s", strings.Join(args, " "))
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:        dr.parentDir,
		Env:        dr.depotToolsEnv,
		InheritEnv: true,
		Name:       dr.gclient,
		Args:       args,
	}); err != nil {
		return 0, err
	}

	// Run "gclient sync" to get the DEPS to the correct new revisions.
	sklog.Info("Running command: gclient sync --nohooks")
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:        dr.workdir,
		Env:        dr.depotToolsEnv,
		InheritEnv: true,
		Name:       "python",
		Args:       []string{dr.gclient, "sync", "--nohooks"},
	}); err != nil {
		return 0, err
	}

	// Build the commit message.
	commitMsg, err := dr.buildCommitMsg(&CommitMsgVars{
		Bugs:           bugs,
		ChildPath:      dr.childPath,
		ChildRepo:      dr.childRepoUrl,
		CqExtraTrybots: cqExtraTrybots,
		IncludeLog:     dr.includeLog,
		Reviewers:      emails,
		Revisions:      revs,
		RollingFrom:    from,
		RollingTo:      to,
		ServerURL:      dr.serverURL,
	})
	if err != nil {
		return 0, err
	}

	// Run the pre-upload steps.
	sklog.Infof("Running pre-upload steps.")
	for _, s := range dr.PreUploadSteps() {
		if err := s(ctx, dr.depotToolsEnv, dr.httpClient, dr.parentDir); err != nil {
			return 0, fmt.Errorf("Failed pre-upload step: %s", err)
		}
	}

	// Commit.
	if _, err := exec.RunCwd(ctx, dr.parentDir, "git", "commit", "-a", "-m", commitMsg); err != nil {
		return 0, err
	}

	// Upload the CL.
	sklog.Infof("Running command git %s", strings.Join(args, " "))
	uploadCmd := &exec.Command{
		Dir:        dr.parentDir,
		Env:        dr.depotToolsEnv,
		InheritEnv: true,
		Name:       "git",
		Args:       []string{"cl", "upload", "--bypass-hooks", "-f", "-v", "-v"},
		Timeout:    2 * time.Minute,
	}
	if dryRun {
		uploadCmd.Args = append(uploadCmd.Args, "--cq-dry-run")
	} else {
		uploadCmd.Args = append(uploadCmd.Args, "--use-commit-queue")
	}
	uploadCmd.Args = append(uploadCmd.Args, "--gerrit")
	if emails != nil && len(emails) > 0 {
		emailStr := strings.Join(emails, ",")
		uploadCmd.Args = append(uploadCmd.Args, "--send-mail", "--cc", emailStr)
	}
	uploadCmd.Args = append(uploadCmd.Args, "-m", commitMsg)

	// Upload the CL.
	sklog.Infof("Running command: git %s", strings.Join(uploadCmd.Args, " "))
	if _, err := exec.RunCommand(ctx, uploadCmd); err != nil {
		return 0, err
	}

	// Obtain the issue number.
	sklog.Infof("Retrieving issue number of uploaded CL.")
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return 0, err
	}
	defer util.RemoveAll(tmp)
	jsonFile := path.Join(tmp, "issue.json")
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:        dr.parentDir,
		Env:        dr.depotToolsEnv,
		InheritEnv: true,
		Name:       "git",
		Args:       []string{"cl", "issue", fmt.Sprintf("--json=%s", jsonFile)},
	}); err != nil {
		return 0, err
	}
	f, err := os.Open(jsonFile)
	if err != nil {
		return 0, err
	}
	var issue issueJson
	if err := json.NewDecoder(f).Decode(&issue); err != nil {
		return 0, err
	}
	return issue.Issue, nil
}
