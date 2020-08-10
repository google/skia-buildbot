package repo_manager

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
)

// FreeTypeRepoManagerConfig provides configuration for FreeTypeRepoManager.
type FreeTypeRepoManagerConfig struct {
	NoCheckoutDEPSRepoManagerConfig
}

// See documentation for RepoManagerConfig interface.
func (c *FreeTypeRepoManagerConfig) NoCheckout() bool {
	return false
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
