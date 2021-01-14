package repo_manager

import (
	"context"
	"fmt"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
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
	fuchsiaSDKVersionFilePathLinux = "build/fuchsia/linux.sdk.sha1"
	fuchsiaSDKVersionFilePathMac   = "build/fuchsia/mac.sdk.sha1"
	fuchsiaSDKDepID                = "FuchsiaSDK"
)

// FuchsiaSDKRepoManagerConfig provides configuration for the Fuchia SDK
// RepoManager.
type FuchsiaSDKRepoManagerConfig struct {
	NoCheckoutRepoManagerConfig
	Gerrit        *codereview.GerritConfig `json:"gerrit,omitempty"`
	IncludeMacSDK bool                     `json:"includeMacSDK,omitempty"`
}

// ValidStrategies implements roller.RepoManagerConfig.
func (c *FuchsiaSDKRepoManagerConfig) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
	}
}

// splitParentChild breaks the FuchsiaSDKRepoManagerConfig into parent and child
// configs.
func (c *FuchsiaSDKRepoManagerConfig) splitParentChild() (parent.GitilesConfig, child.FuchsiaSDKConfig, error) {
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
	parentCfg := parent.GitilesConfig{
		DependencyConfig: version_file_common.DependencyConfig{
			VersionFileConfig: version_file_common.VersionFileConfig{
				ID:   fuchsiaSDKDepID,
				Path: fuchsiaSDKVersionFilePathLinux,
			},
			TransitiveDeps: transitiveDeps,
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

// FuchsiaSDKRepoManagerConfigToProto converts a FuchsiaSDKRepoManagerConfig to
// a config.ParentChildRepoManagerConfig.
func FuchsiaSDKRepoManagerConfigToProto(cfg *FuchsiaSDKRepoManagerConfig) *config.ParentChildRepoManagerConfig {
	parentCfg, childCfg, err := cfg.splitParentChild()
	if err != nil {
		panic(err) // TODO(borenet): Handle this.
	}
	return &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_GitilesParent{
			GitilesParent: parent.GitilesConfigToProto(&parentCfg),
		},
		Child: &config.ParentChildRepoManagerConfig_FuchsiaSdkChild{
			FuchsiaSdkChild: child.FuchsiaSDKConfigToProto(&childCfg),
		},
	}
}

// ProtoToFuchsiaSDKRepoManagerConfig converts a
// config.ParentChildRepoManagerConfig to a FuchsiaSDKRepoManagerConfig.
func ProtoToFuchsiaSDKRepoManagerConfig(parent *config.GitilesParentConfig, child *config.FuchsiaSDKChildConfig) *FuchsiaSDKRepoManagerConfig {
	childBranch, err := config_vars.NewTemplate("master")
	if err != nil {
		panic(err) // TODO(borenet): Handle this!
	}
	parentBranch, err := config_vars.NewTemplate(parent.Gitiles.Branch)
	if err != nil {
		panic(err) // TODO(borenet): Handle this!
	}
	return &FuchsiaSDKRepoManagerConfig{
		NoCheckoutRepoManagerConfig: NoCheckoutRepoManagerConfig{
			CommonRepoManagerConfig: CommonRepoManagerConfig{
				ChildBranch:  childBranch,
				ChildPath:    "placeholder", // TODO(borenet)
				ParentBranch: parentBranch,
				ParentRepo:   parent.Gitiles.RepoUrl,
			},
		},
		Gerrit:        codereview.ProtoToGerritConfig(parent.Gerrit),
		IncludeMacSDK: child.IncludeMacSdk,
	}
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
	return newParentChildRepoManager(ctx, parentRM, childRM, nil)
}
