package fs_failurestore

import (
	"context"
	"fmt"
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
}

// storeEntry represents a failure that is stored in Firestore.
type storeEntry struct {
	Digest types.Digest `firestore:"digest"`
	Reason diff.DiffErr `firestore:"reason"`
	TS     time.Time    `firestore:"ts"` // in milliseconds since the epoch
}

// toDigestFailure converts a storeEntry into a *diff.DigestFailure.
func (e storeEntry) toDigestFailure() *diff.DigestFailure {
	return &diff.DigestFailure{
		Digest: e.Digest,
		Reason: e.Reason,
		TS:     e.TS.UnixNano() / int64(time.Millisecond), // This is expressed in milliseconds since epoch.
	}
}

// toStoreEntry converts a *diff.DigestFailure into a *storeEntry.
func toStoreEntry(failure *diff.DigestFailure) *storeEntry {
	// Convert failure.TS (which is expressed in milliseconds since epoch) into a time.Time instance.
	nanosSinceEpoch := int64(time.Duration(failure.TS) * time.Millisecond)
	timestamp := time.Unix(0, nanosSinceEpoch)

	return &storeEntry{
		Digest: failure.Digest,
		Reason: failure.Reason,
		TS:     timestamp,
	}
}

// New returns a new instance of the Firestore-backed failurestore.FailureStore implementation.
func New(client *ifirestore.Client) *FSImpl {
	return &FSImpl{
		client: client,
	}
}

// UnavailableDigests implements the failurestore.FailureStore interface.
func (s *FSImpl) UnavailableDigests(ctx context.Context) (map[types.Digest]*diff.DigestFailure, error) {
	defer metrics2.FuncTimer().Stop()

	// Retrieve all failures.
	docs, err := s.client.Collection(failureStoreCollection).Documents(ctx).GetAll()
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Populate results.
	results := make(map[types.Digest]*diff.DigestFailure, len(docs))
	for _, doc := range docs {
		entry := storeEntry{}
		if err := doc.DataTo(&entry); err != nil {
			return nil, skerr.Wrap(err)
		}
		results[entry.Digest] = entry.toDigestFailure()
	}

	return results, nil
}

// AddDigestFailure implements the failurestore.FailureStore interface.
func (s *FSImpl) AddDigestFailure(ctx context.Context, failure *diff.DigestFailure) error {
	defer metrics2.FuncTimer().Stop()

	// Write failure to Firestore.
	docRef := s.digestFailureToDocRef(failure)
	entry := toStoreEntry(failure)
	if _, err := s.client.Set(ctx, docRef, entry, maxWriteAttempts, maxOperationTime); err != nil {
		return skerr.Wrapf(err, "writing failure to Firestore (Digest=%q): %v", failure.Digest, failure)
	}

	return nil
}

// PurgeDigestFailures implements the failurestore.FailureStore interface.
func (s *FSImpl) PurgeDigestFailures(ctx context.Context, digests types.DigestSlice) error {
	defer metrics2.FuncTimer().Stop()

	// Retrieve all unavailable digests. We'll loop through them and search for those that match the
	// given digests.
	//
	// Notes:
	//  - An alternative (and perhaps more obvious) design would be to query the collection once per
	//    each of the given digests to find any matching documents.
	//  - Instead, we retrieve the whole collection and loop through the documents.
	//  - This is acceptable because the number of documents in the collection is very small, thus one
	//    single query should be faster than one query per digest.
	//  - We should revisit this design if said assumption no longer holds in the future.
	unavailableDigests, err := s.UnavailableDigests(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Gather *firestore.DocumentRefs to delete.
	targetDocRefs := make([]*firestore.DocumentRef, 0, len(digests))
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
			return skerr.Wrapf(err, "deleting failure with Firestore document ID %q", docRef.ID)
		})
	}
	err = g.Wait()
	return skerr.Wrap(err)
}

// digestFailureToDocRef takes a digest failure and returns the *firestore.DocumentRef of its
// corresponding entry in Firestore.
func (s *FSImpl) digestFailureToDocRef(failure *diff.DigestFailure) *firestore.DocumentRef {
	id := fmt.Sprintf("%s-%d", failure.Digest, failure.TS)
	return s.client.Collection(failureStoreCollection).Doc(id)
}

// Make sure FSImpl fulfills the FailureStore interface
var _ failurestore.FailureStore = (*FSImpl)(nil)
