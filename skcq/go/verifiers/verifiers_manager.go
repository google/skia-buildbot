package verifiers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/skcq/go/config"
	"go.skia.org/infra/skcq/go/types"
)

var (
	// vmTimeNowFunc allows tests to mock out time.Now() for testing.
	vmTimeNowFunc = time.Now
)

// SkCQVerifiersManager implements VerifiersManager.
type SkCQVerifiersManager struct {
	throttlerManager types.ThrottlerManager
	httpClient       *http.Client
	criaClient       *http.Client
	cr               codereview.CodeReview
	// Allow lists will be cached here so that they are not continuously
	// newly instantiated.
	allowlistCache map[string]allowed.Allow
}

// NewSkCQVerifiersManager returns an instance of SkCQVerifiersManager.
func NewSkCQVerifiersManager(throttlerManager types.ThrottlerManager, httpClient, criaClient *http.Client, cr codereview.CodeReview) *SkCQVerifiersManager {
	return &SkCQVerifiersManager{
		throttlerManager: throttlerManager,
		httpClient:       httpClient,
		criaClient:       criaClient,
		cr:               cr,
		allowlistCache:   map[string]allowed.Allow{},
	}
}

// GetVerifiers implements the VerifierManager interface.
func (vm *SkCQVerifiersManager) GetVerifiers(ctx context.Context, cfg *config.SkCQCfg, ci *gerrit.ChangeInfo, isSubmittedTogetherChange bool, configReader config.ConfigReader) ([]types.Verifier, []string, error) {
	// Instantiate the 2 slices that will be populated and returned.
	clVerifiers := []types.Verifier{}
	togetherChanges := []*gerrit.ChangeInfo{}

	// Get footers map to pass into the different verifiers that need them.
	commitMsg, err := vm.cr.GetCommitMessage(ctx, ci.Issue)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "Could not get commit message of %d", ci.Issue)
	}
	footersMap := git.GetFootersMap(commitMsg)

	// Check if the change is a CQ run (vs dry-run) first. This is done because
	// if a change has both CQ+2 and CQ+1 votes then we want to consider the CQ+2
	// vote first.
	if isSubmittedTogetherChange || vm.cr.IsCQ(ctx, ci) {
		// Do not need to run these verifiers if it is a submitted together change.
		// This is done because checking if the CQ+2 triggerer is a committer
		// was already done in the original change.
		// Also, we do not need to look for the submitted together changes for
		// this submitted together change. It is unecessary and would probably
		// cause an infinite-loop of some kind.
		if !isSubmittedTogetherChange {
			// Get committer list from the cache, or set it if it does not exist.
			committerList, ok := vm.allowlistCache[cfg.CommitterList]
			if !ok {
				committerList, err = allowed.NewAllowedFromChromeInfraAuth(vm.criaClient, cfg.CommitterList)
				if err != nil {
					return nil, nil, skerr.Wrapf(err, "Could not create an allowed from %s", cfg.CommitterList)
				}
				vm.allowlistCache[cfg.CommitterList] = committerList
			}
			// Verify the CQ+2 triggerer is a committer.
			cqVerifier, err := NewCQAccessListVerifier(vm.httpClient, committerList, cfg.CommitterList)
			if err != nil {
				return nil, nil, skerr.Wrapf(err, "Error when creating CQAccessListVerifier")
			}
			clVerifiers = append(clVerifiers, cqVerifier)

			// Verify all the submitted together changes (if any exist).
			togetherChanges, err = vm.cr.GetSubmittedTogether(ctx, ci)
			if err != nil {
				return nil, nil, skerr.Wrapf(err, "Error when getting submitted together chagnes for SubmittedTogetherVerifier")
			}
			if len(togetherChanges) > 0 {
				togetherChangesVerifier, err := NewSubmittedTogetherVerifier(ctx, vm, togetherChanges, cfg, ci, configReader, footersMap)
				if err != nil {
					return nil, nil, skerr.Wrapf(err, "Error when creating SubmittedTogetherVerifier")
				}
				clVerifiers = append(clVerifiers, togetherChangesVerifier)
			}
		}

		// Verify that the change does not have "Commit: false".
		commitFooterVerifier, err := NewCommitFooterVerifier(footersMap)
		if err != nil {
			return nil, nil, skerr.Wrapf(err, "Error when creating CommitFooterVerifier")
		}
		clVerifiers = append(clVerifiers, commitFooterVerifier)

		// Verify that the change is not WIP.
		wipVerifier, err := NewWIPVerifier()
		if err != nil {
			return nil, nil, skerr.Wrapf(err, "Error when creating WIPVerifier")
		}
		clVerifiers = append(clVerifiers, wipVerifier)

		// Verify the change is submittable.
		submittableVerifier, err := NewSubmittableVerifier()
		if err != nil {
			return nil, nil, skerr.Wrapf(err, "Error when creating SubmittableVerifier")
		}
		clVerifiers = append(clVerifiers, submittableVerifier)

		if cfg.TreeStatusURL != "" {
			// Verify that the tree is open.
			treeStatusVerifier, err := NewTreeStatusVerifier(vm.httpClient, cfg.TreeStatusURL, footersMap)
			if err != nil {
				return nil, nil, skerr.Wrapf(err, "Error when creating TreeStatusVerifier")
			}
			clVerifiers = append(clVerifiers, treeStatusVerifier)
		}

		// Verify that the change can be submitted without throttling.
		throttlerVerifier, err := NewThrottlerVerifier(cfg.ThrottlerCfg, vm.throttlerManager)
		if err != nil {
			return nil, nil, skerr.Wrapf(err, "Error when creating ThrottlerVerifier")
		}
		clVerifiers = append(clVerifiers, throttlerVerifier)

	} else if vm.cr.IsDryRun(ctx, ci) {
		// Get dry-run list from the cache or set it if it does not exist.
		dryRunList, ok := vm.allowlistCache[cfg.DryRunAccessList]
		if !ok {
			dryRunList, err = allowed.NewAllowedFromChromeInfraAuth(vm.criaClient, cfg.DryRunAccessList)
			if err != nil {
				return nil, nil, skerr.Wrapf(err, "Could not create an allowed from %s", cfg.DryRunAccessList)
			}
			vm.allowlistCache[cfg.DryRunAccessList] = dryRunList
		}
		// Verify that the CQ+1 triggerer has access to run try jobs.
		dryRunVerifier, err := NewDryRunAccessListVerifier(vm.httpClient, dryRunList, cfg.DryRunAccessList)
		if err != nil {
			return nil, nil, skerr.Wrapf(err, "Error when creating DryRunVerifier")
		}
		clVerifiers = append(clVerifiers, dryRunVerifier)
	}

	// Verifiers common to both dry runs and CQ.

	if cfg.TasksJSONPath != "" {
		// Verify that try jobs ran.
		tasksCfg, err := configReader.GetTasksCfg(ctx, cfg.TasksJSONPath)
		if err != nil {
			return nil, nil, skerr.Wrapf(err, "Error getting tasks cfg")
		}
		tryJobsVerifier, err := NewTryJobsVerifier(vm.httpClient, vm.cr, tasksCfg, footersMap, cfg.VisibilityType)
		if err != nil {
			return nil, nil, skerr.Wrapf(err, "Error when creating TryJobsVerifier")
		}
		clVerifiers = append(clVerifiers, tryJobsVerifier)
	}

	togetherChangeIDs := []string{}
	for _, t := range togetherChanges {
		togetherChangeIDs = append(togetherChangeIDs, fmt.Sprintf("%d", t.Issue))
	}
	return clVerifiers, togetherChangeIDs, nil
}

// RunVerifiers implements the VerifierManager interface.
func (vm *SkCQVerifiersManager) RunVerifiers(ctx context.Context, ci *gerrit.ChangeInfo, verifiers []types.Verifier, startTime int64) []*types.VerifierStatus {
	verifierStatuses := []*types.VerifierStatus{}
	for _, v := range verifiers {
		status := &types.VerifierStatus{
			Name:    v.Name(),
			StartTs: startTime,
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
				status.StopTs = vmTimeNowFunc().Unix()
			case types.VerifierFailureState:
				status.StopTs = vmTimeNowFunc().Unix()
			}
		}
		verifierStatuses = append(verifierStatuses, status)
	}
	return verifierStatuses
}
