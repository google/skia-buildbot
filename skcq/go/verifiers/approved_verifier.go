package verifiers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/skcq/go/types"
)

func NewApprovedVerifier(httpClient *http.Client, committerAllowed *allowed.AllowedFromChromeInfraAuth, criaGroup string) (Verifier, error) {
	return &ApprovedVerifier{
		criaGroupName:    criaGroup,
		committerAllowed: committerAllowed,
	}, nil
}

type ApprovedVerifier struct {
	criaGroupName    string
	committerAllowed *allowed.AllowedFromChromeInfraAuth
}

func (av *ApprovedVerifier) Name() string {
	return "ApprovedVerifier"
}

func (av *ApprovedVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state types.VerifierState, reason string, err error) {
	committersWhoApproved := GetAllowedVoters(ci, av.committerAllowed, gerrit.LabelCodeReview, gerrit.LabelCodeReviewApprove)
	if len(committersWhoApproved) > 0 {
		return types.VerifierSuccessState, fmt.Sprintf("Approved by committers: %s", strings.Join(committersWhoApproved, ",")), nil
	}
	// Implement waiting for approval only for owners who are committers?
	// Also, only if there are reviewers and if at least one is a committer.
	return types.VerifierFailureState, "This CL requires approval from a committer", nil
}

func (cv *ApprovedVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo, cleanupPatchsetID int64) {
	return
}
