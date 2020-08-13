package repo_manager

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
)

// GitilesCIPDDEPSRepoManagerConfig provides configuration for
// GitilesCIPDDEPSRepoManager.
type GitilesCIPDDEPSRepoManagerConfig struct {
	Parent parent.GitilesConfig `json:"parent"`
	Child  child.CIPDConfig     `json:"child"`
}

// See documentation for RepoManagerConfig interface.
func (r *GitilesCIPDDEPSRepoManagerConfig) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
	}
}

// See documentation for util.Validator interface.
func (c *GitilesCIPDDEPSRepoManagerConfig) Validate() error {
	if err := c.Parent.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if err := c.Child.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// NewGitilesCIPDDEPSRepoManager returns a RepoManager instance which does not use
// a local checkout.
func NewGitilesCIPDDEPSRepoManager(ctx context.Context, c *GitilesCIPDDEPSRepoManagerConfig, reg *config_vars.Registry, workdir string, g gerrit.GerritInterface, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	parentRM, err := parent.NewGitilesFile(ctx, c.Parent, reg, client, serverURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	childRM, err := child.NewCIPD(ctx, c.Child, client, workdir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM, nil)
}
