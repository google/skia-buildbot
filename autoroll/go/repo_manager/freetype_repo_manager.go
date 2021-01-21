package repo_manager

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
)

// FreeTypeRepoManagerConfig provides configuration for FreeTypeRepoManager.
type FreeTypeRepoManagerConfig struct {
	NoCheckoutDEPSRepoManagerConfig
}

// NoCheckout implements roller.RepoManagerConfig.
func (c *FreeTypeRepoManagerConfig) NoCheckout() bool {
	return false
}

// FreeTypeRepoManagerConfigToProto converts a FreeTypeRepoManagerConfig to a
// config.FreeTypeRepoManagerConfig.
func FreeTypeRepoManagerConfigToProto(cfg *FreeTypeRepoManagerConfig) *config.FreeTypeRepoManagerConfig {
	return &config.FreeTypeRepoManagerConfig{
		Parent: &config.FreeTypeParentConfig{
			Gitiles: &config.GitilesParentConfig{
				Gitiles: &config.GitilesConfig{
					Branch:  cfg.ParentBranch.RawTemplate(),
					RepoUrl: cfg.ParentRepo,
				},
				Dep: &config.DependencyConfig{
					Primary: &config.VersionFileConfig{
						Id:   cfg.ChildRepo,
						Path: deps_parser.DepsFileName,
					},
				},
				Gerrit: codereview.GerritConfigToProto(cfg.Gerrit),
			},
		},
		Child: &config.GitilesChildConfig{
			Gitiles: &config.GitilesConfig{
				Branch:  cfg.ChildBranch.RawTemplate(),
				RepoUrl: cfg.ChildRepo,
			},
		},
	}
}

// ProtoToFreeTypeRepoManagerConfig converts a config.FreeTypeRepoManagerConfig
// to a FreeTypeRepoManagerConfig.
func ProtoToFreeTypeRepoManagerConfig(cfg *config.FreeTypeRepoManagerConfig) (*FreeTypeRepoManagerConfig, error) {
	childBranch, err := config_vars.NewTemplate(cfg.Child.Gitiles.Branch)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	parentBranch, err := config_vars.NewTemplate(cfg.Parent.Gitiles.Gitiles.Branch)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &FreeTypeRepoManagerConfig{
		NoCheckoutDEPSRepoManagerConfig: NoCheckoutDEPSRepoManagerConfig{
			NoCheckoutRepoManagerConfig: NoCheckoutRepoManagerConfig{
				CommonRepoManagerConfig: CommonRepoManagerConfig{
					ChildBranch:      childBranch,
					ChildPath:        cfg.Child.Path,
					ParentBranch:     parentBranch,
					ParentRepo:       cfg.Parent.Gitiles.Gitiles.RepoUrl,
					ChildRevLinkTmpl: "", // TODO(borenet)
				},
			},
			Gerrit:    codereview.ProtoToGerritConfig(cfg.Parent.Gitiles.Gerrit),
			ChildRepo: cfg.Child.Gitiles.RepoUrl,
		},
	}, nil
}

// NewFreeTypeRepoManager returns a RepoManager instance which rolls FreeType
// in DEPS and updates header files and README.chromium.
func NewFreeTypeRepoManager(ctx context.Context, c *FreeTypeRepoManagerConfig, reg *config_vars.Registry, workdir string, g gerrit.GerritInterface, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	parentCfg, childCfg, err := c.NoCheckoutDEPSRepoManagerConfig.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	parentRM, err := parent.NewFreeTypeParent(ctx, parentCfg, reg, workdir, client, serverURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	childRM, err := child.NewGitiles(ctx, childCfg, reg, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM, nil)
}
