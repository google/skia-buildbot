package buildbucket_cis

import (
	"context"
	"errors"

	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/golden/go/continuous_integration"
)

type CISImpl struct {
	bbClient *buildbucket.Client
	bucket   string
}

func New(client *buildbucket.Client, bucket string) (*CISImpl, error) {
	return &CISImpl{
		bbClient: client,
		bucket:   bucket,
	}, nil
}

func (c *CISImpl) GetTryJob(ctx context.Context, id string) (continuous_integration.TryJob, error) {
	return continuous_integration.TryJob{}, errors.New("not impl")
}

// Make sure CISImpl fulfills the continuous_integration.Client Interface
var _ continuous_integration.Client = (*CISImpl)(nil)
