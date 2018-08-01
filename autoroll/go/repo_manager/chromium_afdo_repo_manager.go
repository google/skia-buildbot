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

	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

/*
	Repo manager which rolls Android AFDO profiles into Chromium.
*/

const (
	AFDO_COMMIT_MSG_TMPL = `Roll AFDO from %s to %s

This CL may cause a small binary size increase, roughly proportional
to how long it's been since our last AFDO profile roll. For larger
increases (around or exceeding 100KB), please file a bug against
gbiv@chromium.org. Additional context: https://crbug.com/805539
` + COMMIT_MSG_FOOTER_TMPL
)

var (
	// "Constants"

	AFDO_VERSION_FILE_PATH = path.Join("chrome", "android", "profiles", "newest.txt")

	// Use this function to instantiate a RepoManager. This is able to be
	// overridden for testing.
	NewAFDORepoManager func(context.Context, *AFDORepoManagerConfig, string, *gerrit.Gerrit, string, string, *http.Client) (RepoManager, error) = newAfdoRepoManager
)

// Shorten the AFDO version.
func afdoShortVersion(long string) string {
	return strings.TrimPrefix(strings.TrimSuffix(long, ".afdo.bz2"), "chromeos-chrome-amd64-")
}

// AFDORepoManagerConfig provides configuration for the AFDO RepoManager.
type AFDORepoManagerConfig struct {
	DepotToolsRepoManagerConfig
}

// afdoRepoManager is a RepoManager which rolls Android AFDO profile version
// numbers into Chromium. Unlike other rollers, there is no child repo to sync;
// the version number is obtained from Google Cloud Storage.
type afdoRepoManager struct {
	*depotToolsRepoManager
	afdoVersionFile  string
	authClient       *http.Client
	commitsNotRolled int      // Protected by infoMtx.
	versions         []string // Protected by infoMtx.
}

func newAfdoRepoManager(ctx context.Context, c *AFDORepoManagerConfig, workdir string, g *gerrit.Gerrit, recipeCfgFile, serverURL string, authClient *http.Client) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	drm, err := newDepotToolsRepoManager(ctx, c.DepotToolsRepoManagerConfig, path.Join(workdir, "repo_manager"), recipeCfgFile, serverURL, g)
	if err != nil {
		return nil, err
	}

	rv := &afdoRepoManager{
		afdoVersionFile:       path.Join(drm.parentDir, AFDO_VERSION_FILE_PATH),
		authClient:            authClient,
		depotToolsRepoManager: drm,
	}
	return rv, nil
}

