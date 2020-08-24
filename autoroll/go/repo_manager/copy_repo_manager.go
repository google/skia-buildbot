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
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
)

// CopyRepoManagerConfig provides configuration for the copy
// RepoManager.
type CopyRepoManagerConfig struct {
	NoCheckoutRepoManagerConfig
	Gerrit *codereview.GerritConfig `json:"gerrit,omitempty"`

	// ChildRepo is the URL of the child repo.
	ChildRepo string `json:"childRepo"`

	// Copies indicates which files and directories to copy from the
	// child repo into the parent repo. If not specified, the whole repo
	// is copied.
	Copies []parent.CopyEntry `json:"copies,omitempty"`

	// Path restricts the revisions which can be rolled to those which change
	// the given path.  Note that this may produce strange results if the git
	// history for the path is not linear.
	Path string `json:"path,omitempty"`

	// VersionFile is the path within the repo which contains the current
	// version of the Child.
	VersionFile string `json:"versionFile"`
}

// Validate implements the util.Validator interface.
func (c *CopyRepoManagerConfig) Validate() error {
	if _, err := c.splitParentChild(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// splitParentChild splits the CopyRepoManagerConfig into a parent.CopyConfig
// and a child.GitilesConfig.
// TODO(borenet): Update the config format to directly define the parent
// and child. We shouldn't need most of the New.*RepoManager functions.
func (c CopyRepoManagerConfig) splitParentChild() (*ParentChildConfig, error) {
	cfg := &ParentChildConfig{
		Parent: parent.Config{
			Copy: &parent.CopyConfig{
				GitilesConfig: parent.GitilesConfig{
					GitilesConfig: gitiles_common.GitilesConfig{
						Branch:  c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ParentBranch,
						RepoURL: c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ParentRepo,
					},
					DependencyConfig: version_file_common.DependencyConfig{
						VersionFileConfig: version_file_common.VersionFileConfig{
							ID:   c.ChildRepo,
							Path: c.VersionFile,
						},
					},
					Gerrit: c.Gerrit,
				},
				Copies: c.Copies,
			},
		},
		Child: child.Config{
			Gitiles: &child.GitilesConfig{
				GitilesConfig: gitiles_common.GitilesConfig{
					Branch:  c.ChildBranch,
					RepoURL: c.ChildRepo,
				},
				Path: c.Path,
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		return nil, skerr.Wrapf(err, "generated config is invalid")
	}
	return cfg, nil
}

// NewCopyRepoManager returns a RepoManager instance which rolls a dependency
// which is copied directly into a subdir of the parent repo.
func NewCopyRepoManager(ctx context.Context, c *CopyRepoManagerConfig, reg *config_vars.Registry, workdir, rollerName string, gerritClient *gerrit.Gerrit, githubClient *github.GitHub, recipeCfgFile, serverURL string, httpClient *http.Client, cr codereview.CodeReview, local bool) (*ParentChildRepoManager, error) {
	cfg, err := c.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildFromConfig(ctx, cfg, reg, httpClient, gerritClient, githubClient, serverURL, workdir, rollerName, cr.UserName(), cr.UserEmail(), recipeCfgFile)
}
