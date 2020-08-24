package repo_manager

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
)

// FreeTypeRepoManagerConfig provides configuration for FreeTypeRepoManager.
type FreeTypeRepoManagerConfig struct {
	NoCheckoutDEPSRepoManagerConfig
}

// NoCheckout implements the RepoManagerConfig interface.
func (c *FreeTypeRepoManagerConfig) NoCheckout() bool {
	return false
}

// NewFreeTypeRepoManager returns a RepoManager instance which rolls FreeType
// in DEPS and updates header files and README.chromium.
func NewFreeTypeRepoManager(ctx context.Context, c *FreeTypeRepoManagerConfig, reg *config_vars.Registry, workdir, rollerName string, gerritClient *gerrit.Gerrit, githubClient *github.GitHub, recipeCfgFile, serverURL string, httpClient *http.Client, cr codereview.CodeReview, local bool) (*ParentChildRepoManager, error) {
	cfg, err := c.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildFromConfig(ctx, cfg, reg, httpClient, gerritClient, githubClient, serverURL, workdir, rollerName, cr.UserName(), cr.UserEmail(), recipeCfgFile)
}
