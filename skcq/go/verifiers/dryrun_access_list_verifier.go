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

// NewDryRunAccessListVerifier returns an instance of DryRunAccessListVerifier.
func NewDryRunAccessListVerifier(httpClient *http.Client, dryRunAllowed allowed.Allow, criaGroup string) (types.Verifier, error) {
	return &DryRunAccessListVerifier{
		criaGroupName: criaGroup,
		dryRunAllowed: dryRunAllowed,
	}, nil
}

// DryRunAccessListVerifier implements the types.Verifier interface.
type DryRunAccessListVerifier struct {
	criaGroupName string
	dryRunAllowed allowed.Allow
}

// Name implements the types.Verifier interface.
func (dv *DryRunAccessListVerifier) Name() string {
	return "DryRunAccessListVerifier"
}

// Verify implements the types.Verifier interface.
func (dv *DryRunAccessListVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state types.VerifierState, reason string, err error) {
	allowedDryRunVoters := GetAllowedVoters(ci, dv.dryRunAllowed, gerrit.LabelCommitQueue, gerrit.LabelCommitQueueDryRun)
	if len(allowedDryRunVoters) > 0 {
		return types.VerifierSuccessState, fmt.Sprintf("CQ+1 voted by allowed dry-run voters: %s", strings.Join(allowedDryRunVoters, ",")), nil
	}
	return types.VerifierFailureState, "CQ+1 requires a vote by an allowed dry-run voter", nil
}

// Cleanup implements the types.Verifier interface.
func (dv *DryRunAccessListVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo, cleanupPatchsetID int64) {
	return
}
