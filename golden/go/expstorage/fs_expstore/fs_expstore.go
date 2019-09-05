// Implements an ExpectationsStore based on Firestore. See FIRESTORE.md for the schema
// and design rationale.
package fs_expstore

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/cenkalti/backoff"
	"go.skia.org/infra/go/eventbus"
	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
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
	// These are the collections in Firestore.
	expectationsCollection  = "expstore_expectations"
	triageRecordsCollection = "expstore_triage_records"
	triageChangesCollection = "expstore_triage_changes"

	// Fields in the Collections we query by.
	committedField = "committed"
	digestField    = "digest"
	groupingField  = "grouping"
	issueField     = "issue"
	recordIDField  = "record_id"
	tsField        = "ts"

	maxOperationTime = 2 * time.Minute
	// loadShards was determined empirically on a data set of about 550k expectationEntry
	// 1 shard -> ???
	// 10 shards -> 215s
	// 100 shards -> 21s
	// 256 shards -> 10s
	// 512 shards -> 9s
	// 4096 shards -> 9s
	masterShards = 512

	// There will not be very many entries on issues, relative to the MasterBranch, so
	// we can get away with many fewer shards to avoid the overhead of so many
	// simultaneous queries.
	issueShards = 4
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
	s := string(e.Grouping) + "|" + string(e.Digest)
	// firestore gets cranky if there are / in key names
	return strings.Replace(s, "/", "-", -1)
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
func New(client *ifirestore.Client, eventBus eventbus.EventBus, mode AccessMode) (*Store, error) {
	defer metrics2.FuncTimer().Stop()
	f := &Store{
		client:         client,
		eventBus:       eventBus,
		eventExpChange: expstorage.EV_EXPSTORAGE_CHANGED,
		globalEvent:    true,
		issue:          types.MasterBranch,
		mode:           mode,
	}
	// pre-load the cache. This simplifies the mutex handling in Get().
	_, err := f.getMasterExpectations()
	if err != nil {
		return nil, skerr.Fmt("could not perform initial get")
	}
	return f, nil
}

