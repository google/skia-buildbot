package fs_failurestore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore/failurestore"
	"go.skia.org/infra/golden/go/types"
	"golang.org/x/sync/errgroup"
)

const (
	// Firestore collection name.
	failureStoreCollection = "failurestore_failures"

	maxWriteAttempts = 5
	maxOperationTime = time.Minute
)

// FSImpl is the Firestore-backed implementation of MetricsStore.
type FSImpl struct {
	client *ifirestore.Client

	cachedFailures    map[types.Digest]*diff.DigestFailure // Caches all failures for fast lookup.
	cachedFailuresMux sync.RWMutex
}

// storeEntry represents a failure that is stored in Firestore.
type storeEntry struct {
	Digest types.Digest `firestore:"digest"`
	Reason diff.DiffErr `firestore:"reason"`
	TS     int64        `firestore:"ts"` // in milliseconds since the epoch
}

// toDigestFailure converts a storeEntry into a *diff.DigestFailure.
func (e storeEntry) toDigestFailure() *diff.DigestFailure {
	return &diff.DigestFailure{
		Digest: e.Digest,
		Reason: e.Reason,
		TS:     e.TS,
	}
}

// toStoreEntry converts a *diff.DigestFailure into a *storeEntry.
func toStoreEntry(failure *diff.DigestFailure) *storeEntry {
	return &storeEntry{
		Digest: failure.Digest,
		Reason: failure.Reason,
		TS:     failure.TS,
	}
}

// New returns a new instance of the Firestore-backed failurestore.FailureStore implementation.
func New(client *ifirestore.Client) *FSImpl {
	return &FSImpl{
		client:         client,
		cachedFailures: make(map[types.Digest]*diff.DigestFailure),
	}
}

// UnavailableDigests implements the failurestore.FailureStore interface.
func (s *FSImpl) UnavailableDigests() map[types.Digest]*diff.DigestFailure {
	s.cachedFailuresMux.RLock()
	defer s.cachedFailuresMux.RUnlock()
	return s.cachedFailures
}

// AddDigestFailureIfNew implements the failurestore.FailureStore interface.
func (s *FSImpl) AddDigestFailureIfNew(failure *diff.DigestFailure) error {
	unavailable := s.UnavailableDigests()
	if _, ok := unavailable[failure.Digest]; !ok {
		return s.AddDigestFailure(failure)
	}
	return nil
}

// AddDigestFailure implements the failurestore.FailureStore interface.
func (s *FSImpl) AddDigestFailure(failure *diff.DigestFailure) error {
	defer metrics2.FuncTimer().Stop()
	ctx := context.TODO() // TODO(lovisolo): Add a ctx argument to the interface method.

	// Write failure to Firestore.
	docRef := s.digestFailureToDocRef(failure)
	entry := toStoreEntry(failure)
	if _, err := s.client.Set(ctx, docRef, entry, maxWriteAttempts, maxOperationTime); err != nil {
		return skerr.Wrapf(err, "writing failure to Firestore (Digest=%q): %v", failure.Digest, failure)
	}

	// Refresh failures cache.
	return s.loadDigestFailures()
}

// PurgeDigestFailures implements the failurestore.FailureStore interface.
func (s *FSImpl) PurgeDigestFailures(digests types.DigestSlice) error {
	defer metrics2.FuncTimer().Stop()
	ctx := context.TODO() // TODO(lovisolo): Add a ctx argument to the interface method.

	// Gather *firestore.DocumentRefs to delete.
	targetDocRefs := make([]*firestore.DocumentRef, 0, len(digests))
	unavailableDigests := s.UnavailableDigests()
	for _, d := range digests {
		if _, ok := unavailableDigests[d]; ok {
			targetDocRefs = append(targetDocRefs, s.digestFailureToDocRef(unavailableDigests[d]))
		}
	}

	// Parallel-delete them.
	g, grpCtx := errgroup.WithContext(ctx)
	for _, docRef := range targetDocRefs {
		g.Go(func() error {
			_, err := s.client.Delete(grpCtx, docRef, maxWriteAttempts, maxOperationTime)
			if err != nil {
				return skerr.Wrapf(err, "error deleting failure with Firestore document ID %q", docRef.ID)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return skerr.Wrap(err)
	}

	// Refresh cache.
	return s.loadDigestFailures()
}

// loadDigestFailures loads and caches the digest failures in memory.
func (s *FSImpl) loadDigestFailures() error {
	defer metrics2.FuncTimer().Stop()
	ctx := context.TODO() // TODO(lovisolo): Add a ctx argument to the interface method.

	// Retrieve all failures.
	docs, err := s.client.Collection(failureStoreCollection).Documents(ctx).GetAll()
	if err != nil {
		return skerr.Wrap(err)
	}

	// Clean cache.
	s.cachedFailuresMux.Lock()
	defer s.cachedFailuresMux.Unlock()
	s.cachedFailures = make(map[types.Digest]*diff.DigestFailure)

	// Populate cache.
	for _, doc := range docs {
		entry := storeEntry{}
		if err := doc.DataTo(&entry); err != nil {
			return skerr.Wrap(err)
		}
		s.cachedFailures[entry.Digest] = entry.toDigestFailure()
	}

	return nil
}

// digestFailureToDocRef takes a digest failure and returns the *firestore.DocumentRef of its
// corresponding entry in Firestore.
func (s *FSImpl) digestFailureToDocRef(failure *diff.DigestFailure) *firestore.DocumentRef {
	id := fmt.Sprintf("%s-%d", failure.Digest, failure.TS)
	return s.client.Collection(failureStoreCollection).Doc(id)
}

// Make sure FSImpl fulfills the FailureStore interface
var _ failurestore.FailureStore = (*FSImpl)(nil)
