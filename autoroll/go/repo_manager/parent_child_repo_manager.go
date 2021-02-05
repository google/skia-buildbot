package repo_manager

import (
	"context"
	"net/http"
	"path/filepath"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/child/revision_filter"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// parentChildRepoManager combines a Parent and a Child to implement the
// RepoManager interface.
type parentChildRepoManager struct {
	child.Child
	parent.Parent
	revFilter revision_filter.RevisionFilter
}

// newParentChildRepoManager returns a RepoManager which pairs a Parent with a
// Child.
func newParentChildRepoManager(ctx context.Context, c *config.ParentChildRepoManagerConfig, reg *config_vars.Registry, workdir, rollerName, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview) (*parentChildRepoManager, error) {
	var childRM child.Child
	var parentRM parent.Parent
	var err error

	// Some Child implementations require that they are created by the Parent,
	// so we have to create the Parent first.
	var childCheckout *git.Checkout
	if c.GetDepsLocalGerritParent() != nil {
		parentCfg := c.GetDepsLocalGerritParent()
		childPath := parentCfg.DepsLocal.ChildPath
		if parentCfg.DepsLocal.ChildSubdir != "" {
			childPath = filepath.Join(parentCfg.DepsLocal.ChildSubdir, childPath)
		}
		childFullPath := filepath.Join(workdir, childPath)
		childCheckout = &git.Checkout{GitDir: git.GitDir(childFullPath)}
		parentRM, err = parent.NewDEPSLocalGerrit(ctx, parentCfg, reg, client, serverURL, workdir, rollerName, recipeCfgFile, cr)
	} else if c.GetDepsLocalGithubParent() != nil {
		parentCfg := c.GetDepsLocalGithubParent()
		childPath := parentCfg.DepsLocal.ChildPath
		if parentCfg.DepsLocal.ChildSubdir != "" {
			childPath = filepath.Join(parentCfg.DepsLocal.ChildSubdir, childPath)
		}
		childFullPath := filepath.Join(workdir, childPath)
		childCheckout = &git.Checkout{GitDir: git.GitDir(childFullPath)}
		parentRM, err = parent.NewDEPSLocalGitHub(ctx, parentCfg, reg, client, serverURL, workdir, rollerName, recipeCfgFile, cr)
	} else if c.GetGitCheckoutGithubFileParent() != nil {
		parentRM, err = parent.NewGitCheckoutGithubFile(ctx, c.GetGitCheckoutGithubFileParent(), reg, client, serverURL, workdir, rollerName, cr)
	} else if c.GetGitilesParent() != nil {
		parentRM, err = parent.NewGitilesFile(ctx, c.GetGitilesParent(), reg, client, serverURL)
	}
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Create the Child.
	if c.GetCipdChild() != nil {
		childRM, err = child.NewCIPD(ctx, c.GetCipdChild(), client, workdir)
	} else if c.GetFuchsiaSdkChild() != nil {
		childRM, err = child.NewFuchsiaSDK(ctx, c.GetFuchsiaSdkChild(), client)
	} else if c.GetGitilesChild() != nil {
		childRM, err = child.NewGitiles(ctx, c.GetGitilesChild(), reg, client)
	} else if c.GetGitCheckoutChild() != nil {
		childRM, err = child.NewGitCheckout(ctx, c.GetGitCheckoutChild(), reg, workdir, cr, childCheckout)
	} else if c.GetGitCheckoutGithubChild() != nil {
		childRM, err = child.NewGitCheckoutGithub(ctx, c.GetGitCheckoutGithubChild(), reg, workdir, cr, childCheckout)
	} else if c.GetSemverGcsChild() != nil {
		childRM, err = child.NewSemVerGCS(ctx, c.GetSemverGcsChild(), reg, client)
	}
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Some Parent implementations require a Child to be passed in.
	if c.GetCopyParent() != nil {
		parentRM, err = parent.NewCopy(ctx, c.GetCopyParent(), reg, client, serverURL, workdir, cr.UserName(), cr.UserEmail(), childRM)
	}
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Revision filter.
	var revFilter revision_filter.RevisionFilter
	if c.GetBuildbucketRevisionFilter() != nil {
		rfConfig := c.GetBuildbucketRevisionFilter()
		revFilter, err = revision_filter.NewBuildbucketRevisionFilter(client, rfConfig.Project, rfConfig.Bucket)
	}
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// TODO(borenet): Fill this out.
	return &parentChildRepoManager{
		Child:     childRM,
		Parent:    parentRM,
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
