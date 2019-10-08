package bolt_and_fs_metricsstore

import (
	"sync"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore/metricsstore/bolt_metricsstore"
	"go.skia.org/infra/golden/go/diffstore/metricsstore/fs_metricsstore"
	"go.skia.org/infra/golden/go/types"
)

// This package provides a metricsstore.MetricsStore implementation to assist with the migration
// from the current Bolt-backed MetricsStore implementation to the new Firestore-backed one.
//
// From the client's perspective, this implementation acts like a wrapper around the Bolt-backed
// implementation and should be indistinguishable from it.
//
// However, it also holds an instance of the Firestore-backed implementation. Whenever a method from
// its public API is called (e.g. LoadDiffMetrics), it executes that same method on instances in
// parallel, logs any errors returned by the Firestore-backed instance, and  passes the Bolt-backed
// instances's return values back to the client.
//
// This will allow us to detect any errors in the Firestore-backed implementation without disrupting
// the user, by looking at the diffserver logs and the corresponding Firestore collections.

// StoreImpl is an implementation of metricsstore.MetricsStore that acts as a wrapper around the
// Firestore- and the Bolt-backed implementations of MetricsStore. See package-level comments for
// details.
type StoreImpl struct {
	boltStore *bolt_metricsstore.BoltImpl
	fsStore   *fs_metricsstore.StoreImpl
}

// New returns a metricsstore.MetricsStore instance backed both by Bolt and Firestore. See
// package-level comments for details.
func New(boltBaseDir string, client *ifirestore.Client) (*StoreImpl, error) {
	boltStore, err := bolt_metricsstore.New(boltBaseDir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &StoreImpl{
		boltStore: boltStore,
		fsStore:   fs_metricsstore.New(client),
	}, nil
}

// PurgeDiffMetrics implements the metricsstore.MetricsStore interface.
func (s *StoreImpl) PurgeDiffMetrics(digests types.DigestSlice) error {
	wg := sync.WaitGroup{} // No need for an errgroup as we only return the error returned by the Bolt implementation.
	wg.Add(2)
	var boltErr, fsErr error

	// Bolt-backed implementation.
	go func() {
		defer wg.Done()
		boltErr = s.boltStore.PurgeDiffMetrics(digests)
	}()

	// Firestore-backed implementation.
	go func() {
		defer wg.Done()
		fsErr = s.fsStore.PurgeDiffMetrics(digests)
	}()

	wg.Wait()
	if fsErr != nil {
		sklog.Errorf("Firestore error: %s", fsErr)
	}
	return boltErr
}

// SaveDiffMetrics implements the metricsstore.MetricsStore interface.
func (s *StoreImpl) SaveDiffMetrics(id string, diffMetrics *diff.DiffMetrics) error {
	wg := sync.WaitGroup{} // No need for an errgroup as we only return the error returned by the Bolt implementation.
	wg.Add(2)
	var boltErr, fsErr error

	// Bolt-backed implementation.
	go func() {
		defer wg.Done()
		boltErr = s.boltStore.SaveDiffMetrics(id, diffMetrics)
	}()

	// Firestore-backed implementation.
	go func() {
		defer wg.Done()
		fsErr = s.fsStore.SaveDiffMetrics(id, diffMetrics)
	}()

	wg.Wait()
	if fsErr != nil {
		sklog.Errorf("Firestore error: %s", fsErr)
	}
	return boltErr
}

// LoadDiffMetrics implements the metricsstore.MetricsStore interface.
func (s *StoreImpl) LoadDiffMetrics(id string) (*diff.DiffMetrics, error) {
	wg := sync.WaitGroup{} // No need for an errgroup as we only return the error returned by the Bolt implementation.
	wg.Add(2)
	var diffMetrics *diff.DiffMetrics
	var boltErr, fsErr error

	// Bolt-backed implementation.
	go func() {
		defer wg.Done()
		diffMetrics, boltErr = s.boltStore.LoadDiffMetrics(id)
	}()

	// Firestore-backed implementation.
	go func() {
		defer wg.Done()
		_, fsErr = s.fsStore.LoadDiffMetrics(id)
	}()

	wg.Wait()
	if fsErr != nil {
		sklog.Errorf("Firestore error: %s", fsErr)
	}
	return diffMetrics, boltErr
}
