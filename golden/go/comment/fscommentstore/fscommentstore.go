// Package fscommentstore contains a Firestore-based implementation of comment.Store.
package fscommentstore

import (
	"context"
	"sort"
	"sync"
	"time"

	"cloud.google.com/go/firestore"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/comment"
	"go.skia.org/infra/golden/go/comment/trace"
	"go.skia.org/infra/golden/go/fs_utils"
)

const (
	// These are the collections in Firestore.
	traceCommentCollection = "commentstore_tracecomments"

	maxReadAttempts  = 5
	maxWriteAttempts = 5
	maxOperationTime = time.Minute
)

// StoreImpl is the Firestore-based implementation of comment.Store. It uses query snapshots to
// synchronize a local commentCache with the data in Firestore, thus it does not need to poll. We
// don't do any searches on our data, so keeping all the trace comments in RAM (backed by Firestore)
// is a fine solution for the sake of performance (and avoiding unnecessary Firestore traffic).
type StoreImpl struct {
	client *ifirestore.Client

	cacheMutex   sync.RWMutex
	commentCache map[trace.ID]trace.Comment
}

// commentEntry represents how an trace.Comment is stored in Firestore.
type commentEntry struct {
	CreatedBy    string              `firestore:"createdby"`
	UpdatedBy    string              `firestore:"updatedby"`
	CreatedTS    time.Time           `firestore:"createdts"`
	UpdatedTS    time.Time           `firestore:"updatedts"`
	QueryToMatch paramtools.ParamSet `firestore:"query"`
	Comment      string              `firestore:"comment"`
}

// toTraceComment converts a Firestore commentEntry into a trace.Comment. The QueryToMatch field
// will be a normalized copy of the one from the passed in entry (to avoid a race condition between
// a potentially cached commentEntry and the returned value).
func toTraceComment(id trace.ID, r commentEntry) trace.Comment {
	q := r.QueryToMatch.Copy()
	q.Normalize()
	return trace.Comment{
		ID:           id,
		CreatedBy:    r.CreatedBy,
		UpdatedBy:    r.UpdatedBy,
		CreatedTS:    r.CreatedTS,
		UpdatedTS:    r.UpdatedTS,
		Comment:      r.Comment,
		QueryToMatch: q,
	}
}

// toEntry converts a trace.Comment into a Firestore commentEntry. The QueryToMatch field will be
// a normalized copy of the one from the passed in comment (to avoid a race condition between
// a potentially cached commentEntry and the passed in value).
func toEntry(r trace.Comment) commentEntry {
	q := r.QueryToMatch.Copy()
	q.Normalize()
	return commentEntry{
		CreatedBy:    r.CreatedBy,
		UpdatedBy:    r.UpdatedBy,
		CreatedTS:    r.CreatedTS,
		UpdatedTS:    r.UpdatedTS,
		Comment:      r.Comment,
		QueryToMatch: q,
	}
}

// New returns a new StoreImpl.
func New(ctx context.Context, client *ifirestore.Client) *StoreImpl {
	s := &StoreImpl{
		client:       client,
		commentCache: map[trace.ID]trace.Comment{},
	}
	s.startQueryIterator(ctx)
	return s
}

// startQueryIterator sets up the listener to the Query Snapshots which keep the local commentCache of
// comments in sync with those in Firestore.
func (s *StoreImpl) startQueryIterator(ctx context.Context) {
	go func() {
		// We don't ever expect there to be a lot (>10,000) of trace comments, so a single,
		// un-sharded snapshot should suffice to read in all the stored rules. If it doesn't, look
		// at how fs_expectationstore does it.
		snapFactory := func() *firestore.QuerySnapshotIterator {
			return s.client.Collection(traceCommentCollection).Snapshots(ctx)
		}
		err := fs_utils.ListenAndRecover(ctx, nil, snapFactory, s.updateCacheWithEntriesFrom)
		if err != nil {
			sklog.Errorf("Unrecoverable error: %s", err)
		}
	}()
}

