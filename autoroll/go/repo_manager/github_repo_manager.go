package repo_manager

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	GITHUB_UPSTREAM_REMOTE_NAME = "remote"
)

var (
	// Use this function to instantiate a NewGithubRepoManager. This is able to be
	// overridden for testing.
	NewGithubRepoManager func(context.Context, *GithubRepoManagerConfig, string, *github.GitHub, string, string) (RepoManager, error) = newGithubRepoManager
)

// GithubRepoManagerConfig provides configuration for the Github RepoManager.
type GithubRepoManagerConfig struct {
	DepotToolsRepoManagerConfig
	GithubParentPath string `json:"githubParentPath"`
}

// Validate the config.
func (c *GithubRepoManagerConfig) Validate() error {
	return c.DepotToolsRepoManagerConfig.Validate()
}

// githubRepoManager is a struct used by the autoroller for managing checkouts.
type githubRepoManager struct {
	*depsRepoManager
	githubClient *github.GitHub
}

// newGithubRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func newGithubRepoManager(ctx context.Context, c *GithubRepoManagerConfig, workdir string, githubClient *github.GitHub, recipeCfgFile, serverURL string) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	wd := path.Join(workdir, strings.TrimSuffix(path.Base(c.DepotToolsRepoManagerConfig.ParentRepo), ".git"))
	drm, err := newDepotToolsRepoManager(ctx, c.DepotToolsRepoManagerConfig, wd, recipeCfgFile, serverURL, nil)
	if err != nil {
		return nil, err
	}
	dr := &depsRepoManager{
		depotToolsRepoManager: drm,
		rollDep:               path.Join(drm.depotTools, ROLL_DEP),
	}
	if c.GithubParentPath != "" {
		dr.parentDir = path.Join(wd, c.GithubParentPath)
	}
	user, err := githubClient.GetAuthenticatedUser()
	if err != nil {
		return nil, err
	}
	dr.user = *user.Login
	gr := &githubRepoManager{
		depsRepoManager: dr,
		githubClient:    githubClient,
	}

	// TODO(borenet): This update can be extremely expensive. Consider
	// moving it out of the startup critical path.
	return gr, gr.Update(ctx)
}

// Update syncs code in the relevant repositories.
func (gr *githubRepoManager) Update(ctx context.Context) error {
	// Sync the projects.
	gr.repoMtx.Lock()
	defer gr.repoMtx.Unlock()

	sklog.Info("Updating github repository")

	// If parentDir does not exist yet then create the directory structure.
	if _, err := os.Stat(gr.parentDir); err != nil {
		if os.IsNotExist(err) {
			if err := gr.createAndSyncParent(ctx); err != nil {
				return fmt.Errorf("Could not create and sync %s: %s", gr.parentDir, err)
			}
			// Run gclient hooks to bring in any required binaries.
			if _, err := exec.RunCwd(ctx, gr.parentDir, filepath.Join(gr.depotTools, "gclient"), "runhooks"); err != nil {
				return fmt.Errorf("Error when running gclient runhooks on %s: %s", gr.parentDir, err)
			}
		} else {
			return fmt.Errorf("Error when running os.Stat on %s: %s", gr.parentDir, err)
		}
	}

	// Check to see whether there is an upstream yet.
	remoteOutput, err := exec.RunCwd(ctx, gr.parentDir, "git", "remote", "show")
	if err != nil {
		return err
	}
	if !strings.Contains(remoteOutput, GITHUB_UPSTREAM_REMOTE_NAME) {
		if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "remote", "add", GITHUB_UPSTREAM_REMOTE_NAME, gr.parentRepo); err != nil {
			return err
		}
	}
	// Fetch upstream.
	if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "fetch", GITHUB_UPSTREAM_REMOTE_NAME, gr.parentBranch); err != nil {
		return err
	}

	// gclient sync to get latest version of child repo to find the next roll
	// rev from.
	if err := gr.createAndSyncParentWithRemote(ctx, GITHUB_UPSTREAM_REMOTE_NAME); err != nil {
		return fmt.Errorf("Could not create and sync parent repo: %s", err)
	}

	// Get the last roll revision.
	lastRollRev, err := gr.getLastRollRev(ctx)
	if err != nil {
		return err
	}

	// Get the next roll revision.
	nextRollRev, err := gr.strategy.GetNextRollRev(ctx, gr.childRepo, lastRollRev)
	if err != nil {
		return err
	}

	// Find the number of not-rolled child repo commits.
	notRolled, err := gr.getCommitsNotRolled(ctx, lastRollRev)
	if err != nil {
		return err
	}

	gr.infoMtx.Lock()
	defer gr.infoMtx.Unlock()
	gr.lastRollRev = lastRollRev
	gr.nextRollRev = nextRollRev
	gr.commitsNotRolled = notRolled

	sklog.Infof("lastRollRev is: %s", gr.lastRollRev)
	sklog.Infof("nextRollRev is: %s", nextRollRev)
	sklog.Infof("commitsNotRolled: %d", gr.commitsNotRolled)
	return nil
}

