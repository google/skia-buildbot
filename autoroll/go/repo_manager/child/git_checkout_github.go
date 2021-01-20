package child

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
)

var (
	// githubPullRequestLinksRE finds Github pull request links in commit
	// messages.
	githubPullRequestLinksRE = regexp.MustCompile(`(?m) \((#[0-9]+)\)$`)
)

// GitCheckoutGithubConfig provides configuration for a Child which uses a local
// Git checkout of a Github repo.
type GitCheckoutGithubConfig struct {
	GitCheckoutConfig
	GithubRepoName string `json:"githubRepoName"`
	GithubUserName string `json:"githubUserName"`
}

// Validate implements util.Validator.
func (c GitCheckoutGithubConfig) Validate() error {
	if err := c.GitCheckoutConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.GithubRepoName == "" {
		return skerr.Fmt("GithubRepoName is required")
	}
	if c.GithubUserName == "" {
		return skerr.Fmt("GithubUserName is required")
	}
	return nil
}

// GitCheckoutGithubConfigToProto converts a GitCheckoutGithubConfig to a
// config.GitCheckoutGitHubChildConfig.
func GitCheckoutGithubConfigToProto(cfg *GitCheckoutGithubConfig) *config.GitCheckoutGitHubChildConfig {
	return &config.GitCheckoutGitHubChildConfig{
		GitCheckout: GitCheckoutConfigToProto(&cfg.GitCheckoutConfig),
		RepoOwner:   cfg.GithubUserName,
		RepoName:    cfg.GithubRepoName,
	}
}

// ProtoToGitCheckoutGithubConfig converts a config.GitCheckoutGitHubChildConfig
// to a GitCheckoutGithubChildConfig.
func ProtoToGitCheckoutGithubConfig(cfg *config.GitCheckoutGitHubChildConfig) (*GitCheckoutGithubConfig, error) {
	co, err := ProtoToGitCheckoutConfig(cfg.GitCheckout)
	if err != nil {
		return nil, err
	}
	return &GitCheckoutGithubConfig{
		GitCheckoutConfig: *co,
		GithubRepoName:    cfg.RepoName,
		GithubUserName:    cfg.RepoOwner,
	}, nil
}

// GitCheckoutGithubChild is an implementation of Child which uses a local Git
// checkout of a Github repo.
type GitCheckoutGithubChild struct {
	*GitCheckoutChild
	repoName string
	userName string
}

// NewGitCheckoutGithub returns an implementation of Child which uses a local
// Git checkout of a Github repo.
func NewGitCheckoutGithub(ctx context.Context, c GitCheckoutGithubConfig, reg *config_vars.Registry, client *http.Client, workdir, userName, userEmail string, co *git.Checkout) (*GitCheckoutGithubChild, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	child, err := NewGitCheckout(ctx, c.GitCheckoutConfig, reg, workdir, userName, userEmail, co)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &GitCheckoutGithubChild{
		GitCheckoutChild: child,
		repoName:         c.GithubRepoName,
		userName:         c.GithubUserName,
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
