package repo_manager

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

//const (
//	GITHUB_UPSTREAM_REMOTE_NAME = "remote"
//)

var (
	// Use this function to instantiate a NewGithubRepoManager. This is able to be
	// overridden for testing.
	NewGithubRepoManager func(context.Context, *GithubRepoManagerConfig, string, *github.GitHub, string, string) (RepoManager, error) = newGithubRepoManager
)

// GithubRepoManagerConfig provides configuration for the Github RepoManager.
type GithubRepoManagerConfig struct {
	CommonRepoManagerConfig
	// What extra stuff should go here?
	// 	// URL of the parent repo.
	ParentRepo string `json:"parentRepo"`
	// The roller will update this file with the child repo's revision.
	RevisionFile string `json:"revisionFile"`
	// URL of the child repo.
	ChildRepo string `json:"childRepo"`
}

// Validate the config.
// Any extra validation required here or not?
//func (c *GithubRepoManagerConfig) Validate() error
//{
//	return c.DepotToolsRepoManagerConfig.Validate()
//}

// TODO(Rmistry): All this needed or not???
// githubRepoManager is a struct used by the autoroller for managing checkouts.
type githubRepoManager struct {
	*commonRepoManager
	githubClient *github.GitHub
	parentDir    string
	parentRepo   string
	revisionFile string
}

// newGithubRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func newGithubRepoManager(ctx context.Context, c *GithubRepoManagerConfig, workdir string, githubClient *github.GitHub, recipeCfgFile, serverURL string) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	wd := path.Join(workdir, "github_repo")

	// Find the user.
	user, err := githubClient.GetAuthenticatedUser()
	if err != nil {
		return nil, err
	}

	// Create and populate the parent directory if needed.
	parentDir := path.Join(wd, strings.TrimSuffix(path.Base(c.ParentRepo), ".git"))
	if _, err := os.Stat(parentDir); err != nil {
		// Use the user's fork.
		repoTokens := strings.Split(c.ParentRepo, ":")
		repoWithoutUser := strings.Split(repoTokens[1], "/")[1]
		userFork := fmt.Sprintf("%s:%s/%s", repoTokens[0], user, repoWithoutUser)

		if _, err := git.GitDir(wd).Git(ctx, "clone", userFork); err != nil {
			return nil, err
		}
	}
	// Create and populate the child directory if needed.
	childDir := path.Join(wd, strings.TrimSuffix(path.Base(c.ChildRepo), ".git"))
	if _, err := os.Stat(childDir); err != nil {
		if _, err := git.GitDir(wd).Git(ctx, "clone", c.ChildRepo); err != nil {
			return nil, err
		}
	}

	crm, err := newCommonRepoManager(c.CommonRepoManagerConfig, wd, serverURL, nil)
	if err != nil {
		return nil, err
	}
	crm.user = *user.Login

	gr := &githubRepoManager{
		commonRepoManager: crm,
		githubClient:      githubClient,
		parentDir:         parentDir,
		parentRepo:        c.ParentRepo,
		revisionFile:      c.RevisionFile,
	}

	return gr, nil
}

// See documentation for RepoManager interface.
func (rm *githubRepoManager) Update(ctx context.Context) error {
	// Sync the projects.
	rm.repoMtx.Lock()
	defer rm.repoMtx.Unlock()

	// Update the repositories.
	parentCheckout := &git.Checkout{GitDir: git.GitDir(rm.parentDir)}
	if err := (parentCheckout.Update(ctx)); err != nil {
		return err
	}
	if err := rm.childRepo.Update(ctx); err != nil {
		return err
	}

	// Check to see whether there is an upstream yet.
	remoteOutput, err := git.GitDir(rm.parentDir).Git(ctx, "remote", "show")
	if err != nil {
		return err
	}
	remoteFound := false
	remoteLines := strings.Split(remoteOutput, "\n")
	for _, remoteLine := range remoteLines {
		if remoteLine == GITHUB_UPSTREAM_REMOTE_NAME {
			remoteFound = true
			break
		}
	}
	if !remoteFound {
		if _, err := git.GitDir(rm.parentDir).Git(ctx, "remote", "add", GITHUB_UPSTREAM_REMOTE_NAME, rm.parentRepo); err != nil {
			return err
		}
	}
	// Fetch upstream.
	if _, err := git.GitDir(rm.parentDir).Git(ctx, "fetch", GITHUB_UPSTREAM_REMOTE_NAME, rm.parentBranch); err != nil {
		return err
	}

	// Read the file to determine the last roll rev.
	lastRollRevBytes, err := ioutil.ReadFile(path.Join(rm.parentDir, rm.revisionFile))
	if err != nil {
		return err
	}
	lastRollRev := strings.TrimSpace(string(lastRollRevBytes))

	// Find the number of not-rolled child repo commits.
	notRolled, err := rm.getCommitsNotRolled(ctx, lastRollRev)
	if err != nil {
		return err
	}

	// Get the next roll revision.
	nextRollRev, err := rm.getNextRollRev(ctx, notRolled, lastRollRev)
	if err != nil {
		return err
	}

	rm.infoMtx.Lock()
	defer rm.infoMtx.Unlock()
	rm.lastRollRev = lastRollRev
	rm.nextRollRev = nextRollRev
	rm.commitsNotRolled = len(notRolled)

	sklog.Infof("lastRollRev is: %s", rm.lastRollRev)
	sklog.Infof("nextRollRev is: %s", nextRollRev)
	sklog.Infof("commitsNotRolled: %d", rm.commitsNotRolled)
	return nil
}

