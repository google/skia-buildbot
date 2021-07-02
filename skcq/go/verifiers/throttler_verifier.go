package verifiers

import (
	"context"
	"fmt"
	"time"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/skcq/go/config"
	"go.skia.org/infra/skcq/go/throttler"
	"go.skia.org/infra/skcq/go/types"
)

// NewThrottlerVerifier returns an instance of ThrottlerVerifier.
func NewThrottlerVerifier(throttlerCfg *config.ThrottlerCfg, throttlerManager types.ThrottlerManager) (types.Verifier, error) {
	if throttlerCfg == nil {
		throttlerCfg = throttler.GetDefaultThrottlerCfg()
	}
	return &ThrottlerVerifier{
		throttlerCfg:     throttlerCfg,
		throttlerManager: throttlerManager,
	}, nil
}

// ThrottlerVerifier implements the types.Verifier interface.
type ThrottlerVerifier struct {
	throttlerCfg     *config.ThrottlerCfg
	throttlerManager types.ThrottlerManager
}

// Name implements the types.Verifier interface.
func (tv *ThrottlerVerifier) Name() string {
	return "ThrottlerVerifier"
}

// Verify implements the types.Verifier interface.
func (tv *ThrottlerVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state types.VerifierState, reason string, err error) {
	repoBranch := fmt.Sprintf("%s/%s", ci.Project, ci.Branch)
	throttle := tv.throttlerManager.Throttle(repoBranch, time.Now())
	if throttle {
		return types.VerifierWaitingState, fmt.Sprintf("SkCQ has committed %d changes in %d seconds. Waiting to submit this change", tv.throttlerCfg.MaxBurst, tv.throttlerCfg.BurstDelaySecs), nil
	} else {
		return types.VerifierSuccessState, "Change can be submitted and does not need to be throttled", nil
	}
}

// Cleanup implements the types.Verifier interface.
func (tv *ThrottlerVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo, cleanupPatchsetID int64) {
	return
}
