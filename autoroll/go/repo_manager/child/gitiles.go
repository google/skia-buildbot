package child

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/git"
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
	notRolledRevs, err := c.LogLinear(ctx, lastRollRev, tipRev)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "Failed to retrieve not-rolled revisions")
	}
	return tipRev, notRolledRevs, nil
}

// See documentation for Child interface.
func (c *gitilesChild) Download(ctx context.Context, rev *revision.Revision, dest string) error {
	// If the checkout does not already exist in dest, create it.
	gitDir := filepath.Join(dest, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		if err := git.Clone(ctx, c.URL, dest, false); err != nil {
			return skerr.Wrap(err)
		}
	}

	// Fetch and reset to the given revision.
	co := &git.Checkout{GitDir: git.GitDir(dest)}
	if err := co.Fetch(ctx); err != nil {
		return skerr.Wrap(err)
	}
	if _, err := co.Git(ctx, "reset", "--hard", rev.Id); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// gitilesChild implements Child.
var _ Child = &gitilesChild{}
