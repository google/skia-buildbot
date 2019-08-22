package buildbucket_cis

import (
	"context"

	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/continuous_integration"
)

type CISImpl struct {
	bbClient buildbucket.BuildBucketInterface
}

func New(client buildbucket.BuildBucketInterface, bucket string) *CISImpl {
	return &CISImpl{
		bbClient: client,
	}
}

// GetTryJob implements the continuous_integration.Client interface.
func (c *CISImpl) GetTryJob(ctx context.Context, id string) (continuous_integration.TryJob, error) {
	tj, err := c.bbClient.GetBuild(ctx, id)
	if err != nil {
		return continuous_integration.TryJob{}, skerr.Wrapf(err, "fetching Tryjob %s from buildbucket", id)
	}
	return continuous_integration.TryJob{
		SystemID: id,
		Status:   statusToEnum(tj.Status),
		Updated:  tj.Completed,
	}, nil
}

func statusToEnum(s string) continuous_integration.TJStatus {
	switch s {
	case buildbucket.STATUS_STARTED:
		return continuous_integration.Running
	case buildbucket.STATUS_COMPLETED:
		return continuous_integration.Complete
	}
	// would only be possible if somehow BB thought the job was scheduled, even
	// though it had run.
	return continuous_integration.Running
}

// Make sure CISImpl fulfills the continuous_integration.Client interface.
var _ continuous_integration.Client = (*CISImpl)(nil)
