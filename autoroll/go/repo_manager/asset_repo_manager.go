package repo_manager

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

/*
	Repo Manager which rolls infra asset versions.
*/

const (
	// Path to asset VERSION files within a given repo.
	ASSET_VERSION_TMPL = "infra/bots/assets/%s/VERSION"

	// Commit message for AssetRepoManager.
	// TODO(borenet): autoroll.FromGerritChangeInfo wants to parse the
	// revisions from the roll subject. In this case, it makes no sense
	// bit it's still required for the roller to operate. We should find
	// a way around it so that we can be more free with our commit message.
	TMPL_COMMIT_MESSAGE_ASSETS = `Roll {{.Asset}} {{.From}}..{{.To}}

The AutoRoll server is located here: {{.ServerURL}}

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md

If the roll is causing failures, please contact the current sheriff, who should
be CC'd on the roll, and stop the roller if necessary.

TBR={{.Emails}}
`
)

var (
	commitMsgTmplAssets = template.Must(template.New("commitMsgAssets").Parse(TMPL_COMMIT_MESSAGE_ASSETS))
)

// AssetRepoManagerConfig provides configuration for the AssetRepoManager.
type AssetRepoManagerConfig struct {
	DepotToolsRepoManagerConfig

	// Which assets to roll.
	Asset string `json:"asset"`

	// URL of the child repo.
	ChildRepo string `json:"childRepo"`
}

// Validate the AssetRepoManagerConfig.
func (c *AssetRepoManagerConfig) Validate() error {
	if err := c.DepotToolsRepoManagerConfig.Validate(); err != nil {
		return err
	}
	if c.Asset == "" {
		return errors.New("Asset is required.")
	}
	if c.ChildRepo == "" {
		return errors.New("ChildRepo is required.")
	}
	return nil
}

// assetRepoManager is a RepoManager which rolls infra asset versions so that
// the "parent" repo's versions match the "child" repo's versions.
type assetRepoManager struct {
	*depotToolsRepoManager
	asset string
}

// NewAssetRepoManager returns a RepoManager instance which rolls infra asset
// versions so that the "parent" repo's versions match the "child" repo's
// versions.
func NewAssetRepoManager(ctx context.Context, c *AssetRepoManagerConfig, workdir string, g gerrit.GerritInterface, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	drm, err := newDepotToolsRepoManager(ctx, c.DepotToolsRepoManagerConfig, workdir, recipeCfgFile, serverURL, g, client, cr, local)
	if err != nil {
		return nil, err
	}
	rv := &assetRepoManager{
		depotToolsRepoManager: drm,
		asset:                 c.Asset,
	}
	// By default, depotToolsRepoManager assumes that the child repo is
	// nested inside the parent repo and is obtained via "gclient sync".
	// This is not the case for AssetRepoManager, so we have to sync it
	// ourselves.
	childRepo, err := git.NewCheckout(ctx, c.ChildRepo, workdir)
	if err != nil {
		return nil, err
	}
	rv.childRepo = childRepo
	rv.childDir = childRepo.Dir()
	return rv, nil
}

// See documentation for noCheckoutRepoManagerBuildCommitMessageFunc.
func (rm *assetRepoManager) buildCommitMessage(from, to, serverURL, cqExtraTrybots string, emails []string) (string, error) {
	rm.infoMtx.RLock()
	defer rm.infoMtx.RUnlock()

	// Build the commit message.
	data := struct {
		Asset     string
		Emails    string
		From      string
		ServerURL string
		To        string
	}{
		Asset:     rm.asset,
		Emails:    strings.Join(emails, ","),
		From:      from,
		ServerURL: serverURL,
		To:        to,
	}
	var buf bytes.Buffer
	if err := commitMsgTmplAssets.Execute(&buf, data); err != nil {
		return "", err
	}
	msg := buf.String()
	if cqExtraTrybots != "" {
		msg += fmt.Sprintf("\nCQ_INCLUDE_TRYBOTS=%s", cqExtraTrybots)
	}
	return buf.String(), nil
}

// Read the version of the given asset from the given repo at the given commit.
func readAssetVersion(ctx context.Context, repo git.GitDir, commit, asset string) (string, error) {
	return repo.GetFile(ctx, fmt.Sprintf(ASSET_VERSION_TMPL, asset), commit)
}

