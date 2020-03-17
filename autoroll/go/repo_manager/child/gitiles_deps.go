package child

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/repo_manager/helpers"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
)

// gitilesDepsChild is an implementation of Child which extends gitilesChild to
// record all of the DEPS entries for each revision.
type gitilesDepsChild struct {
	gitilesChild *gitilesChild
}

// NewGitilesDepsChild returns an implementation of Child which extends
// gitilesChild to record all of the DEPS entries for each revision.
func NewGitilesDepsChild(ctx context.Context, c GitilesChildConfig, client *http.Client) (*gitilesDepsChild, error) {
	gc, err := NewGitilesChild(ctx, c, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &gitilesDepsChild{
		gitilesChild: gc,
	}, nil
}

// addDeps retrieves the DEPS file for the given Revision, reads the entries,
// and adds them to the Dependencies of the Revision.
func (c *gitilesDepsChild) addDeps(ctx context.Context, rev *revision.Revision) error {
	depsFile, cleanup, err := helpers.GetDEPSFile(ctx, c.gitilesChild.repo, rev.Id)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer cleanup()
	deps, err := helpers.RevInfo(ctx, depsFile)
	if err != nil {
		return skerr.Wrap(err)
	}
	rev.Dependencies = make(map[string]string, len(deps))
	for _, dep := range deps {
		rev.Dependencies[dep.Id] = dep.Version
	}
	return nil
}

// See documentation for Child interface.
func (c *gitilesDepsChild) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	rev, err := c.gitilesChild.GetRevision(ctx, id)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := c.addDeps(ctx, rev); err != nil {
		return nil, skerr.Wrap(err)
	}
	return rev, nil
}

// See documentation for Child interface.
func (c *gitilesDepsChild) Update(ctx context.Context, lastRollRev *revision.Revision) (*revision.Revision, []*revision.Revision, error) {
	tipRev, notRolledRevs, err := c.gitilesChild.Update(ctx, lastRollRev)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	// TODO(borenet): Is tipRev never not in notRolledRevs?
	needTipRev := true
	for _, rev := range notRolledRevs {
		if err := c.addDeps(ctx, rev); err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		if rev.Id == tipRev.Id {
			tipRev.Dependencies = rev.Dependencies
			needTipRev = false
		}
	}
	if needTipRev {
		if err := c.addDeps(ctx, tipRev); err != nil {
			return nil, nil, skerr.Wrap(err)
		}
	}
	return tipRev, notRolledRevs, nil
}
