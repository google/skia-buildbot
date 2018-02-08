package repo_manager

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
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	GCLIENT  = "gclient.py"
	ROLL_DEP = "roll-dep"

	TMPL_CQ_INCLUDE_TRYBOTS = "CQ_INCLUDE_TRYBOTS=%s"
)

var (
	// Use this function to instantiate a RepoManager. This is able to be
	// overridden for testing.
	NewDEPSRepoManager func(context.Context, string, string, string, string, string, string, *gerrit.Gerrit, NextRollStrategy, []string, bool, string, string) (RepoManager, error) = newDEPSRepoManager
)

// issueJson is the structure of "git cl issue --json"
type issueJson struct {
	Issue    int64  `json:"issue"`
	IssueUrl string `json:"issue_url"`
}

// depsRepoManager is a struct used by DEPs AutoRoller for managing checkouts.
type depsRepoManager struct {
	*depotToolsRepoManager
	includeLog bool
	rollDep    string
}

// newDEPSRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func newDEPSRepoManager(ctx context.Context, workdir, parentRepo, parentBranch, childPath, childBranch, depot_tools string, g *gerrit.Gerrit, strategy NextRollStrategy, preUploadStepNames []string, includeLog bool, gclientSpec, serverURL string) (RepoManager, error) {
	gclient := path.Join(depot_tools, GCLIENT)
	rollDep := path.Join(depot_tools, ROLL_DEP)

	wd := path.Join(workdir, "repo_manager")
	if err := os.MkdirAll(wd, os.ModePerm); err != nil {
		return nil, err
	}
	parentBase := strings.TrimSuffix(path.Base(parentRepo), ".git")
	parentDir := path.Join(wd, parentBase)
	childDir := path.Join(wd, childPath)
	childRepo := &git.Checkout{GitDir: git.GitDir(childDir)}

	user, err := g.GetUserEmail()
	if err != nil {
		return nil, fmt.Errorf("Failed to determine Gerrit user: %s", err)
	}
	sklog.Infof("Repo Manager user: %s", user)

	preUploadSteps, err := GetPreUploadSteps(preUploadStepNames)
	if err != nil {
		return nil, err
	}

	dr := &depsRepoManager{
		depotToolsRepoManager: &depotToolsRepoManager{
			commonRepoManager: &commonRepoManager{
				parentBranch:   parentBranch,
				childDir:       childDir,
				childPath:      childPath,
				childRepo:      childRepo,
				childBranch:    childBranch,
				g:              g,
				preUploadSteps: preUploadSteps,
				serverURL:      serverURL,
				strategy:       strategy,
				user:           user,
				workdir:        wd,
			},
			depot_tools: depot_tools,
			gclient:     gclient,
			gclientSpec: gclientSpec,
			parentDir:   parentDir,
			parentRepo:  parentRepo,
		},
		includeLog: includeLog,
		rollDep:    rollDep,
	}

	// TODO(borenet): This update can be extremely expensive. Consider
	// moving it out of the startup critical path.
	return dr, dr.Update(ctx)
}

// Update syncs code in the relevant repositories.
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

	// Get the next roll revision.
	nextRollRev, err := dr.strategy.GetNextRollRev(ctx, dr.childRepo, lastRollRev)
	if err != nil {
		return err
	}

	// Find the number of not-rolled child repo commits.
	notRolled, err := dr.getCommitsNotRolled(ctx, lastRollRev)
	if err != nil {
		return err
	}

	dr.infoMtx.Lock()
	defer dr.infoMtx.Unlock()
	dr.lastRollRev = lastRollRev
	dr.nextRollRev = nextRollRev
	dr.commitsNotRolled = notRolled
	return nil
}

// getLastRollRev returns the commit hash of the last-completed DEPS roll.
func (dr *depsRepoManager) getLastRollRev(ctx context.Context) (string, error) {
	output, err := exec.RunCwd(ctx, dr.parentDir, "python", dr.gclient, "revinfo")
	if err != nil {
		return "", err
	}
	split := strings.Split(output, "\n")
	for _, s := range split {
		if strings.HasPrefix(s, dr.childPath) {
			subs := strings.Split(s, "@")
			if len(subs) != 2 {
				return "", fmt.Errorf("Failed to parse output of `gclient revinfo` (wrong number of entries for %s):\n\n%s\n", dr.childPath, output)
			}
			return subs[1], nil
		}
	}
	return "", fmt.Errorf("Failed to parse output of `gclient revinfo` (no entry for %s):\n\n%s\n", dr.childPath, output)
}

