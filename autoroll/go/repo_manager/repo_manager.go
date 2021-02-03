package repo_manager

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/proto"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
)

// RepoManager is the interface used by different Autoroller implementations
// to manage checkouts.
type RepoManager interface {
	// Create a new roll attempt.
	CreateNewRoll(ctx context.Context, rollingFrom *revision.Revision, rollingTo *revision.Revision, revisions []*revision.Revision, reviewers []string, dryRun bool, commitMsg string) (int64, error)

	// Update the RepoManager's view of the world. Depending on the
	// implementation, this may sync repos and may take some time. Returns
	// the currently-rolled Revision, the tip-of-tree Revision, and a list
	// of all revisions which have not yet been rolled (ie. those between
	// the current and tip-of-tree, including the latter), in reverse
	// chronological order.
	Update(context.Context) (*revision.Revision, *revision.Revision, []*revision.Revision, error)

	// GetRevision returns a revision.Revision instance from the given
	// revision ID.
	GetRevision(context.Context, string) (*revision.Revision, error)
}

// New returns a RepoManager instance based on the given RepoManagerConfig.
func New(ctx context.Context, c proto.RepoManagerConfig, reg *config_vars.Registry, workdir, rollerName, recipeCfgFile, serverURL, serviceAccount string, client *http.Client, cr codereview.CodeReview, isInternal bool, local bool) (RepoManager, error) {
	if c == nil {
		return nil, skerr.Fmt("No RepoManagerConfig was provided.")
	}
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	if rmc, ok := c.(*proto.AndroidRepoManagerConfig); ok {
		return NewAndroidRepoManager(ctx, rmc, reg, workdir, serverURL, serviceAccount, client, cr, isInternal, local)
	} else if rmc, ok := c.(*proto.CommandRepoManagerConfig); ok {
		return NewCommandRepoManager(ctx, rmc, reg, workdir, serverURL, cr)
	} else if rmc, ok := c.(*proto.FreeTypeRepoManagerConfig); ok {
		return NewFreeTypeRepoManager(ctx, rmc, reg, workdir, serverURL, client, cr, local)
	} else if rmc, ok := c.(*proto.FuchsiaSDKAndroidRepoManagerConfig); ok {
		return NewFuchsiaSDKAndroidRepoManager(ctx, rmc, reg, workdir, serverURL, client, cr, local)
	} else if rmc, ok := c.(*proto.ParentChildRepoManagerConfig); ok {
		return newParentChildRepoManager(ctx, rmc, reg, workdir, rollerName, recipeCfgFile, serverURL, client, cr)
	}
	return nil, skerr.Fmt("Unknown RepoManager type.")
}
