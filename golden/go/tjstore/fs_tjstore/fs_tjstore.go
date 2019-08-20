// Package fs_tjstore implements the tjstore.Store interface with
// a FireStore backend.
package fs_tjstore

import (
	"context"
	"errors"

	"go.skia.org/infra/go/firestore"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/tjstore"
)

type StoreImpl struct {
	fsClient *firestore.Client
	cisName  string
}

func New(client *firestore.Client, cisName string) *StoreImpl {
	return &StoreImpl{
		fsClient: client,
		cisName:  cisName,
	}
}

// GetTryJob implements the tjstore.Store interface.
func (s *StoreImpl) GetTryJob(ctx context.Context, id string) (ci.TryJob, error) {
	return ci.TryJob{}, errors.New("not impl")
}

// GetRunningTryJobs implements the tjstore.Store interface.
func (s *StoreImpl) GetRunningTryJobs(ctx context.Context) ([]ci.TryJob, error) {
	return []ci.TryJob{}, errors.New("not impl")
}

// GetResults implements the tjstore.Store interface.
func (s *StoreImpl) GetResults(ctx context.Context, psID tjstore.CombinedPSID) ([]tjstore.TryJobResult, error) {
	return []tjstore.TryJobResult{}, errors.New("not impl")
}

// PutTryJob implements the tjstore.Store interface.
func (s *StoreImpl) PutTryJob(ctx context.Context, psID tjstore.CombinedPSID, tj ci.TryJob) error {
	return errors.New("not impl")
}

// PutResults implements the tjstore.Store interface.
func (s *StoreImpl) PutResults(ctx context.Context, psID tjstore.CombinedPSID, r []tjstore.TryJobResult) error {
	return errors.New("not impl")
}

// Make sure StoreImpl fulfills the tjstore.Store interface.
var _ tjstore.Store = (*StoreImpl)(nil)
