package repo_manager

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	cipd_api "go.chromium.org/luci/cipd/client/cipd"

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

const (
	cipdPackageUrlTmpl = "%s/p/%s/+/%s"

	cipdGithubTitleTmpl = "Roll %s from %s to %s"
	cipdCommitMsgTmpl   = cipdGithubTitleTmpl + `

` + COMMIT_MSG_FOOTER_TMPL
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
	if c.CipdAssetName == "" {
		return fmt.Errorf("CipdAssetName is required.")
	}
	if c.CipdAssetTag == "" {
		return fmt.Errorf("CipdAssetTag is required.")
	}
	return c.GithubDEPSRepoManagerConfig.Validate()
}

// githubCipdDEPSRepoManager is a struct used by the autoroller for managing checkouts.
type githubCipdDEPSRepoManager struct {
	*githubDEPSRepoManager
	cipdAssetName string
	cipdAssetTag  string
	CipdClient    cipd.CIPDClient
}

// newGithubCipdDEPSRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func newGithubCipdDEPSRepoManager(ctx context.Context, c *GithubCipdDEPSRepoManagerConfig, workdir, rollerName string, githubClient *github.GitHub, recipeCfgFile, serverURL string, httpClient *http.Client, cr codereview.CodeReview, local bool) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	wd := path.Join(workdir, strings.TrimSuffix(path.Base(c.DepotToolsRepoManagerConfig.ParentRepo), ".git"))
	drm, err := newDepotToolsRepoManager(ctx, c.DepotToolsRepoManagerConfig, wd, recipeCfgFile, serverURL, nil, httpClient, cr, local)
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
	}
	sklog.Infof("Roller name is: %s\n", rollerName)
	cipdClient, err := cipd.NewClient(httpClient, path.Join(workdir, "cipd"))
	if err != nil {
		return nil, err
	}
	gcr := &githubCipdDEPSRepoManager{
		githubDEPSRepoManager: gr,
		cipdAssetName:         c.CipdAssetName,
		cipdAssetTag:          c.CipdAssetTag,
		CipdClient:            cipdClient,
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
			if err := rm.createAndSyncParentWithRemoteAndBranch(ctx, "origin", rm.rollBranchName, rm.rollBranchName); err != nil {
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
	if err := rm.createAndSyncParentWithRemoteAndBranch(ctx, GITHUB_UPSTREAM_REMOTE_NAME, rm.rollBranchName, rm.parentBranch); err != nil {
		return fmt.Errorf("Could not create and sync parent repo: %s", err)
	}

	// Get the last roll revision.
	lastRollRev, err := rm.getLastRollRev(ctx)
	if err != nil {
		return err
	}

	// Find the not-rolled child repo commits.
	notRolledRevs, err := rm.getNotRolledRevs(ctx, lastRollRev)
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
		childRepo, err := exec.RunCwd(ctx, rm.parentDir, "git", "remote", "get-url", "origin")
		if err != nil {
			return err
		}
		rm.childRepoUrl = childRepo
	}

	rm.lastRollRev = lastRollRev
	rm.nextRollRev = nextRollRev
	rm.notRolledRevs = notRolledRevs

	sklog.Infof("lastRollRev is: %s", rm.lastRollRev.Id)
	sklog.Infof("nextRollRev is: %s", nextRollRev.Id)
	sklog.Infof("len(notRolledRevs): %v", len(rm.notRolledRevs))
	return nil
}

func (rm *githubCipdDEPSRepoManager) cipdInstanceToRevision(instance *cipd_api.InstanceInfo) *revision.Revision {
	return &revision.Revision{
		Id:          instance.Pin.InstanceID,
		Display:     instance.Pin.String(),
		Description: instance.Pin.String(),
		Timestamp:   time.Time(instance.RegisteredTs),
		URL:         fmt.Sprintf(cipdPackageUrlTmpl, cipd.SERVICE_URL, rm.cipdAssetName, instance.Pin.InstanceID),
	}
}

