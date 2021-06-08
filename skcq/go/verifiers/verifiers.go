package verifiers

import (
	"context"
	"fmt"
	"net/http"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/skcq/go/types"
	"go.skia.org/infra/skcq/go/config"
)

type Verifier interface {
	// Name of the verifier.
	Name() string

	// Verify runs the verifier and returns a VerifierState with a string
	// explaining why it is in that state.
	Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state types.VerifierState, reason string, err error)

	// Cleanup runs any cleanup tasks that the verifier needs to execute
	// when a change is removed from the CQ. Does not return an error
	// but all errors will be logged.
	Cleanup(ctx context.Context, ci *gerrit.ChangeInfo)
}

// GetVerifiers returns all the verifiers that apply to the specified change using the specified config.
// If isSubmittedTogetherChange change is true then it is treated as a CQ change and we do not check
// if the CQ+2 triggerer is a committer because it does not matter for submitted together changes.
func GetVerifiers(ctx context.Context, httpClient *http.Client, cfg *config.SkCQCfg, cr codereview.CodeReview, ci *gerrit.ChangeInfo, isSubmittedTogetherChange bool, gitilesRepo *gitiles.Repo, configReader *config.GitilesConfigReader) ([]Verifier, []*gerrit.ChangeInfo, error) {
	clVerifiers := []Verifier{}
	togetherChanges := []*gerrit.ChangeInfo{}

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
			cqVerifier, err := NewCQAccessListVerifier(httpClient, cfg.CommitterList)
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
				togetherChangesVerifier, err := NewSubmittedTogetherVerifier(ctx, togetherChanges, httpClient, cfg, cr, ci, gitilesRepo, configReader)
				if err != nil {
					return nil, nil, skerr.Fmt("Error when creating SubmittedTogetherVerifier: %s", err)
				}
				clVerifiers = append(clVerifiers, togetherChangesVerifier)
			}
		}

		// Verify that the change is not WIP.
		wipVerifier, err := NewWIPVerifier()
		if err != nil {
			return nil, nil, skerr.Fmt("Error when creating WIPVerifier: %s", err)
		}
		clVerifiers = append(clVerifiers, wipVerifier)

		// Verify the change has approval from a committer.
		approvedVerifier, err := NewApprovedVerifier(httpClient, cfg.CommitterList)
		if err != nil {
			return nil, nil, skerr.Fmt("Error when creating ApprovedVerifier: %s", err)
		}
		clVerifiers = append(clVerifiers, approvedVerifier)

		if cfg.TreeStatusURL != "" {
			// Verify that the tree is open.
			treeStatusVerifier, err := NewTreeStatusVerifier(httpClient, cfg.TreeStatusURL)
			if err != nil {
				return nil, nil, skerr.Fmt("Error when creating TreeStatusVerifier: %s", err)
			}
			clVerifiers = append(clVerifiers, treeStatusVerifier)
		}

	} else if cr.IsDryRun(ctx, ci) {
		// Verify that the CQ+1 triggerred has access to run try jobs.
		dryRunVerifier, err := NewDryRunAccessListVerifier(httpClient, cfg.DryRunAccessList)
		if err != nil {
			return nil, nil, skerr.Fmt("Error when creating DryRunVerifier: %s", err)
		}
		clVerifiers = append(clVerifiers, dryRunVerifier)
	}

	// Verifiers common to both dry runs and CQ.

	// Verify that try jobs ran.
	tasksCfg, err := configReader.GetTasksCfg(ctx, cfg.TasksJSONPath)
	if err != nil {
		return nil, nil, skerr.Fmt("Error getting tasks cfg: %s", err)
	}
	tryJobsVerifier, err := NewTryJobsVerifier(httpClient, cr, tasksCfg)
	if err != nil {
		return nil, nil, skerr.Fmt("Error when creating TryJobsVerifier: %s", err)
	}
	clVerifiers = append(clVerifiers, tryJobsVerifier)

	// // NOT WORKING
	// // Verify that the change has no merge conflicts.
	// mergeConflictVerifier, err := NewMergeConflictVerifier()
	// if err != nil {
	// 	return nil, skerr.Fmt("Error when creating MergeConflictVerifier: %s", err)
	// }
	// clVerifiers = append(clVerifiers, mergeConflictVerifier)

	return clVerifiers, togetherChanges, nil
}

// RunVerifiers runs all the specified verifiers for the change.
func RunVerifiers(ctx context.Context, ci *gerrit.ChangeInfo, verifiers []Verifier, startTime int64) (successMsgsFromVerfiers, waitMsgsFromVerifiers, rejectMsgFromVerifiers []string) {
	// TODO(Rmistry): What happens if we comment out the below
	successMsgsFromVerfiers = []string{}
	waitMsgsFromVerifiers = []string{}
	rejectMsgFromVerifiers = []string{}
	verifierStatuses := []types.
	for _, v := range verifiers {
		verifierState, reason, err := v.Verify(ctx, ci, startTime)
		if err != nil {
			// Should we always consider errors as transient errors? We will always log them for alerts.
			errMsg := fmt.Sprintf("%s: Hopefully a transient error: %s", v.Name(), err)
			sklog.Errorf(errMsg)
			waitMsgsFromVerifiers = append(waitMsgsFromVerifiers, errMsg)
		} else {
			switch verifierState {
			case types.VerifierSuccessState:
				successMsgsFromVerfiers = append(successMsgsFromVerfiers, reason)
			case types.VerifierWaitingState:
				waitMsgsFromVerifiers = append(waitMsgsFromVerifiers, reason)
			case types.VerifierFailureState:
				rejectMsgFromVerifiers = append(rejectMsgFromVerifiers, reason)
			}
		}
	}
	return successMsgsFromVerfiers, waitMsgsFromVerifiers, rejectMsgFromVerifiers
}

// GetAllowedVoters is a utility function that looks through the labels on a gerrit change to gather the
// email addresses of voters who voted the specified labelValue and who are in the allowedCRIA group.
func GetAllowedVoters(ci *gerrit.ChangeInfo, allowedCRIA *allowed.AllowedFromChromeInfraAuth, labelName string, labelValue int) []string {
	allowedVoters := []string{}
	if val, ok := ci.Labels[labelName]; ok {
		for _, ld := range val.All {
			if ld.Value == labelValue {
				if allowedCRIA.Member(ld.Email) {
					allowedVoters = append(allowedVoters, ld.Email)
				}
			}
		}
	}
	return allowedVoters
}
