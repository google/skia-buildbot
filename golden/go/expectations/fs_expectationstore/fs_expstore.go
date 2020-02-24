// Package fs_expectationstore an expectations.Store based on Firestore. See FIRESTORE.md for the
// schema and design rationale.
package fs_expectationstore

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"golang.org/x/sync/errgroup"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/fs_utils"
	"go.skia.org/infra/golden/go/shared"
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
	expectationsCollection  = "expstore_expectations_v2"
	triageRecordsCollection = "expstore_triage_records_v2"
	triageChangesCollection = "expstore_triage_changes_v2"

	// Fields in the Collections we query by.
	committedField = "committed"
	digestField    = "digest"
	groupingField  = "grouping"
	crsCLIDField   = "crs_cl_id"
	recordIDField  = "record_id"
	tsField        = "ts"

	maxOperationTime = 2 * time.Minute

	// There will not be very many entries on ChangeLists, relative to the masterBranch, so
	// we can get away with many fewer shards to avoid the overhead of so many
	// simultaneous queries.
	clShards = 4

	// snapshotShards was determined empirically on a data set of about 550k expectationEntry
	// The more shards here, the more overhead and contention with the masterShards,
	// so we aim for the sweet spot, erring on the side of too few shards.
	// Times are for the New() function (i.e. initial fetch)
	// 1 shard -> ???
	// 8 shards -> 49s
	// 16 shards -> 25s
	// 32 shards -> 17s
	// 64 shards -> 15s
	// 96 shards -> ???
	// 128 shards -> ???
	// 512 shards -> ???
	snapshotShards = 32

	masterBranch = ""

	// recoverTime is the minimum amount of time to wait before recreating any QuerySnapshotIterator
	// if it fails. A random amount of time should be added to this, proportional to recoverTime.
	recoverTime = 30 * time.Second
)

// Store implements expectations.Store backed by
// Firestore. It has a write-through caching mechanism.
type Store struct {
	client     *ifirestore.Client
	mode       AccessMode
	crsAndCLID string // crs+"_"+id. Empty string means master branch.

	// notifier allows this Store to communicate with the outside world when
	// expectations change.
	notifier expectations.ChangeNotifier

	// cache is an in-memory representation of the expectations in Firestore. It is updated with
	// masterQuerySnapshots
	cache *expectations.Expectations

	masterQuerySnapshots []*firestore.QuerySnapshotIterator
}

// expectationEntry is the document type stored in the expectationsCollection.
type expectationEntry struct {
	Grouping   types.TestName     `firestore:"grouping"`
	Digest     types.Digest       `firestore:"digest"`
	Label      expectations.Label `firestore:"label"`
	Updated    time.Time          `firestore:"updated"`
	CRSAndCLID string             `firestore:"crs_cl_id"`
}

// ID returns the deterministic ID that lets us update existing entries.
func (e *expectationEntry) ID() string {
	s := string(e.Grouping) + "|" + string(e.Digest)
	// firestore gets cranky if there are / in key names
	return strings.Replace(s, "/", "-", -1)
}

// triageRecord is the document type stored in the triageRecordsCollection.
type triageRecord struct {
	UserName   string    `firestore:"user"`
	TS         time.Time `firestore:"ts"`
	CRSAndCLID string    `firestore:"crs_cl_id"`
	Changes    int       `firestore:"changes"`
	Committed  bool      `firestore:"committed"`
}

// triageChanges is the document type stored in the triageChangesCollection.
type triageChanges struct {
	RecordID    string             `firestore:"record_id"`
	Grouping    types.TestName     `firestore:"grouping"`
	Digest      types.Digest       `firestore:"digest"`
	LabelBefore expectations.Label `firestore:"before"`
	LabelAfter  expectations.Label `firestore:"after"`
}

// New returns a new Store using the given firestore client. The Store will track
// masterBranch- see ForChangeList() for getting Stores that track ChangeLists.
// The passed in context is used for the QuerySnapshots (in ReadOnly mode).
func New(ctx context.Context, client *ifirestore.Client, cn expectations.ChangeNotifier, mode AccessMode) (*Store, error) {
	defer metrics2.FuncTimer().Stop()
	defer shared.NewMetricsTimer("expstore_init").Stop()
	f := &Store{
		client:     client,
		notifier:   cn,
		crsAndCLID: masterBranch,
		mode:       mode,
		cache:      &expectations.Expectations{},
	}

	err := f.initQuerySnapshot(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not get initial query snapshot")
	}

	sklog.Infof("Loaded %d master expectations for %d tests", f.cache.Len(), f.cache.NumTests())

	// Starts several go routines to listen to the snapshots created earlier.
	f.listenToQuerySnapshots(ctx)

	return f, nil
}

