package verifiers

import (
	"fmt"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/skcq/go/types"
)

// GetAllowedVoters is a utility function that looks through the labels on a gerrit change to gather the
// email addresses of voters who voted the specified labelValue and who are in the allow group.
func GetAllowedVoters(ci *gerrit.ChangeInfo, allow allowed.Allow, labelName string, labelValue int) []string {
	allowedVoters := []string{}
	if val, ok := ci.Labels[labelName]; ok {
		for _, ld := range val.All {
			if ld.Value == labelValue {
				if allow.Member(ld.Email) {
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
