package repo_manager

import (
	"context"
	"fmt"

	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
)

type ParentChildRepoManagerConfig struct {
	parent parent.ParentConfig
	child  child.ChildConfig
}

func (c ParentChildRepoManagerConfig) Validate() error {
	if err := c.parent.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if err := c.child.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

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
		return nil, nil, nil, fmt.Errorf("Failed to update Parent: %s", err)
	}
	lastRollRev, err := rm.Child.GetRevision(ctx, lastRollRevId)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Failed to obtain last-rolled revision %q: %s", lastRollRevId, err)
	}
	tipRev, notRolledRevs, err := rm.Child.Update(ctx, lastRollRev)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Failed to get next revision to roll from Child: %s", err)
	}
	return lastRollRev, tipRev, notRolledRevs, nil
}

var _ RepoManager = &parentChildRepoManager{}
