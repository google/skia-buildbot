package verifiers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/skcq/go/types"
)

func NewDryRunAccessListVerifier(httpClient *http.Client, criaGroup string) (Verifier, error) {
	dryRunAllowed, err := allowed.NewAllowedFromChromeInfraAuth(httpClient, criaGroup)
	if err != nil {
		return nil, skerr.Fmt("Could not create an allowed from %s: %s", criaGroup, err)
	}
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

func (dv *DryRunAccessListVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo) {
	return
}
