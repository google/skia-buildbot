package verifiers

import (
	"context"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/skcq/go/types"
)

func NewWIPVerifier() (Verifier, error) {
	return &WIPVerifier{}, nil
}

type WIPVerifier struct{}

func (wv *WIPVerifier) Name() string {
	return "WIPVerifier"
}

func (wv *WIPVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state types.VerifierState, reason string, err error) {
	if ci.WorkInProgress {
		return types.VerifierFailureState, "SkCQ cannot submit a WIP change", nil
	}
	return types.VerifierSuccessState, "This CL is not WIP", nil
}

func (wv *WIPVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo, cleanupPatchsetID int64) {
	return
}
