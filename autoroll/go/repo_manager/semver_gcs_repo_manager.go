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

type SemVerGCSRepoManagerConfig struct {
	Parent parent.GitilesConfig  `json:"parent"`
	Child  child.SemVerGCSConfig `json:"child"`
}

func (c *SemVerGCSRepoManagerConfig) Validate() error {
	if err := c.Parent.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if err := c.Child.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// NewSemVerGCSRepoManager returns a gcsRepoManager which uses semantic
// versioning to compare object versions.
func NewSemVerGCSRepoManager(ctx context.Context, c *SemVerGCSRepoManagerConfig, reg *config_vars.Registry, workdir string, g gerrit.GerritInterface, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	parentRM, err := parent.NewGitilesFile(ctx, c.Parent, reg, client, serverURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	childRM, err := child.NewSemVerGCS(ctx, c.Child, reg, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM, nil)
}
