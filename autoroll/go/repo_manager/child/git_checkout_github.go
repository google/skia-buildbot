package child

import (
	"context"
	"fmt"
	"regexp"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/proto"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
)

var (
	// githubPullRequestLinksRE finds Github pull request links in commit
	// messages.
	githubPullRequestLinksRE = regexp.MustCompile(`(?m) \((#[0-9]+)\)$`)
)

// GitCheckoutGithubChild is an implementation of Child which uses a local Git
// checkout of a Github repo.
type GitCheckoutGithubChild struct {
	*GitCheckoutChild
	repoName string
	userName string
}

// NewGitCheckoutGithub returns an implementation of Child which uses a local
// Git checkout of a Github repo.
func NewGitCheckoutGithub(ctx context.Context, c *proto.GitCheckoutGitHubChildConfig, reg *config_vars.Registry, workdir string, cr codereview.CodeReview, co *git.Checkout) (*GitCheckoutGithubChild, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	child, err := NewGitCheckout(ctx, c.GitCheckout, reg, workdir, cr, co)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &GitCheckoutGithubChild{
		GitCheckoutChild: child,
		repoName:         c.RepoName,
		userName:         c.RepoOwner,
	}, nil
}

// fixPullRequestLinks fixes pull request linkification in the commit details.
func (c *GitCheckoutGithubChild) fixPullRequestLinks(rev *revision.Revision) error {
	// Github autolinks PR numbers to be of the same repository in logStr. Fix this by
	// explicitly adding the child repo to the PR number.
	rev.Description = githubPullRequestLinksRE.ReplaceAllString(rev.Description, fmt.Sprintf(" (%s/%s$1)", c.userName, c.repoName))
	rev.Details = githubPullRequestLinksRE.ReplaceAllString(rev.Details, fmt.Sprintf(" (%s/%s$1)", c.userName, c.repoName))
	return nil
}

// GetRevision implements Child.
func (c *GitCheckoutGithubChild) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	rev, err := c.GitCheckoutChild.GetRevision(ctx, id)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := c.fixPullRequestLinks(rev); err != nil {
		return nil, skerr.Wrap(err)
	}
	return rev, nil
}

// Update implements Child.
func (c *GitCheckoutGithubChild) Update(ctx context.Context, lastRollRev *revision.Revision) (*revision.Revision, []*revision.Revision, error) {
	tipRev, notRolledRevs, err := c.GitCheckoutChild.Update(ctx, lastRollRev)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	if err := c.fixPullRequestLinks(tipRev); err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	for _, rev := range notRolledRevs {
		if err := c.fixPullRequestLinks(rev); err != nil {
			return nil, nil, skerr.Wrap(err)
		}
	}
	return tipRev, notRolledRevs, nil
}

// GitCheckoutGithubChild implements Child.
var _ Child = &GitCheckoutGithubChild{}
