package repo_manager

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	GITHUB_COMMIT_MSG_TMPL = `Roll {{.ChildPath}} {{.From}}..{{.To}} ({{.NumCommits}} commits)

{{.ChildRepoCompareUrl}}

git log {{.From}}..{{.To}} --date=short --no-merges --format='%%ad %%ae %%s'
{{.LogStr}}

{{.Footer}}
`
)

var (
	// Use this function to instantiate a NewGithubRepoManager. This is able to be
	// overridden for testing.
	NewGithubRepoManager func(context.Context, *GithubRepoManagerConfig, string, *github.GitHub, string, string) (RepoManager, error) = newGithubRepoManager

	githubCommitMsgTmpl = template.Must(template.New("githubCommitMsg").Parse(GITHUB_COMMIT_MSG_TMPL))
)

// GithubRepoManagerConfig provides configuration for the Github RepoManager.
type GithubRepoManagerConfig struct {
	CommonRepoManagerConfig
	// URL of the parent repo.
	ParentRepoURL string `json:"parentRepoURL"`
	// URL of the child repo.
	ChildRepoURL string `json:"childRepoURL"`
	// The roller will update this file with the child repo's revision.
	RevisionFile string `json:"revisionFile"`
}

// githubRepoManager is a struct used by the autoroller for managing checkouts.
type githubRepoManager struct {
	*commonRepoManager
	githubClient  *github.GitHub
	parentRepo    *git.Checkout
	parentRepoURL string
	childRepoURL  string
	revisionFile  string
}

// newGithubRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func newGithubRepoManager(ctx context.Context, c *GithubRepoManagerConfig, workdir string, githubClient *github.GitHub, recipeCfgFile, serverURL string) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	wd := path.Join(workdir, "github_repos")
	if _, err := os.Stat(wd); err != nil {
		if err := os.MkdirAll(wd, 0755); err != nil {
			return nil, err
		}
	}

	// Find the user.
	user, err := githubClient.GetAuthenticatedUser()
	if err != nil {
		return nil, err
	}
	userLogin := *user.Login

	// Create and populate the parent directory if needed.
	_, repo := GetUserAndRepo(c.ParentRepoURL)
	userFork := fmt.Sprintf("git@github.com:%s/%s.git", userLogin, repo)
	parentRepo, err := git.NewCheckout(ctx, userFork, wd)
	if err != nil {
		return nil, err
	}

	crm, err := newCommonRepoManager(c.CommonRepoManagerConfig, wd, serverURL, nil)
	if err != nil {
		return nil, err
	}
	crm.user = userLogin

	// Create and populate the child directory if needed.
	if _, err := os.Stat(crm.childDir); err != nil {
		if err := os.MkdirAll(crm.childDir, 0755); err != nil {
			return nil, err
		}
		if _, err := git.GitDir(crm.childDir).Git(ctx, "clone", c.ChildRepoURL, "."); err != nil {
			return nil, err
		}
	}

	gr := &githubRepoManager{
		commonRepoManager: crm,
		githubClient:      githubClient,
		parentRepo:        parentRepo,
		parentRepoURL:     c.ParentRepoURL,
		childRepoURL:      c.ChildRepoURL,
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
	if err := rm.parentRepo.Update(ctx); err != nil {
		return err
	}
	if err := rm.childRepo.Update(ctx); err != nil {
		return err
	}

	// Check to see whether there is an upstream yet.
	remoteOutput, err := rm.parentRepo.Git(ctx, "remote", "show")
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
		if _, err := rm.parentRepo.Git(ctx, "remote", "add", GITHUB_UPSTREAM_REMOTE_NAME, rm.parentRepoURL); err != nil {
			return err
		}
	}
	// Fetch upstream.
	if _, err := rm.parentRepo.Git(ctx, "fetch", GITHUB_UPSTREAM_REMOTE_NAME, rm.parentBranch); err != nil {
		return err
	}

	// Read the contents of the revision file to determine the last roll rev.
	revisionFileContents, err := rm.githubClient.ReadRawFile(rm.parentBranch, rm.revisionFile)
	if err != nil {
		return err
	}
	lastRollRev := strings.TrimRight(revisionFileContents, "\n")

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
	if _, err := rm.parentRepo.Git(ctx, "clean", "-d", "-f", "-f"); err != nil {
		return err
	}
	_, _ = rm.parentRepo.Git(ctx, "rebase", "--abort")
	if _, err := rm.parentRepo.Git(ctx, "checkout", fmt.Sprintf("%s/%s", GITHUB_UPSTREAM_REMOTE_NAME, rm.parentBranch), "-f"); err != nil {
		return err
	}
	_, _ = rm.parentRepo.Git(ctx, "branch", "-D", ROLL_BRANCH)
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
	if _, err := rm.parentRepo.Git(ctx, "checkout", fmt.Sprintf("%s/%s", GITHUB_UPSTREAM_REMOTE_NAME, rm.parentBranch), "-b", ROLL_BRANCH); err != nil {
		return 0, err
	}
	// Defer cleanup.
	defer func() {
		util.LogErr(rm.cleanParent(ctx))
	}()

	// Make sure the forked repo is at the same hash as the target repo before
	// creating the pull request.
	if _, err := rm.parentRepo.Git(ctx, "push", "origin", ROLL_BRANCH, "-f"); err != nil {
		return 0, err
	}

	// Make sure the right name and email are set.
	if _, err := rm.parentRepo.Git(ctx, "config", "user.name", rm.user); err != nil {
		return 0, err
	}
	if _, err := rm.parentRepo.Git(ctx, "config", "user.email", rm.user); err != nil {
		return 0, err
	}

	// Write the file.
	if err := ioutil.WriteFile(path.Join(rm.parentRepo.Dir(), rm.revisionFile), []byte(to+"\n"), os.ModePerm); err != nil {
		return 0, err
	}

	// Run the pre-upload steps.
	for _, s := range rm.PreUploadSteps() {
		if err := s(ctx, rm.parentRepo.Dir()); err != nil {
			return 0, fmt.Errorf("Error when running pre-upload step: %s", err)
		}
	}

	// Build the commit message.
	user, repo := GetUserAndRepo(rm.childRepoURL)
	githubCompareUrl := fmt.Sprintf("https://github.com/%s/%s/compare/%s...%s", user, repo, from[:12], to[:12])
	logStr, err := rm.childRepo.Git(ctx, "log", fmt.Sprintf("%s..%s", from, to), "--date=short", "--no-merges", "--format=%ad %ae %s")
	if err != nil {
		return 0, err
	}
	logStr = strings.TrimSpace(logStr)

	data := struct {
		ChildPath           string
		ChildRepoCompareUrl string
		From                string
		To                  string
		NumCommits          int
		LogURL              string
		LogStr              string
		ServerURL           string
		Footer              string
	}{
		ChildPath:           rm.childPath,
		ChildRepoCompareUrl: githubCompareUrl,
		From:                from[:12],
		To:                  to[:12],
		NumCommits:          len(strings.Split(logStr, "\n")),
		LogStr:              logStr,
		ServerURL:           rm.serverURL,
		Footer:              fmt.Sprintf(COMMIT_MSG_FOOTER_TMPL, rm.serverURL),
	}
	var buf bytes.Buffer
	if err := githubCommitMsgTmpl.Execute(&buf, data); err != nil {
		return 0, err
	}
	commitMsg := buf.String()

	// Commit.
	if _, err := rm.parentRepo.Git(ctx, "commit", "-a", "-m", commitMsg); err != nil {
		return 0, err
	}

	// Push to the forked repository.
	if _, err := rm.parentRepo.Git(ctx, "push", "origin", ROLL_BRANCH, "-f"); err != nil {
		return 0, err
	}

	// Grab the first line of the commit msg to use as the title of the pull request.
	title := strings.Split(commitMsg, "\n")[0]
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

func GetUserAndRepo(githubRepo string) (string, string) {
	repoTokens := strings.Split(githubRepo, ":")
	user := strings.Split(repoTokens[1], "/")[0]
	repo := strings.TrimRight(strings.Split(repoTokens[1], "/")[1], ".git")
	return user, repo
}

// See documentation for RepoManager interface.
func (rm *githubRepoManager) User() string {
	return rm.user
}

// See documentation for RepoManager interface.
func (rm *githubRepoManager) GetFullHistoryUrl() string {
	return rm.githubClient.GetFullHistoryUrl(rm.user)
}

// See documentation for RepoManager interface.
func (rm *githubRepoManager) GetIssueUrlBase() string {
	return rm.githubClient.GetIssueUrlBase()
}
