package repo_manager

import (
	"context"
	"net/http"
	"path/filepath"

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

// DEPSRepoManagerConfig provides configuration for the DEPS RepoManager.
type DEPSRepoManagerConfig struct {
	DepotToolsRepoManagerConfig
	Gerrit     *codereview.GerritConfig `json:"gerrit,omitempty"`
	ChildRepo  string                   `json:"childRepo"`
	ParentPath string                   `json:"parentPath,omitempty"`
}

// Validate implements the util.Validator interface.
func (c DEPSRepoManagerConfig) Validate() error {
	if err := c.DepotToolsRepoManagerConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if _, err := c.splitParentChild(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// splitParentChild splits the DEPSRepoManagerConfig into a
// parent.DEPSLocalConfig and a child.GitCheckoutConfig.
// TODO(borenet): Update the config format to directly define the parent
// and child. We shouldn't need most of the New.*RepoManager functions.
func (c DEPSRepoManagerConfig) splitParentChild() (*ParentChildConfig, error) {
	childPath := c.ChildPath
	if c.ChildSubdir != "" {
		childPath = filepath.Join(c.ChildSubdir, c.ChildPath)
	}
	cfg := &ParentChildConfig{
		Parent: parent.ConfigUnion{
			GitilesDEPSLocal: &parent.DEPSLocalConfig{
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
			},
		},
		Child: child.Config{
			GitCheckout: &child.GitCheckoutConfig{
				GitCheckoutConfig: git_common.GitCheckoutConfig{
					Branch:      c.ChildBranch,
					RepoURL:     c.ChildRepo,
					RevLinkTmpl: c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.ChildRevLinkTmpl,
				},
				CheckoutPath: childPath,
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		return nil, skerr.Wrapf(err, "generated config is invalid")
	}
	return cfg, nil
}

// NewDEPSRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func NewDEPSRepoManager(ctx context.Context, c *DEPSRepoManagerConfig, reg *config_vars.Registry, workdir, rollerName string, gerritClient *gerrit.Gerrit, githubClient *github.GitHub, recipeCfgFile, serverURL string, httpClient *http.Client, cr codereview.CodeReview, local bool) (*ParentChildRepoManager, error) {
	cfg, err := c.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildFromConfig(ctx, cfg, reg, httpClient, gerritClient, githubClient, serverURL, workdir, rollerName, cr.UserName(), cr.UserEmail(), recipeCfgFile)
}
