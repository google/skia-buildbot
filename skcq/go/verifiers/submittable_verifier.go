package verifiers

import (
	"context"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/skcq/go/types"
)

// NewSubmittableVerifier returns an instance of SubmittableVerifier.
func NewSubmittableVerifier() (types.Verifier, error) {
	return &SubmittableVerifier{}, nil
}

// SubmittableVerifier implements the types.Verifier interface.
type SubmittableVerifier struct{}

// Name implements the types.Verifier interface.
func (sv *SubmittableVerifier) Name() string {
	return "SubmittableVerifier"
}

// Verify implements the types.Verifier interface.
func (sv *SubmittableVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state types.VerifierState, reason string, err error) {
	if ci.Submittable {
		return types.VerifierSuccessState, "CL is submittable", nil
	}
	return types.VerifierFailureState, "CL is not submittable", nil
}

// Cleanup implements the types.Verifier interface.
func (sv *SubmittableVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo, cleanupPatchsetID int64) {
	return
}
