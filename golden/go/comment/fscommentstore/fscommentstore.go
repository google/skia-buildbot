// Package fscommentstore contains a Firestore-based implementation of comment.Store.
package fscommentstore

import (
	"context"
	"math/rand"
	"sort"
	"sync"
	"time"

	"cloud.google.com/go/firestore"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/comment/trace"
	"go.skia.org/infra/golden/go/ignore"
)

const (
	// These are the collections in Firestore.
	traceComments = "commentstore_tracecomments"

	maxReadAttempts  = 5
	maxWriteAttempts = 5
	maxOperationTime = time.Minute

	// recoverTime is the minimum amount of time to wait before recreating any QuerySnapshotIterator
	// if it fails. A random amount of time should be added to this, proportional to recoverTime.
	recoverTime = 30 * time.Second
)

// StoreImpl is the Firestore-based implementation of ignore.Store. It uses query snapshots to
// synchronize a local traceCache with the data in Firestore, thus it does not need to poll. We don't
// do any searches on our data, so keeping all the ignore rules in RAM (backed by Firestore) is
// a fine solution for the sake of performance (and avoiding unnecessary Firestore traffic). Ignore
// Rules generally top out in the 100s, plenty to fit in RAM.
type StoreImpl struct {
	client *ifirestore.Client

	traceMutex sync.RWMutex
	traceCache map[string]trace.Comment
}

// traceEntry represents how an trace.Comment is stored in Firestore.
type traceEntry struct {
	CreatedBy    string              `firestore:"createdby"`
	UpdatedBy    string              `firestore:"updatedby"`
	CreatedTS    time.Time           `firestore:"createdts"`
	UpdatedTS    time.Time           `firestore:"updatedts"`
	QueryToMatch paramtools.ParamSet `firestore:"query"`
	Comment      string              `firestore:"comment"`
}

func toTraceComment(id string, r traceEntry) trace.Comment {
	r.QueryToMatch.Normalize()
	return trace.Comment{
		ID:           id,
		CreatedBy:    r.CreatedBy,
		UpdatedBy:    r.UpdatedBy,
		CreatedTS:    r.CreatedTS,
		UpdatedTS:    r.UpdatedTS,
		Comment:      r.Comment,
		QueryToMatch: r.QueryToMatch,
	}
}

func toEntry(r trace.Comment) traceEntry {
	r.QueryToMatch.Normalize()
	return traceEntry{
		CreatedBy:    r.CreatedBy,
		UpdatedBy:    r.UpdatedBy,
		CreatedTS:    r.CreatedTS,
		UpdatedTS:    r.UpdatedTS,
		Comment:      r.Comment,
		QueryToMatch: r.QueryToMatch,
	}
}

// New returns a new StoreImpl.
func New(ctx context.Context, client *ifirestore.Client) *StoreImpl {
	s := &StoreImpl{
		client:     client,
		traceCache: map[string]trace.Comment{},
	}
	s.startQueryIterator(ctx)
	return s
}

// startQueryIterator sets up the listener to the Query Snapshots which keep the local traceCache of
// ignore rules in sync with those in Firestore.
func (s *StoreImpl) startQueryIterator(ctx context.Context) {
	go func() {
		// TODO(kjlubick) deduplicate this logic with fs_expstore maybe? We'd like to be able to
		//   recover, so maybe we need a variant of QuerySnapshotChannel
		snap := s.client.Collection(traceComments).Snapshots(ctx)
		for {
			if err := ctx.Err(); err != nil {
				sklog.Debugf("Stopping query of ignores due to context error: %s", err)
				snap.Stop()
				return
			}
			qs, err := snap.Next()
			if err != nil {
				sklog.Errorf("reading query snapshot: %s", err)
				snap.Stop()
				if err := ctx.Err(); err != nil {
					// Oh, it was from a context cancellation (e.g. a test), don't recover.
					return
				}
				// sleep and rebuild the snapshot query. Once a SnapshotQueryIterator returns
				// an error, it seems to always return that error.
				t := recoverTime + time.Duration(float32(recoverTime)*rand.Float32())
				time.Sleep(t)
				sklog.Infof("Trying to recreate query snapshot after having slept %s", t)
				snap = s.client.Collection(traceComments).Snapshots(ctx)
				continue
			}
			s.updateCacheWithEntriesFrom(qs)
		}
	}()
}

