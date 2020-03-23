package child

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
)

const (
	GitilesRevTmpl = "%s/+/%s"
)

// GitilesChildConfig provides configuration for gitilesChild.
type GitilesChildConfig struct {
	Branch  string `json:"branch"`
	RepoURL string `json:"repoURL"`
}

// See documentation for util.Validator interface.
func (c *GitilesChildConfig) Validate() error {
	if c.Branch == "" {
		return errors.New("Branch is required.")
	}
	if c.RepoURL == "" {
		return errors.New("RepoURL is required.")
	}
	return nil
}

// NewGitilesChild returns an implementation of Child which uses Gitiles rather
// than a local checkout.
func NewGitilesChild(ctx context.Context, c *GitilesChildConfig, client *http.Client) (Child, error) {
	repo := gitiles.NewRepo(c.RepoURL, client)
	return &gitilesChild{
		branch: c.Branch,
		repo:   repo,
	}, nil
}

// gitilesChild is an implementation of Child which uses Gitiles rather than a
// local checkout.
type gitilesChild struct {
	branch string
	repo   *gitiles.Repo
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
	return tipRev, notRolledRevs, nil
}