func (rm *githubRepoManager) cleanParent(ctx context.Context) error {
	if _, err := git.GitDir(rm.parentDir).Git(ctx, "clean", "-d", "-f", "-f"); err != nil {
		return err
	}
	git.GitDir(rm.parentDir).Git(ctx, "rebase", "--abort")
	if _, err := git.GitDir(rm.parentDir).Git(ctx, "checkout", fmt.Sprintf("%s/%s", GITHUB_UPSTREAM_REMOTE_NAME, rm.parentBranch), "-f"); err != nil {
		return err
	}
	_, _ = git.GitDir(rm.parentDir).Git(ctx, "branch", "-D", ROLL_BRANCH)
	return nil
}

// See documentation for RepoManager interface.
func (rm *githubRepoManager) CreateNewRoll(ctx context.Context, from, to string, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	rm.repoMtx.Lock()
	defer rm.repoMtx.Unlock()

	sklog.Info("Creating a new Github Roll")

	// Clean the checkout, get onto a fresh branch.
	if err := rm.cleanParent(ctx); err != nil {
		return 0, err
	}
	if _, err := git.GitDir(rm.parentDir).Git(ctx, "checkout", fmt.Sprintf("%s/%s", GITHUB_UPSTREAM_REMOTE_NAME, rm.parentBranch), "-b", ROLL_BRANCH); err != nil {
		return 0, err
	}
	// Defer cleanup.
	defer func() {
		util.LogErr(rm.cleanParent(ctx))
	}()

	// Make sure the forked repo is at the same hash as the target repo before
	// creating the pull request.
	if _, err := git.GitDir(rm.parentDir).Git(ctx, "push", "origin", ROLL_BRANCH, "-f"); err != nil {
		return 0, err
	}

	// Make sure the right name and email are set.
	if _, err := git.GitDir(rm.parentDir).Git(ctx, "config", "user.name", rm.user); err != nil {
		return 0, err
	}
	if _, err := git.GitDir(rm.parentDir).Git(ctx, "config", "user.email", rm.user); err != nil {
		return 0, err
	}

	// Write the file.
	if err := ioutil.WriteFile(path.Join(rm.parentDir, rm.revisionFile), []byte(to+"\n"), os.ModePerm); err != nil {
		return 0, err
	}

	// Run the pre-upload steps.
	for _, s := range rm.PreUploadSteps() {
		if err := s(ctx, rm.parentDir); err != nil {
			return 0, fmt.Errorf("Error when running pre-upload step: %s", err)
		}
	}

	sklog.Info("LOOK AT THE FILE LOOK AT THE FILE LOOK AT THE FILE!!!!!!!!!")
	sklog.Fatal("LOOK AT THE FILE!!!!!!!!!")

	// Build the commit message.
	commitMsg := "blah"
	// this is a problem
	//commitMsg, err := rm.buildCommitMsg(ctx, from, to, cqExtraTrybots, nil)
	//if err != nil {
	//	return 0, err
	//}

	// Commit.
	if _, err := git.GitDir(rm.parentDir).Git(ctx, "commit", "-a", "-m", commitMsg); err != nil {
		return 0, err
	}

	// Push to the forked repository.
	if _, err := git.GitDir(rm.parentDir).Git(ctx, "push", "origin", ROLL_BRANCH, "-f"); err != nil {
		return 0, err
	}

	// Grab the first line of the commit msg to use as the title of the pull request.
	title := strings.Split(commitMsg, "\n")[0]
	// Shorten the child path in the title for brevity.
	childPathTokens := strings.Split(rm.childPath, "/")
	shortenedChildName := childPathTokens[len(childPathTokens)-1]
	title = strings.Replace(title, rm.childPath+"/", shortenedChildName, 1)
	// Use the remaining part of the commit message as the pull request description.
	descComment := strings.Split(commitMsg, "\n")[1:]
	// Create a pull request.
	headBranch := fmt.Sprintf("%s:%s", strings.Split(rm.user, "@")[0], ROLL_BRANCH)
	pr, err := rm.githubClient.CreatePullRequest(title, rm.parentBranch, headBranch, strings.Join(descComment, "\n"))
	if err != nil {
		return 0, err
	}

	// Add appropriate label to the pull request.
	label := github.COMMIT_LABEL
	if dryRun {
		label = github.DRYRUN_LABEL
	}
	if err := rm.githubClient.AddLabel(pr.GetNumber(), label); err != nil {
		return 0, err
	}

	// Mention the sheriffs on the pull request so that they are automatically
	// subscribed to it.
	mentions := []string{}
	for _, e := range emails {
		m := fmt.Sprintf("@%s", strings.Split(e, "@")[0])
		mentions = append(mentions, m)
	}
	if err := rm.githubClient.AddComment(pr.GetNumber(), fmt.Sprintf("%s : New roll has been created by %s", strings.Join(mentions, " "), rm.serverURL)); err != nil {
		return 0, err
	}

	return int64(pr.GetNumber()), nil
}

// See documentation for RepoManager interface.
func (rm *githubRepoManager) User() string {
	return rm.user
}

// See documentation for RepoManager interface.
func (rm *githubRepoManager) GetFullHistoryUrl() string {
	user := strings.Split(rm.user, "@")[0]
	return fmt.Sprintf("https://github.com/%s/%s/pulls/%s", rm.githubClient.RepoOwner, rm.githubClient.RepoName, user)
}

// See documentation for RepoManager interface.
func (rm *githubRepoManager) GetIssueUrlBase() string {
	return fmt.Sprintf("https://github.com/%s/%s/pull/", rm.githubClient.RepoOwner, rm.githubClient.RepoName)
}
