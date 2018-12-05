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
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"google.golang.org/api/option"
)

const (
	FUCHSIA_SDK_GS_BUCKET            = "fuchsia"
	FUCHSIA_SDK_GS_PATH              = "sdk"
	FUCHSIA_SDK_GS_LATEST_PATH_LINUX = "sdk/linux-amd64/LATEST_ARCHIVE"
	FUCHSIA_SDK_GS_LATEST_PATH_MAC   = "sdk/mac-amd64/LATEST_ARCHIVE"

	FUCHSIA_SDK_VERSION_FILE_PATH_LINUX = "build/fuchsia/linux.sdk.sha1"
	FUCHSIA_SDK_VERSION_FILE_PATH_MAC   = "build/fuchsia/mac.sdk.sha1"

	FUCHSIA_SDK_COMMIT_MSG_TMPL = `Roll Fuchsia SDK from %s to %s

` + COMMIT_MSG_FOOTER_TMPL
)

var (
	NewFuchsiaSDKRepoManager func(context.Context, *FuchsiaSDKRepoManagerConfig, string, gerrit.GerritInterface, string, string, *http.Client, bool) (RepoManager, error) = newFuchsiaSDKRepoManager
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
}

// fuchsiaSDKRepoManager is a RepoManager which rolls the Fuchsia SDK version
// into Chromium. Unlike other rollers, there is no child repo to sync; the
// version number is obtained from Google Cloud Storage.
type fuchsiaSDKRepoManager struct {
	*noCheckoutRepoManager
	gcsClient        gcs.GCSClient
	gsBucket         string
	lastRollRevLinux *fuchsiaSDKVersion // Protected by infoMtx.
	nextRollRevLinux *fuchsiaSDKVersion // Protected by infoMtx.
	nextRollRevMac   string             // Protected by infoMtx.
	storageClient    *storage.Client
	versionFileLinux string
	versionFileMac   string
	versions         []*fuchsiaSDKVersion // Protected by infoMtx.
}

// Return a fuchsiaSDKRepoManager instance.
func newFuchsiaSDKRepoManager(ctx context.Context, c *FuchsiaSDKRepoManagerConfig, workdir string, g gerrit.GerritInterface, serverURL, gitcookiesPath string, authClient *http.Client, local bool) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(authClient))
	if err != nil {
		return nil, err
	}

	rv := &fuchsiaSDKRepoManager{
		gcsClient:     gcs.NewGCSClient(storageClient, FUCHSIA_SDK_GS_BUCKET),
		gsBucket:      FUCHSIA_SDK_GS_BUCKET,
		storageClient: storageClient,
	}

	ncrm, err := newNoCheckoutRepoManager(ctx, c.NoCheckoutRepoManagerConfig, workdir, g, serverURL, gitcookiesPath, authClient, rv.buildCommitMessage, rv.updateHelper, local)
	if err != nil {
		return nil, err
	}
	rv.noCheckoutRepoManager = ncrm
	return rv, nil
}

// See documentation for noCheckoutRepoManagerBuildCommitMessageFunc.
func (rm *fuchsiaSDKRepoManager) buildCommitMessage(from, to, serverURL, cqExtraTrybots string, emails []string) (string, error) {
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
	return commitMsg, nil
}

// See documentation for noCheckoutRepoManagerUpdateHelperFunc.
func (rm *fuchsiaSDKRepoManager) updateHelper(ctx context.Context, strat strategy.NextRollStrategy, parentRepo *gitiles.Repo, baseCommit string) (string, string, int, map[string]string, error) {
	// Read the version file to determine the last roll rev.
	buf := bytes.NewBuffer([]byte{})
	if err := parentRepo.ReadFileAtRef(FUCHSIA_SDK_VERSION_FILE_PATH_LINUX, baseCommit, buf); err != nil {
		return "", "", 0, nil, err
	}
	lastRollRevLinuxStr := strings.TrimSpace(buf.String())

	// Get the available object hashes. Note that not all of these are SDKs,
	// so they don't necessarily represent versions we could feasibly roll.
	availableVersions := []*fuchsiaSDKVersion{}
	if err := rm.gcsClient.AllFilesInDirectory(ctx, FUCHSIA_SDK_GS_PATH, func(item *storage.ObjectAttrs) {
		vSplit := strings.Split(item.Name, "/")
		availableVersions = append(availableVersions, &fuchsiaSDKVersion{
			Timestamp: item.Updated,
			Version:   vSplit[len(vSplit)-1],
		})
	}); err != nil {
		return "", "", 0, nil, err
	}
	if len(availableVersions) == 0 {
		return "", "", 0, nil, fmt.Errorf("No matching items found.")
	}
	sort.Sort(fuchsiaSDKVersionSlice(availableVersions))

	// Get next SDK version.
	nextRollRevLinuxBytes, err := gcs.FileContentsFromGCS(rm.storageClient, rm.gsBucket, FUCHSIA_SDK_GS_LATEST_PATH_LINUX)
	if err != nil {
		return "", "", 0, nil, err
	}
	nextRollRevLinuxStr := strings.TrimSpace(string(nextRollRevLinuxBytes))

	nextRollRevMacBytes, err := gcs.FileContentsFromGCS(rm.storageClient, rm.gsBucket, FUCHSIA_SDK_GS_LATEST_PATH_MAC)
	if err != nil {
		return "", "", 0, nil, err
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
		return "", "", 0, nil, fmt.Errorf("Last roll rev %q not found in available versions. Not-rolled count will be wrong.", lastRollRevLinuxStr)
	}
	if nextIdx == -1 {
		return "", "", 0, nil, fmt.Errorf("Next roll rev %q not found in available versions. Not-rolled count will be wrong.", nextRollRevLinuxStr)
	}
	// Versions should be in reverse chronological order. Note that this
	// number is not correct because there are things other than SDKs in the
	// GS dir, and because they are content-addresed, we can't tell which
	// ones are relevant to us.
	commitsNotRolled := lastIdx - nextIdx

	rm.infoMtx.Lock()
	defer rm.infoMtx.Unlock()
	rm.lastRollRevLinux = availableVersions[lastIdx]
	rm.nextRollRevLinux = availableVersions[nextIdx]
	rm.nextRollRevMac = nextRollRevMacStr

	rm.versions = availableVersions
	return lastRollRevLinuxStr, nextRollRevLinuxStr, commitsNotRolled, map[string]string{
		FUCHSIA_SDK_VERSION_FILE_PATH_LINUX: nextRollRevLinuxStr,
		FUCHSIA_SDK_VERSION_FILE_PATH_MAC:   nextRollRevMacStr,
	}, nil
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
