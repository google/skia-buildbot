// Package dualclstore contains an implementation of clstore.Store that reads and writes to
// a primary implementation and writes to a secondary implementation. It is designed for migrating
// between two implementations.
package dualclstore

import (
	"context"

	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/code_review"
)

type StoreImpl struct {
	primary   clstore.Store
	secondary clstore.Store
}

// New returns an implementation of clstore.Store that writes to both provided clstores but only
// reads from the primary.
func New(primary, secondary clstore.Store) clstore.Store {
	return &StoreImpl{
		primary:   primary,
		secondary: secondary,
	}
}

// GetChangelist implements clstore.Store by returning data from the primary store.
func (s *StoreImpl) GetChangelist(ctx context.Context, id string) (code_review.Changelist, error) {
	return s.primary.GetChangelist(ctx, id)
}

// GetPatchset implements clstore.Store by returning data from the primary store.
func (s *StoreImpl) GetPatchset(ctx context.Context, clID, psID string) (code_review.Patchset, error) {
	return s.primary.GetPatchset(ctx, clID, psID)
}

// GetPatchsetByOrder implements clstore.Store by returning data from the primary store.
func (s *StoreImpl) GetPatchsetByOrder(ctx context.Context, clID string, psOrder int) (code_review.Patchset, error) {
	return s.primary.GetPatchsetByOrder(ctx, clID, psOrder)
}

// GetChangelists implements clstore.Store by returning data from the primary store.
func (s *StoreImpl) GetChangelists(ctx context.Context, opts clstore.SearchOptions) ([]code_review.Changelist, int, error) {
	return s.primary.GetChangelists(ctx, opts)
}

// GetPatchsets implements clstore.Store by returning data from the primary store.
func (s *StoreImpl) GetPatchsets(ctx context.Context, clID string) ([]code_review.Patchset, error) {
	return s.primary.GetPatchsets(ctx, clID)
}

// PutChangelist implements clstore.Store by writing data to the primary and secondary in parallel.
func (s *StoreImpl) PutChangelist(ctx context.Context, cl code_review.Changelist) error {
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return skerr.Wrapf(s.primary.PutChangelist(ctx, cl), "primary PutChangelist failed")
	})
	eg.Go(func() error {
		return skerr.Wrapf(s.secondary.PutChangelist(ctx, cl), "secondary PutChangelist failed")
	})
	return skerr.Wrap(eg.Wait())
}

// PutPatchset implements clstore.Store by writing data to the primary and secondary in parallel.
func (s *StoreImpl) PutPatchset(ctx context.Context, ps code_review.Patchset) error {
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return skerr.Wrapf(s.primary.PutPatchset(ctx, ps), "primary PutPatchset failed")
	})
	eg.Go(func() error {
		return skerr.Wrapf(s.secondary.PutPatchset(ctx, ps), "secondary PutPatchset failed")
	})
	return skerr.Wrap(eg.Wait())
}
