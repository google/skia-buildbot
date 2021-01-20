package repo_manager

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/github_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
)

// GithubCipdDEPSRepoManagerConfig provides configuration for the Github RepoManager.
type GithubCipdDEPSRepoManagerConfig struct {
	GithubDEPSRepoManagerConfig
	CipdAssetName string `json:"cipdAssetName"`
	CipdAssetTag  string `json:"cipdAssetTag"`
}

// See documentation for RepoManagerConfig interface.
func (c *GithubCipdDEPSRepoManagerConfig) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
	}
}

// See documentation for util.Validator interface.
func (c *GithubCipdDEPSRepoManagerConfig) Validate() error {
	// Unset the unused variables.
	c.ChildBranch = nil
	c.ChildPath = ""
	c.ChildRevLinkTmpl = ""

	_, _, err := c.splitParentChild()
	return skerr.Wrap(err)
}

// splitParentChild splits the GithubCipdDEPSRepoManagerConfig into a
// parent.DEPSLocalConfig and a child.GitCheckoutConfig.
// TODO(borenet): Update the config format to directly define the parent
// and child. We shouldn't need most of the New.*RepoManager functions.
func (c GithubCipdDEPSRepoManagerConfig) splitParentChild() (parent.DEPSLocalConfig, child.CIPDConfig, error) {
	parentCfg := parent.DEPSLocalConfig{
		GitCheckoutConfig: parent.GitCheckoutConfig{
			GitCheckoutConfig: git_common.GitCheckoutConfig{
				Branch:  c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.ParentBranch,
				RepoURL: c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.ParentRepo,
			},
			DependencyConfig: version_file_common.DependencyConfig{
				VersionFileConfig: version_file_common.VersionFileConfig{
					ID:   c.CipdAssetName,
					Path: deps_parser.DepsFileName,
				},
			},
		},
		CheckoutPath:   c.GithubParentPath,
		GClientSpec:    c.GClientSpec,
		PreUploadSteps: c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.PreUploadSteps,
		RunHooks:       c.RunHooks,
	}
	if err := parentCfg.Validate(); err != nil {
		return parent.DEPSLocalConfig{}, child.CIPDConfig{}, skerr.Wrapf(err, "generated parent config is invalid")
	}
	childCfg := child.CIPDConfig{
		Name: c.CipdAssetName,
		Tag:  c.CipdAssetTag,
	}
	if err := childCfg.Validate(); err != nil {
		return parent.DEPSLocalConfig{}, child.CIPDConfig{}, skerr.Wrapf(err, "generated child.CIPDConfig is invalid")
	}
	return parentCfg, childCfg, nil
}

// NewGithubCipdDEPSRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func NewGithubCipdDEPSRepoManager(ctx context.Context, c *GithubCipdDEPSRepoManagerConfig, reg *config_vars.Registry, workdir, rollerName string, githubClient *github.GitHub, recipeCfgFile, serverURL string, httpClient *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	parentCfg, childCfg, err := c.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	uploadRoll := parent.GitCheckoutUploadGithubRollFunc(githubClient, cr.UserName(), rollerName, c.ForkRepoURL)
	parentRM, err := parent.NewDEPSLocal(ctx, parentCfg, reg, httpClient, serverURL, workdir, cr.UserName(), cr.UserEmail(), recipeCfgFile, uploadRoll)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := github_common.SetupGithub(ctx, parentRM.Checkout.Checkout, c.ForkRepoURL); err != nil {
		return nil, skerr.Wrap(err)
	}
	childRM, err := child.NewCIPD(ctx, childCfg, httpClient, workdir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM, nil)
}
