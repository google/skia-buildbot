package verifiers

import (
	"context"
	"fmt"

	"go.skia.org/infra/go/gerrit"
)

func NewWIPVerifier() (Verifier, error) {
	return &WIPVerifier{}, nil
}

type WIPVerifier struct{}

func (wv *WIPVerifier) Name() string {
	return "[WIPVerifier]"
}

func (wv *WIPVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state VerifierState, reason string, err error) {
	if ci.WorkInProgress {
		return FailureState, fmt.Sprintf("%s SkCQ cannot submit a WIP change", wv.Name()), nil
	}
	return SuccessState, fmt.Sprintf("%s This CL is not WIP", wv.Name()), nil
}

func (wv *WIPVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo) {
	return
}
