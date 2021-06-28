package verifiers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/skcq/go/types"
	"go.skia.org/infra/skcq/go/verifiers/interfaces"
)

func NewDryRunAccessListVerifier(httpClient *http.Client, dryRunAllowed *allowed.AllowedFromChromeInfraAuth, criaGroup string) (interfaces.Verifier, error) {
	return &DryRunAccessListVerifier{
		criaGroupName: criaGroup,
		dryRunAllowed: dryRunAllowed,
	}, nil
}

type DryRunAccessListVerifier struct {
	criaGroupName string
	dryRunAllowed *allowed.AllowedFromChromeInfraAuth
}

func (dv *DryRunAccessListVerifier) Name() string {
	return "DryRunAccessListVerifier"
}

func (dv *DryRunAccessListVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state types.VerifierState, reason string, err error) {
	allowedDryRunVoters := GetAllowedVoters(ci, dv.dryRunAllowed, gerrit.LabelCommitQueue, gerrit.LabelCommitQueueDryRun)
	if len(allowedDryRunVoters) > 0 {
		return types.VerifierSuccessState, fmt.Sprintf("CQ+1 voted by allowed dry-run voters: %s", strings.Join(allowedDryRunVoters, ",")), nil
	}
	return types.VerifierFailureState, "CQ+1 requires a vote by an allowed dry-run voter", nil
}

func (dv *DryRunAccessListVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo, cleanupPatchsetID int64) {
	return
}
