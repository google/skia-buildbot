package repo_manager

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gerrit_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
)

// DEPSGitilesRepoManagerConfig provides configuration for the DEPS RepoManager
// which does not use a locally-managed Child repo.
type DEPSGitilesRepoManagerConfig struct {
	DepotToolsRepoManagerConfig
	Gerrit     *codereview.GerritConfig `json:"gerrit,omitempty"`
	ChildRepo  string                   `json:"childRepo"`
	ParentPath string                   `json:"parentPath,omitempty"`
}

// Validate implements util.Validator.
func (c *DEPSGitilesRepoManagerConfig) Validate() error {
	// Set some unused variables on the embedded RepoManager.
	c.ChildPath = "N/A"
	c.ChildRevLinkTmpl = "N/A"
	if err := c.DepotToolsRepoManagerConfig.Validate(); err != nil {
		return err
	}
	// Unset the unused variables.
	c.ChildPath = ""
	c.ChildRevLinkTmpl = ""

	if _, _, err := c.splitParentChild(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// splitParentChild splits the DEPSGitilesRepoManagerConfig into a
// parent.DEPSLocalConfig and a child.GitilesConfig.
// TODO(borenet): Update the config format to directly define the parent
// and child. We shouldn't need most of the New.*RepoManager functions.
func (c DEPSGitilesRepoManagerConfig) splitParentChild() (parent.DEPSLocalConfig, child.GitilesConfig, error) {
	parentCfg := parent.DEPSLocalConfig{
		GitCheckoutConfig: parent.GitCheckoutConfig{
			GitCheckoutConfig: git_common.GitCheckoutConfig{
				Branch:  c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.ParentBranch,
				RepoURL: c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.ParentRepo,
			},
			DependencyConfig: version_file_common.DependencyConfig{
				VersionFileConfig: version_file_common.VersionFileConfig{
					ID:   c.ChildRepo,
					Path: deps_parser.DepsFileName,
				},
			},
		},
		CheckoutPath:   c.ParentPath,
		GClientSpec:    c.GClientSpec,
		PreUploadSteps: c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.PreUploadSteps,
		RunHooks:       c.RunHooks,
	}
	if err := parentCfg.Validate(); err != nil {
		return parent.DEPSLocalConfig{}, child.GitilesConfig{}, skerr.Wrapf(err, "generated parent config is invalid")
	}
	childCfg := child.GitilesConfig{
		GitilesConfig: gitiles_common.GitilesConfig{
			Branch:  c.ChildBranch,
			RepoURL: c.ChildRepo,
		},
	}
	if err := childCfg.Validate(); err != nil {
		return parent.DEPSLocalConfig{}, child.GitilesConfig{}, skerr.Wrapf(err, "generated child config is invalid")
	}
	return parentCfg, childCfg, nil
}

// DEPSGitilesRepoManagerConfigToProto converts a DEPSGitilesRepoManagerConfig
// to a config.ParentChildRepoManagerConfig.
func DEPSGitilesRepoManagerConfigToProto(cfg *DEPSGitilesRepoManagerConfig) (*config.ParentChildRepoManagerConfig, error) {
	parentCfg, childCfg, err := cfg.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_DepsLocalGerritParent{
			DepsLocalGerritParent: &config.DEPSLocalGerritParentConfig{
				DepsLocal: parent.DEPSLocalConfigToProto(&parentCfg),
				Gerrit:    codereview.GerritConfigToProto(cfg.Gerrit),
			},
		},
		Child: &config.ParentChildRepoManagerConfig_GitilesChild{
			GitilesChild: child.GitilesConfigToProto(&childCfg),
		},
	}, nil
}

// ProtoToDEPSGitilesRepoManagerConfig converts a
// config.ParentChildRepomanagerConfig to a DEPSGitilesRepoManagerConfig.
func ProtoToDEPSGitilesRepoManagerConfig(cfg *config.ParentChildRepoManagerConfig) (*DEPSGitilesRepoManagerConfig, error) {
	childCfg := cfg.GetGitilesChild()
	childBranch, err := config_vars.NewTemplate(childCfg.Gitiles.Branch)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	parentCfg := cfg.GetDepsLocalGerritParent()
	parentBranch, err := config_vars.NewTemplate(parentCfg.DepsLocal.GitCheckout.GitCheckout.Branch)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &DEPSGitilesRepoManagerConfig{
		DepotToolsRepoManagerConfig: DepotToolsRepoManagerConfig{
			CommonRepoManagerConfig: CommonRepoManagerConfig{
				ChildBranch:    childBranch,
				ChildPath:      parentCfg.DepsLocal.ChildPath,
				ChildSubdir:    parentCfg.DepsLocal.ChildSubdir,
				ParentBranch:   parentBranch,
				ParentRepo:     parentCfg.DepsLocal.GitCheckout.GitCheckout.RepoUrl,
				PreUploadSteps: parent.ProtoToPreUploadSteps(parentCfg.DepsLocal.PreUploadSteps),
			},
			GClientSpec: parentCfg.DepsLocal.GclientSpec,
			RunHooks:    parentCfg.DepsLocal.RunHooks,
		},
		Gerrit:     codereview.ProtoToGerritConfig(parentCfg.Gerrit),
		ChildRepo:  childCfg.Gitiles.RepoUrl,
		ParentPath: parentCfg.DepsLocal.CheckoutPath,
	}, nil
}

// NewDEPSGitilesRepoManager returns a RepoManager which uses a local DEPS
// checkout but whose Child is managed using Gitiles.
func NewDEPSGitilesRepoManager(ctx context.Context, c *DEPSGitilesRepoManagerConfig, reg *config_vars.Registry, workdir string, g *gerrit.Gerrit, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	parentCfg, childCfg, err := c.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	uploadRoll := parent.GitCheckoutUploadGerritRollFunc(g)
	parentRM, err := parent.NewDEPSLocal(ctx, parentCfg, reg, client, serverURL, workdir, cr.UserName(), cr.UserEmail(), recipeCfgFile, uploadRoll)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := gerrit_common.SetupGerrit(ctx, parentRM.Checkout.Checkout, g); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Find the path to the child repo.
	childRM, err := child.NewGitiles(ctx, childCfg, reg, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM, nil)
}
