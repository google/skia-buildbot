package repo_manager

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/child/revision_filter"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// ParentConfig provides configuration for  a

// ParentChildConfig provides configuration for a ParentChildRepoManager.
// NOTE: This currently only covers a subset of possible Parent and Child.
type ParentChildConfig struct {
	Parent parent.ConfigUnion `json:"parent"`
	Child  child.Config       `json:"child"`

	BuildbucketRevisionFilter *revision_filter.BuildbucketRevisionFilterConfig `json:"buildbucketFilter"`
}

// Validate implements the util.Validator interface.
func (c ParentChildConfig) Validate() error {
	if err := c.Parent.Validate(); err != nil {
		return skerr.Wrapf(err, "parent config is invalid")
	}
	if err := c.Child.Validate(); err != nil {
		return skerr.Wrapf(err, "child config is invalid")
	}
	return nil
}

// ParentChildRepoManager combines a Parent and a Child to implement the
// RepoManager interface.
type ParentChildRepoManager struct {
	child.Child
	parent.Parent
	revFilter revision_filter.RevisionFilter
}

// newParentChildFromConfig returns a RepoManager which pairs a Parent with a
// Child, based on the given ParentChildConfig.
func newParentChildFromConfig(ctx context.Context, c *ParentChildConfig, reg *config_vars.Registry, client *http.Client, gerritClient *gerrit.Gerrit, githubClient *github.GitHub, serverURL, workdir, rollerName, userName, userEmail, recipeCfgFile string) (*ParentChildRepoManager, error) {
	childRM, err := child.New(ctx, c.Child, reg, client, workdir, userName, userEmail)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	parentRM, err := parent.New(ctx, c.Parent, reg, client, gerritClient, githubClient, serverURL, workdir, rollerName, userName, userEmail, recipeCfgFile, childRM)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	var rf revision_filter.RevisionFilter
	if c.BuildbucketRevisionFilter != nil {
		rf, err = revision_filter.NewBuildbucketRevisionFilter(client, c.BuildbucketRevisionFilter.Project, c.BuildbucketRevisionFilter.Bucket)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	return newParentChildRepoManager(ctx, parentRM, childRM, rf)
}

// newParentChildRepoManager returns a RepoManager which pairs a Parent with a
// Child.
func newParentChildRepoManager(ctx context.Context, p parent.Parent, c child.Child, revFilter revision_filter.RevisionFilter) (*ParentChildRepoManager, error) {
	return &ParentChildRepoManager{
		Child:     c,
		Parent:    p,
		revFilter: revFilter,
	}, nil
}

// Update implements the RepoManager interface.
func (rm *ParentChildRepoManager) Update(ctx context.Context) (*revision.Revision, *revision.Revision, []*revision.Revision, error) {
	lastRollRevID, err := rm.Parent.Update(ctx)
	if err != nil {
		return nil, nil, nil, skerr.Wrapf(err, "failed to update Parent")
	}
	lastRollRev, err := rm.Child.GetRevision(ctx, lastRollRevID)
	if err != nil {
		sklog.Errorf("Last roll rev %q not found. This is acceptable for some rollers which allow outside versions to be rolled manually (eg. AFDO roller). A human should verify that this is indeed caused by a manual roll. Attempting to continue with no last-rolled revision. The revisions listed in the commit message will be incorrect!", lastRollRevID)
		lastRollRev = &revision.Revision{Id: lastRollRevID}
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
var _ RepoManager = &ParentChildRepoManager{}
