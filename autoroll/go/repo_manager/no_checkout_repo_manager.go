package repo_manager

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
)

/*
	Repo manager which does not use a local checkout.

	Use this repo manager as a helper but not directly.
*/

// NoCheckoutRepoManagerConfig provides configuration for the noCheckoutRepoManager.
type NoCheckoutRepoManagerConfig struct {
	CommonRepoManagerConfig
}

// See documentation for RepoManagerConfig interface.
func (c *NoCheckoutRepoManagerConfig) NoCheckout() bool {
	return true
}

func (c *NoCheckoutRepoManagerConfig) Validate() error {
	if err := c.CommonRepoManagerConfig.Validate(); err != nil {
		return err
	}
	if len(c.PreUploadSteps) > 0 {
		return errors.New("Checkout-less rollers don't support pre-upload steps")
	}
	return nil
}

// noCheckoutRepoManager is a RepoManager which rolls without the use of a
// local checkout.
type noCheckoutRepoManager struct {
	*commonRepoManager
	baseCommit    string
	baseCommitMtx sync.RWMutex
	createRoll    noCheckoutCreateRollHelperFunc
	gerritConfig  *codereview.GerritConfig
	parentRepo    *gitiles.Repo
	updateHelper  noCheckoutUpdateHelperFunc
}

// noCheckoutUpdateHelperFunc is a function called by
// noCheckoutRepoManager.Update() which returns the last roll revision, next
// roll revision, and a list of not-yet-rolled revisions, or any error which
// occurred. The parameters are the parent repo and its base commit.
// commonRepoManager.repoMtx is locked while noCheckoutUpdateHelperFunc is
// running.
type noCheckoutUpdateHelperFunc func(context.Context, *gitiles.Repo, string) (*revision.Revision, *revision.Revision, []*revision.Revision, error)

// noCheckoutCreateRollHelperFunc is a function called by
// noCheckoutRepoManager.CreateNewRoll() which returns a commit message for
// a given roll, plus a map of file names to new contents, given the previous
// roll revision, next roll revision, URL of the server, extra trybots for the
// CQ, TBR emails, and base commit (HEAD of the target parent repo branch as of
// the last call to Update).
type noCheckoutCreateRollHelperFunc func(context.Context, *revision.Revision, *revision.Revision, []*revision.Revision, string, string, []string, string) (string, map[string]string, error)

// Return a noCheckoutRepoManager instance.
func newNoCheckoutRepoManager(ctx context.Context, c NoCheckoutRepoManagerConfig, workdir string, g gerrit.GerritInterface, serverURL string, client *http.Client, cr codereview.CodeReview, createRoll noCheckoutCreateRollHelperFunc, updateHelper noCheckoutUpdateHelperFunc, local bool) (*noCheckoutRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	crm, err := newCommonRepoManager(ctx, c.CommonRepoManagerConfig, workdir, serverURL, g, client, cr, local)
	if err != nil {
		return nil, err
	}
	rv := &noCheckoutRepoManager{
		commonRepoManager: crm,
		createRoll:        createRoll,
		gerritConfig:      cr.Config().(*codereview.GerritConfig),
		parentRepo:        gitiles.NewRepo(c.ParentRepo, client),
		updateHelper:      updateHelper,
	}
	return rv, nil
}

// See documentation for RepoManager interface.
func (rm *noCheckoutRepoManager) CreateNewRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	rm.baseCommitMtx.RLock()
	baseCommit := rm.baseCommit
	rm.baseCommitMtx.RUnlock()

	// Build the roll.
	commitMsg, nextRollChanges, err := rm.createRoll(ctx, from, to, rolling, rm.serverURL, cqExtraTrybots, emails, baseCommit)
	if err != nil {
		return 0, err
	}

	// Create the change.
	ci, err := gerrit.CreateAndEditChange(ctx, rm.g, rm.gerritConfig.Project, rm.parentBranch, commitMsg, baseCommit, func(ctx context.Context, g gerrit.GerritInterface, ci *gerrit.ChangeInfo) error {
		for file, contents := range nextRollChanges {
			if contents == "" {
				if err := g.DeleteFile(ctx, ci, file); err != nil {
					return fmt.Errorf("Failed to delete %s file: %s", file, err)
				}
			} else {
				if err := g.EditFile(ctx, ci, file, contents); err != nil {
					return fmt.Errorf("Failed to edit %s file: %s", file, err)
				}
			}
		}
		return nil
	})
	if err != nil {
		if ci != nil {
			if err2 := rm.g.Abandon(ctx, ci, "Failed to create roll CL"); err2 != nil {
				return 0, fmt.Errorf("Failed to create roll with: %s\nAnd failed to abandon the change with: %s", err, err2)
			}
		}
		return 0, err
	}

	// Mark the change as ready for review, if necessary.
	if err := rm.unsetWIP(ctx, ci, 0); err != nil {
		return 0, err
	}

	// Set the CQ bit as appropriate.
	labels := rm.g.Config().SetCqLabels
	if dryRun {
		labels = rm.g.Config().SetDryRunLabels
	}
	labels = gerrit.MergeLabels(labels, rm.g.Config().SelfApproveLabels)
	if err = rm.g.SetReview(ctx, ci, "", labels, emails); err != nil {
		// TODO(borenet): Should we try to abandon the CL?
		return 0, fmt.Errorf("Failed to set review: %s", err)
	}

	// Manually submit if necessary.
	if !rm.g.Config().HasCq {
		if err := rm.g.Submit(ctx, ci); err != nil {
			// TODO(borenet): Should we try to abandon the CL?
			return 0, fmt.Errorf("Failed to submit: %s", err)
		}
	}

	return ci.Issue, nil
}

// See documentation for RepoManager interface.
func (rm *noCheckoutRepoManager) Update(ctx context.Context) (*revision.Revision, *revision.Revision, []*revision.Revision, error) {
	rm.repoMtx.Lock()
	defer rm.repoMtx.Unlock()
	// Find HEAD of the desired parent branch. We make sure to provide the
	// base commit of our change, to avoid clobbering other changes to the
	// DEPS file.
	baseCommit, err := rm.parentRepo.Details(ctx, rm.parentBranch)
	if err != nil {
		return nil, nil, nil, err
	}

	// Get the next roll rev, and the list of versions in between the last
	// and next rolls.
	lastRollRev, tipRev, notRolledRevs, err := rm.updateHelper(ctx, rm.parentRepo, baseCommit.Hash)
	if err != nil {
		return nil, nil, nil, err
	}

	rm.baseCommitMtx.Lock()
	defer rm.baseCommitMtx.Unlock()
	rm.baseCommit = baseCommit.Hash
	return lastRollRev, tipRev, notRolledRevs, nil
}

// See documentation for RepoManager interface.
func (r *noCheckoutRepoManager) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	return nil, errors.New("NOT IMPLEMENTED")
}
