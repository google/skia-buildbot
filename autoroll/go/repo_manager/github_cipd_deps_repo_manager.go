package repo_manager

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
)

// GithubCipdDEPSRepoManagerConfig provides configuration for the Github RepoManager.
type GithubCipdDEPSRepoManagerConfig struct {
	GithubDEPSRepoManagerConfig
	CipdAssetName string `json:"cipdAssetName"`
	CipdAssetTag  string `json:"cipdAssetTag"`
}

// ValidStrategies implements the RepoManagerConfig interface.
func (c *GithubCipdDEPSRepoManagerConfig) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
	}
}

// Validate implements the util.Validator interface.
func (c *GithubCipdDEPSRepoManagerConfig) Validate() error {
	_, err := c.splitParentChild()
	return skerr.Wrap(err)
}

// splitParentChild splits the GithubCipdDEPSRepoManagerConfig into a
// parent.DEPSLocalConfig and a child.GitCheckoutConfig.
// TODO(borenet): Update the config format to directly define the parent
// and child. We shouldn't need most of the New.*RepoManager functions.
func (c GithubCipdDEPSRepoManagerConfig) splitParentChild() (*ParentChildConfig, error) {
	cfg := &ParentChildConfig{
		Parent: parent.ConfigUnion{
			GithubDEPSLocal: &parent.GithubDEPSLocalConfig{
				DEPSLocalConfig: parent.DEPSLocalConfig{
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
				},
				ForkRepoURL: c.ForkRepoURL,
			},
		},
		Child: child.Config{
			CIPD: &child.CIPDConfig{
				Name: c.CipdAssetName,
				Tag:  c.CipdAssetTag,
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		return nil, skerr.Wrapf(err, "generated config is invalid")
	}
	return cfg, nil
}

// NewGithubCipdDEPSRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func NewGithubCipdDEPSRepoManager(ctx context.Context, c *GithubCipdDEPSRepoManagerConfig, reg *config_vars.Registry, workdir, rollerName string, gerritClient *gerrit.Gerrit, githubClient *github.GitHub, recipeCfgFile, serverURL string, httpClient *http.Client, cr codereview.CodeReview, local bool) (*ParentChildRepoManager, error) {
	cfg, err := c.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildFromConfig(ctx, cfg, reg, httpClient, gerritClient, githubClient, serverURL, workdir, rollerName, cr.UserName(), cr.UserEmail(), recipeCfgFile)
}
