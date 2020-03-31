package repo_manager

import (
	"context"
	"errors"
	"net/http"
	"regexp"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
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
	Gerrit *codereview.GerritConfig `json:"gerrit"`
	// URL of the child repo.
	ChildRepo string `json:"childRepo"` // TODO(borenet): Can we just get this from DEPS?

	// TransitiveDeps is an optional mapping of dependency ID (eg. repo URL)
	// to the paths within the parent and child repo, respectively, where
	// those dependencies are versioned, eg. "DEPS".
	TransitiveDeps map[string][]string `json:"transitiveDeps"`
}

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
	for dep, paths := range c.TransitiveDeps {
		if dep == "" {
			return skerr.Fmt("Dependency ID missing for TransitiveDeps")
		}
		if len(paths) != 2 {
			return skerr.Fmt("Expected exactly two paths for TransitiveDeps")
		}
		if paths[0] == "" || paths[1] == "" {
			return skerr.Fmt("TransitiveDeps paths must not be empty")
		}
	}
	return nil
}

// splitNoCheckoutDEPSConfig splits the NoCheckoutDEPSRepoManagerConfig into a
// parent.GitilesDEPSConfig and a child.GitilesConfig.
// TODO(borenet): Update the config format to directly define the parent
// and child. We shouldn't need most of the New.*RepoManager functions.
func splitNoCheckoutDEPSConfig(c NoCheckoutDEPSRepoManagerConfig) (parent.GitilesDEPSConfig, child.GitilesConfig, error) {
	var childDeps, parentDeps map[string]string
	if c.TransitiveDeps != nil {
		childDeps = make(map[string]string, len(c.TransitiveDeps))
		parentDeps = make(map[string]string, len(c.TransitiveDeps))
		for dep, paths := range c.TransitiveDeps {
			// Validate() ensures that len(paths) == 2.
			parentDeps[dep] = paths[0]
			childDeps[dep] = paths[1]
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
			Gerrit:  c.Gerrit,
			Branch:  c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ParentBranch,
			RepoURL: c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ParentRepo,
		},
		Dep:            c.ChildRepo,
		TransitiveDeps: parentDeps,
	}
	childCfg := child.GitilesConfig{
		Branch:       c.ChildBranch,
		RepoURL:      c.ChildRepo,
		Dependencies: childDeps,
	}
	return parentCfg, childCfg, nil
}

// NewNoCheckoutDEPSRepoManager returns a RepoManager instance which does not use
// a local checkout.
func NewNoCheckoutDEPSRepoManager(ctx context.Context, c *NoCheckoutDEPSRepoManagerConfig, reg *config_vars.Registry, workdir string, g gerrit.GerritInterface, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	parentCfg, childCfg, err := splitNoCheckoutDEPSConfig(*c)
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