// updateCacheWithEntriesFrom loops through all the changes in the given snapshot and updates
// the commentCache with those new values (or deletes the old ones).
func (s *StoreImpl) updateCacheWithEntriesFrom(_ context.Context, qs *firestore.QuerySnapshot) error {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	for _, dc := range qs.Changes {
		id := trace.ID(dc.Doc.Ref.ID)
		if dc.Kind == firestore.DocumentRemoved {
			delete(s.commentCache, id)
			continue
		}
		entry := commentEntry{}
		if err := dc.Doc.DataTo(&entry); err != nil {
			sklog.Errorf("corrupt data in firestore, could not unmarshal commentEntry with id %s", id)
			continue
		}
		s.commentCache[id] = toTraceComment(id, entry)
	}
	return nil
}

// CreateComment implements the comment.Store interface.
func (s *StoreImpl) CreateComment(ctx context.Context, c trace.Comment) (trace.ID, error) {
	doc := s.client.Collection(traceCommentCollection).NewDoc()
	if _, err := s.client.Create(ctx, doc, toEntry(c), maxWriteAttempts, maxOperationTime); err != nil {
		return "", skerr.Wrapf(err, "storing new trace comment to Firestore (%#v)", c)
	}
	c.ID = trace.ID(doc.ID)
	// Make a copy of the map to make sure our cache doesn't accidentally share it with the
	// passed-in version.
	c.QueryToMatch = c.QueryToMatch.Copy()
	// Update the cache (this helps avoid flakes on unit tests).
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	s.commentCache[c.ID] = c
	return c.ID, nil
}

// ListComments implements the comment.Store interface. It returns the local commentCache of trace
// comments, never re-fetching from Firestore because the query snapshots should keep the local
// commentCache up to date, give or take a few seconds.
func (s *StoreImpl) ListComments(_ context.Context) ([]trace.Comment, error) {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()
	rv := make([]trace.Comment, 0, len(s.commentCache))
	for _, c := range s.commentCache {
		// Return a copy of the QueryToMatch map, so subsequent mutations do not impact the
		// cached version.
		c.QueryToMatch = c.QueryToMatch.Copy()
		rv = append(rv, c)
	}
	sort.Slice(rv, func(i, j int) bool {
		return rv[i].UpdatedTS.Before(rv[j].UpdatedTS)
	})
	return rv, nil
}

// UpdateComment implements the comment.Store interface.
func (s *StoreImpl) UpdateComment(ctx context.Context, comment trace.Comment) error {
	if comment.ID == "" {
		return skerr.Fmt("ID for trace comment cannot be empty")
	}
	doc := s.client.Collection(traceCommentCollection).Doc(string(comment.ID))
	dSnap, err := s.client.Get(ctx, doc, maxReadAttempts, maxOperationTime)
	if err != nil {
		return skerr.Wrapf(err, "getting trace comment %s before updating", comment.ID)
	}
	var oldComment commentEntry
	if err := dSnap.DataTo(&oldComment); err != nil {
		return skerr.Wrapf(err, "corrupt data in firestore for id %s", comment.ID)
	}
	updatedComment := toEntry(comment)
	updatedComment.CreatedBy = oldComment.CreatedBy
	updatedComment.CreatedTS = oldComment.CreatedTS
	if _, err := s.client.Set(ctx, doc, updatedComment, maxWriteAttempts, maxOperationTime); err != nil {
		return skerr.Wrapf(err, "updating trace comment %s", comment.ID)
	}
	// Update the cache (this helps avoid flakes on unit tests).
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	s.commentCache[comment.ID] = toTraceComment(comment.ID, updatedComment)
	return nil
}

// DeleteComment implements the comment.Store interface.
func (s *StoreImpl) DeleteComment(ctx context.Context, id trace.ID) error {
	if id == "" {
		return skerr.Fmt("ID for trace comment cannot be empty")
	}
	toDelete := s.client.Collection(traceCommentCollection).Doc(string(id))
	if _, err := s.client.Delete(ctx, toDelete, maxWriteAttempts, maxOperationTime); err != nil {
		return skerr.Wrapf(err, "deleting trace comment with id %s", id)
	}
	// Update the cache (this helps avoid flakes on unit tests).
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	delete(s.commentCache, id)
	return nil
}

// Make sure Store fulfills the comment.Store interface
var _ comment.Store = (*StoreImpl)(nil)
