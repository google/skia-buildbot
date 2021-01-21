package repo_manager

import (
	"context"
	"net/http"
	"path/filepath"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/github_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
)

// GithubDEPSRepoManagerConfig provides configuration for the Github RepoManager.
type GithubDEPSRepoManagerConfig struct {
	DepotToolsRepoManagerConfig
	GitHub      *codereview.GithubConfig `json:"github,omitempty"`
	ChildRepo   string                   `json:"childRepo"`
	ForkRepoURL string                   `json:"forkRepoURL"`
	// Optional config to use if parent path is different than
	// workdir + parent repo.
	GithubParentPath string `json:"githubParentPath,omitempty"`

	// TransitiveDeps is an optional set of dependencies shared by the Parent
	// and Child which are updated in the Parent to match the versions of the
	// Child.
	TransitiveDeps []*version_file_common.TransitiveDepConfig `json:"transitiveDeps,omitempty"`
}

// Validate implements util.Validator.
func (c *GithubDEPSRepoManagerConfig) Validate() error {
	_, _, err := c.splitParentChild()
	if err != nil {
		return skerr.Wrap(err)
	}
	if c.ForkRepoURL == "" {
		return skerr.Fmt("ForkRepoURL is required")
	}
	return nil
}

// splitParentChild splits the GithubDEPSRepoManagerConfig into a
// parent.DEPSLocalConfig and a child.GitCheckoutConfig.
// TODO(borenet): Update the config format to directly define the parent
// and child. We shouldn't need most of the New.*RepoManager functions.
func (c GithubDEPSRepoManagerConfig) splitParentChild() (parent.DEPSLocalGithubConfig, child.GitCheckoutConfig, error) {
	var childDeps []*version_file_common.VersionFileConfig
	if c.TransitiveDeps != nil {
		childDeps = make([]*version_file_common.VersionFileConfig, 0, len(c.TransitiveDeps))
		for _, dep := range c.TransitiveDeps {
			childDeps = append(childDeps, dep.Child)
		}
	}
	parentCfg := parent.DEPSLocalGithubConfig{
		DEPSLocalConfig: parent.DEPSLocalConfig{
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
					TransitiveDeps: c.TransitiveDeps,
				},
			},
			CheckoutPath:   c.GithubParentPath,
			ChildPath:      c.ChildPath,
			ChildSubdir:    c.ChildSubdir,
			GClientSpec:    c.GClientSpec,
			PreUploadSteps: c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.PreUploadSteps,
			RunHooks:       c.RunHooks,
		},
		GitHub:      c.GitHub,
		ForkRepoURL: c.ForkRepoURL,
	}
	if err := parentCfg.Validate(); err != nil {
		return parent.DEPSLocalGithubConfig{}, child.GitCheckoutConfig{}, skerr.Wrapf(err, "generated parent config is invalid")
	}
	childCfg := child.GitCheckoutConfig{
		GitCheckoutConfig: git_common.GitCheckoutConfig{
			Branch:       c.ChildBranch,
			RepoURL:      c.ChildRepo,
			RevLinkTmpl:  c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.ChildRevLinkTmpl,
			Dependencies: childDeps,
		},
	}
	if err := childCfg.Validate(); err != nil {
		return parent.DEPSLocalGithubConfig{}, child.GitCheckoutConfig{}, skerr.Wrapf(err, "generated child config is invalid")
	}
	return parentCfg, childCfg, nil
}

// GithubDEPSRepoManagerConfigToProto converts a GithubDEPSRepoManagerConfig to
// a config.ParentChildRepoManagerConfig.
func GithubDEPSRepoManagerConfigToProto(cfg *GithubDEPSRepoManagerConfig) (*config.ParentChildRepoManagerConfig, error) {
	parentCfg, childCfg, err := cfg.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_DepsLocalGithubParent{
			DepsLocalGithubParent: parent.DEPSLocalGithubConfigToProto(&parentCfg),
		},
		Child: &config.ParentChildRepoManagerConfig_GitCheckoutChild{
			GitCheckoutChild: child.GitCheckoutConfigToProto(&childCfg),
		},
	}, nil
}

// ProtoToGithubDEPSRepoManagerConfig converts a
// config.ParentChildRepoManagerConfig to a GithubDEPSRepoManagerConfig.
func ProtoToGithubDEPSRepoManagerConfig(cfg *config.ParentChildRepoManagerConfig) (*GithubDEPSRepoManagerConfig, error) {
	childCfg := cfg.GetGitCheckoutChild()
	childBranch, err := config_vars.NewTemplate(childCfg.GitCheckout.Branch)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	parentCfg := cfg.GetDepsLocalGithubParent()
	parentBranch, err := config_vars.NewTemplate(parentCfg.DepsLocal.GitCheckout.GitCheckout.Branch)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &GithubDEPSRepoManagerConfig{
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
		GitHub:           codereview.ProtoToGithubConfig(parentCfg.Github),
		ChildRepo:        childCfg.GitCheckout.RepoUrl,
		ForkRepoURL:      parentCfg.ForkRepoUrl,
		GithubParentPath: parentCfg.DepsLocal.CheckoutPath,
		TransitiveDeps:   version_file_common.ProtoToTransitiveDepConfigs(parentCfg.DepsLocal.GitCheckout.Dep.Transitive),
	}, nil
}

// NewGithubDEPSRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func NewGithubDEPSRepoManager(ctx context.Context, c *GithubDEPSRepoManagerConfig, reg *config_vars.Registry, workdir, rollerName string, githubClient *github.GitHub, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	parentCfg, childCfg, err := c.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// TODO(borenet): Move the following into parent.NewDEPSLocalGithub().
	uploadRoll := parent.GitCheckoutUploadGithubRollFunc(githubClient, cr.UserName(), rollerName, parentCfg.ForkRepoURL)
	parentRM, err := parent.NewDEPSLocal(ctx, parentCfg.DEPSLocalConfig, reg, client, serverURL, workdir, cr.UserName(), cr.UserEmail(), recipeCfgFile, uploadRoll)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := github_common.SetupGithub(ctx, parentRM.Checkout.Checkout, parentCfg.ForkRepoURL); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Find the path to the child repo.
	childPath := filepath.Join(workdir, parentCfg.ChildPath)
	childCheckout := &git.Checkout{GitDir: git.GitDir(childPath)}
	childRM, err := child.NewGitCheckout(ctx, childCfg, reg, workdir, cr.UserName(), cr.UserEmail(), childCheckout)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM, nil)
}
