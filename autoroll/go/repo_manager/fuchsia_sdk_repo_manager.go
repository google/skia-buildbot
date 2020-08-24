package repo_manager

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
)

const (
	fuchsiaSDKVersionFilePathLinux = "build/fuchsia/linux.sdk.sha1"
	fuchsiaSDKVersionFilePathMac   = "build/fuchsia/mac.sdk.sha1"
)

// FuchsiaSDKRepoManagerConfig provides configuration for the Fuchia SDK
// RepoManager.
type FuchsiaSDKRepoManagerConfig struct {
	NoCheckoutRepoManagerConfig
	Gerrit        *codereview.GerritConfig `json:"gerrit,omitempty"`
	IncludeMacSDK bool                     `json:"includeMacSDK,omitempty"`
}

// ValidStrategies implements the RepoManagerConfig interface.
func (c *FuchsiaSDKRepoManagerConfig) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
	}
}

// splitParentChild breaks the FuchsiaSDKRepoManagerConfig into parent and child
// configs.
func (c *FuchsiaSDKRepoManagerConfig) splitParentChild() (*ParentChildConfig, error) {
	var transitiveDeps []*version_file_common.TransitiveDepConfig
	if c.IncludeMacSDK {
		transitiveDeps = []*version_file_common.TransitiveDepConfig{
			{
				Child: &version_file_common.VersionFileConfig{
					ID:   child.FuchsiaSDKGSLatestPathMac,
					Path: fuchsiaSDKVersionFilePathMac,
				},
				Parent: &version_file_common.VersionFileConfig{
					ID:   child.FuchsiaSDKGSLatestPathMac,
					Path: fuchsiaSDKVersionFilePathMac,
				},
			},
		}
	}
	cfg := &ParentChildConfig{
		Parent: parent.Config{
			GitilesFile: &parent.GitilesConfig{
				DependencyConfig: version_file_common.DependencyConfig{
					VersionFileConfig: version_file_common.VersionFileConfig{
						ID:   "FuchsiaSDK",
						Path: fuchsiaSDKVersionFilePathLinux,
					},
					TransitiveDeps: transitiveDeps,
				},
				GitilesConfig: gitiles_common.GitilesConfig{
					Branch:  c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ParentBranch,
					RepoURL: c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ParentRepo,
				},
				Gerrit: c.Gerrit,
			},
		},
		Child: child.Config{
			FuchsiaSDK: &child.FuchsiaSDKConfig{
				IncludeMacSDK: c.IncludeMacSDK,
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		return nil, skerr.Wrapf(err, "generated config is invalid")
	}
	return cfg, nil
}

// NewFuchsiaSDKRepoManager returns a RepoManager instance which rolls the
// Fuchsia SDK.
func NewFuchsiaSDKRepoManager(ctx context.Context, c *FuchsiaSDKRepoManagerConfig, reg *config_vars.Registry, workdir, rollerName string, gerritClient *gerrit.Gerrit, githubClient *github.GitHub, recipeCfgFile, serverURL string, httpClient *http.Client, cr codereview.CodeReview, local bool) (*ParentChildRepoManager, error) {
	cfg, err := c.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildFromConfig(ctx, cfg, reg, httpClient, gerritClient, githubClient, serverURL, workdir, rollerName, cr.UserName(), cr.UserEmail(), recipeCfgFile)
}
