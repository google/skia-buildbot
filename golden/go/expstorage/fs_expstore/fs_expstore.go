// Implements an ExpectationsStore based on Firestore. See FIRESTORE.md for the schema
// and design rationale.
package fs_expstore

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/cenkalti/backoff"
	"go.skia.org/infra/go/eventbus"
	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"
)

// AccessMode indicates if this ExpectationsStore can update existing Expectations
// in the backing store or if if can only read them.
type AccessMode int

const (
	ReadOnly AccessMode = iota
	ReadWrite
)

var (
	ReadOnlyErr = errors.New("expectationStore is in read-only mode")
)

const (
	// Should be used to create the firestore.NewClient that is passed into New.
	ExpectationStoreCollection = "expstore"

	// These are the collections in Firestore.
	expectationsCollection  = "expectations"
	triageRecordsCollection = "triage_records"
	triageChangesCollection = "triage_changes"

	// Columns in the Collections we query by.
	committedCol = "committed"
	digestCol    = "digest"
	groupingCol  = "grouping"
	issueCol     = "issue"
	recordIDCol  = "record_id"
	tsCol        = "ts"

	maxOperationTime = 2 * time.Minute
)

// Store implements expstorage.ExpectationsStore backed by
// Firestore. It has a write-through caching mechanism.
type Store struct {
	client *ifirestore.Client
	mode   AccessMode
	issue  int64 // Gerrit or GitHub issue, or MasterBranch

	// eventBus allows this Store to communicate with the outside world when
	// expectations change.
	eventBus eventbus.EventBus
	// globalEvent keeps track whether we want to send events within this instance
	// or on the global eventbus.
	globalEvent bool
	// eventExpChange keeps track of which event to fire when the expectations change.
	// This will be for either the MasterExpectations or for an IssueExpectations.
	eventExpChange string

	// cacheMutex protects the write-through cache object.
	cacheMutex sync.RWMutex
	cache      types.Expectations
}

// expectationEntry is the document type stored in the expectationsCollection.
type expectationEntry struct {
	Grouping types.TestName `firestore:"grouping"`
	Digest   types.Digest   `firestore:"digest"`
	Label    types.Label    `firestore:"label"`
	Updated  time.Time      `firestore:"updated"`
	Issue    int64          `firestore:"issue"`
}

// ID returns the deterministic ID that lets us update existing entries.
func (e *expectationEntry) ID() string {
	return string(e.Grouping) + "|" + string(e.Digest)
}

// triageRecord is the document type stored in the triageRecordsCollection.
type triageRecord struct {
	UserName  string    `firestore:"user"`
	TS        time.Time `firestore:"ts"`
	Issue     int64     `firestore:"issue"`
	Changes   int       `firestore:"changes"`
	Committed bool      `firestore:"committed"`
}

// triageChanges is the document type stored in the triageChangesCollection.
type triageChanges struct {
	RecordID    string         `firestore:"record_id"`
	Grouping    types.TestName `firestore:"grouping"`
	Digest      types.Digest   `firestore:"digest"`
	LabelBefore types.Label    `firestore:"before"`
	LabelAfter  types.Label    `firestore:"after"`
}

// New returns a new Store using the given firestore client. The Store will track
// MasterBranch- see ForIssue() for getting Stores that track ChangeLists.
func New(client *ifirestore.Client, eventBus eventbus.EventBus, mode AccessMode) *Store {
	return &Store{
		client:         client,
		eventBus:       eventBus,
		eventExpChange: expstorage.EV_EXPSTORAGE_CHANGED,
		globalEvent:    true,
		issue:          expstorage.MasterBranch,
		mode:           mode,
	}
}

