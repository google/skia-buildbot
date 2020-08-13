package repo_manager

import (
	"context"
	"net/http"
	"os"
	"path"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/child/revision_filter"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
)

// GithubRepoManagerConfig provides configuration for the Github RepoManager.
type GithubRepoManagerConfig struct {
	Parent parent.GitCheckoutGithubFileConfig `json:"parent"`
	Child  child.GitCheckoutGithubConfig      `json:"child"`
}

// See documentation for util.Validator interface.
func (c *GithubRepoManagerConfig) Validate() error {
	if err := c.Parent.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if err := c.Child.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// NewGithubRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func NewGithubRepoManager(ctx context.Context, c *GithubRepoManagerConfig, reg *config_vars.Registry, workdir, rollerName string, githubClient *github.GitHub, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	wd := path.Join(workdir, "github_repos")
	if _, err := os.Stat(wd); err != nil {
		if err := os.MkdirAll(wd, 0755); err != nil {
			return nil, skerr.Wrap(err)
		}
	}

	c.Parent.ForkBranchName = rollerName
	parentRM, err := parent.NewGitCheckoutGithubFile(ctx, c.Parent, reg, client, githubClient, serverURL, wd, cr.UserName(), cr.UserEmail(), rollerName, nil)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	childRM, err := child.NewGitCheckoutGithub(ctx, c.Child, reg, client, wd, cr.UserName(), cr.UserEmail(), nil)
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
