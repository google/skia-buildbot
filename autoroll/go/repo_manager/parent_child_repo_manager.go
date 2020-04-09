package repo_manager

import (
	"context"

	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// parentChildRepoManager combines a Parent and a Child to implement the
// RepoManager interface.
type parentChildRepoManager struct {
	child.Child
	parent.Parent
}

// newParentChildRepoManager returns a RepoManager which pairs a Parent with a
// Child.
func newParentChildRepoManager(ctx context.Context, p parent.Parent, c child.Child) (*parentChildRepoManager, error) {
	return &parentChildRepoManager{
		Child:  c,
		Parent: p,
	}, nil
}

// See documentation for RepoManager interface.
func (rm *parentChildRepoManager) Update(ctx context.Context) (*revision.Revision, *revision.Revision, []*revision.Revision, error) {
	lastRollRevId, err := rm.Parent.Update(ctx)
	if err != nil {
		return nil, nil, nil, skerr.Wrapf(err, "failed to update Parent")
	}
	lastRollRev, err := rm.Child.GetRevision(ctx, lastRollRevId)
	if err != nil {
		sklog.Errorf("Last roll rev %q not found. This is acceptable for some rollers which allow outside versions to be rolled manually (eg. AFDO roller). A human should verify that this is indeed caused by a manual roll. Attempting to continue with no last-rolled revision. The revisions listed in the commit message will be incorrect!", lastRollRevId)
		lastRollRev = &revision.Revision{Id: lastRollRevId}
	}
	tipRev, notRolledRevs, err := rm.Child.Update(ctx, lastRollRev)
	if err != nil {
		return nil, nil, nil, skerr.Wrapf(err, "failed to get next revision to roll from Child")
	}
	return lastRollRev, tipRev, notRolledRevs, nil
}

// parentChildRepoManager implements RepoManager.
var _ RepoManager = &parentChildRepoManager{}