// ForChangeList implements the ExpectationsStore interface.
func (f *Store) ForChangeList(id, crs string) expectations.Store {
	if id == masterBranch {
		// It is invalid to re-request the master branch
		return nil
	}
	return &Store{
		client:     f.client,
		notifier:   nil, // we do not need to notify when ChangeList expectations change.
		crsAndCLID: crs + "_" + id,
		mode:       f.mode,
		cache:      &expectations.Expectations{},
	}
}

// Get implements the ExpectationsStore interface.
func (f *Store) Get(ctx context.Context) (expectations.ReadOnly, error) {
	if f.crsAndCLID == masterBranch {
		defer metrics2.NewTimer("gold_get_expectations", map[string]string{"master_branch": "true"}).Stop()
		return f.cache, nil
	}
	defer metrics2.NewTimer("gold_get_expectations", map[string]string{"master_branch": "false"}).Stop()
	return f.getExpectationsForCL(ctx)
}

// GetCopy implements the ExpectationsStore interface.
func (f *Store) GetCopy(ctx context.Context) (*expectations.Expectations, error) {
	if f.crsAndCLID == masterBranch {
		defer metrics2.NewTimer("gold_get_expectations", map[string]string{"master_branch": "true"}).Stop()
		return f.cache.DeepCopy(), nil
	}
	defer metrics2.NewTimer("gold_get_expectations", map[string]string{"master_branch": "false"}).Stop()
	return f.getExpectationsForCL(ctx)
}

// initQuerySnapshot creates many firestore.QuerySnapshotIterator objects based on a shard of
// all expectations and does the first Next() on them (which will try to return all data
// in those shards). This data is loaded into the cache. Without sharding the queries, this times
//  out with many expectations because of the fact that the first call to Next() fetches all data
// currently there.
func (f *Store) initQuerySnapshot(ctx context.Context) error {
	q := f.client.Collection(expectationsCollection).Where(crsCLIDField, "==", masterBranch)
	queries := fs_utils.ShardQueryOnDigest(q, digestField, snapshotShards)

	f.masterQuerySnapshots = make([]*firestore.QuerySnapshotIterator, snapshotShards)
	es := make([][]expectationEntry, snapshotShards)
	var eg errgroup.Group
	for shard, q := range queries {
		func(shard int, q firestore.Query) {
			eg.Go(func() error {
				snap := q.Snapshots(ctx)
				qs, err := snap.Next()
				if err != nil {
					return skerr.Wrapf(err, "getting initial snapshot data")
				}
				es[shard] = extractExpectationEntries(qs)

				f.masterQuerySnapshots[shard] = snap
				return nil
			})
		}(shard, q)
	}
	err := eg.Wait()
	if err != nil {
		return skerr.Wrap(err)
	}

	for _, entries := range es {
		for _, e := range entries {
			f.cache.Set(e.Grouping, e.Digest, e.Label)
		}
	}

	return nil
}

// listenToQuerySnapshots takes the f.masterQuerySnapshots from earlier and spins up N
// go routines that listen to those snapshots. If they see new triages (i.e. expectationEntry),
// they update the f.cache (which is protected by cacheMutex).
func (f *Store) listenToQuerySnapshots(ctx context.Context) {
	metrics2.GetCounter("stopped_expstore_shards").Reset()
	for i := 0; i < snapshotShards; i++ {
		go func(shard int) {
			for {
				if err := ctx.Err(); err != nil {
					f.masterQuerySnapshots[shard].Stop()
					sklog.Debugf("Stopping query of snapshots on shard %d due to context err: %s", shard, err)
					metrics2.GetCounter("stopped_expstore_shards").Inc(1)
					return
				}
				qs, err := f.masterQuerySnapshots[shard].Next()
				if err != nil {
					sklog.Errorf("reading query snapshot %d: %s", shard, err)
					f.masterQuerySnapshots[shard].Stop()
					if err := ctx.Err(); err != nil {
						// Oh, it was from a context cancellation (e.g. a test), don't recover.
						return
					}
					// sleep and rebuild the snapshot query. Once a SnapshotQueryIterator returns
					// an error, it seems to always return that error. We sleep for a
					// semi-randomized amount of time to spread out the re-building of shards
					// (as it is likely all the shards will fail at about the same time).
					t := recoverTime + time.Duration(float32(recoverTime)*rand.Float32())
					time.Sleep(t)
					sklog.Infof("Trying to recreate query snapshot %d after having slept %s", shard, t)
					q := f.client.Collection(expectationsCollection).Where(crsCLIDField, "==", masterBranch)
					queries := fs_utils.ShardQueryOnDigest(q, digestField, snapshotShards)
					// This will trigger a complete re-request of this shard's data, to catch any
					// updates that happened while we were not listening.
					f.masterQuerySnapshots[shard] = queries[shard].Snapshots(ctx)
					continue
				}
				entries := extractExpectationEntries(qs)
				func() {
					for _, e := range entries {
						f.cache.Set(e.Grouping, e.Digest, e.Label)
					}
				}()

				if f.notifier != nil {
					for _, e := range entries {
						f.notifier.NotifyChange(expectations.Delta{
							Grouping: e.Grouping,
							Digest:   e.Digest,
							Label:    e.Label,
						})
					}
				}
			}
		}(i)
	}
}