// ForIssue implements the ExpectationsStore interface.
func (f *Store) ForIssue(id int64) expstorage.ExpectationsStore {
	if id == expstorage.MasterBranch {
		// This should likely never happen, but if it does, lock the master branch
		// down to read-only to prevent multiple threads writing to it at once.
		// One of the core assumptions of this implementation is that there is only
		// ever one process writing to the master branch.
		return New(f.client, f.eventBus, ReadOnly)
	}
	return &Store{
		client:         f.client,
		eventBus:       f.eventBus,
		eventExpChange: expstorage.EV_TRYJOB_EXP_CHANGED,
		globalEvent:    false,
		issue:          id,
		mode:           f.mode,
	}
}

// Get implements the ExpectationsStore interface.
func (f *Store) Get() (types.Expectations, error) {
	if f.issue == expstorage.MasterBranch {
		defer metrics2.NewTimer("gold_get_expectations", map[string]string{"master_branch": "true"}).Stop()
		// On the initial load, we need to make sure we have write-access
		// to the mutex. Otherwise, read access is sufficient.
		if f.cache == nil {
			f.cacheMutex.Lock()
			defer f.cacheMutex.Unlock()
		} else {
			f.cacheMutex.RLock()
			defer f.cacheMutex.RUnlock()
		}
		return f.getMasterExpectations()
	}
	defer metrics2.NewTimer("gold_get_expectations", map[string]string{"master_branch": "false"}).Stop()
	return f.getIssueExpectations()
}

// getMasterExpectations returns an Expectations object which is safe to mutate
// based on the current state. It is expected the caller has taken care of any mutex grabbing.
func (f *Store) getMasterExpectations() (types.Expectations, error) {
	if f.cache == nil {
		c, err := f.loadExpectations(expstorage.MasterBranch)
		if err != nil {
			return nil, skerr.Fmt("could not load master expectations from firestore: %s", err)
		}
		f.cache = c
	}
	return f.cache.DeepCopy(), nil
}

// getIssueExpectations returns an Expectations object which is safe to mutate
// that has all issue-specific Expectations.
// It fetches everything from firestore every time, as there could be multiple
// readers and writers and thus caching isn't safe.
func (f *Store) getIssueExpectations() (types.Expectations, error) {
	issueExp, err := f.loadExpectations(f.issue)
	if err != nil {
		return nil, skerr.Fmt("could not load expectations delta for issue %d from firestore: %s", f.issue, err)
	}
	return issueExp, nil
}

// loadExpectations returns an Expectations object from the expectationsCollection,
// with all Expectations belonging to the passed in issue (can be MasterBranch).
func (f *Store) loadExpectations(issue int64) (types.Expectations, error) {
	defer metrics2.FuncTimer().Stop()
	e := types.Expectations{}
	q := f.client.Collection(expectationsCollection).Where(issueCol, "==", issue)
	maxRetries := 3
	err := f.client.IterDocs("loadExpectations", strconv.FormatInt(issue, 10), q, maxRetries, maxOperationTime, func(doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		entry := expectationEntry{}
		if err := doc.DataTo(&entry); err != nil {
			id := doc.Ref.ID
			return skerr.Fmt("corrupt data in firestore, could not unmarshal entry with id %s: %s", id, err)
		}
		e.AddDigest(entry.Grouping, entry.Digest, entry.Label)
		return nil
	})
	return e, err
}

