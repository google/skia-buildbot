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
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
)

// GithubDEPSRepoManagerConfig provides configuration for the Github RepoManager.
type GithubDEPSRepoManagerConfig struct {
	DepotToolsRepoManagerConfig
	ChildRepo   string `json:"childRepo"`
	ForkRepoURL string `json:"forkRepoURL"`
	// Optional config to use if parent path is different than
	// workdir + parent repo.
	GithubParentPath string `json:"githubParentPath,omitempty"`

	// TransitiveDeps is an optional set of dependencies shared by the Parent
	// and Child which are updated in the Parent to match the versions of the
	// Child.
	TransitiveDeps []*version_file_common.TransitiveDepConfig `json:"transitiveDeps,omitempty"`
}

// Validate the config.
func (c *GithubDEPSRepoManagerConfig) Validate() error {
	_, err := c.splitParentChild()
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
func (c GithubDEPSRepoManagerConfig) splitParentChild() (*ParentChildConfig, error) {
	var childDeps []*version_file_common.VersionFileConfig
	if c.TransitiveDeps != nil {
		childDeps = make([]*version_file_common.VersionFileConfig, 0, len(c.TransitiveDeps))
		for _, dep := range c.TransitiveDeps {
			childDeps = append(childDeps, dep.Child)
		}
	}
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
								ID:   c.ChildRepo,
								Path: deps_parser.DepsFileName,
							},
							TransitiveDeps: c.TransitiveDeps,
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
			GitCheckout: &child.GitCheckoutConfig{
				GitCheckoutConfig: git_common.GitCheckoutConfig{
					Branch:       c.ChildBranch,
					RepoURL:      c.ChildRepo,
					RevLinkTmpl:  c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.ChildRevLinkTmpl,
					Dependencies: childDeps,
				},
				CheckoutPath: c.ChildPath,
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		return nil, skerr.Wrapf(err, "generated config is invalid")
	}
	return cfg, nil
}

// NewGithubDEPSRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func NewGithubDEPSRepoManager(ctx context.Context, c *GithubDEPSRepoManagerConfig, reg *config_vars.Registry, workdir, rollerName string, gerritClient *gerrit.Gerrit, githubClient *github.GitHub, recipeCfgFile, serverURL string, httpClient *http.Client, cr codereview.CodeReview, local bool) (*ParentChildRepoManager, error) {
	cfg, err := c.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildFromConfig(ctx, cfg, reg, httpClient, gerritClient, githubClient, serverURL, workdir, rollerName, cr.UserName(), cr.UserEmail(), recipeCfgFile)
}
