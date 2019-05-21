package repo_manager

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"google.golang.org/api/option"
)

const (
	FUCHSIA_SDK_GS_BUCKET            = "fuchsia"
	FUCHSIA_SDK_GS_PATH              = "sdk/core"
	FUCHSIA_SDK_GS_LATEST_PATH_LINUX = "sdk/core/linux-amd64/LATEST_ARCHIVE"
	FUCHSIA_SDK_GS_LATEST_PATH_MAC   = "sdk/core/mac-amd64/LATEST_ARCHIVE"

	FUCHSIA_SDK_VERSION_FILE_PATH_LINUX = "build/fuchsia/linux.sdk.sha1"
	FUCHSIA_SDK_VERSION_FILE_PATH_MAC   = "build/fuchsia/mac.sdk.sha1"

	FUCHSIA_SDK_COMMIT_MSG_TMPL = `Roll Fuchsia SDK from %s to %s

` + COMMIT_MSG_FOOTER_TMPL
)

var (
	NewFuchsiaSDKRepoManager func(context.Context, *FuchsiaSDKRepoManagerConfig, string, gerrit.GerritInterface, string, string, *http.Client, codereview.CodeReview, bool) (RepoManager, error) = newFuchsiaSDKRepoManager
)

// fuchsiaSDKVersion corresponds to one version of the Fuchsia SDK.
type fuchsiaSDKVersion struct {
	Timestamp time.Time
	Version   string
}

// Return true iff this fuchsiaSDKVersion is newer than the other.
func (a *fuchsiaSDKVersion) Greater(b *fuchsiaSDKVersion) bool {
	return a.Timestamp.After(b.Timestamp)
}

type fuchsiaSDKVersionSlice []*fuchsiaSDKVersion

func (s fuchsiaSDKVersionSlice) Len() int {
	return len(s)
}

func (s fuchsiaSDKVersionSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// We sort newest to oldest.
func (s fuchsiaSDKVersionSlice) Less(i, j int) bool {
	return s[i].Greater(s[j])
}

// Shorten the Fuchsia SDK version hash.
func fuchsiaSDKShortVersion(long string) string {
	return long[:12]
}

// FuchsiaSDKRepoManagerConfig provides configuration for the Fuchia SDK
// RepoManager.
type FuchsiaSDKRepoManagerConfig struct {
	NoCheckoutRepoManagerConfig
	IncludeMacSDK bool `json:"includeMacSDK"`
}

// fuchsiaSDKRepoManager is a RepoManager which rolls the Fuchsia SDK version
// into Chromium. Unlike other rollers, there is no child repo to sync; the
// version number is obtained from Google Cloud Storage.
type fuchsiaSDKRepoManager struct {
	*noCheckoutRepoManager
	gcsClient         gcs.GCSClient
	gsBucket          string
	gsLatestPathLinux string
	gsLatestPathMac   string
	gsListPath        string
	lastRollRevLinux  *fuchsiaSDKVersion // Protected by infoMtx.
	lastRollRevMac    string             // Protected by infoMtx.
	nextRollRevLinux  *fuchsiaSDKVersion // Protected by infoMtx.
	nextRollRevMac    string             // Protected by infoMtx.
	storageClient     *storage.Client
	versionFileLinux  string
	versionFileMac    string
	versions          []*fuchsiaSDKVersion // Protected by infoMtx.
}

// Return a fuchsiaSDKRepoManager instance.
func newFuchsiaSDKRepoManager(ctx context.Context, c *FuchsiaSDKRepoManagerConfig, workdir string, g gerrit.GerritInterface, serverURL, gitcookiesPath string, authClient *http.Client, cr codereview.CodeReview, local bool) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("Failed to validate config: %s", err)
	}
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(authClient))
	if err != nil {
		return nil, fmt.Errorf("Failed to create storage client: %s", err)
	}

	rv := &fuchsiaSDKRepoManager{
		gcsClient:         gcsclient.New(storageClient, FUCHSIA_SDK_GS_BUCKET),
		gsBucket:          FUCHSIA_SDK_GS_BUCKET,
		gsLatestPathLinux: FUCHSIA_SDK_GS_LATEST_PATH_LINUX,
		gsLatestPathMac:   FUCHSIA_SDK_GS_LATEST_PATH_MAC,
		gsListPath:        FUCHSIA_SDK_GS_PATH,
		storageClient:     storageClient,
		versionFileLinux:  FUCHSIA_SDK_VERSION_FILE_PATH_LINUX,
	}
	if c.IncludeMacSDK {
		rv.versionFileMac = FUCHSIA_SDK_VERSION_FILE_PATH_MAC
	}

	ncrm, err := newNoCheckoutRepoManager(ctx, c.NoCheckoutRepoManagerConfig, workdir, g, serverURL, gitcookiesPath, authClient, cr, rv.createRoll, rv.updateHelper, local)
	if err != nil {
		return nil, fmt.Errorf("Failed to create NoCheckoutRepoManager: %s", err)
	}
	rv.noCheckoutRepoManager = ncrm
	return rv, nil
}

