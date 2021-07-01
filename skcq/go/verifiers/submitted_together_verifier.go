package verifiers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/skcq/go/config"
	"go.skia.org/infra/skcq/go/footers"
	"go.skia.org/infra/skcq/go/types"
)

// NewSubmittedTogetherVerifier returns an instance of SubmittedTogetherVerifier.
func NewSubmittedTogetherVerifier(ctx context.Context, vm types.VerifiersManager, togetherChanges []*gerrit.ChangeInfo, httpClient *http.Client, skCQCfg *config.SkCQCfg, cr codereview.CodeReview, ci *gerrit.ChangeInfo, configReader config.ConfigReader, footersMap map[string]string) (types.Verifier, error) {
	togetherChangesToVerifiers := map[*gerrit.ChangeInfo][]types.Verifier{}
	for _, tc := range togetherChanges {
		tcVerifiers, _, err := vm.GetVerifiers(ctx, httpClient, skCQCfg, cr, tc, true /* isSubmittedTogetherChange */, configReader)
		if err != nil {
			return nil, skerr.Wrapf(err, "Could not get verifiers for the together change %d", tc.Issue)
		}
		togetherChangesToVerifiers[tc] = tcVerifiers
	}
	return &SubmittedTogetherVerifier{
		togetherChangesToVerifiers: togetherChangesToVerifiers,
		vm:                         vm,
		footersMap:                 footersMap,
	}, nil
}

// SubmittedTogetherVerifier implements the types.Verifier interface.
type SubmittedTogetherVerifier struct {
	togetherChangesToVerifiers map[*gerrit.ChangeInfo][]types.Verifier
	vm                         types.VerifiersManager
	footersMap                 map[string]string
}

// Name implements the types.Verifier interface.
func (stv *SubmittedTogetherVerifier) Name() string {
	return "SubmittedTogetherVerifier"
}

// Verify implements the types.Verifier interface.
func (stv *SubmittedTogetherVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state types.VerifierState, reason string, err error) {
	// If there are submitted together changes and NoDependencyChecks has been specified then return failure.
	if len(stv.togetherChangesToVerifiers) > 0 {
		noDepChecks := git.GetBoolFooterVal(stv.footersMap, footers.NoDependencyChecksFooter, ci.Issue)
		if noDepChecks {
			return types.VerifierFailureState, fmt.Sprintf("Failing because \"%s: %t\" has been specified and there are %d changes that will be submitted together", footers.NoDependencyChecksFooter, noDepChecks, len(stv.togetherChangesToVerifiers)), nil
		}
	}

	successMsgsAccrossAllChanges := []string{}
	for ts, tsVerifiers := range stv.togetherChangesToVerifiers {
		verifierStatuses := stv.vm.RunVerifiers(ctx, ts, tsVerifiers, startTime)
		failureMsgs, waitingMsgs, successMsgs := GetStatusStringsFromVerifierStatuses(verifierStatuses)
		successMsgsAccrossAllChanges = append(successMsgsAccrossAllChanges, successMsgs...)
		if len(failureMsgs) > 0 {
			return types.VerifierFailureState, fmt.Sprintf("Submitted together change https://skia-review.googlesource.com/c/%d has failed the verifiers:\n\n\t%s\n", ts.Issue, strings.Join(failureMsgs, "\n\t")), nil
		} else if len(waitingMsgs) > 0 {
			return types.VerifierWaitingState, fmt.Sprintf("Submitted together change https://skia-review.googlesource.com/c/%d is waiting for verifiers:\n\n\t%s\n", ts.Issue, strings.Join(waitingMsgs, "\n\t")), nil
		}
	}
	return types.VerifierSuccessState, fmt.Sprintf("Successfully ran all verifiers of submitted together changes:\n%s", strings.Join(successMsgsAccrossAllChanges, "\n")), nil
}

// Cleanup implements the types.Verifier interface.
func (stv *SubmittedTogetherVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo, cleanupPatchsetID int64) {
	// Loop through all verifiers of all together changes and run cleanup on them.
	for ts, tsVerifiers := range stv.togetherChangesToVerifiers {
		for _, tsVerifier := range tsVerifiers {
			tsVerifier.Cleanup(ctx, ts, cleanupPatchsetID)
		}
	}
	return
}
