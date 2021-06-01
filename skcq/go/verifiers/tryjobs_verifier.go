package verifiers

import (
	"fmt"
	"net/http"
	"strings"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
)

func NewTryJobsVerifier(httpClient *http.Client, criaGroup string) (Verifier, error) {
	committerAllowed, err := allowed.NewAllowedFromChromeInfraAuth(httpClient, criaGroup)
	if err != nil {
		return nil, skerr.Fmt("Could not create an allowed from %s: %s", criaGroup, err)
	}
	return &TryJobsVerifier{
		criaGroupName:    criaGroup,
		committerAllowed: committerAllowed,
	}, nil
}

type TryJobsVerifier struct {
	criaGroupName    string
	committerAllowed *allowed.AllowedFromChromeInfraAuth
}

func (av *TryJobsVerifier) Name() string {
	return "[TryJobsVerifier]"
}

func (av *TryJobsVerifier) Verify(ci *gerrit.ChangeInfo) (state VerifierState, reason string, err error) {
	committersWhoApproved := GetAllowedVoters(ci, av.committerAllowed, gerrit.LabelCodeReview, gerrit.LabelCodeReviewApprove)
	if len(committersWhoApproved) > 0 {
		return SuccessState, fmt.Sprintf("%s Approved by committers %s", av.Name(), strings.Join(committersWhoApproved, ",")), nil
	}
	// Implement waiting for approval only for owners who are committers?
	// Also, only if there are reviewers and if at least one is a committer.
	return FailureState, fmt.Sprintf("%s This CL requires approval from a committer", av.Name()), nil
}

func (cv *TryJobsVerifier) Cleanup(ci *gerrit.ChangeInfo) {
	return
}
