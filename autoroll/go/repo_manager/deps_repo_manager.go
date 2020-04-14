package repo_manager

import (
	"context"
	"net/http"
	"sync"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
)

// issueJson is the structure of "git cl issue --json"
type issueJson struct {
	Issue    int64  `json:"issue"`
	IssueUrl string `json:"issue_url"`
}

// depsRepoManager is a struct used by DEPs AutoRoller for managing checkouts.
type depsRepoManager struct {
	*depotToolsRepoManager
	childRepoUrl    string
	childRepoUrlMtx sync.RWMutex
}

// DEPSRepoManagerConfig provides configuration for the DEPS RepoManager.
type DEPSRepoManagerConfig struct {
	DepotToolsRepoManagerConfig
	Gerrit     *codereview.GerritConfig `json:"gerrit"`
	ChildRepo  string                   `json:"childRepo"`
	ParentPath string                   `json:"parentPath"`
}

// See documentation for util.Validator interface.
func (c DEPSRepoManagerConfig) Validate() error {
	if err := c.DepotToolsRepoManagerConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if _, _, err := c.splitParentChild(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// splitParentChild splits the NoCheckoutDEPSRepoManagerConfig into a
// parent.DEPSLocalConfig and a child.GitilesConfig.
// TODO(borenet): Update the config format to directly define the parent
// and child. We shouldn't need most of the New.*RepoManager functions.
func (c DEPSRepoManagerConfig) splitParentChild() (parent.DEPSLocalConfig, child.GitilesConfig, error) {
	parentCfg := parent.DEPSLocalConfig{
		GitCheckoutGerritConfig: parent.GitCheckoutGerritConfig{
			GitCheckoutConfig: parent.GitCheckoutConfig{
				BaseConfig: parent.BaseConfig{
					ChildPath:       c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.ChildPath,
					ChildRepo:       c.ChildRepo,
					IncludeBugs:     c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.IncludeBugs,
					IncludeLog:      c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.IncludeLog,
					CommitMsgTmpl:   c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.CommitMsgTmpl,
					MonorailProject: c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.BugProject,
				},
				Branch:  c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.ParentBranch,
				RepoURL: c.DepotToolsRepoManagerConfig.CommonRepoManagerConfig.ParentRepo,
			},
			Gerrit: c.Gerrit,
		},
		Dep:          c.ChildRepo,
		CheckoutPath: c.ParentPath,
		GClientSpec:  c.GClientSpec,
	}
	if err := parentCfg.Validate(); err != nil {
		return parent.DEPSLocalConfig{}, child.GitilesConfig{}, skerr.Wrapf(err, "generated parent config is invalid")
	}
	childCfg := child.GitilesConfig{
		GitilesConfig: gitiles_common.GitilesConfig{
			Branch:  c.ChildBranch,
			RepoURL: c.ChildRepo,
		},
	}
	if err := childCfg.Validate(); err != nil {
		return parent.DEPSLocalConfig{}, child.GitilesConfig{}, skerr.Wrapf(err, "generated child config is invalid")
	}
	return parentCfg, childCfg, nil
}

// NewDEPSRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func NewDEPSRepoManager(ctx context.Context, c *DEPSRepoManagerConfig, reg *config_vars.Registry, workdir string, g *gerrit.Gerrit, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	parentCfg, childCfg, err := c.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	parentRM, err := parent.NewDEPSLocal(ctx, parentCfg, reg, client, serverURL, workdir, recipeCfgFile)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	childRM, err := child.NewGitiles(ctx, childCfg, reg, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM)
}
