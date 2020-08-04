package child

import (
	"context"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vfs"
)

// GitCheckoutConfig provides configuration for a Child which uses a local
// Git checkout.
type GitCheckoutConfig struct {
	git_common.GitCheckoutConfig
}

// GitCheckoutChild is an implementation of Child which uses a local Git
// checkout.
type GitCheckoutChild struct {
	*git_common.Checkout
}

// NewGitCheckout returns an implementation of Child which uses a local Git
// checkout.
func NewGitCheckout(ctx context.Context, c GitCheckoutConfig, reg *config_vars.Registry, workdir, userName, userEmail string, co *git.Checkout) (*GitCheckoutChild, error) {
	checkout, err := git_common.NewCheckout(ctx, c.GitCheckoutConfig, reg, workdir, userName, userEmail, co)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &GitCheckoutChild{Checkout: checkout}, nil
}

// See documentation for Child interface.
func (c *GitCheckoutChild) Update(ctx context.Context, lastRollRev *revision.Revision) (*revision.Revision, []*revision.Revision, error) {
	tipRev, _, err := c.Checkout.Update(ctx)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	notRolledRevs, err := c.LogFirstParent(ctx, lastRollRev, tipRev)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	return tipRev, notRolledRevs, nil
}

// VFS implements the Child interface.
func (c *GitCheckoutChild) VFS(ctx context.Context, rev *revision.Revision) (vfs.FS, error) {
	return c.Checkout.VFS(ctx, rev.Id)
}

// GitCheckoutChild implements Child.
var _ Child = &GitCheckoutChild{}