// CreateNewRoll creates and uploads a new DEPS roll to the given commit.
// Returns the issue number of the uploaded roll.
func (dr *depsRepoManager) CreateNewRoll(ctx context.Context, from, to string, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
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
	commits, err := cr.RevList(ctx, fmt.Sprintf("%s..%s", from, to))
	if err != nil {
		return 0, fmt.Errorf("Failed to list revisions: %s", err)
	}

	if _, err := exec.RunCwd(ctx, dr.parentDir, "git", "config", "user.name", dr.user); err != nil {
		return 0, err
	}
	if _, err := exec.RunCwd(ctx, dr.parentDir, "git", "config", "user.email", dr.user); err != nil {
		return 0, err
	}

	// Find Chromium bugs.
	bugs := []string{}
	for _, c := range commits {
		d, err := cr.Details(ctx, c)
		if err != nil {
			return 0, fmt.Errorf("Failed to obtain commit details: %s", err)
		}
		b := util.BugsFromCommitMsg(d.Body)
		for _, bug := range b[util.PROJECT_CHROMIUM] {
			bugs = append(bugs, bug)
		}
	}

	// Run roll-dep.
	args := []string{dr.childPath, "--roll-to", to}
	if len(bugs) > 0 {
		args = append(args, "--bug", strings.Join(bugs, ","))
	}
	if !dr.includeLog {
		args = append(args, "--no-log")
	}
	sklog.Infof("Running command: roll-dep %s", strings.Join(args, " "))
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  dr.parentDir,
		Env:  dr.GetEnvForDepotTools(),
		Name: dr.rollDep,
		Args: args,
	}); err != nil {
		return 0, err
	}
	// Build the commit message, starting with the message provided by roll-dep.
	commitMsg, err := exec.RunCwd(ctx, dr.parentDir, "git", "log", "-n1", "--format=%B", "HEAD")
	if err != nil {
		return 0, err
	}
	commitMsg += fmt.Sprintf(COMMIT_MSG_FOOTER_TMPL, dr.serverURL)
	if cqExtraTrybots != "" {
		commitMsg += "\n" + fmt.Sprintf(TMPL_CQ_INCLUDE_TRYBOTS, cqExtraTrybots)
	}

	// Run the pre-upload steps.
	for _, s := range dr.PreUploadSteps() {
		if err := s(ctx, dr.parentDir); err != nil {
			return 0, fmt.Errorf("Failed pre-upload step: %s", err)
		}
	}

	// Upload the CL.
	uploadCmd := &exec.Command{
		Dir:     dr.parentDir,
		Env:     dr.GetEnvForDepotTools(),
		Name:    "git",
		Args:    []string{"cl", "upload", "--bypass-hooks", "-f", "-v", "-v"},
		Timeout: 2 * time.Minute,
	}
	if dryRun {
		uploadCmd.Args = append(uploadCmd.Args, "--cq-dry-run")
	} else {
		uploadCmd.Args = append(uploadCmd.Args, "--use-commit-queue")
	}
	uploadCmd.Args = append(uploadCmd.Args, "--gerrit")
	tbr := "\nTBR="
	if emails != nil && len(emails) > 0 {
		emailStr := strings.Join(emails, ",")
		tbr += emailStr
		uploadCmd.Args = append(uploadCmd.Args, "--send-mail", "--cc", emailStr)
	}
	commitMsg += tbr
	uploadCmd.Args = append(uploadCmd.Args, "-m", commitMsg)

	// Upload the CL.
	sklog.Infof("Running command: git %s", strings.Join(uploadCmd.Args, " "))
	if _, err := exec.RunCommand(ctx, uploadCmd); err != nil {
		return 0, err
	}

	// Obtain the issue number.
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return 0, err
	}
	defer util.RemoveAll(tmp)
	jsonFile := path.Join(tmp, "issue.json")
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  dr.parentDir,
		Env:  dr.GetEnvForDepotTools(),
		Name: "git",
		Args: []string{"cl", "issue", fmt.Sprintf("--json=%s", jsonFile)},
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
