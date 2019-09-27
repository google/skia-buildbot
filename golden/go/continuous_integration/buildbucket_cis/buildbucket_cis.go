package buildbucket_cis

import (
	"context"
	"strconv"
	"strings"
	"time"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
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
	buildId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return ci.TryJob{}, skerr.Wrapf(err, "Invalid TryJob ID %q", id)
	}
	tj, err := c.bbClient.GetBuild(ctx, buildId)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return ci.TryJob{}, ci.ErrNotFound
		}
		return ci.TryJob{}, skerr.Wrapf(err, "fetching Tryjob %s from buildbucket", id)
	}
	ts := time.Unix(tj.CreateTime.Seconds, int64(tj.CreateTime.Nanos)).UTC()
	if tj.Status&buildbucketpb.Status_ENDED_MASK > 0 {
		ts = time.Unix(tj.EndTime.Seconds, int64(tj.EndTime.Nanos)).UTC()
	}
	return ci.TryJob{
		SystemID:    id,
		DisplayName: tj.Builder.Builder,
		Updated:     ts,
	}, nil
}

// Make sure CISImpl fulfills the continuous_integration.Client interface.
var _ ci.Client = (*CISImpl)(nil)
