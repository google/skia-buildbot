// Implements an ExpectationsStore based on Firestore. See FIRESTORE.md for the schema
// and design rationale.
package fs_expstore

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
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
	"go.skia.org/infra/golden/go/fs_utils"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/types"
	"golang.org/x/sync/errgroup"
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
	// masterShards was determined empirically on a data set of about 550k expectationEntry
	// 1 shard -> ???
	// 10 shards -> 215s
	// 100 shards -> 21s
	// 256 shards -> 10s
	// 512 shards -> 9s
	// 4096 shards -> 9s
	masterShards = 512

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

// Store implements expstorage.ExpectationsStore backed by
// Firestore. It has a write-through caching mechanism.
type Store struct {
	client     *ifirestore.Client
	mode       AccessMode
	crsAndCLID string // crs+"_"+id. Empty string means master branch.

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

	masterQuerySnapshots []*firestore.QuerySnapshotIterator
}

// expectationEntry is the document type stored in the expectationsCollection.
type expectationEntry struct {
	Grouping   types.TestName `firestore:"grouping"`
	Digest     types.Digest   `firestore:"digest"`
	Label      types.Label    `firestore:"label"`
	Updated    time.Time      `firestore:"updated"`
	CRSAndCLID string         `firestore:"crs_cl_id"`
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
	RecordID    string         `firestore:"record_id"`
	Grouping    types.TestName `firestore:"grouping"`
	Digest      types.Digest   `firestore:"digest"`
	LabelBefore types.Label    `firestore:"before"`
	LabelAfter  types.Label    `firestore:"after"`
}

// New returns a new Store using the given firestore client. The Store will track
// masterBranch- see ForChangeList() for getting Stores that track ChangeLists.
// The passed in context is used for the QuerySnapshots (in ReadOnly mode).
func New(ctx context.Context, client *ifirestore.Client, eventBus eventbus.EventBus, mode AccessMode) (*Store, error) {
	defer metrics2.FuncTimer().Stop()
	defer shared.NewMetricsTimer("expstore_init").Stop()
	f := &Store{
		client:         client,
		eventBus:       eventBus,
		eventExpChange: expstorage.EV_EXPSTORAGE_CHANGED,
		globalEvent:    true,
		crsAndCLID:     masterBranch,
		mode:           mode,
	}

	if mode == ReadOnly {
		// Start the snapshots (which will ignore any results)
		err := f.initQuerySnapshot(ctx)
		if err != nil {
			return nil, skerr.Wrapf(err, "could not get initial query snapshot")
		}
	}

	// pre-load the cache. This simplifies the mutex handling in Get().
	_, err := f.getMasterExpectations()
	if err != nil {
		return nil, skerr.Wrapf(err, "could not perform initial get")
	}
	sklog.Infof("Loaded master expectations for %d tests", len(f.cache))

	if mode == ReadOnly {
		// Starts several go routines to listen to the snapshots created earlier.
		// If there were any new entries added after the snapshots were created, but before
		// we did a full fetch, we'll see them on the first pass through.
		f.listenToQuerySnapshots(ctx)
	}

	return f, nil
}

// ForChangeList implements the ExpectationsStore interface.
func (f *Store) ForChangeList(id, crs string) expstorage.ExpectationsStore {
	if id == masterBranch {
		// It is invalid to re-request the master branch
		return nil
	}
	return &Store{
		client:         f.client,
		eventBus:       f.eventBus,
		eventExpChange: expstorage.EV_TRYJOB_EXP_CHANGED,
		globalEvent:    false,
		crsAndCLID:     crs + "_" + id,
		mode:           f.mode,
	}
}

// Get implements the ExpectationsStore interface.
func (f *Store) Get() (types.Expectations, error) {
	if f.crsAndCLID == masterBranch {
		defer metrics2.NewTimer("gold_get_expectations", map[string]string{"master_branch": "true"}).Stop()
		f.cacheMutex.RLock()
		defer f.cacheMutex.RUnlock()
		return f.getMasterExpectations()
	}
	defer metrics2.NewTimer("gold_get_expectations", map[string]string{"master_branch": "false"}).Stop()
	return f.getExpectationsForCL()
}

// getMasterExpectations returns an Expectations object which is safe to mutate
// based on the current state. It is expected the caller has taken care of any mutex grabbing.
func (f *Store) getMasterExpectations() (types.Expectations, error) {
	if f.cache == nil {
		c, err := f.loadExpectationsSharded(masterBranch, masterShards)
		if err != nil {
			return nil, skerr.Fmt("could not load master expectations from firestore: %s", err)
		}
		f.cache = c
	}
	return f.cache.DeepCopy(), nil
}

