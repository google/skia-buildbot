// Package dualtjstore contains an implementation of tjstore.Store that reads and writes to
// a primary implementation and writes to a secondary implementation. It is designed for migrating
// between two implementations.
package dualtjstore

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/go/skerr"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/tjstore"
)

type StoreImpl struct {
	primary   tjstore.Store
	secondary tjstore.Store
}

// New returns an implementation of tjstore.Store that writes to both provided tjstores but only
// reads from the primary.
func New(primary, secondary tjstore.Store) tjstore.Store {
	return &StoreImpl{
		primary:   primary,
		secondary: secondary,
	}
}

// GetTryJob implements tjstore.Store reading data from primary.
func (s *StoreImpl) GetTryJob(ctx context.Context, id, cisName string) (ci.TryJob, error) {
	return s.primary.GetTryJob(ctx, id, cisName)
}

// GetTryJobs implements tjstore.Store reading data from primary.
func (s *StoreImpl) GetTryJobs(ctx context.Context, psID tjstore.CombinedPSID) ([]ci.TryJob, error) {
	return s.primary.GetTryJobs(ctx, psID)
}

// GetResults implements tjstore.Store reading data from primary.
func (s *StoreImpl) GetResults(ctx context.Context, psID tjstore.CombinedPSID, updatedAfter time.Time) ([]tjstore.TryJobResult, error) {
	return s.primary.GetResults(ctx, psID, updatedAfter)
}

// PutTryJob implements tjstore.Store by writing data to the primary and secondary in parallel.
func (s *StoreImpl) PutTryJob(ctx context.Context, psID tjstore.CombinedPSID, tj ci.TryJob) error {
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return skerr.Wrapf(s.primary.PutTryJob(ctx, psID, tj), "primary PutTryJob failed")
	})
	eg.Go(func() error {
		return skerr.Wrapf(s.secondary.PutTryJob(ctx, psID, tj), "secondary PutTryJob failed")
	})
	return skerr.Wrap(eg.Wait())
}

// PutResults implements tjstore.Store by writing data to the primary and secondary in parallel.
func (s *StoreImpl) PutResults(ctx context.Context, psID tjstore.CombinedPSID, sourceFile string, r []tjstore.TryJobResult, ts time.Time) error {
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return skerr.Wrapf(s.primary.PutResults(ctx, psID, sourceFile, r, ts), "primary PutResults failed")
	})
	eg.Go(func() error {
		return skerr.Wrapf(s.secondary.PutResults(ctx, psID, sourceFile, r, ts), "secondary PutResults failed")
	})
	return skerr.Wrap(eg.Wait())
}
