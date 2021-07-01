package verifiers

import (
	"context"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/skcq/go/types"
)

// NewWIPVerifier returns an instance of WIPVerifier.
func NewWIPVerifier() (types.Verifier, error) {
	return &WIPVerifier{}, nil
}

// WIPVerifier implements the types.Verifier interface.
type WIPVerifier struct{}

// Name implements the types.Verifier interface.
func (wv *WIPVerifier) Name() string {
	return "WIPVerifier"
}

// Verify implements the types.Verifier interface.
func (wv *WIPVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state types.VerifierState, reason string, err error) {
	if ci.WorkInProgress {
		return types.VerifierFailureState, "SkCQ cannot submit a WIP change", nil
	}
	return types.VerifierSuccessState, "This CL is not WIP", nil
}

// Cleanup implements the types.Verifier interface.
func (wv *WIPVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo, cleanupPatchsetID int64) {
	return
}
