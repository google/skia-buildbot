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
	st := statusToEnum(tj.Status)
	ts := tj.Created
	if st == ci.Complete {
		ts = tj.Completed
	}
	return ci.TryJob{
		SystemID: id,
		Status:   st,
		Updated:  ts,
	}, nil
}

func statusToEnum(s string) ci.TJStatus {
	switch s {
	case buildbucket.STATUS_STARTED:
		return ci.Running
	case buildbucket.STATUS_COMPLETED:
		return ci.Complete
	}
	// would only be possible if somehow BB thought the job was scheduled, even
	// though it had run.
	return ci.Running
}

// Make sure CISImpl fulfills the continuous_integration.Client interface.
var _ ci.Client = (*CISImpl)(nil)
