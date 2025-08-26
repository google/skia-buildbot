package deepvalidation

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/go/chrome_branch"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
)

// DeepValidate performs more in-depth validation of the config file,
// including checks requiring HTTP requests and possibly authentication to
// determine, for example, whether the configured repos, CIPD packages, etc,
// actually exist. This builds on the simple Validate() methods of the config
// structs.
func DeepValidate(ctx context.Context, client, githubHttpClient *http.Client, c *config.Config) error {
	cbc := chrome_branch.NewClient(client)
	reg, err := config_vars.NewRegistry(ctx, cbc)
	if err != nil {
		return skerr.Wrap(err)
	}
	dv := &deepvalidator{
		client:           client,
		reg:              reg,
		githubHttpClient: githubHttpClient,
	}
	return dv.deepValidate(ctx, c)
}

// deepvalidator is a helper for running deep validation which wraps up shared
// elements needed by most validation functions.
type deepvalidator struct {
	client           *http.Client
	reg              *config_vars.Registry
	githubHttpClient *http.Client
}

// deepValidate performs deep validation of the Config, making external
// network requests as needed.
func (dv *deepvalidator) deepValidate(ctx context.Context, c *config.Config) error {
	if c.GetGerrit() != nil {
		if err := dv.gerritConfig(ctx, c.GetGerrit()); err != nil {
			return skerr.Wrap(err)
		}
	}
	if c.GetGithub() != nil {
		if err := dv.gitHubConfig(ctx, c.GetGithub()); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// gerritConfig performs validation of the GerritConfig, making
// external network requests as needed.
func (dv *deepvalidator) gerritConfig(ctx context.Context, c *config.GerritConfig) error {
	cfg, ok := codereview.GerritConfigs[c.Config]
	if !ok {
		return skerr.Fmt("failed deep-validating GerritConfig: unknown config: %v", c.Config)
	}
	g, err := gerrit.NewGerritWithConfig(cfg, c.Url, dv.client)
	if err != nil {
		return skerr.Wrapf(err, "failed deep-validating GerritConfig")
	}
	results, err := g.Search(ctx, 1, false, gerrit.SearchProject(c.Project))
	if err != nil {
		return skerr.Wrapf(err, "failed deep-validating GerritConfig")
	}
	if len(results) == 0 {
		return skerr.Fmt("failed deep-validating GerritConfig: no changes found in project %q", c.Project)
	}
	return nil
}

// gitHubConfig performs validation of the GitHubConfig, making
// external network requests as needed.
func (dv *deepvalidator) gitHubConfig(ctx context.Context, c *config.GitHubConfig) error {
	gh, err := github.NewGitHub(ctx, c.RepoOwner, c.RepoName, dv.githubHttpClient)
	if err != nil {
		return skerr.Wrapf(err, "failed deep-validating GitHubConfig for %s/%s", c.RepoOwner, c.RepoName)
	}
	// Just perform an arbitrary read request which uses the configured owner
	// and repo name.
	if _, err := gh.ListOpenPullRequests(); err != nil {
		return skerr.Wrapf(err, "failed deep-validating GitHubConfig for %s/%s", c.RepoOwner, c.RepoName)
	}

	return nil
}
