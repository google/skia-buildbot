package repo_manager

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"path"
	"strings"

	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/sklog"
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
	NewAFDORepoManager func(context.Context, *AFDORepoManagerConfig, string, gerrit.GerritInterface, string, string, *http.Client) (RepoManager, error) = newAfdoRepoManager
)

// Shorten the AFDO version.
func afdoShortVersion(long string) string {
	return strings.TrimPrefix(strings.TrimSuffix(long, ".afdo.bz2"), "chromeos-chrome-amd64-")
}

// AFDORepoManagerConfig provides configuration for the AFDO RepoManager.
type AFDORepoManagerConfig struct {
	NoCheckoutRepoManagerConfig
}

// afdoRepoManager is a RepoManager which rolls Android AFDO profile version
// numbers into Chromium. Unlike other rollers, there is no child repo to sync;
// the version number is obtained from Google Cloud Storage.
type afdoRepoManager struct {
	*noCheckoutRepoManager
	afdoVersionFile string
	authClient      *http.Client
	versions        []string // Protected by infoMtx.
}

// Return an afdoRepoManager instance.
func newAfdoRepoManager(ctx context.Context, c *AFDORepoManagerConfig, workdir string, g gerrit.GerritInterface, serverURL, gitcookiesPath string, client *http.Client) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	rv := &afdoRepoManager{
		afdoVersionFile: AFDO_VERSION_FILE_PATH,
		authClient:      client,
	}
	ncrm, err := newNoCheckoutRepoManager(ctx, c.NoCheckoutRepoManagerConfig, workdir, g, serverURL, gitcookiesPath, client, rv.buildCommitMessage, rv.updateHelper)
	if err != nil {
		return nil, err
	}
	rv.noCheckoutRepoManager = ncrm
	return rv, nil
}

// See documentation for noCheckoutRepoManagerBuildCommitMessageFunc.
func (rm *afdoRepoManager) buildCommitMessage(from, to, serverURL, cqExtraTrybots string, emails []string) (string, error) {
	commitMsg := fmt.Sprintf(AFDO_COMMIT_MSG_TMPL, afdoShortVersion(from), afdoShortVersion(to), serverURL)
	if len(emails) > 0 {
		commitMsg += fmt.Sprintf("TBR=%s", strings.Join(emails, ","))
	}
	return commitMsg, nil
}

// See documentation for noCheckoutRepoManagerUpdateHelperFunc.
func (rm *afdoRepoManager) updateHelper(ctx context.Context, strat strategy.NextRollStrategy, parentRepo *gitiles.Repo, baseCommit string) (string, string, int, map[string]string, error) {
	// Read the version file to determine the last roll rev.
	buf := bytes.NewBuffer([]byte{})
	if err := parentRepo.ReadFileAtRef(rm.afdoVersionFile, baseCommit, buf); err != nil {
		return "", "", 0, nil, err
	}
	lastRollRev := strings.TrimSpace(buf.String())

	// Get the next roll rev, and the list of versions in between the last
	// and next rolls.
	nextRollRev, err := strat.GetNextRollRev(ctx, nil)
	if err != nil {
		return "", "", 0, nil, err
	}
	if nextRollRev == "" {
		nextRollRev = lastRollRev
	}

	versions := strat.(*strategy.AFDOStrategy).GetVersions()
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
	// This seems backwards, but the versions are in descending order.
	commitsNotRolled := lastIdx - nextIdx

	rm.infoMtx.Lock()
	defer rm.infoMtx.Unlock()
	rm.versions = versions
	return lastRollRev, nextRollRev, commitsNotRolled, map[string]string{AFDO_VERSION_FILE_PATH: nextRollRev}, nil
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
func (r *afdoRepoManager) CreateNextRollStrategy(ctx context.Context, s string) (strategy.NextRollStrategy, error) {
	return strategy.GetNextRollStrategy(ctx, s, r.childBranch, DEFAULT_REMOTE, "", []string{}, r.childRepo, r.authClient)
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
