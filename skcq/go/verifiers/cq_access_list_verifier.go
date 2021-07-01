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

// NewCQAccessListVerifier returns an instance of CQAccessListVerifier.
func NewCQAccessListVerifier(httpClient *http.Client, cqAllowed allowed.Allow, criaGroup string) (types.Verifier, error) {
	return &CQAccessListVerifier{
		criaGroupName: criaGroup,
		cqAllowed:     cqAllowed,
	}, nil
}

// CQAccessListVerifier implements the types.Verifier interface.
type CQAccessListVerifier struct {
	criaGroupName string
	cqAllowed     allowed.Allow
}

// Name implements the types.Verifier interface.
func (cv *CQAccessListVerifier) Name() string {
	return "CQAccessListVerifier"
}

// Verify implements the types.Verifier interface.
func (cv *CQAccessListVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state types.VerifierState, reason string, err error) {
	committersWhoCQed := GetAllowedVoters(ci, cv.cqAllowed, gerrit.LabelCommitQueue, gerrit.LabelCommitQueueSubmit)
	if len(committersWhoCQed) > 0 {
		return types.VerifierSuccessState, fmt.Sprintf("CQ+2 has been voted by committers: %s", strings.Join(committersWhoCQed, ",")), nil
	}
	return types.VerifierFailureState, "CQ+2 requires a vote from a committer", nil
}

// Cleanup implements the types.Verifier interface.
func (cv *CQAccessListVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo, cleanupPatchsetID int64) {
	return
}
