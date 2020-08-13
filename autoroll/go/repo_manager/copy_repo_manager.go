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

// CopyRepoManagerConfig provides configuration for the copy
// RepoManager.
type CopyRepoManagerConfig struct {
	Parent parent.CopyConfig   `json:"parent"`
	Child  child.GitilesConfig `json:"child"`
}

// Validate the config.
func (c *CopyRepoManagerConfig) Validate() error {
	if err := c.Parent.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if err := c.Child.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// NewCopyRepoManager returns a RepoManager instance which rolls a dependency
// which is copied directly into a subdir of the parent repo.
func NewCopyRepoManager(ctx context.Context, c *CopyRepoManagerConfig, reg *config_vars.Registry, workdir string, g *gerrit.Gerrit, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	childRM, err := child.NewGitiles(ctx, c.Child, reg, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	parentRM, err := parent.NewCopy(ctx, c.Parent, reg, client, serverURL, workdir, cr.UserName(), cr.UserEmail(), childRM)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return newParentChildRepoManager(ctx, parentRM, childRM, nil)
}
