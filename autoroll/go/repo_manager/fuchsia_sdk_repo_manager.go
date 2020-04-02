package repo_manager

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/gitiles_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
)

const (
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

// FuchsiaSDKRepoManagerConfig provides configuration for the Fuchia SDK
// RepoManager.
type FuchsiaSDKRepoManagerConfig struct {
	NoCheckoutRepoManagerConfig
	Gerrit        *codereview.GerritConfig `json:"gerrit"`
	IncludeMacSDK bool                     `json:"includeMacSDK"`
}

// See documentation for RepoManagerConfig interface.
func (c *FuchsiaSDKRepoManagerConfig) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
	}
}

// splitParentChild breaks the FuchsiaSDKRepoManagerConfig into parent and child
// configs.
func (c *FuchsiaSDKRepoManagerConfig) splitParentChild() (parent.GitilesDEPSConfig, child.FuchsiaSDKConfig, error) {
	var parentDeps map[string]string
	if c.IncludeMacSDK {
		parentDeps = map[string]string{
			child.FUCHSIA_SDK_GS_LATEST_PATH_MAC: FUCHSIA_SDK_VERSION_FILE_PATH_MAC,
		}
	}
	commitMsgTmpl := TMPL_COMMIT_MSG_FUCHSIA_SDK
	if c.CommitMsgTmpl != "" {
		commitMsgTmpl = c.CommitMsgTmpl
	}
	parentCfg := parent.GitilesDEPSConfig{
		GitilesConfig: parent.GitilesConfig{
			BaseConfig: parent.BaseConfig{
				ChildPath:       c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ChildPath,
				ChildRepo:       "TODO",
				IncludeBugs:     c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.IncludeBugs,
				IncludeLog:      c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.IncludeLog,
				CommitMsgTmpl:   commitMsgTmpl,
				MonorailProject: c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.BugProject,
			},
			GitilesConfig: gitiles_common.GitilesConfig{
				Branch:  c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ParentBranch,
				RepoURL: c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ParentRepo,
			},
			Gerrit: c.Gerrit,
		},
		Dep:            "TODO",
		TransitiveDeps: parentDeps,
	}
	if err := parentCfg.Validate(); err != nil {
		return parent.GitilesDEPSConfig{}, child.FuchsiaSDKConfig{}, skerr.Wrapf(err, "generated parent config is invalid")
	}
	childCfg := child.FuchsiaSDKConfig{
		IncludeMacSDK: c.IncludeMacSDK,
	}
	if err := childCfg.Validate(); err != nil {
		return parent.GitilesDEPSConfig{}, child.FuchsiaSDKConfig{}, skerr.Wrapf(err, "generated child config is invalid")
	}
	return parentCfg, childCfg, nil
}

// NewFuchsiaSDKRepoManager returns a RepoManager instance which rolls the
// Fuchsia SDK.
func NewFuchsiaSDKRepoManager(ctx context.Context, c *FuchsiaSDKRepoManagerConfig, reg *config_vars.Registry, workdir string, g gerrit.GerritInterface, serverURL string, authClient *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("Failed to validate config: %s", err)
	}
	parentCfg, childCfg, err := c.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	parentRM, err := parent.NewGitilesDEPS(ctx, parentCfg, reg, client, serverURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	childRM, err := child.NewFuchsiaSDK(ctx, childCfg, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM)
}

// See documentation for noCheckoutRepoManagerCreateRollHelperFunc.
func (rm *fuchsiaSDKRepoManager) createRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, serverURL, cqExtraTrybots string, emails []string, baseCommit string) (string, map[string]string, error) {
	commitMsg, err := rm.buildCommitMsg(&parent.CommitMsgVars{
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
