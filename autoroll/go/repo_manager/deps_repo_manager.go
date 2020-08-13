package repo_manager

import (
	"context"
	"net/http"
	"path/filepath"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gerrit_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
)

// issueJson is the structure of "git cl issue --json"
type issueJson struct {
	Issue    int64  `json:"issue"`
	IssueUrl string `json:"issue_url"`
}

// DEPSRepoManagerConfig provides configuration for the DEPS RepoManager.
type DEPSRepoManagerConfig struct {
	Parent parent.DEPSLocalConfig  `json:"parent"`
	Child  child.GitCheckoutConfig `json:"child"`

	// Path of the child repo within the checkout root.
	ChildPath string `json:"childPath"`

	// ChildSubdir indicates the subdirectory of the workdir in which
	// the ChildPath should be rooted. In most cases, this should be empty,
	// but if ChildPath is relative to the parent repo dir (eg. when DEPS
	// specifies use_relative_paths), then this is required.
	ChildSubdir string `json:"childSubdir,omitempty"`
}

// See documentation for util.Validator interface.
func (c DEPSRepoManagerConfig) Validate() error {
	if err := c.Parent.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if err := c.Child.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// NewDEPSRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func NewDEPSRepoManager(ctx context.Context, c *DEPSRepoManagerConfig, reg *config_vars.Registry, workdir string, g *gerrit.Gerrit, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	parentRM, err := parent.NewDEPSLocal(ctx, c.Parent, reg, client, serverURL, workdir, cr.UserName(), cr.UserEmail(), recipeCfgFile)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := gerrit_common.SetupGerrit(ctx, parentRM.Checkout.Checkout, g); err != nil {
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