// getNotRolledRevs is a utility function that uses CIPD to find the not-yet-rolled versions of
// the specified package.
// Note: that this just finds all versions of the package between the last rolled version and the
// version currently pointed to by cipdAssetTag; we can't know whether the ref we're tracking was
// ever actually applied to any of the package instances in between.
func (rm *githubCipdDEPSRepoManager) getNotRolledRevs(ctx context.Context, lastRollRev *revision.Revision) ([]*revision.Revision, error) {
	head, err := rm.CipdClient.ResolveVersion(ctx, rm.cipdAssetName, rm.cipdAssetTag)
	if err != nil {
		return nil, err
	}
	if lastRollRev.Id == head.InstanceID {
		return []*revision.Revision{}, nil
	}
	iter, err := rm.CipdClient.ListInstances(ctx, rm.cipdAssetName)
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
			if id == lastRollRev.Id {
				return notRolledRevs, nil
			}
			if foundHead {
				notRolledRevs = append(notRolledRevs, rm.cipdInstanceToRevision(&instance))
			}
		}
	}
	return notRolledRevs, nil
}

// See documentation for RepoManager interface.
func (rm *githubCipdDEPSRepoManager) RolledPast(ctx context.Context, rev *revision.Revision) (bool, error) {
	rm.infoMtx.RLock()
	defer rm.infoMtx.RUnlock()
	// TODO(rmistry): This is incorrect.
	return rm.lastRollRev.Id == rev.Id, nil
}

func (rm *githubCipdDEPSRepoManager) getLastRollRev(ctx context.Context) (*revision.Revision, error) {
	output, err := exec.RunCwd(ctx, rm.parentDir, "python", rm.gclient, "getdep", "-r", fmt.Sprintf("%s:%s", rm.childPath, rm.cipdAssetName))
	if err != nil {
		return nil, err
	}
	hash := strings.TrimSpace(output)
	if hash == "" {
		return nil, fmt.Errorf("Got invalid output for `gclient getdep`: %s", output)
	}
	instance, err := rm.CipdClient.Describe(ctx, rm.cipdAssetName, hash)
	if err != nil {
		return nil, err
	}
	return rm.cipdInstanceToRevision(&instance.InstanceInfo), nil
}

// See documentation for RepoManager interface.
func (rm *githubCipdDEPSRepoManager) CreateNewRoll(ctx context.Context, from, to *revision.Revision, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
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
	args := []string{"setdep", "-r", fmt.Sprintf("%s:%s@%s", rm.childPath, rm.cipdAssetName, to.Id)}
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  rm.parentDir,
		Env:  rm.depotToolsEnv,
		Name: rm.gclient,
		Args: args,
	}); err != nil {
		return 0, err
	}

	// Make the checkout match the new DEPS.
	sklog.Info("Running gclient sync on the checkout")
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  rm.depsRepoManager.parentDir,
		Env:  rm.depotToolsEnv,
		Name: rm.gclient,
		Args: []string{"sync", "-D", "-f"},
	}); err != nil {
		return 0, fmt.Errorf("Error when running gclient sync to make checkout match the new DEPS: %s", err)
	}

	// Run the pre-upload steps.
	for _, s := range rm.PreUploadSteps() {
		if err := s(ctx, rm.depotToolsEnv, rm.httpClient, rm.parentDir); err != nil {
			return 0, fmt.Errorf("Error when running pre-upload step: %s", err)
		}
	}

	// Build the commitMsg.
	commitMsg := fmt.Sprintf(cipdCommitMsgTmpl, rm.cipdAssetName, from, to, rm.serverURL)

	// Commit.
	if _, err := git.GitDir(rm.parentDir).Git(ctx, "commit", "-a", "-m", commitMsg); err != nil {
		return 0, err
	}

	// Push to the forked repository.
	if _, err := git.GitDir(rm.parentDir).Git(ctx, "push", "origin", rm.rollBranchName, "-f"); err != nil {
		return 0, err
	}

	// Create a pull request.
	title := fmt.Sprintf(cipdGithubTitleTmpl, rm.cipdAssetName, from.Id[:5]+"...", to.Id[:5]+"...")
	headBranch := fmt.Sprintf("%s:%s", rm.codereview.UserName(), rm.rollBranchName)
	pr, err := rm.githubClient.CreatePullRequest(title, rm.parentBranch, headBranch, commitMsg)
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
	}
}

// See documentation for RepoManager interface.
func (r *githubCipdDEPSRepoManager) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	instance, err := r.CipdClient.Describe(ctx, r.cipdAssetName, id)
	if err != nil {
		return nil, err
	}
	return r.cipdInstanceToRevision(&instance.InstanceInfo), nil
}
