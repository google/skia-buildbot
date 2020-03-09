package repo_manager

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
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
	FUCHSIA_SDK_GS_LATEST_PATH_LINUX = "development/LATEST_LINUX"
	FUCHSIA_SDK_GS_LATEST_PATH_MAC   = "development/LATEST_MAC"

	FUCHSIA_SDK_VERSION_FILE_PATH_LINUX = "build/fuchsia/linux.sdk.sha1"
	FUCHSIA_SDK_VERSION_FILE_PATH_MAC   = "build/fuchsia/mac.sdk.sha1"

	TMPL_COMMIT_MSG_FUCHSIA_SDK = `Roll Fuchsia SDK from {{.RollingFrom.String}} to {{.RollingTo.String}}

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
{{.ServerURL}}
Please CC {{stringsJoin .Reviewers ","}} on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md

{{if .CqExtraTrybots}}Cq-Include-Trybots: {{.CqExtraTrybots}}
{{end}}Tbr: {{stringsJoin .Reviewers ","}}
`
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

// FuchsiaSDKRepoManagerConfig provides configuration for the Fuchia SDK
// RepoManager.
type FuchsiaSDKRepoManagerConfig struct {
	NoCheckoutRepoManagerConfig
	IncludeMacSDK bool `json:"includeMacSDK"`
}

// See documentation for RepoManagerConfig interface.
func (r *FuchsiaSDKRepoManagerConfig) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
	}
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
	storageClient     *storage.Client
	versionFileLinux  string
	versionFileMac    string

	fuchsiaSDKInfoMtx sync.RWMutex
	lastRollRevMac    string // Protected by fuchsiaSDKInfoMtx.
	tipRevMac         string // Protected by fuchsiaSDKInfoMtx.
}

// Return a fuchsiaSDKRepoManager instance.
func NewFuchsiaSDKRepoManager(ctx context.Context, c *FuchsiaSDKRepoManagerConfig, reg *config_vars.Registry, workdir string, g gerrit.GerritInterface, serverURL string, authClient *http.Client, cr codereview.CodeReview, local bool) (RepoManager, error) {
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
		storageClient:     storageClient,
		versionFileLinux:  FUCHSIA_SDK_VERSION_FILE_PATH_LINUX,
	}
	if c.IncludeMacSDK {
		rv.versionFileMac = FUCHSIA_SDK_VERSION_FILE_PATH_MAC
	}
	if c.CommitMsgTmpl == "" {
		c.CommitMsgTmpl = TMPL_COMMIT_MSG_FUCHSIA_SDK
	}
	ncrm, err := newNoCheckoutRepoManager(ctx, c.NoCheckoutRepoManagerConfig, reg, workdir, g, serverURL, authClient, cr, rv.createRoll, rv.updateHelper, local)
	if err != nil {
		return nil, fmt.Errorf("Failed to create NoCheckoutRepoManager: %s", err)
	}
	rv.noCheckoutRepoManager = ncrm
	return rv, nil
}

// See documentation for noCheckoutRepoManagerCreateRollHelperFunc.
func (rm *fuchsiaSDKRepoManager) createRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, serverURL, cqExtraTrybots string, emails []string, baseCommit string) (string, map[string]string, error) {
	commitMsg, err := rm.buildCommitMsg(&CommitMsgVars{
		CqExtraTrybots: cqExtraTrybots,
		Reviewers:      emails,
		RollingFrom:    from,
		RollingTo:      to,
		ServerURL:      rm.serverURL,
	})
	if err != nil {
		return "", nil, err
	}

	// Create the roll changes.
	edits := map[string]string{
		rm.versionFileLinux: to.Id,
	}
	// Hack: include the Mac version if required.
	if rm.versionFileMac != "" && rm.lastRollRevMac != rm.tipRevMac {
		edits[rm.versionFileMac] = rm.tipRevMac
	}
	return commitMsg, edits, nil
}

func fuchsiaSDKVersionToRevision(ver string) *revision.Revision {
	return &revision.Revision{
		Id: ver,
	}
}

// See documentation for noCheckoutRepoManagerUpdateHelperFunc.
func (rm *fuchsiaSDKRepoManager) updateHelper(ctx context.Context, parentRepo *gitiles.Repo, baseCommit string) (*revision.Revision, *revision.Revision, []*revision.Revision, error) {
	// Read the version file to determine the last roll rev.
	buf := bytes.NewBuffer([]byte{})
	if err := parentRepo.ReadFileAtRef(ctx, rm.versionFileLinux, baseCommit, buf); err != nil {
		return nil, nil, nil, fmt.Errorf("Failed to read %s at %s: %s", rm.versionFileLinux, baseCommit, err)
	}
	lastRollRevLinuxStr := strings.TrimSpace(buf.String())

	buf = bytes.NewBuffer([]byte{})
	if rm.versionFileMac != "" {
		if err := parentRepo.ReadFileAtRef(ctx, rm.versionFileMac, baseCommit, buf); err != nil {
			return nil, nil, nil, fmt.Errorf("Failed to read %s at %s: %s", rm.versionFileMac, baseCommit, err)
		}
	}
	lastRollRevMacStr := strings.TrimSpace(buf.String())

	// Get latest SDK version.
	tipRevLinuxBytes, err := gcs.FileContentsFromGCS(rm.storageClient, rm.gsBucket, rm.gsLatestPathLinux)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Failed to read latest SDK version (linux): %s", err)
	}
	tipRevLinuxStr := strings.TrimSpace(string(tipRevLinuxBytes))

	tipRevMacBytes, err := gcs.FileContentsFromGCS(rm.storageClient, rm.gsBucket, rm.gsLatestPathMac)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Failed to read latest SDK version (mac): %s", err)
	}
	tipRevMacStr := strings.TrimSpace(string(tipRevMacBytes))

	// We cannot compute notRolledRevs correctly because there are things
	// other than SDKs in the GCS dir, and because they are content-
	// addressed, we can't tell which ones are relevant to us, so we only
	// include the latest and don't bother loading the list of versions
	// from GCS.
	notRolledRevs := []*revision.Revision{}
	if tipRevLinuxStr != lastRollRevLinuxStr {
		notRolledRevs = append(notRolledRevs, fuchsiaSDKVersionToRevision(tipRevLinuxStr))
	}

	rm.fuchsiaSDKInfoMtx.Lock()
	defer rm.fuchsiaSDKInfoMtx.Unlock()
	rm.lastRollRevMac = lastRollRevMacStr
	rm.tipRevMac = tipRevMacStr

	return fuchsiaSDKVersionToRevision(lastRollRevLinuxStr), fuchsiaSDKVersionToRevision(tipRevLinuxStr), notRolledRevs, nil
}

// See documentation for RepoManager interface.
func (r *fuchsiaSDKRepoManager) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	return fuchsiaSDKVersionToRevision(id), nil
}
