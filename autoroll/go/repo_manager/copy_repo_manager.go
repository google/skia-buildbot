package repo_manager

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
)

// CopyRepoManagerConfig provides configuration for the copy
// RepoManager.
type CopyRepoManagerConfig struct {
	DepotToolsRepoManagerConfig
	Gerrit *codereview.GerritConfig `json:"gerrit"`

	// ChildRepo is the URL of the child repo.
	ChildRepo string `json:"childRepo"`

	// Copies indicates which files and directories to copy from the
	// child repo into the parent repo. If not specified, the whole repo
	// is copied.
	Copies []parent.CopyEntry `json:"copies,omitempty"`

	// VersionFile is the path within the repo which contains the current
	// version of the Child.
	VersionFile string `json:"versionFile"`
}

// Validate the config.
func (c *CopyRepoManagerConfig) Validate() error {
	if _, _, err := c.splitParentChild(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// splitParentChild splits the CopyRepoManagerConfig into a parent.CopyConfig
// and a child.GitilesConfig.
// TODO(borenet): Update the config format to directly define the parent
// and child. We shouldn't need most of the New.*RepoManager functions.
func (c CopyRepoManagerConfig) splitParentChild() (parent.CopyConfig, child.GitilesConfig, error) {
	parentCfg := parent.CopyConfig{
		GitCheckoutGerritConfig: parent.GitCheckoutGerritConfig{
			GitCheckoutConfig: parent.GitCheckoutConfig{
				GitCheckoutConfig: git_common.GitCheckoutConfig{
					Branch:  c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.ParentBranch,
					RepoURL: c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.ParentRepo,
				},
				DependencyConfig: version_file_common.DependencyConfig{
					VersionFileConfig: version_file_common.VersionFileConfig{
						ID:   c.ChildRepo,
						Path: c.VersionFile,
					},
				},
			},
			Gerrit: c.Gerrit,
		},
		Copies: c.Copies,
	}
	if err := parentCfg.Validate(); err != nil {
		return parent.CopyConfig{}, child.GitilesConfig{}, skerr.Wrapf(err, "generated parent config is invalid")
	}
	childCfg := child.GitilesConfig{
		GitilesConfig: gitiles_common.GitilesConfig{
			Branch:  c.ChildBranch,
			RepoURL: c.ChildRepo,
		},
	}
	if err := childCfg.Validate(); err != nil {
		return parent.CopyConfig{}, child.GitilesConfig{}, skerr.Wrapf(err, "generated child config is invalid")
	}
	return parentCfg, childCfg, nil
}

// NewCopyRepoManager returns a RepoManager instance which rolls a dependency
// which is copied directly into a subdir of the parent repo.
func NewCopyRepoManager(ctx context.Context, c *CopyRepoManagerConfig, reg *config_vars.Registry, workdir string, g *gerrit.Gerrit, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	parentCfg, childCfg, err := c.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	childRM, err := child.NewGitiles(ctx, childCfg, reg, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	uploadRoll := parent.GitCheckoutUploadGerritRollFunc(g)
	parentRM, err := parent.NewCopy(ctx, parentCfg, reg, client, serverURL, workdir, cr.UserName(), cr.UserEmail(), childRM, uploadRoll)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := parent.SetupGerrit(ctx, parentRM, g); err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM)
}
