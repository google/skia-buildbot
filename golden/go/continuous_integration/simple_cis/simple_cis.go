// Package simple_cis is a simple implementation of CIS that
// pretends every TryJob exists. It can be used as a placeholder.
package simple_cis

import (
	"context"
	"time"

	ci "go.skia.org/infra/golden/go/continuous_integration"
)

type CISImpl struct {
	system string
}

func New(system string) *CISImpl {
	return &CISImpl{
		system: system,
	}
}

// GetTryJob implements the continuous_integration.Client interface.
func (c *CISImpl) GetTryJob(ctx context.Context, id string) (ci.TryJob, error) {
	return ci.TryJob{
		SystemID:    id,
		System:      c.system,
		DisplayName: id,
		Updated:     time.Now(),
	}, nil
}

// Make sure CISImpl fulfills the continuous_integration.Client interface.
var _ ci.Client = (*CISImpl)(nil)
