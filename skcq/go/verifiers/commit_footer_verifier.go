package verifiers

import (
	"context"
	"fmt"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/skcq/go/footers"
	"go.skia.org/infra/skcq/go/types"
)

// NewCommitFooterVerifier returns an instance of CommitFooterVerifier.
func NewCommitFooterVerifier(footersMap map[string]string) (types.Verifier, error) {
	return &CommitFooterVerifier{
		footersMap: footersMap,
	}, nil
}

// CommitFooterVerifier implements the types.Verifier interface.
type CommitFooterVerifier struct {
	footersMap map[string]string
}

// Name implements the types.Verifier interface.
func (cv *CommitFooterVerifier) Name() string {
	return "CommitFooterVerifier"
}

// Verify implements the types.Verifier interface.
func (cv *CommitFooterVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state types.VerifierState, reason string, err error) {
	commitFooter := git.GetStringFooterVal(cv.footersMap, footers.CommitFooter)
	if commitFooter == "false" {
		return types.VerifierFailureState, fmt.Sprintf("\"%s: %s\" has been specified. Cannot do CQ run.", footers.CommitFooter, commitFooter), nil
	}
	return types.VerifierSuccessState, fmt.Sprintf("%s is not specified.", footers.CommitFooter), nil
}

// Cleanup implements the types.Verifier interface.
func (cv *CommitFooterVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo, cleanupPatchsetID int64) {
	return
}
