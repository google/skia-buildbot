package verifiers

import (
	"context"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/skcq/go/types"
	"go.skia.org/infra/skcq/go/verifiers/interfaces"
)

func NewSubmittableVerifier() (interfaces.Verifier, error) {
	return &SubmittableVerifier{}, nil
}

type SubmittableVerifier struct {
}

func (sv *SubmittableVerifier) Name() string {
	return "SubmittableVerifier"
}

func (sv *SubmittableVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state types.VerifierState, reason string, err error) {
	if ci.Submittable {
		return types.VerifierSuccessState, "CL is submittable", nil
	}
	return types.VerifierFailureState, "CL is not submittable", nil
}

func (sv *SubmittableVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo, cleanupPatchsetID int64) {
	return
}
