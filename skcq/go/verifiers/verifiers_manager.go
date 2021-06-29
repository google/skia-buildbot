package verifiers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/skcq/go/config"
	"go.skia.org/infra/skcq/go/footers"
	"go.skia.org/infra/skcq/go/types"
)

var (
	// Allow lists will be cached here so that they are not continuously
	// instantiated at every poll iteration.
	AllowlistCache = map[string]*allowed.AllowedFromChromeInfraAuth{}

	// vmTimeNowFunc allows tests to mock out time.Now() for testing.
	vmTimeNowFunc = time.Now
)

// SkCQVerifiersManager implements VerifiersManager. Useful for mocking out
// functions for testing.
type SkCQVerifiersManager struct{}

// GetVerifiers returns all the verifiers that apply to the specified change using the specified config.
// If isSubmittedTogetherChange change is true then it is treated as a CQ change and we do not check
// if the CQ+2 triggerer is a committer because it does not matter for submitted together changes.
func (vm *SkCQVerifiersManager) GetVerifiers(ctx context.Context, httpClient *http.Client, cfg *config.SkCQCfg, cr codereview.CodeReview, ci *gerrit.ChangeInfo, isSubmittedTogetherChange bool, configReader *config.GitilesConfigReader) ([]types.Verifier, []string, error) {
	// Instantiate the 2 slices that will be returned.
	clVerifiers := []types.Verifier{}
	togetherChanges := []*gerrit.ChangeInfo{}

	// Get footers map to pass into the different verifiers that need them.
	commitMsg, err := cr.GetCommitMessage(ctx, ci)
	if err != nil {
		return nil, nil, skerr.Fmt("Could not get commit message of %d: %s", ci.Issue, err)
	}
	footersMap := footers.GetFootersMap(commitMsg)

	// Check for if the change is in CQ (vs dry-run) first. This is done because if a change
	// has both CQ+2 and CQ+1 votes then we want to consider the CQ+2 vote first.
	if isSubmittedTogetherChange || cr.IsCQ(ctx, ci) {
		// Do not need to run these verifiers if it is a submitted together change.
		// This is done because checking if the CQ+2 triggerer is a committer
		// was already done in the original change.
		// Also, we do not need to look for the submitted together changes for
		// this submitted together change. It is unecessary and would probably cause an
		// infinite-loop of some kind.
		if !isSubmittedTogetherChange {
			// Verify the CQ+2 triggerer is a committer.
			// Get committer list from the cache, or set it if it does not exist.
			committerList, ok := AllowlistCache[cfg.CommitterList]
			if !ok {
				committerList, err = allowed.NewAllowedFromChromeInfraAuth(httpClient, cfg.CommitterList)
				if err != nil {
					return nil, nil, skerr.Fmt("Could not create an allowed from %s: %s", cfg.CommitterList, err)
				}
				AllowlistCache[cfg.CommitterList] = committerList
			}
			cqVerifier, err := NewCQAccessListVerifier(httpClient, committerList, cfg.CommitterList)
			if err != nil {
				return nil, nil, skerr.Fmt("Error when creating CQAccessListVerifier: %s", err)
			}
			clVerifiers = append(clVerifiers, cqVerifier)

			// Verify all the submitted together changes (if any exist).
			togetherChanges, err = cr.GetSubmittedTogether(ctx, ci)
			if err != nil {
				return nil, nil, skerr.Fmt("Error when getting submitted together chagnes for SubmittedTogetherVerifier: %s", err)
			}
			if len(togetherChanges) > 0 {
				togetherChangesVerifier, err := NewSubmittedTogetherVerifier(ctx, vm, togetherChanges, httpClient, cfg, cr, ci, configReader, footersMap)
				if err != nil {
					return nil, nil, skerr.Fmt("Error when creating SubmittedTogetherVerifier: %s", err)
				}
				clVerifiers = append(clVerifiers, togetherChangesVerifier)
			}
		}

		// Verify that the change does not have "Commit: false".
		commitFooterVerifier, err := NewCommitFooterVerifier(footersMap)
		if err != nil {
			return nil, nil, skerr.Fmt("Error when creating CommitFooterVerifier: %s", err)
		}
		clVerifiers = append(clVerifiers, commitFooterVerifier)

		// Verify that the change is not WIP.
		wipVerifier, err := NewWIPVerifier()
		if err != nil {
			return nil, nil, skerr.Fmt("Error when creating WIPVerifier: %s", err)
		}
		clVerifiers = append(clVerifiers, wipVerifier)

		// Verify the change is submittable.
		submittableVerifier, err := NewSubmittableVerifier()
		if err != nil {
			return nil, nil, skerr.Fmt("Error when creating SubmittableVerifier: %s", err)
		}
		clVerifiers = append(clVerifiers, submittableVerifier)

		if cfg.TreeStatusURL != "" {
			// Verify that the tree is open.
			treeStatusVerifier, err := NewTreeStatusVerifier(httpClient, cfg.TreeStatusURL, footersMap)
			if err != nil {
				return nil, nil, skerr.Fmt("Error when creating TreeStatusVerifier: %s", err)
			}
			clVerifiers = append(clVerifiers, treeStatusVerifier)
		}

		// Verify that the change can be submitted without throttling.
		throttlerVerifier, err := NewThrottlerVerifier(cfg.ThrottlerCfg)
		if err != nil {
			return nil, nil, skerr.Fmt("Error when creating ThrottlerVerifier: %s", err)
		}
		clVerifiers = append(clVerifiers, throttlerVerifier)

	} else if cr.IsDryRun(ctx, ci) {
		// Verify that the CQ+1 triggerred has access to run try jobs.
		// Get dry-run list from the cache or set it if it does not exist.
		dryRunList, ok := AllowlistCache[cfg.DryRunAccessList]
		if !ok {
			dryRunList, err = allowed.NewAllowedFromChromeInfraAuth(httpClient, cfg.DryRunAccessList)
			if err != nil {
				return nil, nil, skerr.Fmt("Could not create an allowed from %s: %s", cfg.DryRunAccessList, err)
			}
			AllowlistCache[cfg.DryRunAccessList] = dryRunList
		}
		dryRunVerifier, err := NewDryRunAccessListVerifier(httpClient, dryRunList, cfg.DryRunAccessList)
		if err != nil {
			return nil, nil, skerr.Fmt("Error when creating DryRunVerifier: %s", err)
		}
		clVerifiers = append(clVerifiers, dryRunVerifier)
	}

	// Verifiers common to both dry runs and CQ.

	if cfg.TasksJSONPath != "" {
		// Verify that try jobs ran.
		tasksCfg, err := configReader.GetTasksCfg(ctx, cfg.TasksJSONPath)
		if err != nil {
			return nil, nil, skerr.Fmt("Error getting tasks cfg: %s", err)
		}
		tryJobsVerifier, err := NewTryJobsVerifier(httpClient, cr, tasksCfg, footersMap, cfg.Internal, cfg.Staging)
		if err != nil {
			return nil, nil, skerr.Fmt("Error when creating TryJobsVerifier: %s", err)
		}
		clVerifiers = append(clVerifiers, tryJobsVerifier)
	}

	togetherChangeIDs := []string{}
	for _, t := range togetherChanges {
		togetherChangeIDs = append(togetherChangeIDs, fmt.Sprintf("%d", t.Issue))
	}
	return clVerifiers, togetherChangeIDs, nil
}

// RunVerifiers runs all the specified verifiers for the change.
func (vm *SkCQVerifiersManager) RunVerifiers(ctx context.Context, ci *gerrit.ChangeInfo, verifiers []types.Verifier, startTime int64) []*types.VerifierStatus {
	verifierStatuses := []*types.VerifierStatus{}
	for _, v := range verifiers {
		status := &types.VerifierStatus{
			Name:  v.Name(),
			Start: startTime,
		}
		verifierState, reason, err := v.Verify(ctx, ci, startTime)
		if err != nil {
			// Always consider errors from verify as transient errors so that they
			// are retried by SkCQ. Always log them so that alerts appear due to
			// error rate alerts.
			errMsg := fmt.Sprintf("%s: Hopefully a transient error: %s", v.Name(), err)
			sklog.Errorf(errMsg)
			status.State = types.VerifierWaitingState
			status.Reason = errMsg
		} else {
			status.State = verifierState
			status.Reason = reason
			switch verifierState {
			case types.VerifierSuccessState:
				status.Stop = vmTimeNowFunc().Unix()
			case types.VerifierFailureState:
				status.Stop = vmTimeNowFunc().Unix()
			}
		}
		verifierStatuses = append(verifierStatuses, status)
	}
	return verifierStatuses
}
