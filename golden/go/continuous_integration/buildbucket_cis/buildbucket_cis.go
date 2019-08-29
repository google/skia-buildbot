package buildbucket_cis

import (
	"context"
	"strings"

	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/skerr"
	ci "go.skia.org/infra/golden/go/continuous_integration"
)

type CISImpl struct {
	bbClient buildbucket.BuildBucketInterface
}

func New(client buildbucket.BuildBucketInterface) *CISImpl {
	return &CISImpl{
		bbClient: client,
	}
}

// GetTryJob implements the continuous_integration.Client interface.
func (c *CISImpl) GetTryJob(ctx context.Context, id string) (ci.TryJob, error) {
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
