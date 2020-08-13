package repo_manager

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
)

// GithubCipdDEPSRepoManagerConfig provides configuration for the Github RepoManager.
type GithubCipdDEPSRepoManagerConfig struct {
	Parent      parent.DEPSLocalConfig `json:"parent"`
	Child       child.CIPDConfig       `json:"child"`
	ForkRepoURL string                 `json:"forkRepoURL"`
}

// See documentation for RepoManagerConfig interface.
func (c *GithubCipdDEPSRepoManagerConfig) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
	}
}

// See documentation for util.Validator interface.
func (c *GithubCipdDEPSRepoManagerConfig) Validate() error {
	if err := c.Parent.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if err := c.Child.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// NewGithubCipdDEPSRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func NewGithubCipdDEPSRepoManager(ctx context.Context, c *GithubCipdDEPSRepoManagerConfig, reg *config_vars.Registry, workdir, rollerName string, githubClient *github.GitHub, recipeCfgFile, serverURL string, httpClient *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	parentRM, err := parent.NewDEPSLocal(ctx, c.Parent, reg, httpClient, serverURL, workdir, cr.UserName(), cr.UserEmail(), recipeCfgFile)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := parentRM.SetGithub(ctx, parent.GithubConfig{
		ForkBranchName: rollerName,
		ForkRepoURL:    c.ForkRepoURL,
		UserName:       cr.UserName(),
	}, githubClient); err != nil {
		return nil, skerr.Wrap(err)
	}
	childRM, err := child.NewCIPD(ctx, c.Child, httpClient, workdir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM, nil)
}
