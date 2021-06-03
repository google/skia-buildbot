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
	"go.skia.org/infra/skcq/go/config"
)

type VerifierState string

const (
	SuccessState VerifierState = "Success"
	WaitingState VerifierState = "Waiting"
	FailureState VerifierState = "Failure"
)

type Verifier interface {
	// Name of the verifier.
	Name() string

	// UPDATE THIS DOC!!!
	// Verify runs the verifier.
	// If verification was not successful but SkCQ should wait for the result, then a waitMsg will be
	// returned.
	// If verification was not successful and SkCQ should fail the change, then a rejectMsg will be
	// returned.
	// If there is another infra related error then error will be non-nil.
	// Successful verification will return an empty waitMsg, an empty rejectMsg and nil error.
	// Verify(ci *gerrit.ChangeInfo) (waitMsg string, rejectMsg string, err error)
	Verify(ctx context.Context, ci *gerrit.ChangeInfo) (state VerifierState, reason string, err error)

	// Cleanup runs any cleanup tasks that the verifier needs to execute before the change is
	// removed from the CQ.
	Cleanup(ctx context.Context, ci *gerrit.ChangeInfo)

	// Add a SKIP? (looking at the footers of a ChangeInfo??)
}

// GetVerifiers returns all the verifiers that apply to the specified change using the specified config.
// If isSubmittedTogetherChange change is true then it is treated as a CQ change and we do not check
// if the CQ+2 triggerer is a committer because it does not matter for submitted together changes.
func GetVerifiers(ctx context.Context, httpClient *http.Client, cfg *config.SkCQCfg, cr codereview.CodeReview, ci *gerrit.ChangeInfo, isSubmittedTogetherChange bool, gitilesRepo *gitiles.Repo) ([]Verifier, error) {
	clVerifiers := []Verifier{}

	if isSubmittedTogetherChange || cr.IsCQ(ctx, ci) {
		if !isSubmittedTogetherChange {
			// Verify the CQ+2 triggerer is a committer.
			fmt.Println("DEBUGGING")
			fmt.Println(httpClient)
			fmt.Println(cfg.CommitterList)
			cqVerifier, err := NewCQAccessListVerifier(httpClient, cfg.CommitterList)
			if err != nil {
				return nil, skerr.Fmt("Error when creating CQAccessListVerifier: %s", err)
			}
			clVerifiers = append(clVerifiers, cqVerifier)

			// Verify all the submitted together changes (if any exist).
			togetherChanges, err := cr.GetSubmittedTogether(ctx, ci)
			if err != nil {
				return nil, skerr.Fmt("Error when getting submitted together chagnes for SubmittedTogetherVerifier: %s", err)
			}
			if len(togetherChanges) > 0 {
				togetherChangesVerifier, err := NewSubmittedTogetherVerifier(ctx, togetherChanges, httpClient, cfg, cr, ci, gitilesRepo)
				if err != nil {
					return nil, skerr.Fmt("Error when creating SubmittedTogetherVerifier: %s", err)
				}
				clVerifiers = append(clVerifiers, togetherChangesVerifier)
			}
		}

		// Verify that the change is not WIP.
		wipVerifier, err := NewWIPVerifier()
		if err != nil {
			return nil, skerr.Fmt("Error when creating WIPVerifier: %s", err)
		}
		clVerifiers = append(clVerifiers, wipVerifier)

		// Verify the change has approval from a committer.
		approvedVerifier, err := NewApprovedVerifier(httpClient, cfg.CommitterList)
		if err != nil {
			return nil, skerr.Fmt("Error when creating ApprovedVerifier: %s", err)
		}
		clVerifiers = append(clVerifiers, approvedVerifier)

		if cfg.TreeStatusURL != "" {
			// Verify that the tree is open.
			treeStatusVerifier, err := NewTreeStatusVerifier(httpClient, cfg.TreeStatusURL)
			if err != nil {
				return nil, skerr.Fmt("Error when creating TreeStatusVerifier: %s", err)
			}
			clVerifiers = append(clVerifiers, treeStatusVerifier)
		}

	} else if cr.IsDryRun(ctx, ci) {
		// Verify that the CQ+1 triggerred has access to run try jobs.
		dryRunVerifier, err := NewDryRunAccessListVerifier(httpClient, cfg.DryRunAccessList)
		if err != nil {
			return nil, skerr.Fmt("Error when creating DryRunVerifier: %s", err)
		}
		clVerifiers = append(clVerifiers, dryRunVerifier)
	}

	// Verifiers common to both dry runs and CQ.

	// Verify that try jobs ran.
	patchsetIDs := ci.GetPatchsetIDs()
	latestPatchsetID := patchsetIDs[len(patchsetIDs)-1]
	gerritChangeRef := fmt.Sprintf("%s%d/%d/%d", gerrit.ChangeRefPrefix, ci.Issue%100, ci.Issue, latestPatchsetID)
	tasksCfg, err := config.GetTasksCfg(ctx, gitilesRepo, ci.Project, gerritChangeRef, cfg.TasksJSONPath)
	if err != nil {
		return nil, skerr.Fmt("Error getting tasks cfg: %s", err)
	}
	fmt.Println("GOT TASKS CFG FROM")
	fmt.Println(gerritChangeRef)
	fmt.Println(len(tasksCfg.Jobs))
	tryJobsVerifier, err := NewTryJobsVerifier(httpClient, cr, tasksCfg)
	if err != nil {
		return nil, skerr.Fmt("Error when creating TryJobsVerifier: %s", err)
	}
	clVerifiers = append(clVerifiers, tryJobsVerifier)

	// // NOT WORKING
	// // Verify that the change has no merge conflicts.
	// mergeConflictVerifier, err := NewMergeConflictVerifier()
	// if err != nil {
	// 	return nil, skerr.Fmt("Error when creating MergeConflictVerifier: %s", err)
	// }
	// clVerifiers = append(clVerifiers, mergeConflictVerifier)

	return clVerifiers, nil
}

// RunVerifiers runs all the specified verifiers for the change.
func RunVerifiers(ctx context.Context, ci *gerrit.ChangeInfo, verifiers []Verifier) (successMsgsFromVerfiers, waitMsgsFromVerifiers, rejectMsgFromVerifiers []string) {
	// TODO(Rmistry): What happens if we comment out the below
	successMsgsFromVerfiers = []string{}
	waitMsgsFromVerifiers = []string{}
	rejectMsgFromVerifiers = []string{}
	for _, v := range verifiers {
		verifierState, reason, err := v.Verify(ctx, ci)
		if err != nil {
			// Should we always consider errors as transient errors? We will always log them for alerts.
			errMsg := fmt.Sprintf("%s: Hopefully a transient error: %s", v.Name(), err)
			sklog.Errorf(errMsg)
			waitMsgsFromVerifiers = append(waitMsgsFromVerifiers, errMsg)
		} else {
			switch verifierState {
			case SuccessState:
				successMsgsFromVerfiers = append(successMsgsFromVerfiers, reason)
			case WaitingState:
				waitMsgsFromVerifiers = append(waitMsgsFromVerifiers, reason)
			case FailureState:
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