// ForIssue implements the ExpectationsStore interface.
func (f *Store) ForIssue(id int64) expstorage.ExpectationsStore {
	if types.IsMasterBranch(id) {
		// It is invalid to re-request the master branch
		return nil
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
	if f.issue == types.MasterBranch {
		defer metrics2.NewTimer("gold_get_expectations", map[string]string{"master_branch": "true"}).Stop()
		f.cacheMutex.RLock()
		defer f.cacheMutex.RUnlock()
		return f.getMasterExpectations()
	}
	defer metrics2.NewTimer("gold_get_expectations", map[string]string{"master_branch": "false"}).Stop()
	return f.getIssueExpectations()
}

// getMasterExpectations returns an Expectations object which is safe to mutate
// based on the current state. It is expected the caller has taken care of any mutex grabbing.
func (f *Store) getMasterExpectations() (types.Expectations, error) {
	if f.mode == ReadOnly || f.cache == nil {
		c, err := f.loadExpectationsSharded(types.MasterBranch, masterShards)
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
	issueExp, err := f.loadExpectationsSharded(f.issue, issueShards)
	if err != nil {
		return nil, skerr.Fmt("could not load expectations delta for issue %d from firestore: %s", f.issue, err)
	}
	return issueExp, nil
}

// loadExpectationsSharded returns an Expectations object from the expectationsCollection,
// with all Expectations belonging to the passed in issue (can be MasterBranch).
func (f *Store) loadExpectationsSharded(issue int64, shards int) (types.Expectations, error) {
	defer metrics2.FuncTimer().Stop()
	q := f.client.Collection(expectationsCollection).Where(issueField, "==", issue)

	es := make([]types.Expectations, shards)
	queries := shardQueryOnDigest(q, shards)

	maxRetries := 3
	err := f.client.IterDocsInParallel("loadExpectations", strconv.FormatInt(issue, 10), queries, maxRetries, maxOperationTime, func(i int, doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		entry := expectationEntry{}
		if err := doc.DataTo(&entry); err != nil {
			id := doc.Ref.ID
			return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal entry with id %s", id)
		}
		if es[i] == nil {
			es[i] = types.Expectations{}
		}
		es[i].AddDigest(entry.Grouping, entry.Digest, entry.Label)
		return nil
	})

	e := types.Expectations{}
	for _, ne := range es {
		e.MergeExpectations(ne)
	}
	return e, err
}

// shardQueryOnDigest splits a query up to work on a subset of the data based on
// the digests. We split the MD5 space up into N shards by making N-1 shard points
// and adding Where clauses to make N queries that are between those points.
func shardQueryOnDigest(baseQuery firestore.Query, shards int) []firestore.Query {
	queries := make([]firestore.Query, 0, shards)
	zeros := strings.Repeat("0", 16)
	s := uint64(0)
	for i := 0; i < shards-1; i++ {
		// An MD5 hash is 128 bits, which we encode to hexadecimal (32 chars).
		// We can produce an MD5 hash by taking a 64 bit unsigned int, turning
		// that to hexadecimal (16 chars), then appending 16 zeros.
		startHash := fmt.Sprintf("%016x\n", s) + zeros

		s += (math.MaxUint64/uint64(shards) + 1)
		endHash := fmt.Sprintf("%016x\n", s) + zeros

		// The first n queries are formulated to be between two shard points
		queries = append(queries, baseQuery.Where(digestField, ">=", startHash).Where(digestField, "<", endHash))
	}
	lastHash := fmt.Sprintf("%016x\n", s) + zeros
	// The last query is just a greater than the last shard point
	queries = append(queries, baseQuery.Where(digestField, ">=", lastHash))
	return queries
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
		sklog.Debugf("Storing new expectations [%d, %d]", i, stop)
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
		return nil, 0, skerr.Fmt("offset: %d and size: %d must be positive", offset, size)
	}
	tags := map[string]string{
		"with_details": "false",
	}
	if details {
		tags["with_details"] = "true"
	}
	defer metrics2.NewTimer("gold_query_log", tags).Stop()

	// Fetch the records, which have everything except the details.
	q := f.client.Collection(triageRecordsCollection).OrderBy(tsField, firestore.Desc).Offset(offset).Limit(size)
	q = q.Where(issueField, "==", f.issue).Where(committedField, "==", true)
	var rv []expstorage.TriageLogEntry
	d := fmt.Sprintf("offset: %d, size %d", offset, size)
	err := f.client.IterDocs("query_log", d, q, 3, maxOperationTime, func(doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		tr := triageRecord{}
		if err := doc.DataTo(&tr); err != nil {
			id := doc.Ref.ID
			return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal triage record with id %s", id)
		}
		rv = append(rv, expstorage.TriageLogEntry{
			ID:          doc.Ref.ID,
			Name:        tr.UserName,
			TS:          tr.TS.Unix() * 1000,
			ChangeCount: tr.Changes,
		})
		return nil
	})
	if err != nil {
		return nil, 0, skerr.Wrapf(err, "could not request triage records [%d: %d]", offset, size)
	}

	if len(rv) == 0 || !details {
		return rv, len(rv), nil
	}

	// Make a query for each of the records to fetch the changes belonging to that record.
	qs := make([]firestore.Query, 0, len(rv))
	for _, r := range rv {
		q := f.client.Collection(triageChangesCollection).Where(recordIDField, "==", r.ID)
		// Sort them by grouping, then Digest for determinism
		q = q.OrderBy(groupingField, firestore.Asc).OrderBy(digestField, firestore.Asc)
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
			return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal triage changes with id %s", id)
		}
		rv[i].Details = append(rv[i].Details, expstorage.TriageDetail{
			TestName: tc.Grouping,
			Digest:   tc.Digest,
			Label:    tc.LabelAfter.String(),
		})
		return nil
	})
	if err != nil {
		return nil, 0, skerr.Wrapf(err, "could not query details")
	}

	return rv, len(rv), nil
}

// UndoChange implements the ExpectationsStore interface.
func (f *Store) UndoChange(ctx context.Context, changeID, userID string) (types.Expectations, error) {
	defer metrics2.FuncTimer().Stop()
	if f.mode == ReadOnly {
		return nil, ReadOnlyErr
	}
	// Verify the original change id exists.
	dr := f.client.Collection(triageRecordsCollection).Doc(changeID)
	doc, err := f.client.Get(dr, 3, maxOperationTime)
	if err != nil || !doc.Exists() {
		return nil, skerr.Wrapf(err, "could not find change to undo with id %s", changeID)
	}

	q := f.client.Collection(triageChangesCollection).Where(recordIDField, "==", changeID)
	delta := types.Expectations{}
	err = f.client.IterDocs("undo_query", changeID, q, 3, maxOperationTime, func(doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		tc := triageChanges{}
		if err := doc.DataTo(&tc); err != nil {
			id := doc.Ref.ID
			return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal triage changes with id %s", id)
		}
		delta.AddDigest(tc.Grouping, tc.Digest, tc.LabelBefore)
		return nil
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "could not get delta to undo %s", changeID)
	}

	if err = f.AddChange(ctx, delta, userID); err != nil {
		return nil, skerr.Wrapf(err, "could not apply delta to undo %s", changeID)
	}

	return delta, nil
}

// Make sure Store fulfills the ExpectationsStore interface
var _ expstorage.ExpectationsStore = (*Store)(nil)
