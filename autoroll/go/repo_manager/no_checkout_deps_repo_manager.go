package repo_manager

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/helpers"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
)

// NoCheckoutDEPSRepoManagerConfig provides configuration for RepoManagers which
// don't use a local checkout.
type NoCheckoutDEPSRepoManagerConfig struct {
	NoCheckoutRepoManagerConfig
	child.GitilesChildConfig
}

func (c *NoCheckoutDEPSRepoManagerConfig) Validate() error {
	if err := c.NoCheckoutRepoManagerConfig.Validate(); err != nil {
		return err
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
	return c.GitilesChildConfig.Validate()
}

type noCheckoutDEPSRepoManager struct {
	*noCheckoutRepoManager
	child         child.Child
	depotTools    string
	gclient       string
	parentRepoUrl string
}

// NewNoCheckoutDEPSRepoManager returns a RepoManager instance which does not use
// a local checkout.
func NewNoCheckoutDEPSRepoManager(ctx context.Context, c *NoCheckoutDEPSRepoManagerConfig, reg *config_vars.Registry, workdir string, g gerrit.GerritInterface, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(workdir, os.ModePerm); err != nil {
		return nil, err
	}

	depotTools, err := depot_tools.GetDepotTools(ctx, workdir, recipeCfgFile)
	if err != nil {
		return nil, err
	}
	gclient := filepath.Join(depotTools, GCLIENT)
	ch, err := child.NewGitilesChild(ctx, c.GitilesChildConfig, client, gclient)
	if err != nil {
		return nil, err
	}
	rv := &noCheckoutDEPSRepoManager{
		child:         ch,
		depotTools:    depotTools,
		gclient:       gclient,
		parentRepoUrl: c.ParentRepo,
	}
	ncrm, err := newNoCheckoutRepoManager(ctx, c.NoCheckoutRepoManagerConfig, reg, workdir, g, serverURL, client, cr, rv.createRoll, rv.updateHelper, local)
	if err != nil {
		return nil, err
	}
	rv.noCheckoutRepoManager = ncrm

	return rv, nil
}

// See documentation for noCheckoutRepoManagerCreateRollHelperFunc.
func (rm *noCheckoutDEPSRepoManager) createRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, serverURL, cqExtraTrybots string, emails []string, baseCommit string) (string, map[string]string, error) {
	// Download the DEPS file from the parent repo.
	depsFile, cleanup, err := helpers.GetDEPSFile(ctx, rm.parentRepo, rm.gclient, baseCommit)
	if err != nil {
		return "", nil, err
	}
	defer cleanup()

	// Write the new DEPS content.
	if err := helpers.SetDep(ctx, rm.gclient, depsFile, rm.childPath, to.Id); err != nil {
		return "", nil, err
	}

	// Update any transitive DEPS.
	transitiveDeps := []*TransitiveDep{}
	if len(rm.transitiveDeps) > 0 {
		for childPath, parentPath := range rm.transitiveDeps {
			oldRev, err := helpers.GetDep(ctx, rm.childRepo, rm.gclient, depsFile, parentPath)
			if err != nil {
				return "", nil, err
			}
			newRev, ok := to.Dependencies[childPath]
			if !ok {
				return "", nil, skerr.Fmt("To-revision %s is missing dependency entry for %s", to.Id, childPath)
			}
			if oldRev != newRev {
				if err := helpers.SetDep(ctx, rm.gclient, depsFile, parentPath, newRev); err != nil {
					return "", nil, err
				}
				transitiveDeps = append(transitiveDeps, &TransitiveDep{
					ParentPath:  parentPath,
					RollingFrom: oldRev,
					RollingTo:   newRev,
				})
			}
		}
	}

	// Read the updated DEPS content.
	newDEPSContent, err := ioutil.ReadFile(depsFile)
	if err != nil {
		return "", nil, err
	}

	// Build the commit message.
	commitMsg, err := rm.buildCommitMsg(&CommitMsgVars{
		ChildPath:      rm.childPath,
		ChildRepo:      rm.childRepoUrl,
		CqExtraTrybots: cqExtraTrybots,
		Reviewers:      emails,
		Revisions:      rolling,
		RollingFrom:    from,
		RollingTo:      to,
		ServerURL:      serverURL,
		TransitiveDeps: transitiveDeps,
	})
	if err != nil {
		return "", nil, fmt.Errorf("Failed to build commit msg: %s", err)
	}
	return commitMsg, map[string]string{"DEPS": string(newDEPSContent)}, nil
}

// See documentation for noCheckoutRepoManagerUpdateHelperFunc.
func (rm *noCheckoutDEPSRepoManager) updateHelper(ctx context.Context, parentRepo *gitiles.Repo, baseCommit string) (*revision.Revision, *revision.Revision, []*revision.Revision, error) {
	// Find the last roll rev.
	depsFile, cleanup, err := helpers.GetDEPSFile(ctx, rm.parentRepo, rm.gclient, baseCommit)
	if err != nil {
		return nil, nil, nil, err
	}
	defer cleanup()
	lastRollHash, err := helpers.GetDep(ctx, rm.childRepo, rm.gclient, depsFile, rm.childPath)
	if err != nil {
		return nil, nil, nil, err
	}
	lastRollRev, err := rm.child.GetRevision(ctx, lastRollHash)
	if err != nil {
		return nil, nil, nil, err
	}
	tipRev, notRolledRevs, err := rm.child.Update(ctx, lastRollRev)
	if err != nil {
		return nil, nil, nil, err
	}
	return lastRollRev, tipRev, notRolledRevs, nil
}

// See documentation for RepoManager interface.
func (rm *noCheckoutDEPSRepoManager) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	return rm.child.GetRevision(ctx, id)
}
