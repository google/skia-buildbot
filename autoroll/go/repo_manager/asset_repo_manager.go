package repo_manager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
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

CQ_INCLUDE_TRYBOTS={{.CqExtraTrybots}}
TBR={{.Emails}}
`
)

var (
	commitMsgTmplAssets = template.Must(template.New("commitMsgAssets").Parse(TMPL_COMMIT_MESSAGE_ASSETS))
)

// AssetRepoManagerConfig provides configuration for the AssetRepoManager.
type AssetRepoManagerConfig struct {
	NoCheckoutRepoManagerConfig

	// Which assets to roll.
	Asset string `json:"asset"`

	// URL of the child repo.
	ChildRepo string `json:"childRepo"`
}

// Validate the AssetRepoManagerConfig.
func (c *AssetRepoManagerConfig) Validate() error {
	if err := c.NoCheckoutRepoManagerConfig.Validate(); err != nil {
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
	*noCheckoutRepoManager
	asset        string
	childBranch  string
	childRepo    *gitiles.Repo
	childRepoUrl string
	lastVersions map[string]string
}

// NewAssetRepoManager returns a RepoManager instance which rolls infra asset
// versions so that the "parent" repo's versions match the "child" repo's
// versions.
func NewAssetRepoManager(ctx context.Context, c *AssetRepoManagerConfig, workdir string, g gerrit.GerritInterface, serverURL, gitcookiesPath string, client *http.Client) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	rv := &assetRepoManager{
		asset:        c.Asset,
		childBranch:  c.ChildBranch,
		childRepo:    gitiles.NewRepo(c.ChildRepo, gitcookiesPath, client),
		childRepoUrl: c.ChildRepo,
	}
	ncrm, err := newNoCheckoutRepoManager(ctx, c.NoCheckoutRepoManagerConfig, workdir, g, serverURL, gitcookiesPath, client, rv.buildCommitMessage, rv.updateHelper)
	if err != nil {
		return nil, err
	}
	rv.noCheckoutRepoManager = ncrm
	return rv, nil
}

// See documentation for noCheckoutRepoManagerBuildCommitMessageFunc.
func (rm *assetRepoManager) buildCommitMessage(from, to, serverURL, cqExtraTrybots string, emails []string) (string, error) {
	rm.infoMtx.RLock()
	defer rm.infoMtx.RUnlock()

	// Build the commit message.
	data := struct {
		Asset          string
		CqExtraTrybots string
		Emails         string
		From           string
		ServerURL      string
		To             string
	}{
		Asset:          rm.asset,
		CqExtraTrybots: cqExtraTrybots,
		Emails:         strings.Join(emails, ","),
		From:           from,
		ServerURL:      serverURL,
		To:             to,
	}
	var buf bytes.Buffer
	if err := commitMsgTmplAssets.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Read the version of the given asset from the given repo at the given commit.
func readAssetVersion(repo *gitiles.Repo, commit, asset string) (string, error) {
	buf := bytes.NewBuffer([]byte{})
	if err := repo.ReadFileAtRef(fmt.Sprintf(ASSET_VERSION_TMPL, asset), commit, buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// See documentation for noCheckoutRepoManagerUpdateHelperFunc.
func (rm *assetRepoManager) updateHelper(ctx context.Context, strat strategy.NextRollStrategy, parentRepo *gitiles.Repo, baseCommit string) (string, string, int, map[string]string, error) {
	// Obtain the hash of the current child HEAD.
	childHead, err := rm.childRepo.GetCommit(rm.childBranch)
	if err != nil {
		return "", "", 0, nil, err
	}

	// Read the asset versions from both repos.
	lastRollRev, err := readAssetVersion(rm.parentRepo, baseCommit, rm.asset)
	if err != nil {
		return "", "", 0, nil, err
	}
	nextRollRev, err := readAssetVersion(rm.childRepo, childHead.Hash, rm.asset)
	if err != nil {
		return "", "", 0, nil, err
	}

	// Subtract the last-rolled version number from the next version number
	// to obtain commitsNotRolled.
	lastInt, err := strconv.Atoi(lastRollRev)
	if err != nil {
		return "", "", 0, nil, err
	}
	nextInt, err := strconv.Atoi(nextRollRev)
	if err != nil {
		return "", "", 0, nil, err
	}
	commitsNotRolled := nextInt - lastInt

	// Prepare the changes for the next roll.
	nextRollChanges := map[string]string{
		fmt.Sprintf(ASSET_VERSION_TMPL, rm.asset): nextRollRev,
	}
	return lastRollRev, nextRollRev, commitsNotRolled, nextRollChanges, nil
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
