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
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
)

var (
	getDepRegex = regexp.MustCompile("[a-f0-9]+")
)

// TransitiveDepEntry provides one half of the configuration for a transitive
// dependency, ie. the parent or the child.
type TransitiveDepEntry struct {
	// Id is the dependency ID, eg. repo URL.
	Id string `json:"id"`
	// Path is the path to the file within the repo which pins this
	// dependency, eg. "DEPS".
	Path string `json:"path"`
}

// See documentation for util.Validator interface.
func (e TransitiveDepEntry) Validate() error {
	if e.Id == "" {
		return skerr.Fmt("Id is required for TransitiveDepEntry")
	}
	if e.Path == "" {
		return skerr.Fmt("Path is required for TransitiveDepEntry")
	}
	return nil
}

// TransitiveDepConfig provides configuration for a single transitive
// dependency.
type TransitiveDepConfig struct {
	Child  TransitiveDepEntry `json:"child"`
	Parent TransitiveDepEntry `json:"parent"`
}

// NoCheckoutDEPSRepoManagerConfig provides configuration for RepoManagers which
// don't use a local checkout.
type NoCheckoutDEPSRepoManagerConfig struct {
	NoCheckoutRepoManagerConfig
	Gerrit *codereview.GerritConfig `json:"gerrit"`
	// URL of the child repo.
	ChildRepo string `json:"childRepo"` // TODO(borenet): Can we just get this from DEPS?

	// TransitiveDeps is an optional mapping of dependency ID (eg. repo URL)
	// to the paths within the parent and child repo, respectively, where
	// those dependencies are versioned, eg. "DEPS".
	TransitiveDeps []*TransitiveDepConfig `json:"transitiveDeps"`
}

// See documentation for util.Validator interface.
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
		if _, err := GetPreUploadStep(s); err != nil {
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
// parent.GitilesDEPSConfig and a child.GitilesConfig.
// TODO(borenet): Update the config format to directly define the parent
// and child. We shouldn't need most of the New.*RepoManager functions.
func (c NoCheckoutDEPSRepoManagerConfig) splitParentChild() (parent.GitilesDEPSConfig, child.GitilesConfig, error) {
	var childDeps, parentDeps map[string]string
	if c.TransitiveDeps != nil {
		childDeps = make(map[string]string, len(c.TransitiveDeps))
		parentDeps = make(map[string]string, len(c.TransitiveDeps))
		for _, dep := range c.TransitiveDeps {
			parentDeps[dep.Parent.Id] = dep.Parent.Path
			childDeps[dep.Child.Id] = dep.Child.Path
		}
	}
	parentCfg := parent.GitilesDEPSConfig{
		GitilesConfig: parent.GitilesConfig{
			BaseConfig: parent.BaseConfig{
				ChildPath:       c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ChildPath,
				ChildRepo:       c.ChildRepo,
				IncludeBugs:     c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.IncludeBugs,
				IncludeLog:      c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.IncludeLog,
				CommitMsgTmpl:   c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.CommitMsgTmpl,
				MonorailProject: c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.BugProject,
			},
			GitilesConfig: gitiles_common.GitilesConfig{
				Branch:  c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ParentBranch,
				RepoURL: c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ParentRepo,
			},
			Gerrit: c.Gerrit,
		},
		Dep:            c.ChildRepo,
		TransitiveDeps: parentDeps,
	}
	if err := parentCfg.Validate(); err != nil {
		return parent.GitilesDEPSConfig{}, child.GitilesConfig{}, skerr.Wrapf(err, "generated parent config is invalid")
	}
	childCfg := child.GitilesConfig{
		GitilesConfig: gitiles_common.GitilesConfig{
			Branch:       c.ChildBranch,
			RepoURL:      c.ChildRepo,
			Dependencies: childDeps,
		},
	}
	if err := childCfg.Validate(); err != nil {
		return parent.GitilesDEPSConfig{}, child.GitilesConfig{}, skerr.Wrapf(err, "generated child config is invalid")
	}
	return parentCfg, childCfg, nil
}

// NewNoCheckoutDEPSRepoManager returns a RepoManager instance which does not use
// a local checkout.
func NewNoCheckoutDEPSRepoManager(ctx context.Context, c *NoCheckoutDEPSRepoManagerConfig, reg *config_vars.Registry, workdir string, g gerrit.GerritInterface, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	parentCfg, childCfg, err := c.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	parentRM, err := parent.NewGitilesDEPS(ctx, parentCfg, reg, client, serverURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	childRM, err := child.NewGitiles(ctx, childCfg, reg, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM)
}
