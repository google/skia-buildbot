package verifiers

import (
	"context"
	"fmt"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/skcq/go/types"
)

var (
	// Allow lists will be cached here so that they are not continuously
	// instantiated at every poll iteration.
	AllowlistCache = map[string]*allowed.AllowedFromChromeInfraAuth{}
)

// Verifier is the interface implemented by all verifiers.
type Verifier interface {
	// Name of the verifier.
	Name() string

	// Verify runs the verifier and returns a VerifierState with a string
	// explaining why it is in that state.
	Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state types.VerifierState, reason string, err error)

	// Cleanup runs any cleanup tasks that the verifier needs to execute
	// when a change is removed from the CQ. Does not return an error
	// but all errors will be logged.
	Cleanup(ctx context.Context, ci *gerrit.ChangeInfo, cleanupPatchsetID int64)
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

// GetStatusStringsFromVerifierStatuses is a utility method to return user
// readable failure/waiting/success strings from VerifierStatuses
func GetStatusStringsFromVerifierStatuses(verifierStatuses []*types.VerifierStatus) (failureMsgs, waitingMsgs, successMsgs []string) {
	for _, vs := range verifierStatuses {
		if vs.State == types.VerifierFailureState {
			failureMsgs = append(failureMsgs, fmt.Sprintf("[%s]: %s", vs.Name, vs.Reason))
		} else if vs.State == types.VerifierWaitingState {
			waitingMsgs = append(waitingMsgs, fmt.Sprintf("[%s]: %s", vs.Name, vs.Reason))
		} else {
			successMsgs = append(successMsgs, fmt.Sprintf("[%s]: %s", vs.Name, vs.Reason))
		}
	}
	return failureMsgs, waitingMsgs, successMsgs
}