// buildCommitMessage is a helper function which builds the commit message for
// a given roll.
func (rm *fuchsiaSDKRepoManager) buildCommitMessage(from, to, serverURL, cqExtraTrybots string, emails []string) string {
	commitMsg := fmt.Sprintf(FUCHSIA_SDK_COMMIT_MSG_TMPL, fuchsiaSDKShortVersion(from), fuchsiaSDKShortVersion(to), rm.serverURL)
	if cqExtraTrybots != "" {
		commitMsg += "\n" + fmt.Sprintf(TMPL_CQ_INCLUDE_TRYBOTS, cqExtraTrybots)
	}
	tbr := "\nTBR="
	if emails != nil && len(emails) > 0 {
		emailStr := strings.Join(emails, ",")
		tbr += emailStr
	}
	commitMsg += tbr
	return commitMsg
}

// See documentation for noCheckoutRepoManagerCreateRollHelperFunc.
func (rm *fuchsiaSDKRepoManager) createRoll(ctx context.Context, from, to, serverURL, cqExtraTrybots string, emails []string) (string, map[string]string, error) {
	commitMsg := rm.buildCommitMessage(from, to, serverURL, cqExtraTrybots, emails)

	// Create the roll changes.
	edits := map[string]string{
		rm.versionFileLinux: to,
	}
	// Hack: include the Mac version if required.
	if rm.versionFileMac != "" && rm.lastRollRevMac != rm.nextRollRevMac {
		edits[rm.versionFileMac] = rm.nextRollRevMac
	}
	return commitMsg, edits, nil
}

