package repo_manager

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"go.skia.org/infra/autoroll/go/strategy"
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

	// Gerrit project for the parent repo.
	GerritProject string `json:"gerritProject"`
	// URL of the parent repo.
	ParentRepo string `json:"parentRepo"`
}

func (c *NoCheckoutRepoManagerConfig) Validate() error {
	if err := c.CommonRepoManagerConfig.Validate(); err != nil {
		return err
	}
	if c.GerritProject == "" {
		return errors.New("GerritProject is required.")
	}
	if c.ParentRepo == "" {
		return errors.New("ParentRepo is required.")
	}
	if len(c.PreUploadSteps) > 0 {
		return errors.New("Checkout-less rollers don't support pre-upload steps")
	}
	return nil
}

// noCheckoutRepoManager is a RepoManager which rolls Android AFDO profile version
// numbers into Chromium. Unlike other rollers, there is no child repo to sync;
// the version number is obtained from Google Cloud Storage.
type noCheckoutRepoManager struct {
	*commonRepoManager
	baseCommit         string
	buildCommitMessage noCheckoutBuildCommitMessageFunc
	gerritProject      string
	nextRollChanges    map[string]string
	parentRepo         *gitiles.Repo
	updateHelper       noCheckoutUpdateHelperFunc
}

// noCheckoutUpdateHelperFunc is a function called by noCheckoutRepoManager.Update()
// which returns the last roll revision, next roll revision, number of not-yet-rolled
// revisions, and a map of file names to contents indicating what should be changed
// in the next roll. The parameters are the parent repo and its base commit.
type noCheckoutUpdateHelperFunc func(context.Context, strategy.NextRollStrategy, *gitiles.Repo, string) (string, string, int, map[string]string, error)

// noCheckoutBuildCommitMessageFunc is a function called by
// noCheckoutRepoManager.CreateNewRoll() which returns the commit message for
// a given roll given the previous roll revision, next roll revision, URL of the
// server, extra trybots for the CQ, and TBR emails.
type noCheckoutBuildCommitMessageFunc func(string, string, string, string, []string) (string, error)

// Return a noCheckoutRepoManager instance.
func newNoCheckoutRepoManager(ctx context.Context, c NoCheckoutRepoManagerConfig, workdir string, g gerrit.GerritInterface, serverURL, gitcookiesPath string, client *http.Client, buildCommitMessage noCheckoutBuildCommitMessageFunc, updateHelper noCheckoutUpdateHelperFunc) (*noCheckoutRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	crm, err := newCommonRepoManager(c.CommonRepoManagerConfig, workdir, serverURL, g, client)
	if err != nil {
		return nil, err
	}
	rv := &noCheckoutRepoManager{
		commonRepoManager:  crm,
		buildCommitMessage: buildCommitMessage,
		gerritProject:      c.GerritProject,
		parentRepo:         gitiles.NewRepo(c.ParentRepo, gitcookiesPath, client),
		updateHelper:       updateHelper,
	}
	return rv, nil
}

// See documentation for RepoManager interface.
func (rm *noCheckoutRepoManager) CreateNewRoll(ctx context.Context, from, to string, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	// Build the commit message.
	commitMsg, err := rm.buildCommitMessage(from, to, rm.serverURL, cqExtraTrybots, emails)
	if err != nil {
		return 0, err
	}

	rm.infoMtx.Lock()
	defer rm.infoMtx.Unlock()

	// Create the change.
	ci, err := gerrit.CreateAndEditChange(rm.g, rm.gerritProject, rm.parentBranch, commitMsg, rm.baseCommit, func(g gerrit.GerritInterface, ci *gerrit.ChangeInfo) error {
		for file, contents := range rm.nextRollChanges {
			if err := g.EditFile(ci, file, contents); err != nil {
				return fmt.Errorf("Failed to edit %s file: %s", file, err)
			}
		}
		return nil
	})
	if err != nil {
		if ci != nil {
			if err2 := rm.g.Abandon(ci, "Failed to create roll CL"); err2 != nil {
				return 0, fmt.Errorf("Failed to create roll with: %s\nAnd failed to abandon the change with: %s", err, err2)
			}
		}
		return 0, err
	}

	// Set the CQ bit as appropriate.
	cq := gerrit.COMMITQUEUE_LABEL_SUBMIT
	if dryRun {
		cq = gerrit.COMMITQUEUE_LABEL_DRY_RUN
	}
	if err = rm.g.SetReview(ci, "", map[string]interface{}{
		gerrit.CODEREVIEW_LABEL:  gerrit.CODEREVIEW_LABEL_APPROVE,
		gerrit.COMMITQUEUE_LABEL: cq,
	}, emails); err != nil {
		// TODO(borenet): Should we try to abandon the CL?
		return 0, err
	}

	return ci.Issue, nil
}

// See documentation for RepoManager interface.
func (rm *noCheckoutRepoManager) Update(ctx context.Context) error {
	// Find HEAD of the desired parent branch. We make sure to provide the
	// base commit of our change, to avoid clobbering other changes to the
	// DEPS file.
	baseCommit, err := rm.parentRepo.GetCommit(rm.parentBranch)
	if err != nil {
		return err
	}

	// Get the next roll rev, and the list of versions in between the last
	// and next rolls.
	rm.strategyMtx.RLock()
	defer rm.strategyMtx.RUnlock()
	lastRollRev, nextRollRev, commitsNotRolled, nextRollChanges, err := rm.updateHelper(ctx, rm.strategy, rm.parentRepo, baseCommit.Hash)
	if err != nil {
		return err
	}

	rm.infoMtx.Lock()
	defer rm.infoMtx.Unlock()
	rm.baseCommit = baseCommit.Hash
	rm.lastRollRev = lastRollRev
	rm.nextRollRev = nextRollRev
	rm.commitsNotRolled = commitsNotRolled
	rm.nextRollChanges = nextRollChanges
	return nil
}

// See documentation for RepoManager interface.
func (rm *noCheckoutRepoManager) FullChildHash(ctx context.Context, ver string) (string, error) {
	return "", fmt.Errorf("NOT IMPLEMENTED")
}

// See documentation for RepoManager interface.
func (rm *noCheckoutRepoManager) RolledPast(ctx context.Context, ver string) (bool, error) {
	return false, fmt.Errorf("NOT IMPLEMENTED")
}

// See documentation for RepoManager interface.
func (r *noCheckoutRepoManager) CreateNextRollStrategy(ctx context.Context, s string) (strategy.NextRollStrategy, error) {
	return nil, fmt.Errorf("NOT IMPLEMENTED")
}

// See documentation for RepoManager interface.
func (r *noCheckoutRepoManager) DefaultStrategy() string {
	return "NOT IMPLEMENTED"
}

// See documentation for RepoManager interface.
func (r *noCheckoutRepoManager) ValidStrategies() []string {
	return []string{} // NOT IMPLEMENTED
}
