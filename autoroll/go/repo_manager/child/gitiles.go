package child

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"go.skia.org/infra/autoroll/go/repo_manager/helpers"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
)

const (
	GitilesRevTmpl = "%s/+/%s"
)

// GitilesChildConfig provides configuration for gitilesChild.
type GitilesChildConfig struct {
	// Branch to roll.
	ChildBranch string `json:"childBranch"`

	// Repo URL of the Child.
	ChildRepo string `json:"childRepo"`

	// Optional; transitive dependencies to roll. This is a mapping of
	// dependencies of the child repo which are also dependencies of the
	// parent repo and should be rolled at the same time. Keys are paths
	// to transitive dependencies within the child repo (as specified in
	// DEPS), and values are paths to those dependencies within the parent
	// repo.
	TransitiveDeps map[string]string `json:"transitiveDeps"`
}

// See documentation for util.Validator interface.
func (c *GitilesChildConfig) Validate() error {
	if c.ChildBranch == "" {
		return errors.New("ChildBranch is required.")
	}
	if c.ChildRepo == "" {
		return errors.New("ChildRepo is required.")
	}
	return nil
}

// NewGitilesChild returns an implementation of Child which uses Gitiles rather
// than a local checkout.
func NewGitilesChild(ctx context.Context, c GitilesChildConfig, client *http.Client, gclient string) (*gitilesChild, error) {
	repo := gitiles.NewRepo(c.ChildRepo, client)
	return &gitilesChild{
		branch:         c.ChildBranch,
		gclient:        gclient,
		repo:           repo,
		transitiveDeps: c.TransitiveDeps,
	}, nil
}

// gitilesChild is an implementation of Child which uses Gitiles rather than a
// local checkout.
type gitilesChild struct {
	branch         string
	gclient        string
	repo           *gitiles.Repo
	transitiveDeps map[string]string
}

// See documentation for Child interface.
func (c *gitilesChild) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	details, err := c.repo.Details(ctx, id)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to retrieve revision %q", id)
	}
	return revision.FromLongCommit(fmt.Sprintf(GitilesRevTmpl, c.repo.URL, "%s"), details), nil
}

// See documentation for Child interface.
func (c *gitilesChild) Update(ctx context.Context, lastRollRev *revision.Revision) (*revision.Revision, []*revision.Revision, error) {
	tipRev, err := c.GetRevision(ctx, c.branch)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "Failed to retrieve tip rev")
	}
	notRolled, err := c.repo.LogLinear(ctx, lastRollRev.Id, tipRev.Id)
	if err != nil {
		return nil, nil, err
	}
	notRolledRevs := revision.FromLongCommits(fmt.Sprintf(GitilesRevTmpl, c.repo.URL, "%s"), notRolled)

	// Transitive deps.
	if len(c.transitiveDeps) > 0 {
		for _, rev := range append(notRolledRevs, tipRev, lastRollRev) {
			childDepsFile, childCleanup, err := helpers.GetDEPSFile(ctx, c.repo, c.gclient, rev.Id)
			if err != nil {
				return nil, nil, err
			}
			defer childCleanup()
			for childPath := range c.transitiveDeps {
				childRev, err := helpers.GetDep(ctx, c.repo, c.gclient, childDepsFile, childPath)
				if err != nil {
					return nil, nil, err
				}
				if rev.Dependencies == nil {
					rev.Dependencies = map[string]string{}
				}
				rev.Dependencies[childPath] = childRev
			}
		}
	}
	return tipRev, notRolledRevs, nil
}
