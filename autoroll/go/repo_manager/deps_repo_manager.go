package repo_manager

import (
	"context"
	"net/http"
	"path/filepath"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gerrit_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
)

// issueJson is the structure of "git cl issue --json"
type issueJson struct {
	Issue    int64  `json:"issue"`
	IssueUrl string `json:"issue_url"`
}

// DEPSRepoManagerConfig provides configuration for the DEPS RepoManager.
type DEPSRepoManagerConfig struct {
	DepotToolsRepoManagerConfig
	Gerrit     *codereview.GerritConfig `json:"gerrit,omitempty"`
	ChildRepo  string                   `json:"childRepo"`
	ParentPath string                   `json:"parentPath,omitempty"`
}

// Validate implements util.Validator.
func (c *DEPSRepoManagerConfig) Validate() error {
	if err := c.DepotToolsRepoManagerConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if _, _, err := c.splitParentChild(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// splitParentChild splits the DEPSRepoManagerConfig into a
// parent.DEPSLocalConfig and a child.GitCheckoutConfig.
// TODO(borenet): Update the config format to directly define the parent
// and child. We shouldn't need most of the New.*RepoManager functions.
func (c DEPSRepoManagerConfig) splitParentChild() (parent.DEPSLocalConfig, child.GitCheckoutConfig, error) {
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
		ChildPath:      c.ChildPath,
		ChildSubdir:    c.ChildSubdir,
		GClientSpec:    c.GClientSpec,
		PreUploadSteps: c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.PreUploadSteps,
		RunHooks:       c.RunHooks,
	}
	if err := parentCfg.Validate(); err != nil {
		return parent.DEPSLocalConfig{}, child.GitCheckoutConfig{}, skerr.Wrapf(err, "generated parent config is invalid")
	}
	childCfg := child.GitCheckoutConfig{
		GitCheckoutConfig: git_common.GitCheckoutConfig{
			Branch:      c.ChildBranch,
			RepoURL:     c.ChildRepo,
			RevLinkTmpl: c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.ChildRevLinkTmpl,
		},
	}
	if err := childCfg.Validate(); err != nil {
		return parent.DEPSLocalConfig{}, child.GitCheckoutConfig{}, skerr.Wrapf(err, "generated child config is invalid")
	}
	return parentCfg, childCfg, nil
}

// DEPSRepoManagerConfigToProto converts a DEPSRepoManagerConfig to a
// config.ParentChildRepoManagerConfig.
func DEPSRepoManagerConfigToProto(cfg *DEPSRepoManagerConfig) (*config.ParentChildRepoManagerConfig, error) {
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
		Child: &config.ParentChildRepoManagerConfig_GitCheckoutChild{
			GitCheckoutChild: child.GitCheckoutConfigToProto(&childCfg),
		},
	}, nil
}

// ProtoToDEPSRepoManagerConfig converts a config.ParentChildRepoManagerConfig
// to a DEPSRepoManagerConfig.
func ProtoToDEPSRepoManagerConfig(cfg *config.ParentChildRepoManagerConfig) (*DEPSRepoManagerConfig, error) {
	childCfg := cfg.GetGitCheckoutChild()
	childBranch, err := config_vars.NewTemplate(childCfg.GitCheckout.Branch)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	parentCfg := cfg.GetDepsLocalGerritParent()
	parentBranch, err := config_vars.NewTemplate(parentCfg.DepsLocal.GitCheckout.GitCheckout.Branch)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &DEPSRepoManagerConfig{
		DepotToolsRepoManagerConfig: DepotToolsRepoManagerConfig{
			CommonRepoManagerConfig: CommonRepoManagerConfig{
				ChildBranch:      childBranch,
				ChildPath:        parentCfg.DepsLocal.ChildPath,
				ParentBranch:     parentBranch,
				ParentRepo:       parentCfg.DepsLocal.GitCheckout.GitCheckout.RepoUrl,
				ChildRevLinkTmpl: childCfg.GitCheckout.RevLinkTmpl,
				ChildSubdir:      parentCfg.DepsLocal.ChildSubdir,
				PreUploadSteps:   parent.ProtoToPreUploadSteps(parentCfg.DepsLocal.PreUploadSteps),
			},
			GClientSpec: parentCfg.DepsLocal.GclientSpec,
			RunHooks:    parentCfg.DepsLocal.RunHooks,
		},
		Gerrit:     codereview.ProtoToGerritConfig(parentCfg.Gerrit),
		ChildRepo:  childCfg.GitCheckout.RepoUrl,
		ParentPath: parentCfg.DepsLocal.CheckoutPath,
	}, nil
}

// NewDEPSRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func NewDEPSRepoManager(ctx context.Context, c *DEPSRepoManagerConfig, reg *config_vars.Registry, workdir string, g *gerrit.Gerrit, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
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
	childPath := c.ChildPath
	if c.ChildSubdir != "" {
		childPath = filepath.Join(c.ChildSubdir, c.ChildPath)
	}
	childFullPath := filepath.Join(workdir, childPath)
	childCheckout := &git.Checkout{GitDir: git.GitDir(childFullPath)}
	childRM, err := child.NewGitCheckout(ctx, childCfg, reg, workdir, cr.UserName(), cr.UserEmail(), childCheckout)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM, nil)
}
