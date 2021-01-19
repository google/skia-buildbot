package repo_manager

import (
	"context"
	"errors"

	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/child/revision_filter"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// ParentChildRepoManagerConfig provides configuration for a RepoManager which
// combines a Parent and Child.
type ParentChildRepoManagerConfig struct {
	// Exactly one of the following Parent types must be provided.
	CopyParent                  *parent.CopyConfig
	DEPSLocalGitHubParent       *parent.DEPSLocalGithubConfig
	DEPSLocalGerritParent       *parent.DEPSLocalConfig // TODO: Correct type?
	GitCheckoutGithubFileParent *parent.GitCheckoutGithubFileConfig
	GitilesParent               *parent.GitilesConfig

	// Exactly one of the following Child types must be provided.
	CIPDChild              *child.CIPDConfig
	FuchsiaSDKChild        *child.FuchsiaSDKConfig
	GitCheckoutChild       *child.GitCheckoutConfig
	GitCheckoutGitHubChild *child.GitCheckoutGithubConfig
	GitilesChild           *child.GitilesConfig
	SemVerGCSChild         *child.SemVerGCSConfig

	// Revision filters.
	BuildbucketRevisionFilter *revision_filter.BuildbucketRevisionFilterConfig `json:"buildbucketFilter,omitempty"`
}

// Validate implements util.Validator.
func (c *ParentChildRepoManagerConfig) Validate() error {
	return errors.New("NOT IMPLEMENTED") // TODO
}

// parentChildRepoManager combines a Parent and a Child to implement the
// RepoManager interface.
type parentChildRepoManager struct {
	child.Child
	parent.Parent
	revFilter revision_filter.RevisionFilter
}

// newParentChildRepoManager returns a RepoManager which pairs a Parent with a
// Child.
func newParentChildRepoManager(ctx context.Context, p parent.Parent, c child.Child, revFilter revision_filter.RevisionFilter) (*parentChildRepoManager, error) {
	return &parentChildRepoManager{
		Child:     c,
		Parent:    p,
		revFilter: revFilter,
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
	// Optionally filter not-rolled revisions.
	if rm.revFilter != nil {
		if err := revision_filter.MaybeSetInvalid(ctx, rm.revFilter, tipRev); err != nil {
			return nil, nil, nil, skerr.Wrap(err)
		}
		for _, notRolledRev := range notRolledRevs {
			if err := revision_filter.MaybeSetInvalid(ctx, rm.revFilter, notRolledRev); err != nil {
				return nil, nil, nil, skerr.Wrap(err)
			}
		}
	}
	return lastRollRev, tipRev, notRolledRevs, nil
}

// parentChildRepoManager implements RepoManager.
var _ RepoManager = &parentChildRepoManager{}