// updateCacheWithEntriesFrom loops through all the changes in the given snapshot and updates
// the traceCache with those new values (or deletes the old ones).
func (s *StoreImpl) updateCacheWithEntriesFrom(qs *firestore.QuerySnapshot) {
	s.traceMutex.Lock()
	defer s.traceMutex.Unlock()
	for _, dc := range qs.Changes {
		id := dc.Doc.Ref.ID
		if dc.Kind == firestore.DocumentRemoved {
			delete(s.traceCache, id)
			continue
		}
		entry := traceEntry{}
		if err := dc.Doc.DataTo(&entry); err != nil {
			sklog.Errorf("corrupt data in firestore, could not unmarshal traceEntry with id %s", id)
			continue
		}
		s.traceCache[id] = toTraceComment(id, entry)
	}
}

// Create implements the ignore.Store interface.
func (s *StoreImpl) Create(ctx context.Context, r ignore.Rule) error {
	doc := s.client.Collection(traceComments).NewDoc()
	if _, err := s.client.Create(ctx, doc, toEntry(r), maxWriteAttempts, maxOperationTime); err != nil {
		return skerr.Wrapf(err, "storing new ignore rule to Firestore (%#v)", r)
	}
	return nil
}

// List implements the ignore.Store interface. It returns the local traceCache of ignore rules, never
// re-fetching from Firestore because the query snapshots should keep the local traceCache up to date,
// give or take a few seconds.
func (s *StoreImpl) List(_ context.Context) ([]ignore.Rule, error) {
	s.traceMutex.RLock()
	defer s.traceMutex.RUnlock()
	rv := make([]ignore.Rule, 0, len(s.traceCache))
	for _, r := range s.traceCache {
		rv = append(rv, r)
	}
	sort.Slice(rv, func(i, j int) bool {
		return rv[i].Expires.Before(rv[j].Expires)
	})
	return rv, nil
}

// Update implements the ignore.Store interface.
func (s *StoreImpl) Update(ctx context.Context, rule ignore.Rule) error {
	if rule.ID == "" {
		return skerr.Fmt("ID for ignore rule cannot be empty")
	}
	doc := s.client.Collection(traceComments).Doc(rule.ID)
	dSnap, err := s.client.Get(ctx, doc, maxReadAttempts, maxOperationTime)
	if err != nil {
		return skerr.Wrapf(err, "getting ignore rule %s before updating", rule.ID)
	}
	var oldRule traceEntry
	if err := dSnap.DataTo(&oldRule); err != nil {
		return skerr.Wrapf(err, "corrupt data in firestore for id %s", rule.ID)
	}
	updatedRule := toEntry(rule)
	updatedRule.CreatedBy = oldRule.CreatedBy
	if _, err := s.client.Set(ctx, doc, updatedRule, maxWriteAttempts, maxOperationTime); err != nil {
		return skerr.Wrapf(err, "updating ignore rule %s", rule.ID)
	}
	return nil
}

// Delete implements the ignore.Store interface.
func (s *StoreImpl) Delete(ctx context.Context, id string) error {
	if id == "" {
		return skerr.Fmt("ID for ignore rule cannot be empty")
	}
	s.client.Collection(traceComments).Doc(id)
	if _, err := s.client.Collection(traceComments).Doc(id).Delete(ctx); err != nil {
		return skerr.Wrapf(err, "deleting ignore rule with id %s", id)
	}
	return nil
}
