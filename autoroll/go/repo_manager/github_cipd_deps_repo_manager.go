package repo_manager

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// TODO(rmistry): Use a different branch name to see what happens - use the roller name.
// TODO(rmistry): Use https://skia-review.googlesource.com/c/buildbot/+/215085

const (
	cipdPackageUrlTmpl = "%s/p/%s/+/%s"
)

var (
	// Use this function to instantiate a NewGithubCipdDEPSRepoManager. This is able to be
	// overridden for testing.
	NewGithubCipdDEPSRepoManager func(context.Context, *GithubCipdDEPSRepoManagerConfig, string, string, *github.GitHub, string, string, *http.Client, codereview.CodeReview, bool) (RepoManager, error) = newGithubCipdDEPSRepoManager
)

// GithubCipdDEPSRepoManagerConfig provides configuration for the Github RepoManager.
type GithubCipdDEPSRepoManagerConfig struct {
	GithubDEPSRepoManagerConfig
	CipdAssetName string `json:"cipdAssetName"`
	CipdAssetTag  string `json:"cipdAssetTag"`
}

// Validate the config.
func (c *GithubCipdDEPSRepoManagerConfig) Validate() error {
	return c.GithubDEPSRepoManagerConfig.Validate()
}

// githubCipdDEPSRepoManager is a struct used by the autoroller for managing checkouts.
type githubCipdDEPSRepoManager struct {
	*githubDEPSRepoManager
	cipdAssetName string
	cipdAssetTag  string
	cipdClient    *cipd.Client
}

// newGithubCipdDEPSRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func newGithubCipdDEPSRepoManager(ctx context.Context, c *GithubCipdDEPSRepoManagerConfig, workdir, rollerName string, githubClient *github.GitHub, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (RepoManager, error) {
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
	}
	cipdClient, err := cipd.NewClient(client, path.Join(workdir, "cipd"))
	if err != nil {
		return nil, err
	}
	gcr := &githubCipdDEPSRepoManager{
		githubDEPSRepoManager: gr,
		cipdAssetName:         c.CipdAssetName,
		cipdAssetTag:          c.CipdAssetTag,
		cipdClient:            cipdClient,
	}

	return gcr, nil
}

