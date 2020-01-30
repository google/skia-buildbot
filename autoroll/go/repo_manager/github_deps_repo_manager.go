package repo_manager

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	GITHUB_UPSTREAM_REMOTE_NAME = "remote"
)

var (
	// Use this function to instantiate a NewGithubDEPSRepoManager. This is able to be
	// overridden for testing.
	NewGithubDEPSRepoManager func(context.Context, *GithubDEPSRepoManagerConfig, string, string, *github.GitHub, string, string, *http.Client, codereview.CodeReview, bool) (RepoManager, error) = newGithubDEPSRepoManager
)

// GithubDEPSRepoManagerConfig provides configuration for the Github RepoManager.
type GithubDEPSRepoManagerConfig struct {
	DepotToolsRepoManagerConfig
	// Optional config to use if parent path is different than
	// workdir + parent repo.
	GithubParentPath string `json:"githubParentPath,omitempty"`

	// Optional; transitive dependencies to roll. This is a mapping of
	// dependencies of the child repo which are also dependencies of the
	// parent repo and should be rolled at the same time. Keys are paths
	// to transitive dependencies within the child repo (as specified in
	// DEPS), and values are paths to those dependencies within the parent
	// repo.
	TransitiveDeps map[string]string `json:"transitiveDeps"`
}

// Validate the config.
func (c *GithubDEPSRepoManagerConfig) Validate() error {
	return c.DepotToolsRepoManagerConfig.Validate()
}

// githubDEPSRepoManager is a struct used by the autoroller for managing checkouts.
type githubDEPSRepoManager struct {
	*depsRepoManager
	githubClient   *github.GitHub
	githubConfig   *codereview.GithubConfig
	rollBranchName string
	transitiveDeps map[string]string
}

// newGithubDEPSRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func newGithubDEPSRepoManager(ctx context.Context, c *GithubDEPSRepoManagerConfig, workdir, rollerName string, githubClient *github.GitHub, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	wd := path.Join(workdir, strings.TrimSuffix(path.Base(c.DepotToolsRepoManagerConfig.ParentRepo), ".git"))
	drm, err := newDepotToolsRepoManager(ctx, c.DepotToolsRepoManagerConfig, wd, recipeCfgFile, serverURL, nil, client, cr, local)
	if err != nil {
		return nil, err
	}
	dr := &depsRepoManager{
		depotToolsRepoManager: drm,
	}
	if c.GithubParentPath != "" {
		dr.parentDir = path.Join(wd, c.GithubParentPath)
	}
	gr := &githubDEPSRepoManager{
		depsRepoManager: dr,
		githubClient:    githubClient,
		rollBranchName:  rollerName,
		transitiveDeps:  c.TransitiveDeps,
	}

	return gr, nil
}

func (rm *githubDEPSRepoManager) getdep(ctx context.Context, depsFile, depPath string) (string, error) {
	output, err := exec.RunCwd(ctx, path.Dir(depsFile), "python", rm.gclient, "getdep", "-r", depPath)
	if err != nil {
		return "", err
	}
	splitGetdep := strings.Split(strings.TrimSpace(output), "\n")
	rev := strings.TrimSpace(splitGetdep[len(splitGetdep)-1])
	if len(rev) != 40 {
		return "", fmt.Errorf("Got invalid output for `gclient getdep`: %s", output)
	}
	return rev, nil
}

func (rm *githubDEPSRepoManager) setdep(ctx context.Context, depsFile, depPath, rev string) error {
	args := []string{"setdep", "-r", fmt.Sprintf("%s@%s", depPath, rev)}
	_, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  path.Dir(depsFile),
		Env:  depot_tools.Env(rm.depotTools),
		Name: rm.gclient,
		Args: args,
	})
	return err
}

// getDEPSFile obtains and returns the path to the DEPS file, and a cleanup
// function to run when finished with it.
func (rm *githubDEPSRepoManager) getDEPSFile(ctx context.Context, repo *git.Checkout, baseCommit string) (rv string, cleanup func(), rvErr error) {
	repoURL, err := repo.Git(ctx, "remote", "get-url", "origin")
	if err != nil {
		return "", nil, err
	}
	repoURL = strings.TrimSpace(repoURL)

	wd, err := ioutil.TempDir("", "")
	if err != nil {
		return "", nil, err
	}
	defer func() {
		if rvErr != nil {
			util.RemoveAll(wd)
		}
	}()

	// Download the DEPS file from the parent repo.
	contents, err := repo.GetFile(ctx, "DEPS", baseCommit)
	if err != nil {
		return "", nil, err
	}

	// Use "gclient getdep" to retrieve the last roll revision.

	// "gclient getdep" requires a .gclient file.
	if _, err := exec.RunCwd(ctx, wd, "python", rm.gclient, "config", repoURL); err != nil {
		return "", nil, err
	}
	splitRepo := strings.Split(repoURL, "/")
	fakeCheckoutDir := path.Join(wd, strings.TrimSuffix(splitRepo[len(splitRepo)-1], ".git"))
	if err := os.Mkdir(fakeCheckoutDir, os.ModePerm); err != nil {
		return "", nil, err
	}
	depsFile := path.Join(fakeCheckoutDir, "DEPS")
	if err := ioutil.WriteFile(depsFile, []byte(contents), os.ModePerm); err != nil {
		return "", nil, err
	}
	return depsFile, func() { util.RemoveAll(wd) }, nil
}