// AddChange implements the ExpectationsStore interface.
func (f *Store) AddChange(ctx context.Context, newExp types.Expectations, userID string) error {
	defer metrics2.FuncTimer().Stop()
	if f.mode == ReadOnly {
		return ReadOnlyErr
	}
	// Create the entries that we want to write (using the previous values)
	now, entries, changes := func() (time.Time, []expectationEntry, []triageChanges) {
		f.cacheMutex.Lock()
		defer f.cacheMutex.Unlock()
		now := time.Now()
		entries, changes := f.flatten(now, newExp)

		// Write the changes to the locale cache. We do this first so we can free up
		// the read mutex as soon as possible.
		if f.cache == nil {
			f.cache = newExp.DeepCopy()
		} else {
			f.cache.MergeExpectations(newExp)
		}
		return now, entries, changes
	}()

	if f.eventBus != nil {
		f.eventBus.Publish(f.eventExpChange, &expstorage.EventExpectationChange{
			TestChanges: newExp,
			IssueID:     f.issue,
		}, f.globalEvent)
	}

	// Nothing to add
	if len(entries) == 0 {
		return nil
	}

	// firestore can do up to 500 writes at once, we have 2 writes per entry, plus 1 triageRecord
	batchSize := (500 / 2) - 1

	b := f.client.Batch()

	// First write the triage record, with Committed being false (i.e. in progress)
	tr := f.client.Collection(triageRecordsCollection).NewDoc()
	record := triageRecord{
		UserName:  userID,
		TS:        now,
		Issue:     f.issue,
		Changes:   len(entries),
		Committed: false,
	}
	b.Set(tr, record)

	// In batches, add ExpectationEntry and TriageChange Documents
	for i := 0; i < len(entries); i += batchSize {
		stop := i + batchSize
		if stop > len(entries) {
			stop = len(entries)
		}

		for idx, entry := range entries[i:stop] {
			e := f.client.Collection(expectationsCollection).Doc(entry.ID())
			b.Set(e, entry)

			tc := f.client.Collection(triageChangesCollection).NewDoc()
			change := changes[idx]
			change.RecordID = tr.ID
			b.Set(tc, change)
		}

		exp := &backoff.ExponentialBackOff{
			InitialInterval:     time.Second,
			RandomizationFactor: 0.5,
			Multiplier:          2,
			MaxInterval:         maxOperationTime / 4,
			MaxElapsedTime:      maxOperationTime,
			Clock:               backoff.SystemClock,
		}

		o := func() error {
			_, err := b.Commit(ctx)
			return err
		}

		if err := backoff.Retry(o, exp); err != nil {
			// We really hope this doesn't happen, as it may leave the data in a partially
			// broken state.
			return skerr.Fmt("problem writing entries with retry [%d, %d]: %s", i, stop, err)
		}
		// Go on to the next batch, if needed.
		if stop < len(entries) {
			b = f.client.Batch()
		}
	}

	// We have succeeded this potentially long write, so mark it completed.
	update := map[string]interface{}{
		"committed": true,
	}
	_, err := f.client.Set(tr, update, 10, maxOperationTime, firestore.MergeAll)
	return err
}

// flatten creates the data for the Documents to be written for a given Expectations delta.
// It requires that the f.cache is safe to read (i.e. the mutex is held), because
// it needs to determine the previous values.
func (f *Store) flatten(now time.Time, newExp types.Expectations) ([]expectationEntry, []triageChanges) {
	var entries []expectationEntry
	var changes []triageChanges

	for testName, digestMap := range newExp {
		for digest, label := range digestMap {
			entries = append(entries, expectationEntry{
				Grouping: testName,
				Digest:   digest,
				Label:    label,
				Updated:  now,
				Issue:    f.issue,
			})

			changes = append(changes, triageChanges{
				// RecordID will be filled out later
				Grouping:    testName,
				Digest:      digest,
				LabelBefore: f.cache.Classification(testName, digest),
				LabelAfter:  label,
			})
		}
	}
	return entries, changes
}

