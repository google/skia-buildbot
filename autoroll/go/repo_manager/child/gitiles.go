package child

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
)

// GitilesConfig provides configuration for gitilesChild.
type GitilesConfig struct {
	gitiles_common.GitilesConfig
}

// gitilesChild is an implementation of Child which uses Gitiles rather than a
// local checkout.
type gitilesChild struct {
	*gitiles_common.GitilesRepo
}

// NewGitiles returns an implementation of Child which uses Gitiles rather
// than a local checkout.
func NewGitiles(ctx context.Context, c GitilesConfig, reg *config_vars.Registry, client *http.Client) (Child, error) {
	g, err := gitiles_common.NewGitilesRepo(ctx, c.GitilesConfig, reg, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &gitilesChild{
		GitilesRepo: g,
	}, nil
}

// See documentation for Child interface.
func (c *gitilesChild) Update(ctx context.Context, lastRollRev *revision.Revision) (*revision.Revision, []*revision.Revision, error) {
	tipRev, err := c.GetTipRevision(ctx)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "Failed to retrieve tip rev")
	}
	notRolledRevs, err := c.LogFirstParent(ctx, lastRollRev, tipRev)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "Failed to retrieve not-rolled revisions")
	}
	return tipRev, notRolledRevs, nil
}

// See documentation for Child interface.
func (c *gitilesChild) Download(ctx context.Context, rev *revision.Revision, dest string) error {
	return git_common.Clone(ctx, c.URL, dest, rev)
}

// gitilesChild implements Child.
var _ Child = &gitilesChild{}
