package repo_manager

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/gitiles_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
)

// GitilesCIPDDEPSRepoManagerConfig provides configuration for
// GitilesCIPDDEPSRepoManager.
type GitilesCIPDDEPSRepoManagerConfig struct {
	NoCheckoutRepoManagerConfig
	Gerrit *codereview.GerritConfig `json:"gerrit"`
	// Name of the CIPD package to roll.
	CipdAssetName string `json:"cipdAssetName"`
	// Tag used to find the CIPD package version to roll.
	CipdAssetTag string `json:"cipdAssetTag"`
}

// See documentation for RepoManagerConfig interface.
func (r *GitilesCIPDDEPSRepoManagerConfig) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
	}
}

// See documentation for util.Validator interface.
func (c *GitilesCIPDDEPSRepoManagerConfig) Validate() error {
	if err := c.NoCheckoutRepoManagerConfig.Validate(); err != nil {
		return err
	}
	_, _, err := c.splitParentChild()
	return skerr.Wrap(err)
}

// splitParentChild splits the GitilesCIPDDEPSRepoManagerConfig into a
// parent.GitilesDEPSConfig and a child.CIPDConfig.
// TODO(borenet): Update the config format to directly define the parent
// and child. We shouldn't need most of the New.*RepoManager functions.
func (c GitilesCIPDDEPSRepoManagerConfig) splitParentChild() (parent.GitilesDEPSConfig, child.CIPDConfig, error) {
	parentCfg := parent.GitilesDEPSConfig{
		GitilesConfig: parent.GitilesConfig{
			BaseConfig: parent.BaseConfig{
				ChildPath:       c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ChildPath,
				ChildRepo:       c.CipdAssetName,
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
		Dep: c.CipdAssetName,
	}
	if err := parentCfg.Validate(); err != nil {
		return parent.GitilesDEPSConfig{}, child.CIPDConfig{}, skerr.Wrapf(err, "generated parent config is invalid")
	}
	childCfg := child.CIPDConfig{
		Name: c.CipdAssetName,
		Tag:  c.CipdAssetTag,
	}
	if err := childCfg.Validate(); err != nil {
		return parent.GitilesDEPSConfig{}, child.CIPDConfig{}, skerr.Wrapf(err, "generated child config is invalid")
	}
	return parentCfg, childCfg, nil
}

// NewGitilesCIPDDEPSRepoManager returns a RepoManager instance which does not use
// a local checkout.
func NewGitilesCIPDDEPSRepoManager(ctx context.Context, c *GitilesCIPDDEPSRepoManagerConfig, reg *config_vars.Registry, workdir string, g gerrit.GerritInterface, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
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
	childRM, err := child.NewCIPD(ctx, childCfg, client, workdir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM)
}