// See documentation for RepoManager interface.
func (rm *githubDEPSRepoManager) Update(ctx context.Context) (*revision.Revision, *revision.Revision, []*revision.Revision, error) {
	// Sync the projects.
	rm.repoMtx.Lock()
	defer rm.repoMtx.Unlock()

	sklog.Info("Updating github repository")

	// If parentDir does not exist yet then create the directory structure and
	// populate it.
	if _, err := os.Stat(rm.parentDir); err != nil {
		if os.IsNotExist(err) {
			if err := rm.createAndSyncParentWithRemoteAndBranch(ctx, "origin", rm.rollBranchName, rm.rollBranchName); err != nil {
				return nil, nil, nil, fmt.Errorf("Could not create and sync %s: %s", rm.parentDir, err)
			}
			// Run gclient hooks to bring in any required binaries.
			if _, err := exec.RunCommand(ctx, &exec.Command{
				Dir:  rm.parentDir,
				Env:  rm.depotToolsEnv,
				Name: rm.gclient,
				Args: []string{"runhooks"},
			}); err != nil {
				return nil, nil, nil, fmt.Errorf("Error when running gclient runhooks on %s: %s", rm.parentDir, err)
			}
		} else {
			return nil, nil, nil, fmt.Errorf("Error when running os.Stat on %s: %s", rm.parentDir, err)
		}
	}

	// Check to see whether there is an upstream yet.
	remoteOutput, err := git.GitDir(rm.parentDir).Git(ctx, "remote", "show")
	if err != nil {
		return nil, nil, nil, err
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
			return nil, nil, nil, err
		}
	}
	// Fetch upstream.
	if _, err := git.GitDir(rm.parentDir).Git(ctx, "fetch", GITHUB_UPSTREAM_REMOTE_NAME, rm.parentBranch); err != nil {
		return nil, nil, nil, err
	}
	// gclient sync to get latest version of child repo to find the next roll
	// rev from.
	if err := rm.createAndSyncParentWithRemoteAndBranch(ctx, GITHUB_UPSTREAM_REMOTE_NAME, rm.rollBranchName, rm.parentBranch); err != nil {
		return nil, nil, nil, fmt.Errorf("Could not create and sync parent repo: %s", err)
	}

	// Get the last roll revision.
	lastRollHash, err := rm.getdep(ctx, filepath.Join(rm.parentDir, "DEPS"), rm.childPath)
	if err != nil {
		return nil, nil, nil, err
	}
	lastRollDetails, err := rm.childRepo.Details(ctx, lastRollHash)
	if err != nil {
		return nil, nil, nil, err
	}
	lastRollRev := revision.FromLongCommit(rm.childRevLinkTmpl, lastRollDetails)

	// Get the tip-of-tree revision.
	tipRev, err := rm.getTipRev(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	// Find the not-rolled child repo commits.
	notRolledRevs, err := rm.getCommitsNotRolled(ctx, lastRollRev, tipRev)
	if err != nil {
		return nil, nil, nil, err
	}

	// Transitive deps.
	if len(rm.transitiveDeps) > 0 {
		for _, rev := range append(notRolledRevs, tipRev, lastRollRev) {
			childDepsFile, childCleanup, err := rm.getDEPSFile(ctx, rm.childRepo, rev.Id)
			if err != nil {
				return nil, nil, nil, err
			}
			defer childCleanup()
			for childPath := range rm.transitiveDeps {
				childRev, err := rm.getdep(ctx, childDepsFile, childPath)
				if err != nil {
					return nil, nil, nil, err
				}
				if rev.Dependencies == nil {
					rev.Dependencies = map[string]string{}
				}
				rev.Dependencies[childPath] = childRev
			}
		}
	}

	return lastRollRev, tipRev, notRolledRevs, nil
}

