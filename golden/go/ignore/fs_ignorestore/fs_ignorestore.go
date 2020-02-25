// Package fs_ignorestore hosts a Firestore-based implementation of ignore.Store.
package fs_ignorestore

import (
	"context"
	"math/rand"
	"sort"
	"sync"
	"time"

	"cloud.google.com/go/firestore"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/ignore"
)

const (
	// These are the collections in Firestore.
	rulesCollection = "ignorestore_rules"

	maxReadAttempts  = 5
	maxWriteAttempts = 5
	maxOperationTime = time.Minute

	// recoverTime is the minimum amount of time to wait before recreating any QuerySnapshotIterator
	// if it fails. A random amount of time should be added to this, proportional to recoverTime.
	recoverTime = 30 * time.Second
)

// StoreImpl is the Firestore-based implementation of ignore.Store. It uses query snapshots to
// synchronize a local cache with the data in Firestore, thus it does not need to poll. We don't
// do any searches on our data, so keeping all the ignore rules in RAM (backed by Firestore) is
// a fine solution for the sake of performance (and avoiding unnecessary Firestore traffic). Ignore
// Rules generally top out in the 100s, plenty to fit in RAM.
type StoreImpl struct {
	client *ifirestore.Client

	cacheMutex sync.RWMutex
	// This maps ID -> rule, preventing accidental duplication of rules.
	cache map[string]ignore.Rule
}

// ruleEntry represents how an ignore.Rule is stored in Firestore.
type ruleEntry struct {
	CreatedBy string    `firestore:"createdby"`
	UpdatedBy string    `firestore:"updatedby"`
	Expires   time.Time `firestore:"expires"`
	Query     string    `firestore:"query"`
	Note      string    `firestore:"note"`
}

func toRule(id string, r ruleEntry) ignore.Rule {
	return ignore.Rule{
		ID:        id,
		CreatedBy: r.CreatedBy,
		UpdatedBy: r.UpdatedBy,
		Expires:   r.Expires,
		Query:     r.Query,
		Note:      r.Note,
	}
}

func toEntry(r ignore.Rule) ruleEntry {
	return ruleEntry{
		CreatedBy: r.CreatedBy,
		UpdatedBy: r.UpdatedBy,
		Expires:   r.Expires,
		Query:     r.Query,
		Note:      r.Note,
	}
}

// New returns a new StoreImpl.
func New(ctx context.Context, client *ifirestore.Client) *StoreImpl {
	s := &StoreImpl{
		client: client,
		cache:  map[string]ignore.Rule{},
	}
	s.startQueryIterator(ctx)
	return s
}

// startQueryIterator sets up the listener to the Query Snapshots which keep the local cache of
// ignore rules in sync with those in Firestore.
func (s *StoreImpl) startQueryIterator(ctx context.Context) {
	go func() {
		// TODO(kjlubick) deduplicate this logic with fs_expstore maybe? We'd like to be able to
		//   recover, so maybe we need a variant of QuerySnapshotChannel
		snap := s.client.Collection(rulesCollection).Snapshots(ctx)
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
				snap = s.client.Collection(rulesCollection).Snapshots(ctx)
				continue
			}
			s.updateCacheWithEntriesFrom(qs)
		}
	}()
}

// updateCacheWithEntriesFrom loops through all the changes in the given snapshot and updates
// the cache with those new values (or deletes the old ones).
func (s *StoreImpl) updateCacheWithEntriesFrom(qs *firestore.QuerySnapshot) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	for _, dc := range qs.Changes {
		id := dc.Doc.Ref.ID
		if dc.Kind == firestore.DocumentRemoved {
			delete(s.cache, id)
			continue
		}
		entry := ruleEntry{}
		if err := dc.Doc.DataTo(&entry); err != nil {
			sklog.Errorf("corrupt data in firestore, could not unmarshal ruleEntry with id %s", id)
			continue
		}
		s.cache[id] = toRule(id, entry)
	}
}

// Create implements the ignore.Store interface.
func (s *StoreImpl) Create(ctx context.Context, r ignore.Rule) error {
	doc := s.client.Collection(rulesCollection).NewDoc()
	if _, err := s.client.Create(ctx, doc, toEntry(r), maxWriteAttempts, maxOperationTime); err != nil {
		return skerr.Wrapf(err, "storing new ignore rule to Firestore (%#v)", r)
	}
	r.ID = doc.ID
	// Update the cache (this helps avoid flakes on unit tests).
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	s.cache[r.ID] = r
	return nil
}

// List implements the ignore.Store interface. It returns the local cache of ignore rules, never
// re-fetching from Firestore because the query snapshots should keep the local cache up to date,
// give or take a few seconds.
func (s *StoreImpl) List(_ context.Context) ([]ignore.Rule, error) {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()
	rv := make([]ignore.Rule, 0, len(s.cache))
	for _, r := range s.cache {
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
	doc := s.client.Collection(rulesCollection).Doc(rule.ID)
	dSnap, err := s.client.Get(ctx, doc, maxReadAttempts, maxOperationTime)
	if err != nil {
		return skerr.Wrapf(err, "getting ignore rule %s before updating", rule.ID)
	}
	var oldRule ruleEntry
	if err := dSnap.DataTo(&oldRule); err != nil {
		return skerr.Wrapf(err, "corrupt data in firestore for id %s", rule.ID)
	}
	updatedRule := toEntry(rule)
	updatedRule.CreatedBy = oldRule.CreatedBy
	if _, err := s.client.Set(ctx, doc, updatedRule, maxWriteAttempts, maxOperationTime); err != nil {
		return skerr.Wrapf(err, "updating ignore rule %s", rule.ID)
	}
	// Update the cache (this helps avoid flakes on unit tests).
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	s.cache[rule.ID] = toRule(rule.ID, updatedRule)
	return nil
}

// Delete implements the ignore.Store interface.
func (s *StoreImpl) Delete(ctx context.Context, id string) error {
	if id == "" {
		return skerr.Fmt("ID for ignore rule cannot be empty")
	}
	if _, err := s.client.Collection(rulesCollection).Doc(id).Delete(ctx); err != nil {
		return skerr.Wrapf(err, "deleting ignore rule with id %s", id)
	}
	// Update the cache (this helps avoid flakes on unit tests).
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	delete(s.cache, id)
	return nil
}

// Make sure Store fulfills the ignore.Store interface
var _ ignore.Store = (*StoreImpl)(nil)
