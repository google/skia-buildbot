package repo_manager

import (
	"context"
	"net/http"
	"path/filepath"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
)

// GithubDEPSRepoManagerConfig provides configuration for the Github RepoManager.
type GithubDEPSRepoManagerConfig struct {
	Parent      parent.DEPSLocalConfig  `json:"parent"`
	Child       child.GitCheckoutConfig `json:"child"`
	ForkRepoURL string                  `json:"forkRepoURL"`

	// Path of the child repo within the checkout root.
	ChildPath string `json:"childPath"`

	// ChildSubdir indicates the subdirectory of the workdir in which
	// the ChildPath should be rooted. In most cases, this should be empty,
	// but if ChildPath is relative to the parent repo dir (eg. when DEPS
	// specifies use_relative_paths), then this is required.
	ChildSubdir string `json:"childSubdir,omitempty"`
}

// Validate the config.
func (c *GithubDEPSRepoManagerConfig) Validate() error {
	if err := c.Parent.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if err := c.Child.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// NewGithubDEPSRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func NewGithubDEPSRepoManager(ctx context.Context, c *GithubDEPSRepoManagerConfig, reg *config_vars.Registry, workdir, rollerName string, githubClient *github.GitHub, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	parentRM, err := parent.NewDEPSLocal(ctx, c.Parent, reg, client, serverURL, workdir, cr.UserName(), cr.UserEmail(), recipeCfgFile)
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

	// Find the path to the child repo.
	childPath := c.ChildPath
	if c.ChildSubdir != "" {
		childPath = filepath.Join(c.ChildSubdir, c.ChildPath)
	}
	childFullPath := filepath.Join(workdir, childPath)
	childCheckout := &git.Checkout{GitDir: git.GitDir(childFullPath)}
	childRM, err := child.NewGitCheckout(ctx, c.Child, reg, workdir, cr.UserName(), cr.UserEmail(), childCheckout)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM, nil)
}
