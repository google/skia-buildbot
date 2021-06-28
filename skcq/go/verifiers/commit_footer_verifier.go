package verifiers

import (
	"context"
	"fmt"

	"go.skia.org/infra/skcq/go/footers"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/skcq/go/types"
	"go.skia.org/infra/skcq/go/verifiers/interfaces"
)

func NewCommitFooterVerifier(footersMap map[string]string) (interfaces.Verifier, error) { 
	return &CommitFooterVerifier{
		footersMap: footersMap,
	}, nil
}

type CommitFooterVerifier struct {
	footersMap map[string]string
}

func (cv *CommitFooterVerifier) Name() string {
	return "CommitFooterVerifier"
}

func (cv *CommitFooterVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state types.VerifierState, reason string, err error) {
	commitFooter := footers.GetStringVal(cv.footersMap, footers.CommitFooter)
	if commitFooter == "false" {
		return types.VerifierFailureState, fmt.Sprintf("\"%s: %s\" has been specified. Cannot do CQ run.", footers.CommitFooter, commitFooter), nil
	}
	return types.VerifierSuccessState, fmt.Sprintf("%s is not specified.", footers.CommitFooter), nil
}

func (cv *CommitFooterVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo, cleanupPatchsetID int64) {
	return
}