// See documentation for RepoManager interface.
func (rm *githubCipdDEPSRepoManager) Update(ctx context.Context) error {
	// Sync the projects.
	rm.repoMtx.Lock()
	defer rm.repoMtx.Unlock()

	sklog.Info("Updating github repository")

	// If parentDir does not exist yet then create the directory structure and
	// populate it.
	if _, err := os.Stat(rm.parentDir); err != nil {
		if os.IsNotExist(err) {
			if err := rm.createAndSyncParent(ctx); err != nil {
				return fmt.Errorf("Could not create and sync %s: %s", rm.parentDir, err)
			}
			// Run gclient hooks to bring in any required binaries.
			if _, err := exec.RunCommand(ctx, &exec.Command{
				Dir:  rm.parentDir,
				Env:  rm.depotToolsEnv,
				Name: rm.gclient,
				Args: []string{"runhooks"},
			}); err != nil {
				return fmt.Errorf("Error when running gclient runhooks on %s: %s", rm.parentDir, err)
			}
		} else {
			return fmt.Errorf("Error when running os.Stat on %s: %s", rm.parentDir, err)
		}
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
	// gclient sync to get latest version of child repo to find the next roll
	// rev from.
	if err := rm.createAndSyncParentWithRemote(ctx, GITHUB_UPSTREAM_REMOTE_NAME); err != nil {
		return fmt.Errorf("Could not create and sync parent repo: %s", err)
	}

	// All this below needs to be something custom

	// Get the last roll revision.
	lastRollRev, err := rm.getLastRollRev(ctx, rm.childPath, rm.cipdAssetName)
	if err != nil {
		return err
	}

	fmt.Printf("\n\nGOT %s for %s and %s\n\n", lastRollRev, rm.childPath, rm.cipdAssetName)

	// Find the not-rolled child repo commits.
	notRolledRevs, err := getNotRolledRevs(ctx, rm.cipdClient, rm.lastRollRev, rm.cipdAssetName, rm.cipdAssetTag)
	if err != nil {
		return err
	}

	// Get the next roll revision.
	nextRollRev, err := rm.getNextRollRev(ctx, notRolledRevs, lastRollRev)
	if err != nil {
		return err
	}

	rm.infoMtx.Lock()
	defer rm.infoMtx.Unlock()
	if rm.childRepoUrl == "" {
		childRepo, err := exec.RunCwd(ctx, rm.childDir, "git", "remote", "get-url", "origin")
		if err != nil {
			return err
		}
		rm.childRepoUrl = childRepo
	}

	rm.lastRollRev = lastRollRev
	rm.nextRollRev = nextRollRev
	rm.notRolledRevs = notRolledRevs

	sklog.Infof("lastRollRev is: %s", rm.lastRollRev)
	sklog.Infof("nextRollRev is: %s", nextRollRev)
	sklog.Infof("notRolledRevs: %v", rm.notRolledRevs)
	sklog.Fatal("here")
	return nil
}

// getNotRolledRevs is a utility function that uses CIPD to find the not-yet-rolled versions of
// the specified package.
// Note: that this just finds all versions of the package between the last rolled version and the
// version currently pointed to by cipdAssetTag; we can't know whether the ref we're tracking was
// ever actually applied to any of the package instances in between.
func getNotRolledRevs(ctx context.Context, cipdClient *cipd.Client, lastRollRev, cipdAssetName, cipdAssetTag string) ([]*revision.Revision, error) {
	head, err := cipdClient.ResolveVersion(ctx, cipdAssetName, cipdAssetTag)
	if err != nil {
		return nil, err
	}
	iter, err := cipdClient.ListInstances(ctx, cipdAssetName)
	if err != nil {
		return nil, err
	}
	notRolledRevs := []*revision.Revision{}
	foundHead := false
	for {
		instances, err := iter.Next(ctx, 100)
		if err != nil {
			return nil, err
		}
		if len(instances) == 0 {
			break
		}
		for _, instance := range instances {
			id := instance.Pin.InstanceID
			if id == head.InstanceID {
				foundHead = true
			}
			if id == lastRollRev {
				break
			}
			if foundHead {
				notRolledRevs = append(notRolledRevs, &revision.Revision{
					Id:          id,
					Display:     instance.Pin.String(),
					Description: instance.Pin.String(),
					Timestamp:   time.Time(instance.RegisteredTs),
					URL:         fmt.Sprintf(cipdPackageUrlTmpl, cipd.SERVICE_URL, cipdAssetName, id),
				})
			}
		}
	}
	return notRolledRevs, nil
}

// See documentation for RepoManager interface.
func (dr *githubCipdDEPSRepoManager) getLastRollRev(ctx context.Context, path, cipdAsset string) (string, error) {
	output, err := exec.RunCwd(ctx, dr.parentDir, "python", dr.gclient, "getdep", "-r", fmt.Sprintf("%s:%s", path, cipdAsset))
	if err != nil {
		return "", err
	}
	commit := strings.TrimSpace(output)
	if commit == "" {
		return "", fmt.Errorf("Got invalid output for `gclient getdep`: %s", output)
	}
	return commit, nil
}

// TODO(rmistry): FINISH THIS!
// TODO(rmistry): ROLL_BACK
// TODO(Rmistry): set-dep
// TODO(rmistry): license scripts.
// See documentation for RepoManager interface.
func (rm *githubCipdDEPSRepoManager) CreateNewRoll(ctx context.Context, from, to string, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	rm.repoMtx.Lock()
	defer rm.repoMtx.Unlock()

	sklog.Info("Creating a new Github Roll")

	// Clean the checkout, get onto a fresh branch.
	if err := rm.cleanParentWithRemote(ctx, GITHUB_UPSTREAM_REMOTE_NAME); err != nil {
		return 0, err
	}
	if _, err := git.GitDir(rm.parentDir).Git(ctx, "checkout", fmt.Sprintf("%s/%s", GITHUB_UPSTREAM_REMOTE_NAME, rm.parentBranch), "-b", ROLL_BRANCH); err != nil {
		return 0, err
	}
	// Defer cleanup.
	defer func() {
		util.LogErr(rm.cleanParentWithRemote(ctx, GITHUB_UPSTREAM_REMOTE_NAME))
	}()

	// Make sure the forked repo is at the same hash as the target repo before
	// creating the pull request on the ROLL_BRANCH.
	//if _, err := git.GitDir(rm.parentDir).Git(ctx, "push", "origin", rm.parentBranch, "-f"); err != nil {
	//	return 0, err
	//}
	if _, err := git.GitDir(rm.parentDir).Git(ctx, "push", "origin", ROLL_BRANCH, "-f"); err != nil {
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
	args := []string{"setdep", "-r", fmt.Sprintf("%s@%s", rm.childPath, to)}
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  rm.parentDir,
		Env:  rm.depotToolsEnv,
		Name: rm.gclient,
		Args: args,
	}); err != nil {
		return 0, err
	}

	// Make the checkout match the new DEPS.
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  rm.depsRepoManager.parentDir,
		Env:  rm.depotToolsEnv,
		Name: rm.gclient,
		Args: []string{"sync"},
	}); err != nil {
		return 0, fmt.Errorf("Error when running gclient sync to make third_party/ match the new DEPS: %s", err)
	}

	// Run the pre-upload steps.
	for _, s := range rm.PreUploadSteps() {
		if err := s(ctx, rm.depotToolsEnv, rm.httpClient, rm.parentDir); err != nil {
			return 0, fmt.Errorf("Error when running pre-upload step: %s", err)
		}
	}

	// Build the commit message.
	logCmd := []string{"log", fmt.Sprintf("%s..%s", from, to), "--date=short", "--no-merges", "--format=%ad %ae %s"}
	logStr, err := rm.childRepo.Git(ctx, logCmd...)
	if err != nil {
		return 0, err
	}
	logStr = strings.TrimSpace(logStr)
	childRepoCompareURL := fmt.Sprintf("%s/%%2Blog/%s..%s", rm.childRepoUrl, from[:12], to[:12])
	commitMsg, err := GetGithubCommitMsg(logStr, childRepoCompareURL, rm.childPath, from, to, rm.serverURL, logCmd, emails)
	if err != nil {
		return 0, fmt.Errorf("Could not build github commit message: %s", err)
	}

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
	headBranch := fmt.Sprintf("%s:%s", rm.codereview.UserName(), ROLL_BRANCH)
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

// See documentation for RepoManager interface.
func (r *githubCipdDEPSRepoManager) DefaultStrategy() string {
	return strategy.ROLL_STRATEGY_BATCH
}

// See documentation for RepoManager interface.
func (r *githubCipdDEPSRepoManager) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
		strategy.ROLL_STRATEGY_SINGLE,
	}
}
