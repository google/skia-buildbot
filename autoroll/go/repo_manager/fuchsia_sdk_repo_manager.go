package repo_manager

import (
	"context"
	"fmt"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
)

const (
	FuchsiaSDKVersionFilePathLinux = "build/fuchsia/linux.sdk.sha1"
	FuchsiaSDKVersionFilePathMac   = "build/fuchsia/mac.sdk.sha1"

	TmplCommitMsgFuchsiaSDK = `Roll Fuchsia SDK from {{.RollingFrom.String}} to {{.RollingTo.String}}

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
{{.ServerURL}}
Please CC {{stringsJoin .Reviewers ","}} on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+doc/master/autoroll/README.md

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
func (c *FuchsiaSDKRepoManagerConfig) splitParentChild() (parent.GitilesConfig, child.FuchsiaSDKConfig, error) {
	var parentDeps []*version_file_common.VersionFileConfig
	if c.IncludeMacSDK {
		parentDeps = []*version_file_common.VersionFileConfig{
			{
				ID:   child.FuchsiaSDKGSLatestPathMac,
				Path: FuchsiaSDKVersionFilePathMac,
			},
		}
	}
	commitMsgTmpl := TmplCommitMsgFuchsiaSDK
	if c.CommitMsgTmpl != "" {
		commitMsgTmpl = c.CommitMsgTmpl
	}
	parentCfg := parent.GitilesConfig{
		BaseConfig: parent.BaseConfig{
			ChildPath:       c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ChildPath,
			ChildRepo:       "FuchsiaSDK",
			IncludeBugs:     c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.IncludeBugs,
			IncludeLog:      c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.IncludeLog,
			CommitMsgTmpl:   commitMsgTmpl,
			MonorailProject: c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.BugProject,
		},
		DependencyConfig: version_file_common.DependencyConfig{
			VersionFileConfig: version_file_common.VersionFileConfig{
				ID:   "FuchsiaSDK",
				Path: FuchsiaSDKVersionFilePathLinux,
			},
			TransitiveDeps: parentDeps,
		},
		GitilesConfig: gitiles_common.GitilesConfig{
			Branch:  c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ParentBranch,
			RepoURL: c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ParentRepo,
		},
		Gerrit: c.Gerrit,
	}
	if err := parentCfg.Validate(); err != nil {
		return parent.GitilesConfig{}, child.FuchsiaSDKConfig{}, skerr.Wrapf(err, "generated parent config is invalid")
	}
	childCfg := child.FuchsiaSDKConfig{
		IncludeMacSDK: c.IncludeMacSDK,
	}
	if err := childCfg.Validate(); err != nil {
		return parent.GitilesConfig{}, child.FuchsiaSDKConfig{}, skerr.Wrapf(err, "generated child config is invalid")
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
	parentRM, err := parent.NewGitilesFile(ctx, parentCfg, reg, authClient, serverURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	childRM, err := child.NewFuchsiaSDK(ctx, childCfg, authClient)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM)
}