// See documentation for noCheckoutRepoManagerUpdateHelperFunc.
func (rm *fuchsiaSDKRepoManager) updateHelper(ctx context.Context, strat strategy.NextRollStrategy, parentRepo *gitiles.Repo, baseCommit string) (string, string, []*revision.Revision, error) {
	// Read the version file to determine the last roll rev.
	buf := bytes.NewBuffer([]byte{})
	if err := parentRepo.ReadFileAtRef(rm.versionFileLinux, baseCommit, buf); err != nil {
		return "", "", nil, fmt.Errorf("Failed to read %s at %s: %s", rm.versionFileLinux, baseCommit, err)
	}
	lastRollRevLinuxStr := strings.TrimSpace(buf.String())

	buf = bytes.NewBuffer([]byte{})
	if rm.versionFileMac != "" {
		if err := parentRepo.ReadFileAtRef(rm.versionFileMac, baseCommit, buf); err != nil {
			return "", "", nil, fmt.Errorf("Failed to read %s at %s: %s", rm.versionFileMac, baseCommit, err)
		}
	}
	lastRollRevMacStr := strings.TrimSpace(buf.String())

	// Get the available object hashes. Note that not all of these are SDKs,
	// so they don't necessarily represent versions we could feasibly roll.
	availableVersions := []*fuchsiaSDKVersion{}
	if err := rm.gcsClient.AllFilesInDirectory(ctx, rm.gsListPath, func(item *storage.ObjectAttrs) {
		vSplit := strings.Split(item.Name, "/")
		availableVersions = append(availableVersions, &fuchsiaSDKVersion{
			Timestamp: item.Updated,
			Version:   vSplit[len(vSplit)-1],
		})
	}); err != nil {
		return "", "", nil, fmt.Errorf("Failed to list available versions: %s", err)
	}
	if len(availableVersions) == 0 {
		return "", "", nil, fmt.Errorf("No matching items found.")
	}
	sort.Sort(fuchsiaSDKVersionSlice(availableVersions))

	// Get next SDK version.
	nextRollRevLinuxBytes, err := gcs.FileContentsFromGCS(rm.storageClient, rm.gsBucket, rm.gsLatestPathLinux)
	if err != nil {
		return "", "", nil, fmt.Errorf("Failed to read next SDK version (linux): %s", err)
	}
	nextRollRevLinuxStr := strings.TrimSpace(string(nextRollRevLinuxBytes))

	nextRollRevMacBytes, err := gcs.FileContentsFromGCS(rm.storageClient, rm.gsBucket, rm.gsLatestPathMac)
	if err != nil {
		return "", "", nil, fmt.Errorf("Failed to read next SDK version (mac): %s", err)
	}
	nextRollRevMacStr := strings.TrimSpace(string(nextRollRevMacBytes))

	// Find the last and next roll rev in the list of available versions.
	lastIdx := -1
	nextIdx := -1
	for idx, v := range availableVersions {
		if v.Version == lastRollRevLinuxStr {
			lastIdx = idx
		}
		if v.Version == nextRollRevLinuxStr {
			nextIdx = idx
		}
	}
	if lastIdx == -1 {
		return "", "", nil, fmt.Errorf("Last roll rev %q not found in available versions. Not-rolled count will be wrong.", lastRollRevLinuxStr)
	}
	if nextIdx == -1 {
		return "", "", nil, fmt.Errorf("Next roll rev %q not found in available versions. Not-rolled count will be wrong.", nextRollRevLinuxStr)
	}
	// Versions should be in reverse chronological order. We cannot compute
	// notRolledRevs correctly because there are things other than SDKs in
	// the GS dir, and because they are content-addressed, we can't tell
	// which ones are relevant to us.
	notRolledRevs := []*revision.Revision{}

	rm.infoMtx.Lock()
	defer rm.infoMtx.Unlock()
	rm.lastRollRevLinux = availableVersions[lastIdx]
	rm.lastRollRevMac = lastRollRevMacStr
	rm.nextRollRevLinux = availableVersions[nextIdx]
	rm.nextRollRevMac = nextRollRevMacStr

	rm.versions = availableVersions
	return lastRollRevLinuxStr, nextRollRevLinuxStr, notRolledRevs, nil
}

// See documentation for RepoManager interface.
func (rm *fuchsiaSDKRepoManager) FullChildHash(ctx context.Context, ver string) (string, error) {
	rm.infoMtx.RLock()
	defer rm.infoMtx.RUnlock()
	for _, v := range rm.versions {
		if strings.HasPrefix(v.Version, ver) {
			return v.Version, nil
		}
	}
	return "", fmt.Errorf("Unable to find version: %s", ver)
}

// See documentation for RepoManager interface.
func (rm *fuchsiaSDKRepoManager) RolledPast(ctx context.Context, ver string) (bool, error) {
	// TODO(borenet): Use a map?
	var testVer *fuchsiaSDKVersion
	for _, v := range rm.versions {
		if v.Version == ver {
			testVer = v
		}
	}
	if testVer == nil {
		return false, fmt.Errorf("Unknown version: %s", ver)
	}
	rm.infoMtx.RLock()
	defer rm.infoMtx.RUnlock()
	return !testVer.Greater(rm.lastRollRevLinux), nil
}

// See documentation for RepoManager interface.
func (r *fuchsiaSDKRepoManager) CreateNextRollStrategy(ctx context.Context, s string) (strategy.NextRollStrategy, error) {
	// fuchsiaSDKRepoManager implements its own strategy.
	return nil, nil
}

// See documentation for RepoManager interface.
func (r *fuchsiaSDKRepoManager) DefaultStrategy() string {
	return strategy.ROLL_STRATEGY_FUCHSIA_SDK
}

// See documentation for RepoManager interface.
func (r *fuchsiaSDKRepoManager) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_FUCHSIA_SDK,
	}
}