// QueryLog implements the ExpectationsStore interface.
func (f *Store) QueryLog(ctx context.Context, offset, size int, details bool) ([]expstorage.TriageLogEntry, int, error) {
	if offset < 0 || size <= 0 {
		return nil, 0, fmt.Errorf("offset: %d and size: %d must be positive", offset, size)
	}
	tags := map[string]string{
		"with_details": "false",
	}
	if details {
		tags["with_details"] = "true"
	}
	defer metrics2.NewTimer("gold_query_log", tags).Stop()

	// Fetch the records, which have everything except the details.
	q := f.client.Collection(triageRecordsCollection).OrderBy(tsCol, firestore.Desc).Offset(offset).Limit(size)
	q = q.Where(issueCol, "==", f.issue).Where(committedCol, "==", true)
	var rv []expstorage.TriageLogEntry
	d := fmt.Sprintf("offset: %d, size %d", offset, size)
	err := f.client.IterDocs("query_log", d, q, 3, maxOperationTime, func(doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		tr := triageRecord{}
		if err := doc.DataTo(&tr); err != nil {
			id := doc.Ref.ID
			return skerr.Fmt("corrupt data in firestore, could not unmarshal triage record with id %s: %s", id, err)
		}
		rv = append(rv, expstorage.TriageLogEntry{
			ID:          doc.Ref.ID,
			Name:        tr.UserName,
			TS:          tr.TS.Unix(),
			ChangeCount: tr.Changes,
		})
		return nil
	})
	if err != nil {
		return nil, 0, skerr.Fmt("could not request triage records [%d: %d]: %s", offset, size, err)
	}

	if len(rv) == 0 || !details {
		return rv, len(rv), nil
	}

	// Make a query for each of the records to fetch the changes belonging to that record.
	qs := make([]firestore.Query, 0, len(rv))
	for _, r := range rv {
		q := f.client.Collection(triageChangesCollection).Where(recordIDCol, "==", r.ID)
		// Sort them by grouping, then Digest for determinism
		q = q.OrderBy(groupingCol, firestore.Asc).OrderBy(digestCol, firestore.Asc)
		qs = append(qs, q)
	}

	// Then fire them all off in parallel.
	err = f.client.IterDocsInParallel("query_log_details", d, qs, 3, maxOperationTime, func(i int, doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		tc := triageChanges{}
		if err := doc.DataTo(&tc); err != nil {
			id := doc.Ref.ID
			return skerr.Fmt("corrupt data in firestore, could not unmarshal triage changes with id %s: %s", id, err)
		}
		rv[i].Details = append(rv[i].Details, expstorage.TriageDetail{
			TestName: tc.Grouping,
			Digest:   tc.Digest,
			Label:    tc.LabelAfter.String(),
		})
		return nil
	})
	if err != nil {
		return nil, 0, skerr.Fmt("could not query details: %s", err)
	}

	return rv, len(rv), nil
}

// UndoChange implements the ExpectationsStore interface.
func (f *Store) UndoChange(ctx context.Context, changeID, userID string) (types.Expectations, error) {
	defer metrics2.FuncTimer().Stop()
	// Verify the original change id exists.
	dr := f.client.Collection(triageRecordsCollection).Doc(changeID)
	doc, err := f.client.Get(dr, 3, maxOperationTime)
	if err != nil || !doc.Exists() {
		return nil, skerr.Fmt("could not find change to undo with id %s: %s", changeID, err)
	}

	q := f.client.Collection(triageChangesCollection).Where(recordIDCol, "==", changeID)
	delta := types.Expectations{}
	err = f.client.IterDocs("undo_query", changeID, q, 3, maxOperationTime, func(doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		tc := triageChanges{}
		if err := doc.DataTo(&tc); err != nil {
			id := doc.Ref.ID
			return skerr.Fmt("corrupt data in firestore, could not unmarshal triage changes with id %s: %s", id, err)
		}
		delta.AddDigest(tc.Grouping, tc.Digest, tc.LabelBefore)
		return nil
	})
	if err != nil {
		return nil, skerr.Fmt("could not get delta to undo %s: %s", changeID, err)
	}

	if err = f.AddChange(ctx, delta, userID); err != nil {
		return nil, skerr.Fmt("could not apply delta to undo %s: %s", changeID, err)
	}

	return delta, nil
}

// Make sure Store fulfills the ExpectationsStore interface
var _ expstorage.ExpectationsStore = (*Store)(nil)
