package repo_manager

import (
	"context"
	"errors"
	"net/http"
	"regexp"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
)

var (
	getDepRegex = regexp.MustCompile("[a-f0-9]+")
)

// NoCheckoutDEPSRepoManagerConfig provides configuration for RepoManagers which
// don't use a local checkout.
type NoCheckoutDEPSRepoManagerConfig struct {
	NoCheckoutRepoManagerConfig
	Gerrit *codereview.GerritConfig `json:"gerrit,omitempty"`
	// URL of the child repo.
	ChildRepo string `json:"childRepo"` // TODO(borenet): Can we just get this from DEPS?

	// TransitiveDeps is an optional set of dependencies shared by the Parent
	// and Child which are updated in the Parent to match the versions of the
	// Child.
	TransitiveDeps []*version_file_common.TransitiveDepConfig `json:"transitiveDeps,omitempty"`
}

// Validate implements the util.Validator interface.
func (c *NoCheckoutDEPSRepoManagerConfig) Validate() error {
	if err := c.NoCheckoutRepoManagerConfig.Validate(); err != nil {
		return err
	}
	if c.ChildRepo == "" {
		return errors.New("ChildRepo is required.")
	}
	if c.ParentBranch == nil {
		return errors.New("ParentBranch is required.")
	}
	if err := c.ParentBranch.Validate(); err != nil {
		return err
	}
	if c.ParentRepo == "" {
		return errors.New("ParentRepo is required.")
	}
	for _, s := range c.PreUploadSteps {
		if _, err := parent.GetPreUploadStep(s); err != nil {
			return err
		}
	}
	for _, dep := range c.TransitiveDeps {
		if err := dep.Child.Validate(); err != nil {
			return skerr.Wrapf(err, "invalid TransitiveDeps Child")
		}
		if err := dep.Parent.Validate(); err != nil {
			return skerr.Wrapf(err, "invalid TransitiveDeps Parent")
		}
	}
	_, err := c.splitParentChild()
	return skerr.Wrap(err)
}

// splitParentChild splits the NoCheckoutDEPSRepoManagerConfig into a
// parent.GitilesConfig and a child.GitilesConfig.
// TODO(borenet): Update the config format to directly define the parent
// and child. We shouldn't need most of the New.*RepoManager functions.
func (c NoCheckoutDEPSRepoManagerConfig) splitParentChild() (*ParentChildConfig, error) {
	var childDeps []*version_file_common.VersionFileConfig
	if c.TransitiveDeps != nil {
		childDeps = make([]*version_file_common.VersionFileConfig, 0, len(c.TransitiveDeps))
		for _, dep := range c.TransitiveDeps {
			childDeps = append(childDeps, dep.Child)
		}
	}
	cfg := &ParentChildConfig{
		Parent: parent.ConfigUnion{
			GitilesFile: &parent.GitilesConfig{
				DependencyConfig: version_file_common.DependencyConfig{
					VersionFileConfig: version_file_common.VersionFileConfig{
						ID:   c.ChildRepo,
						Path: deps_parser.DepsFileName,
					},
					TransitiveDeps: c.TransitiveDeps,
				},
				GitilesConfig: gitiles_common.GitilesConfig{
					Branch:  c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ParentBranch,
					RepoURL: c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ParentRepo,
				},
				Gerrit: c.Gerrit,
			},
		},
		Child: child.Config{
			Gitiles: &child.GitilesConfig{
				GitilesConfig: gitiles_common.GitilesConfig{
					Branch:       c.ChildBranch,
					RepoURL:      c.ChildRepo,
					Dependencies: childDeps,
				},
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		return nil, skerr.Wrapf(err, "generated config is invalid")
	}
	return cfg, nil
}

// NewNoCheckoutDEPSRepoManager returns a RepoManager instance which does not
// use a local checkout.
func NewNoCheckoutDEPSRepoManager(ctx context.Context, c *NoCheckoutDEPSRepoManagerConfig, reg *config_vars.Registry, workdir, rollerName string, gerritClient *gerrit.Gerrit, githubClient *github.GitHub, recipeCfgFile, serverURL string, httpClient *http.Client, cr codereview.CodeReview, local bool) (*ParentChildRepoManager, error) {
	cfg, err := c.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildFromConfig(ctx, cfg, reg, httpClient, gerritClient, githubClient, serverURL, workdir, rollerName, cr.UserName(), cr.UserEmail(), recipeCfgFile)
}
