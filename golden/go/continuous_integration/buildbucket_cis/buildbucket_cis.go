package buildbucket_cis

import (
	"context"
	"strings"

	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/skerr"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"golang.org/x/time/rate"
)

const (
	// These values are arbitrary guesses, roughly based on values observed
	// by previous implementation in production.
	maxQPS   = rate.Limit(10.0)
	maxBurst = 40
)

type CISImpl struct {
	bbClient buildbucket.BuildBucketInterface
	rl       *rate.Limiter
}

func New(client buildbucket.BuildBucketInterface) *CISImpl {
	return &CISImpl{
		bbClient: client,
		rl:       rate.NewLimiter(maxQPS, maxBurst),
	}
}

// GetTryJob implements the continuous_integration.Client interface.
func (c *CISImpl) GetTryJob(ctx context.Context, id string) (ci.TryJob, error) {
	// Respect the rate limit.
	if err := c.rl.Wait(ctx); err != nil {
		return ci.TryJob{}, skerr.Wrap(err)
	}
	tj, err := c.bbClient.GetBuild(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return ci.TryJob{}, ci.ErrNotFound
		}
		return ci.TryJob{}, skerr.Wrapf(err, "fetching Tryjob %s from buildbucket", id)
	}
	ts := tj.Created
	if tj.Status == buildbucket.STATUS_COMPLETED {
		ts = tj.Completed
	}
	return ci.TryJob{
		SystemID:    id,
		DisplayName: tj.Parameters.BuilderName,
		Updated:     ts,
	}, nil
}

// Make sure CISImpl fulfills the continuous_integration.Client interface.
var _ ci.Client = (*CISImpl)(nil)
