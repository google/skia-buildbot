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

// See documentation for util.Validator interface.
func (c *NoCheckoutDEPSRepoManagerConfig) Validate() error {
	// Set some unused variables on the embedded RepoManager.
	c.ChildPath = "N/A"
	c.ChildRevLinkTmpl = "N/A"
	if err := c.NoCheckoutRepoManagerConfig.Validate(); err != nil {
		return err
	}
	// Unset the unused variables.
	c.ChildPath = ""
	c.ChildRevLinkTmpl = ""

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
	_, _, err := c.splitParentChild()
	return skerr.Wrap(err)
}

// splitParentChild splits the NoCheckoutDEPSRepoManagerConfig into a
// parent.GitilesConfig and a child.GitilesConfig.
// TODO(borenet): Update the config format to directly define the parent
// and child. We shouldn't need most of the New.*RepoManager functions.
func (c NoCheckoutDEPSRepoManagerConfig) splitParentChild() (parent.GitilesConfig, child.GitilesConfig, error) {
	var childDeps []*version_file_common.VersionFileConfig
	if c.TransitiveDeps != nil {
		childDeps = make([]*version_file_common.VersionFileConfig, 0, len(c.TransitiveDeps))
		for _, dep := range c.TransitiveDeps {
			childDeps = append(childDeps, dep.Child)
		}
	}
	parentCfg := parent.GitilesConfig{
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
	}
	if err := parentCfg.Validate(); err != nil {
		return parent.GitilesConfig{}, child.GitilesConfig{}, skerr.Wrapf(err, "generated parent config is invalid")
	}
	childCfg := child.GitilesConfig{
		GitilesConfig: gitiles_common.GitilesConfig{
			Branch:       c.ChildBranch,
			RepoURL:      c.ChildRepo,
			Dependencies: childDeps,
		},
	}
	if err := childCfg.Validate(); err != nil {
		return parent.GitilesConfig{}, child.GitilesConfig{}, skerr.Wrapf(err, "generated child config is invalid")
	}
	return parentCfg, childCfg, nil
}

// NewNoCheckoutDEPSRepoManager returns a RepoManager instance which does not
// use a local checkout.
func NewNoCheckoutDEPSRepoManager(ctx context.Context, c *NoCheckoutDEPSRepoManagerConfig, reg *config_vars.Registry, workdir string, g gerrit.GerritInterface, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	parentCfg, childCfg, err := c.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	parentRM, err := parent.NewGitilesFile(ctx, parentCfg, reg, client, serverURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	childRM, err := child.NewGitiles(ctx, childCfg, reg, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM, nil)
}
