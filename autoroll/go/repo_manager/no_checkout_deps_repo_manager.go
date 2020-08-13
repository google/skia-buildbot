package repo_manager

import (
	"context"
	"net/http"
	"regexp"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
)

var (
	getDepRegex = regexp.MustCompile("[a-f0-9]+")
)

// NoCheckoutDEPSRepoManagerConfig provides configuration for RepoManagers which
// don't use a local checkout.
type NoCheckoutDEPSRepoManagerConfig struct {
	Parent parent.GitilesConfig `json:"parent"`
	Child  child.GitilesConfig  `json:"child"`
}

// See documentation for util.Validator interface.
func (c *NoCheckoutDEPSRepoManagerConfig) Validate() error {
	if err := c.Parent.Validate(); err != nil {
		return err
	}
	if err := c.Parent.Validate(); err != nil {
		return err
	}
	return nil
}

// NewNoCheckoutDEPSRepoManager returns a RepoManager instance which does not
// use a local checkout.
func NewNoCheckoutDEPSRepoManager(ctx context.Context, c *NoCheckoutDEPSRepoManagerConfig, reg *config_vars.Registry, workdir string, g gerrit.GerritInterface, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	parentRM, err := parent.NewGitilesFile(ctx, c.Parent, reg, client, serverURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	childRM, err := child.NewGitiles(ctx, c.Child, reg, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM, nil)
}