// See documentation for RepoManager interface.
func (rm *afdoRepoManager) CreateNewRoll(ctx context.Context, from, to string, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	rm.repoMtx.Lock()
	defer rm.repoMtx.Unlock()

	// Clean the checkout, get onto a fresh branch.
	if err := rm.cleanParent(ctx); err != nil {
		return 0, err
	}
	if _, err := exec.RunCwd(ctx, rm.parentDir, "git", "checkout", "-b", ROLL_BRANCH, "-t", fmt.Sprintf("origin/%s", rm.parentBranch), "-f"); err != nil {
		return 0, err
	}

	// Defer some more cleanup.
	defer func() {
		util.LogErr(rm.cleanParent(ctx))
	}()

	// Create the roll CL.
	if _, err := exec.RunCwd(ctx, rm.parentDir, "git", "config", "user.name", getLocalPartOfEmailAddress(rm.user)); err != nil {
		return 0, err
	}
	if _, err := exec.RunCwd(ctx, rm.parentDir, "git", "config", "user.email", rm.user); err != nil {
		return 0, err
	}

	// Write the file.
	if err := ioutil.WriteFile(rm.afdoVersionFile, []byte(to), os.ModePerm); err != nil {
		return 0, err
	}

	// Commit.
	commitMsg := fmt.Sprintf(AFDO_COMMIT_MSG_TMPL, afdoShortVersion(from), afdoShortVersion(to), rm.serverURL)
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  rm.parentDir,
		Env:  depot_tools.Env(rm.depotTools),
		Name: "git",
		Args: []string{"commit", "-a", "-m", commitMsg},
	}); err != nil {
		return 0, err
	}

	// Upload the CL.
	uploadCmd := &exec.Command{
		Dir:     rm.parentDir,
		Env:     depot_tools.Env(rm.depotTools),
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
		Dir:  rm.parentDir,
		Env:  depot_tools.Env(rm.depotTools),
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

// See documentation for RepoManager interface.
func (rm *afdoRepoManager) PreUploadSteps() []PreUploadStep {
	return nil
}

// See documentation for RepoManager interface.
func (rm *afdoRepoManager) Update(ctx context.Context) error {
	// Sync the projects.
	rm.repoMtx.Lock()
	defer rm.repoMtx.Unlock()

	if err := rm.createAndSyncParent(ctx); err != nil {
		return fmt.Errorf("Could not create and sync parent repo: %s", err)
	}

	// Read the file to determine the last roll rev.
	lastRollRevBytes, err := ioutil.ReadFile(rm.afdoVersionFile)
	if err != nil {
		return err
	}
	lastRollRev := strings.TrimSpace(string(lastRollRevBytes))

	// Get the next roll rev.
	nextRollRev, err := rm.getNextRollRev(ctx, nil, lastRollRev)
	if err != nil {
		return err
	}
	rm.strategyMtx.RLock()
	defer rm.strategyMtx.RUnlock()
	versions := rm.strategy.(*strategy.AFDOStrategy).GetVersions()
	lastIdx := -1
	nextIdx := -1
	for idx, v := range versions {
		if v == lastRollRev {
			lastIdx = idx
		}
		if v == nextRollRev {
			nextIdx = idx
		}
	}
	if lastIdx == -1 {
		sklog.Errorf("Last roll rev %q not found in available versions. Not-rolled count will be wrong.", lastRollRev)
	}
	if nextIdx == -1 {
		sklog.Errorf("Next roll rev %q not found in available versions. Not-rolled count will be wrong.", nextRollRev)
	}

	rm.infoMtx.Lock()
	defer rm.infoMtx.Unlock()
	rm.lastRollRev = lastRollRev
	rm.nextRollRev = nextRollRev
	// This seems backwards, but the versions are in descending order.
	rm.commitsNotRolled = lastIdx - nextIdx
	rm.versions = versions
	return nil
}

// See documentation for RepoManager interface.
func (rm *afdoRepoManager) FullChildHash(ctx context.Context, ver string) (string, error) {
	rm.infoMtx.RLock()
	defer rm.infoMtx.RUnlock()
	for _, v := range rm.versions {
		if strings.Contains(v, ver) {
			return v, nil
		}
	}
	return "", fmt.Errorf("Unable to find version: %s", ver)
}

// See documentation for RepoManager interface.
func (rm *afdoRepoManager) RolledPast(ctx context.Context, ver string) (bool, error) {
	verIsNewer, err := strategy.AFDOVersionGreater(ver, rm.LastRollRev())
	if err != nil {
		return false, err
	}
	return !verIsNewer, nil
}

// See documentation for RepoManager interface.
func (rm *afdoRepoManager) CommitsNotRolled() int {
	rm.infoMtx.RLock()
	defer rm.infoMtx.RUnlock()
	return rm.commitsNotRolled
}

// See documentation for RepoManager interface.
func (r *afdoRepoManager) CreateNextRollStrategy(ctx context.Context, s string) (strategy.NextRollStrategy, error) {
	return strategy.GetNextRollStrategy(ctx, s, r.childBranch, DEFAULT_REMOTE, "", "", r.childRepo, r.authClient)
}

// See documentation for RepoManager interface.
func (r *afdoRepoManager) DefaultStrategy() string {
	return strategy.ROLL_STRATEGY_AFDO
}

// See documentation for RepoManager interface.
func (r *afdoRepoManager) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_AFDO,
	}
}