// cleanParent forces the parent checkout into a clean state.
func (gr *githubRepoManager) cleanParent(ctx context.Context) error {
	// Clean the parent
	if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "clean", "-d", "-f", "-f"); err != nil {
		return err
	}
	_, _ = exec.RunCwd(ctx, gr.parentDir, "git", "rebase", "--abort")
	return nil
}

// CreateNewRoll creates and uploads a new Android roll to the given commit.
// Returns the change number of the uploaded roll.
func (gr *githubRepoManager) CreateNewRoll(ctx context.Context, from, to string, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	gr.repoMtx.Lock()
	defer gr.repoMtx.Unlock()

	sklog.Info("Creating a new Github Roll")

	// Clean the checkout, get onto a fresh branch.
	if err := gr.cleanParentWithRemote(ctx, GITHUB_UPSTREAM_REMOTE_NAME); err != nil {
		return 0, err
	}
	if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "checkout", fmt.Sprintf("%s/%s", GITHUB_UPSTREAM_REMOTE_NAME, gr.parentBranch), "-b", ROLL_BRANCH); err != nil {
		return 0, err
	}
	// Defer cleanup.
	defer func() {
		util.LogErr(gr.cleanParentWithRemote(ctx, GITHUB_UPSTREAM_REMOTE_NAME))
	}()

	// Make sure the forked repo is at the same hash as the target repo before
	// creating the pull request.
	if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "push", "origin", ROLL_BRANCH, "-f"); err != nil {
		return 0, err
	}
	// Run gclient sync to make the child repo match the new DEPS.
	if _, err := exec.RunCwd(ctx, gr.depsRepoManager.parentDir, filepath.Join(gr.depotTools, "gclient"), "sync"); err != nil {
		return 0, fmt.Errorf("Error when running gclient sync: %s", err)
	}

	// Make sure the right name and email are set.
	if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "config", "user.name", gr.user); err != nil {
		return 0, err
	}
	if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "config", "user.email", gr.user); err != nil {
		return 0, err
	}

	// Run roll-dep.
	args := []string{gr.childPath, "--ignore-dirty-tree", "--roll-to", to}
	sklog.Infof("Running command: roll-dep %s", strings.Join(args, " "))
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  gr.parentDir,
		Env:  depot_tools.Env(gr.depotTools),
		Name: gr.rollDep,
		Args: args,
	}); err != nil {
		return 0, err
	}
	// Build the commit message, starting with the message provided by roll-dep.
	commitMsg, err := exec.RunCwd(ctx, gr.parentDir, "git", "log", "-n1", "--format=%B", "HEAD")
	if err != nil {
		return 0, err
	}
	commitMsg += fmt.Sprintf(COMMIT_MSG_FOOTER_TMPL, gr.serverURL)

	// Run the pre-upload steps and collect any errors.
	preUploadErrors := []error{}
	for _, s := range gr.PreUploadSteps() {
		if err := s(ctx, gr.parentDir); err != nil {
			preUploadErrors = append(preUploadErrors, err)
		}
	}

	// Push to the forked repository.
	if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "push", "origin", ROLL_BRANCH, "-f"); err != nil {
		return 0, err
	}

	// Grab the first line of the commit msg to use as the title of the pull request.
	title := strings.Split(commitMsg, "\n")[0]
	// Create a pull request.
	headBranch := fmt.Sprintf("%s:%s", strings.Split(gr.user, "@")[0], ROLL_BRANCH)
	pr, err := gr.githubClient.CreatePullRequest(title, gr.parentBranch, headBranch)
	if err != nil {
		return 0, err
	}

	// Display all pre-upload errors as comments on the pull request.
	for _, preUploadErr := range preUploadErrors {
		sklog.Warningf("Adding pre-upload error comment: %s", preUploadErr.Error())
		if err := gr.githubClient.AddComment(pr.GetNumber(), fmt.Sprintf("Error: %s", preUploadErr.Error())); err != nil {
			return 0, err
		}
	}

	return int64(pr.GetNumber()), nil
}

func (gr *githubRepoManager) User() string {
	return gr.user
}

func (gr *githubRepoManager) GetFullHistoryUrl() string {
	user := strings.Split(gr.user, "@")[0]
	return fmt.Sprintf("https://github.com/%s/%s/pulls/%s", gr.githubClient.RepoOwner, gr.githubClient.RepoName, user)
}

func (gr *githubRepoManager) GetIssueUrlBase() string {
	return fmt.Sprintf("https://github.com/%s/%s/pull/", gr.githubClient.RepoOwner, gr.githubClient.RepoName)
}