// See documentation for RepoManager interface.
func (rm *assetRepoManager) Update(ctx context.Context) error {
	rm.repoMtx.Lock()
	defer rm.repoMtx.Unlock()

	if err := rm.createAndSyncParent(ctx); err != nil {
		return fmt.Errorf("Could not create and sync parent repo: %s", err)
	}

	// By default, depotToolsRepoManager assumes that the child repo is
	// nested inside the parent repo and is obtained via "gclient sync".
	// This is not the case for AssetRepoManager, so we have to sync it
	// ourselves.
	if err := rm.childRepo.Update(ctx); err != nil {
		return err
	}

	// Obtain the hash of the current parent and child HEADs.
	parentHead, err := git.GitDir(rm.parentDir).RevParse(ctx, "origin/"+rm.parentBranch)
	if err != nil {
		return err
	}
	childHead, err := rm.childRepo.RevParse(ctx, "origin/"+rm.childBranch)
	if err != nil {
		return err
	}

	// Read the asset versions from both repos.
	lastRollRev, err := readAssetVersion(ctx, git.GitDir(rm.parentDir), parentHead, rm.asset)
	if err != nil {
		return err
	}
	lastRollRev = strings.TrimSpace(lastRollRev)
	nextRollRev, err := readAssetVersion(ctx, git.GitDir(rm.childDir), childHead, rm.asset)
	if err != nil {
		return err
	}
	nextRollRev = strings.TrimSpace(nextRollRev)

	// Obtain the list of not-yet-rolled revisions.
	lastInt, err := strconv.Atoi(lastRollRev)
	if err != nil {
		return err
	}
	nextInt, err := strconv.Atoi(nextRollRev)
	if err != nil {
		return err
	}
	notRolledRevs := make([]string, 0, nextInt-lastInt)
	for rev := lastInt + 1; rev <= nextInt; rev++ {
		notRolledRevs = append(notRolledRevs, strconv.Itoa(rev))
	}

	rm.infoMtx.Lock()
	defer rm.infoMtx.Unlock()
	rm.lastRollRev = lastRollRev
	rm.nextRollRev = nextRollRev
	rm.notRolledRevs = notRolledRevs
	return nil
}

// See documentation for RepoManager interface.
func (rm *assetRepoManager) CreateNewRoll(ctx context.Context, from, to string, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
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
	if !rm.local {
		if _, err := exec.RunCwd(ctx, rm.parentDir, "git", "config", "user.name", rm.codereview.UserName()); err != nil {
			return 0, err
		}
		if _, err := exec.RunCwd(ctx, rm.parentDir, "git", "config", "user.email", rm.codereview.UserEmail()); err != nil {
			return 0, err
		}
	}
	assetFile := path.Join(rm.parentDir, fmt.Sprintf(ASSET_VERSION_TMPL, rm.asset))
	if err := util.WithWriteFile(assetFile, func(w io.Writer) error {
		_, err := w.Write([]byte(to))
		return err
	}); err != nil {
		return 0, err
	}
	sklog.Infof("Rolling package %s from %s to %s.", rm.asset, from, to)

	// Before running the pre-upload steps, which install Go and the infra
	// repo's dependencies via CIPD, update the version of the Go DEPS CIPD
	// package to install the one requested by the roll.
	// TODO(borenet): Rather than modifying this global variable, which
	// could affect other callers (though there aren't any at the time of
	// writing), find a way to plumb the desired version through the
	// pre-upload step.
	cipd.PkgGoDEPS.Version = cipd.VersionTag(to)

	// Run the pre-upload steps.
	sklog.Infof("Running pre-upload steps.")
	for _, s := range rm.PreUploadSteps() {
		if err := s(ctx, rm.depotToolsEnv, rm.httpClient, rm.parentDir); err != nil {
			return 0, fmt.Errorf("Failed pre-upload step: %s", err)
		}
	}

	// Commit.
	commitMsg, err := rm.buildCommitMessage(from, to, rm.serverURL, cqExtraTrybots, emails)
	if err != nil {
		return 0, err
	}
	if _, err := exec.RunCwd(ctx, rm.parentDir, "git", "commit", "-a", "-m", commitMsg); err != nil {
		return 0, err
	}

	// Upload the CL.
	uploadCmd := &exec.Command{
		Dir:        rm.parentDir,
		Env:        rm.depotToolsEnv,
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
		Dir:        rm.parentDir,
		Env:        rm.depotToolsEnv,
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

	// Mark the change as ready for review, if necessary.
	if err := rm.unsetWIP(nil, issue.Issue); err != nil {
		return 0, err
	}

	return issue.Issue, nil
}

// See documentation for RepoManager interface.
func (rm *assetRepoManager) FullChildHash(ctx context.Context, version string) (string, error) {
	// We're rolling asset version numbers, not commit hashes, and we don't
	// shorten them. Return the given version.
	return version, nil
}

// See documentation for RepoManager interface.
func (r *assetRepoManager) CreateNextRollStrategy(ctx context.Context, s string) (strategy.NextRollStrategy, error) {
	return strategy.GetNextRollStrategy(ctx, s, r.childBranch, DEFAULT_REMOTE, "", []string{}, nil, nil)
}

// See documentation for RepoManager interface.
func (r *assetRepoManager) SetStrategy(s strategy.NextRollStrategy) {
	r.strategyMtx.Lock()
	defer r.strategyMtx.Unlock()
	r.strategy = s
}

// See documentation for RepoManager interface.
func (r *assetRepoManager) DefaultStrategy() string {
	return strategy.ROLL_STRATEGY_BATCH
}

// See documentation for RepoManager interface.
func (r *assetRepoManager) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
	}
}

// See documentation for RepoManager interface.
func (rm *assetRepoManager) RolledPast(ctx context.Context, version string) (bool, error) {
	lastInt, err := strconv.Atoi(rm.lastRollRev)
	if err != nil {
		return false, err
	}
	versionInt, err := strconv.Atoi(version)
	if err != nil {
		return false, err
	}
	return lastInt >= versionInt, nil
}
