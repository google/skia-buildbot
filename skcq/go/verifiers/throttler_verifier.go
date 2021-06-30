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

// TODO(rmistry): The parameter list here and for GetVerifiers is massive..
func NewThrottlerVerifier(throttlerCfg *config.ThrottlerCfg) (types.Verifier, error) {
	if throttlerCfg == nil {
		throttlerCfg = throttler.GetDefaultThrottlerCfg()
	}
	return &ThrottlerVerifier{
		throttlerCfg: throttlerCfg,
	}, nil
}

type ThrottlerVerifier struct {
	throttlerCfg *config.ThrottlerCfg
}

func (tv *ThrottlerVerifier) Name() string {
	return "ThrottlerVerifier"
}

func (tv *ThrottlerVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state types.VerifierState, reason string, err error) {
	repoBranch := fmt.Sprintf("%s/%s", ci.Project, ci.Branch)
	throttle := throttler.Throttle(repoBranch, time.Now())
	if throttle {
		return types.VerifierWaitingState, fmt.Sprintf("SkCQ has committed %d changes in %d seconds. Waiting to submit this change", tv.throttlerCfg.MaxBurst, tv.throttlerCfg.BurstDelaySecs), nil
	} else {
		return types.VerifierSuccessState, "Change can be submitted and does not need to be throttled", nil
	}
}

func (tv *ThrottlerVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo, cleanupPatchsetID int64) {
	return
}