// extractExpectationEntries retrieves all []expectationEntry from a given QuerySnapshot, logging
// any errors (which should be exceedingly rare)
func extractExpectationEntries(qs *firestore.QuerySnapshot) []expectationEntry {
	var entries []expectationEntry
	for _, dc := range qs.Changes {
		if dc.Kind == firestore.DocumentRemoved {
			sklog.Warningf("Unexpected DocumentRemoved event: %#v", dc)
			continue // There will likely never be DocumentRemoved events
		}
		entry := expectationEntry{}
		if err := dc.Doc.DataTo(&entry); err != nil {
			id := dc.Doc.Ref.ID
			sklog.Errorf("corrupt data in firestore, could not unmarshal expectationEntry with id %s", id)
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// getExpectationsForCL returns an Expectations object which is safe to mutate
// that has all cl-specific Expectations.
// It fetches everything from firestore every time, as there could be multiple
// readers and writers and thus caching isn't safe.
func (f *Store) getExpectationsForCL(ctx context.Context) (*expectations.Expectations, error) {
	defer metrics2.FuncTimer().Stop()

	q := f.client.Collection(expectationsCollection).Where(crsCLIDField, "==", f.crsAndCLID)

	es := make([]*expectations.Expectations, clShards)
	queries := fs_utils.ShardQueryOnDigest(q, digestField, clShards)

	maxRetries := 3
	err := f.client.IterDocsInParallel(ctx, "loadExpectations", f.crsAndCLID, queries, maxRetries, maxOperationTime, func(i int, doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		entry := expectationEntry{}
		if err := doc.DataTo(&entry); err != nil {
			id := doc.Ref.ID
			return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal expectationEntry with id %s", id)
		}
		if es[i] == nil {
			es[i] = &expectations.Expectations{}
		}
		es[i].Set(entry.Grouping, entry.Digest, entry.Label)
		return nil
	})

	if err != nil {
		return nil, skerr.Wrapf(err, "fetching expectations for ChangeList %s", f.crsAndCLID)
	}

	e := expectations.Expectations{}
	for _, ne := range es {
		e.MergeExpectations(ne)
	}
	return &e, nil
}

// AddChange implements the ExpectationsStore interface.
func (f *Store) AddChange(ctx context.Context, delta []expectations.Delta, userID string) error {
	defer metrics2.FuncTimer().Stop()
	if f.mode == ReadOnly {
		return ReadOnlyErr
	}
	// Create the entries that we want to write (using the previous values)
	now := time.Now()
	entries, changes := f.flatten(now, delta)

	// Nothing to add
	if len(entries) == 0 {
		return nil
	}

	// firestore can do up to 500 writes at once, we have 2 writes per entry, plus 1 triageRecord
	const batchSize = (ifirestore.MAX_TRANSACTION_DOCS / 2) - 1

	b := f.client.Batch()

	// First write the triage record, with Committed being false (i.e. in progress)
	tr := f.client.Collection(triageRecordsCollection).NewDoc()
	record := triageRecord{
		UserName:   userID,
		TS:         now,
		CRSAndCLID: f.crsAndCLID,
		Changes:    len(entries),
		Committed:  false,
	}
	b.Set(tr, record)
	err := f.client.BatchWrite(ctx, len(entries), batchSize, maxOperationTime, b, func(b *firestore.WriteBatch, i int) error {
		entry := entries[i]
		e := f.client.Collection(expectationsCollection).Doc(entry.ID())
		b.Set(e, entry)

		tc := f.client.Collection(triageChangesCollection).NewDoc()
		change := changes[i]
		change.RecordID = tr.ID
		b.Set(tc, change)
		return nil
	})
	if err != nil {
		// We really hope this doesn't fail, because it could lead to a large batch triage that
		// is partially applied.
		return skerr.Wrap(err)
	}

	// We have succeeded this potentially long write, so mark it completed.
	update := map[string]interface{}{
		committedField: true,
	}
	_, err = f.client.Set(ctx, tr, update, 10, maxOperationTime, firestore.MergeAll)
	return err
}

// flatten creates the data for the Documents to be written for a given Expectations delta.
// It requires that the f.cache is safe to read (i.e. the mutex is held), because
// it needs to determine the previous values.
func (f *Store) flatten(now time.Time, delta []expectations.Delta) ([]expectationEntry, []triageChanges) {
	var entries []expectationEntry
	var changes []triageChanges

	for _, d := range delta {
		entries = append(entries, expectationEntry{
			Grouping:   d.Grouping,
			Digest:     d.Digest,
			Label:      d.Label,
			Updated:    now,
			CRSAndCLID: f.crsAndCLID,
		})

		changes = append(changes, triageChanges{
			// RecordID will be filled out later
			Grouping:    d.Grouping,
			Digest:      d.Digest,
			LabelBefore: f.cache.Classification(d.Grouping, d.Digest),
			LabelAfter:  d.Label,
		})
	}
	return entries, changes
}

// QueryLog implements the ExpectationsStore interface.
func (f *Store) QueryLog(ctx context.Context, offset, size int, details bool) ([]expectations.TriageLogEntry, int, error) {
	if offset < 0 || size <= 0 {
		return nil, -1, skerr.Fmt("offset: %d and size: %d must be positive", offset, size)
	}
	defer metrics2.FuncTimer().Stop()

	// Fetch the records, which have everything except the details.
	q := f.client.Collection(triageRecordsCollection).OrderBy(tsField, firestore.Desc).Offset(offset).Limit(size)
	q = q.Where(crsCLIDField, "==", f.crsAndCLID).Where(committedField, "==", true)
	var rv []expectations.TriageLogEntry
	d := fmt.Sprintf("offset: %d, size %d", offset, size)
	err := f.client.IterDocs(ctx, "query_log", d, q, 3, maxOperationTime, func(doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		tr := triageRecord{}
		if err := doc.DataTo(&tr); err != nil {
			id := doc.Ref.ID
			return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal triageRecord with id %s", id)
		}
		rv = append(rv, expectations.TriageLogEntry{
			ID:          doc.Ref.ID,
			User:        tr.UserName,
			TS:          tr.TS,
			ChangeCount: tr.Changes,
		})
		return nil
	})
	if err != nil {
		return nil, -1, skerr.Wrapf(err, "could not request triage records [%d: %d]", offset, size)
	}

	n := len(rv)
	if n == size && n != 0 {
		// We don't know how many there are and it might be too slow to count, so just give
		// the "many" response.
		n = expectations.CountMany
	} else {
		// We know exactly either 1) how many there are (if n > 0) or 2) an upper bound on how many
		// there are (if n == 0)
		n += offset
	}

	if len(rv) == 0 || !details {
		return rv, n, nil
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
	err = f.client.IterDocsInParallel(ctx, "query_log_details", d, qs, 3, maxOperationTime, func(i int, doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		tc := triageChanges{}
		if err := doc.DataTo(&tc); err != nil {
			id := doc.Ref.ID
			return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal triageChanges with id %s", id)
		}
		rv[i].Details = append(rv[i].Details, expectations.Delta{
			Grouping: tc.Grouping,
			Digest:   tc.Digest,
			Label:    tc.LabelAfter,
		})
		return nil
	})
	if err != nil {
		return nil, -1, skerr.Wrapf(err, "could not query details")
	}

	return rv, n, nil
}

// UndoChange implements the ExpectationsStore interface.
func (f *Store) UndoChange(ctx context.Context, changeID, userID string) error {
	defer metrics2.FuncTimer().Stop()
	if f.mode == ReadOnly {
		return ReadOnlyErr
	}
	// Verify the original change id exists.
	dr := f.client.Collection(triageRecordsCollection).Doc(changeID)
	doc, err := f.client.Get(ctx, dr, 3, maxOperationTime)
	if err != nil || !doc.Exists() {
		return skerr.Wrapf(err, "could not find change to undo with id %s", changeID)
	}

	q := f.client.Collection(triageChangesCollection).Where(recordIDField, "==", changeID)
	var delta []expectations.Delta
	err = f.client.IterDocs(ctx, "undo_query", changeID, q, 3, maxOperationTime, func(doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		tc := triageChanges{}
		if err := doc.DataTo(&tc); err != nil {
			id := doc.Ref.ID
			return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal triageChanges with id %s", id)
		}
		delta = append(delta, expectations.Delta{
			Grouping: tc.Grouping,
			Digest:   tc.Digest,
			Label:    tc.LabelBefore,
		})
		return nil
	})
	if err != nil {
		return skerr.Wrapf(err, "could not get delta to undo %s", changeID)
	}

	if err = f.AddChange(ctx, delta, userID); err != nil {
		return skerr.Wrapf(err, "could not apply delta to undo %s", changeID)
	}

	return nil
}

// Make sure Store fulfills the expectations.Store interface
var _ expectations.Store = (*Store)(nil)