// See documentation for RepoManager interface.
func (rm *githubDEPSRepoManager) CreateNewRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	rm.repoMtx.Lock()
	defer rm.repoMtx.Unlock()

	sklog.Info("Creating a new Github Roll")

	// Clean the checkout, get onto a fresh branch.
	if err := rm.cleanParentWithRemoteAndBranch(ctx, GITHUB_UPSTREAM_REMOTE_NAME, rm.rollBranchName, rm.parentBranch); err != nil {
		return 0, err
	}
	if _, err := git.GitDir(rm.parentDir).Git(ctx, "checkout", fmt.Sprintf("%s/%s", GITHUB_UPSTREAM_REMOTE_NAME, rm.parentBranch), "-b", rm.rollBranchName); err != nil {
		return 0, err
	}
	// Defer cleanup.
	defer func() {
		util.LogErr(rm.cleanParentWithRemoteAndBranch(ctx, GITHUB_UPSTREAM_REMOTE_NAME, rm.rollBranchName, rm.parentBranch))
	}()

	// Make sure the forked repo is at the same hash as the target repo before
	// creating the pull request on both parentBranch and rm.rollBranchName.
	if _, err := git.GitDir(rm.parentDir).Git(ctx, "push", "origin", rm.parentBranch, "-f"); err != nil {
		return 0, err
	}
	if _, err := git.GitDir(rm.parentDir).Git(ctx, "push", "origin", rm.rollBranchName, "-f"); err != nil {
		return 0, err
	}

	// Make sure the right name and email are set.
	if !rm.local {
		if _, err := git.GitDir(rm.parentDir).Git(ctx, "config", "user.name", rm.codereview.UserName()); err != nil {
			return 0, err
		}
		if _, err := git.GitDir(rm.parentDir).Git(ctx, "config", "user.email", rm.codereview.UserEmail()); err != nil {
			return 0, err
		}
	}

	// Run "gclient setdep".
	depsFile := filepath.Join(rm.parentDir, "DEPS")
	if err := rm.setdep(ctx, depsFile, rm.childPath, to.Id); err != nil {
		return 0, err
	}

	// Update any transitive DEPS.
	transitiveDeps := []*TransitiveDep{}
	if len(rm.transitiveDeps) > 0 {
		for childPath, parentPath := range rm.transitiveDeps {
			oldRev, err := rm.getdep(ctx, depsFile, parentPath)
			if err != nil {
				return 0, err
			}
			newRev, ok := to.Dependencies[childPath]
			if !ok {
				return 0, skerr.Fmt("To-revision %s is missing dependency entry for %s", to.Id, childPath)
			}
			if oldRev != newRev {
				if err := rm.setdep(ctx, depsFile, parentPath, newRev); err != nil {
					return 0, err
				}
				transitiveDeps = append(transitiveDeps, &TransitiveDep{
					ParentPath:  parentPath,
					RollingFrom: oldRev,
					RollingTo:   newRev,
				})
			}
		}
	}

	// Make third_party/ match the new DEPS.
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  rm.depsRepoManager.parentDir,
		Env:  rm.depotToolsEnv,
		Name: rm.gclient,
		Args: []string{"sync", "--delete_unversioned_trees", "--force"},
	}); err != nil {
		return 0, fmt.Errorf("Error when running gclient sync to make third_party/ match the new DEPS: %s", err)
	}

	// Run the pre-upload steps.
	for _, s := range rm.preUploadSteps {
		if err := s(ctx, rm.depotToolsEnv, rm.httpClient, rm.parentDir); err != nil {
			return 0, fmt.Errorf("Error when running pre-upload step: %s", err)
		}
	}

	// Build the commit message.
	childRepoUrl, err := rm.getChildRepoUrl(ctx)
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	user, repo := GetUserAndRepo(childRepoUrl)
	details := make([]*vcsinfo.LongCommit, 0, len(rolling))
	for _, c := range rolling {
		d, err := rm.childRepo.Details(ctx, c.Id)
		if err != nil {
			return 0, fmt.Errorf("Failed to get commit details: %s", err)
		}
		// Github autolinks PR numbers to be of the same repository in logStr. Fix this by
		// explicitly adding the child repo to the PR number.
		d.Subject = pullRequestInLogRE.ReplaceAllString(d.Subject, fmt.Sprintf(" (%s/%s$1)", user, repo))
		d.Body = pullRequestInLogRE.ReplaceAllString(d.Body, fmt.Sprintf(" (%s/%s$1)", user, repo))
		details = append(details, d)
	}
	commitMsg, err := rm.buildCommitMsg(&CommitMsgVars{
		ChildPath:      rm.childPath,
		ChildRepo:      childRepoUrl,
		Reviewers:      emails,
		Revisions:      rolling,
		RollingFrom:    from,
		RollingTo:      to,
		ServerURL:      rm.serverURL,
		TransitiveDeps: transitiveDeps,
	})
	if err != nil {
		return 0, fmt.Errorf("Could not build github commit message: %s", err)
	}

	// Commit.
	if _, err := git.GitDir(rm.parentDir).Git(ctx, "commit", "-a", "-m", commitMsg); err != nil {
		return 0, err
	}

	// Push to the forked repository.
	if _, err := git.GitDir(rm.parentDir).Git(ctx, "push", "origin", rm.rollBranchName, "-f"); err != nil {
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
	headBranch := fmt.Sprintf("%s:%s", rm.codereview.UserName(), rm.rollBranchName)
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

	return int64(pr.GetNumber()), nil
}