// initQuerySnapshot creates many firestore.QuerySnapshotIterator objects based on a shard of
// all expectations and does the first Next() on them (which will try to return all data
// in those shards). We ignore that data, as a future call will update the cached expectations.
// It stores these iterators in f.masterQuerySnapshots. Without sharding them, this times out
// with many expectations because of the fact that the first call to Next() fetches all data
// currently there.
// TODO(kjlubick): if this works well for the ReadOnly case, could we maybe do something similar
// for the read-write case? I don't think we'd ever have two writers, but it would open us up
// to that.
func (f *Store) initQuerySnapshot(ctx context.Context) error {
	q := f.client.Collection(expectationsCollection).Where(crsCLIDField, "==", masterBranch)
	queries := fs_utils.ShardQueryOnDigest(q, digestField, snapshotShards)
	f.masterQuerySnapshots = make([]*firestore.QuerySnapshotIterator, snapshotShards)
	var eg errgroup.Group
	for shard, q := range queries {
		func(shard int, q firestore.Query) {
			eg.Go(func() error {
				snap := q.Snapshots(ctx)
				_, err := snap.Next()
				if err != nil {
					return skerr.Wrapf(err, "getting initial snapshot data")
				}
				f.masterQuerySnapshots[shard] = snap
				return nil
			})
		}(shard, q)
	}

	return eg.Wait()
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
				e := types.Expectations{}
				for _, dc := range qs.Changes {
					if dc.Kind == firestore.DocumentRemoved {
						continue // There will likely never be DocumentRemoved events
					}
					entry := expectationEntry{}
					if err := dc.Doc.DataTo(&entry); err != nil {
						id := dc.Doc.Ref.ID
						sklog.Errorf("corrupt data in firestore, could not unmarshal entry with id %s", id)
						continue
					}
					sklog.Debugf("Query Snapshot saw: %s %s %s\n", entry.Grouping, entry.Digest, entry.Label.String())
					e.AddDigest(entry.Grouping, entry.Digest, entry.Label)
				}
				func() {
					f.cacheMutex.Lock()
					defer f.cacheMutex.Unlock()
					f.cache.MergeExpectations(e)
				}()
			}
		}(i)
	}
}

// getExpectationsForCL returns an Expectations object which is safe to mutate
// that has all cl-specific Expectations.
// It fetches everything from firestore every time, as there could be multiple
// readers and writers and thus caching isn't safe.
func (f *Store) getExpectationsForCL() (types.Expectations, error) {
	expDelta, err := f.loadExpectationsSharded(f.crsAndCLID, clShards)
	if err != nil {
		return nil, skerr.Wrapf(err, "loading expectations delta for CL %s from firestore", f.crsAndCLID)
	}
	return expDelta, nil
}

// loadExpectationsSharded returns an Expectations object from the expectationsCollection,
// with all Expectations belonging to the passed in crsAndCLID (can be masterBranch).
func (f *Store) loadExpectationsSharded(crsAndCLID string, shards int) (types.Expectations, error) {
	defer metrics2.FuncTimer().Stop()
	q := f.client.Collection(expectationsCollection).Where(crsCLIDField, "==", crsAndCLID)

	es := make([]types.Expectations, shards)
	queries := fs_utils.ShardQueryOnDigest(q, digestField, shards)

	maxRetries := 3
	err := f.client.IterDocsInParallel("loadExpectations", crsAndCLID, queries, maxRetries, maxOperationTime, func(i int, doc *firestore.DocumentSnapshot) error {
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

	if err != nil {
		return nil, skerr.Wrapf(err, "fetching expectations for ChangeList %s", crsAndCLID)
	}

	e := types.Expectations{}
	for _, ne := range es {
		e.MergeExpectations(ne)
	}
	return e, nil
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
			ExpectationDelta: newExp,
			CRSAndCLID:       f.crsAndCLID,
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
		UserName:   userID,
		TS:         now,
		CRSAndCLID: f.crsAndCLID,
		Changes:    len(entries),
		Committed:  false,
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
			return skerr.Wrapf(err, "writing entries with retry [%d, %d]", i, stop)
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
				Grouping:   testName,
				Digest:     digest,
				Label:      label,
				Updated:    now,
				CRSAndCLID: f.crsAndCLID,
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
	q = q.Where(crsCLIDField, "==", f.crsAndCLID).Where(committedField, "==", true)
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
