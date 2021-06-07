package verifiers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
)

func NewCQAccessListVerifier(httpClient *http.Client, criaGroup string) (Verifier, error) {
	// Instatiate this once and pass it in because it's used in the other place as well.
	cqAllowed, err := allowed.NewAllowedFromChromeInfraAuth(httpClient, criaGroup)
	if err != nil {
		return nil, skerr.Fmt("Could not create an allowed from %s: %s", criaGroup, err)
	}
	return &CQAccessListVerifier{
		criaGroupName: criaGroup,
		cqAllowed:     cqAllowed,
	}, nil
}

type CQAccessListVerifier struct {
	criaGroupName string
	cqAllowed     *allowed.AllowedFromChromeInfraAuth
}

func (cv *CQAccessListVerifier) Name() string {
	return "[CQAccessListVerifier]"
}

func (cv *CQAccessListVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state VerifierState, reason string, err error) {
	committersWhoCQed := GetAllowedVoters(ci, cv.cqAllowed, gerrit.LabelCommitQueue, gerrit.LabelCommitQueueSubmit)
	if len(committersWhoCQed) > 0 {
		return SuccessState, fmt.Sprintf("%s CQ+2 voted by committers %s", cv.Name(), strings.Join(committersWhoCQed, ",")), nil
	}
	return FailureState, fmt.Sprintf("%s CQ+2 requires a vote from a committer", cv.Name()), nil
}

func (cv *CQAccessListVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo) {
	return
}
