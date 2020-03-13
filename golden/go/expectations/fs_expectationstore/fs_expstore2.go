package fs_expectationstore

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"golang.org/x/sync/errgroup"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/fs_utils"
	"go.skia.org/infra/golden/go/types"
)

const (
	partitions         = "expstore_partitions_v3"
	expectationEntries = "entries"
	recordEntries      = "triage_records"
	changeEntries      = "triage_changes"

	masterPartition = "master"

	beginningOfTime = 0
	endOfTime       = math.MaxInt32

	maxRetries = 3

	// The number of shards was determined experimentally based on 500k and then 100k expectation
	// entries. 100k is about what we expect on a master partition. 100 is a typical guess for any
	// particular CL expectation, so we can get away with fewer shards.
	masterPartitionShards = 16
	clPartitionShards     = 2
)

// Store2 implements expectations.Store backed by Firestore. It has a local expectationCache of the
// expectations to reduce load on firestore
type Store2 struct {
	client *ifirestore.Client
	mode   AccessMode

	// Sharding our loading of expectations can drastically improve throughput. The number of shards
	// can be different, depending on the approximate size of partition. We generally assume that
	// the number of expectations on the master partition is much much bigger than on any other
	// partition.
	numShards int

	// CL expectations are kept apart from those on master by dividing how we store expectations
	// into partitions.
	partition string

	// notifier allows this Store to communicate with the outside world when
	// expectations change.
	notifier expectations.ChangeNotifier

	// entryCache is an in-memory representation of the expectations in Firestore.
	entryCache      map[expectations.ID]expectationEntry2
	entryCacheMutex sync.RWMutex

	// returnCache allows us to cache the return value for Get() if there haven't been any changes
	// to the expectations since the previous call.
	returnCache *expectations.Expectations
	// this mutex is for the returnCacheObject (i.e. setting it to nil. expectations.Expectations
	// is thread-safe for setting/updating).
	returnCacheMutex sync.Mutex

	// hasSnapshotsRunning tracks if the snapshots were started due to a call from Initialize().
	hasSnapshotsRunning bool
	now                 func() time.Time
}

// expectationEntry2 is the document type stored in the expectationsCollection.
type expectationEntry2 struct {
	Grouping types.TestName `firestore:"grouping"`
	Digest   types.Digest   `firestore:"digest"`
	Updated  time.Time      `firestore:"updated"`
	LastUsed time.Time      `firestore:"last_used"`
	// This is sorted by FirstIndex and should have no duplicate sets for FirstIndex and LastIndex.
	Ranges []triageRange `firestore:"ranges"`
}

// ID returns the deterministic ID that lets us update existing entries.
func (e *expectationEntry2) ID() string {
	s := string(e.Grouping) + "|" + string(e.Digest)
	// firestore gets cranky if there are / in key names
	return strings.Replace(s, "/", "-", -1)
}

// expectationChange represents the changing of a single expectation entry.
type expectationChange struct {
	RecordID      string             `firestore:"record_id"`
	Grouping      types.TestName     `firestore:"grouping"`
	Digest        types.Digest       `firestore:"digest"`
	AffectedRange triageRange        `firestore:"affected_range"`
	LabelBefore   expectations.Label `firestore:"label_before"`
}

type triageRange struct {
	FirstIndex int                `firestore:"first_index"`
	LastIndex  int                `firestore:"last_index"`
	Label      expectations.Label `firestore:"label"`
}

// triageRecord represents a group of changes made in a single triage action by a user.
type triageRecord2 struct {
	UserName  string    `firestore:"user"`
	TS        time.Time `firestore:"ts"`
	Changes   int       `firestore:"changes"`
	Committed bool      `firestore:"committed"`
}

func New2(client *ifirestore.Client, cn expectations.ChangeNotifier, mode AccessMode) *Store2 {
	return &Store2{
		client:     client,
		notifier:   cn,
		partition:  masterPartition,
		numShards:  masterPartitionShards,
		mode:       mode,
		entryCache: map[expectations.ID]expectationEntry2{},
		now:        time.Now,
	}
}

// Initialize begins several goroutines which monitor firestore QuerySnapshots, will begin
// watching for changes to the expectations, keeping the cache fresh. This also loads the initial
// set of expectation entries into the local RAM cache.
func (s *Store2) Initialize(ctx context.Context) error {
	// Make the initial query of all expectations currently in the store, sharded so as to improve
	// performance.
	queries := fs_utils.ShardOnDigest(s.expectationsCollection(), digestField, s.numShards)
	expectationSnapshots := make([]*firestore.QuerySnapshotIterator, s.numShards)
	es := make([][]expectationEntry2, s.numShards)
	var eg errgroup.Group
	for shard, q := range queries {
		func(shard int, q firestore.Query) {
			eg.Go(func() error {
				snap := q.Snapshots(ctx)
				qs, err := snap.Next()
				if err != nil {
					return skerr.Wrapf(err, "getting initial snapshot data shard[%d]", shard)
				}
				es[shard] = extractExpectationEntries2(qs)
				expectationSnapshots[shard] = snap
				return nil
			})
		}(shard, q)
	}
	err := eg.Wait()
	if err != nil {
		return skerr.Wrap(err)
	}

	// Now we load the RAM cache with all of the loaded expectations.
	s.entryCacheMutex.Lock()
	defer s.entryCacheMutex.Unlock()
	for _, entries := range es {
		for _, e := range entries {
			// Due to how we shard our queries, there shouldn't be any overwriting of one shard
			// over another.
			id := expectations.ID{
				Grouping: e.Grouping,
				Digest:   e.Digest,
			}
			s.entryCache[id] = e
		}
	}

	// Re-using those shards from earlier, we start goroutines to wait for any new expectation
	// entries to be seen or changes to existing ones. When those happen, we update the cache.
	// TODO(kjlubick) add an alert for this metric if it becomes non-zero.
	stoppedShardsMetric := metrics2.GetCounter("stopped_expstore_shards")
	stoppedShardsMetric.Reset()
	for i := 0; i < s.numShards; i++ {
		go func(shard int) {
			snapFactory := func() *firestore.QuerySnapshotIterator {
				queries := fs_utils.ShardOnDigest(s.expectationsCollection(), digestField, s.numShards)
				return queries[shard].Snapshots(ctx)
			}
			// reuse the initial snapshots, so we don't have to re-load all the data again (for
			// which there could be a lot).
			err := fs_utils.ListenAndRecover(ctx, expectationSnapshots[shard], snapFactory, s.updateCacheAndNotify)
			if err != nil {
				sklog.Errorf("Unrecoverable error: %s", err)
			}
			stoppedShardsMetric.Inc(1)
		}(i)
	}
	s.hasSnapshotsRunning = true
	return nil
}

// updateCacheAndNotify updates the cached expectations with the given snapshot and then sends
// notifications to the notifier about the updates.
func (s *Store2) updateCacheAndNotify(_ context.Context, qs *firestore.QuerySnapshot) error {
	entries := extractExpectationEntries2(qs)
	var toNotify []expectations.Delta
	s.entryCacheMutex.Lock()
	defer s.entryCacheMutex.Unlock()
	for _, newEntry := range entries {
		id := expectations.ID{
			Grouping: newEntry.Grouping,
			Digest:   newEntry.Digest,
		}
		existing, ok := s.entryCache[id]
		// We always update to the cached version.
		s.entryCache[id] = newEntry
		if ok {
			// We get notifications when UpdateLastUsed updates timestamps. These don't require an
			// event to be published since they do not affect the labels. The best way to check if
			// something material changed is to compare the Updated timestamp.
			if existing.Updated.Equal(newEntry.Updated) {
				continue
			}
		}
		toNotify = append(toNotify, expectations.Delta{
			Grouping: newEntry.Grouping,
			Digest:   newEntry.Digest,
		})
	}
	if len(toNotify) > 0 {
		// There were a non-zero amount of actual changes to the expectations. Purge the cached
		// version that we return from Get.
		func() {
			s.returnCacheMutex.Lock()
			defer s.returnCacheMutex.Unlock()
			s.returnCache = nil
		}()
	}

	if s.notifier != nil {
		for _, e := range toNotify {
			s.notifier.NotifyChange(expectations.Delta{
				Grouping: e.Grouping,
				Digest:   e.Digest,
				Label:    e.Label,
			})
		}
	}
	return nil
}

// extractExpectationEntries2 retrieves all []expectationEntry2 from a given QuerySnapshot, logging
// any errors (which should be exceedingly rare).
func extractExpectationEntries2(qs *firestore.QuerySnapshot) []expectationEntry2 {
	var entries []expectationEntry2
	for _, dc := range qs.Changes {
		if dc.Kind == firestore.DocumentRemoved {
			// TODO(kjlubick): It would probably be good to return a slice of expectation.IDs that
			//   can get removed from the cache.
			continue
		}
		entry := expectationEntry2{}
		if err := dc.Doc.DataTo(&entry); err != nil {
			id := dc.Doc.Ref.ID
			sklog.Errorf("corrupt data in firestore, could not unmarshal expectationEntry2 with id %s", id)
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// ForChangeList implements the ExpectationsStore interface.
func (s *Store2) ForChangeList(id, crs string) *Store2 {
	if id == "" || crs == "" {
		// These must both be specified
		return nil
	}
	return &Store2{
		client:    s.client,
		numShards: clPartitionShards,
		notifier:  nil, // we do not need to notify when ChangeList expectations change.
		partition: crs + "_" + id,
		mode:      s.mode,
	}
}

// AddChange implements the ExpectationsStore interface.
func (s *Store2) AddChange(ctx context.Context, delta []expectations.Delta, userID string) error {
	defer metrics2.FuncTimer().Stop()
	if s.mode == ReadOnly {
		return ReadOnlyErr
	}
	// Create the entries that we want to write (using the previous values)
	now := s.now()
	// TODO(kjlubick) If we support ranges, these constants will need to be changed.
	entries, changes := s.makeEntriesAndChanges(now, delta, beginningOfTime, endOfTime)

	// Nothing to add
	if len(entries) == 0 {
		return nil
	}

	// firestore can do up to 500 writes at once, we have 2 writes per entry, plus 1 triageRecord
	const batchSize = (ifirestore.MAX_TRANSACTION_DOCS / 2) - 1

	b := s.client.Batch()

	// First write the triage record, with Committed being false (i.e. in progress)
	tr := s.recordsCollection().NewDoc()
	record := triageRecord2{
		UserName:  userID,
		TS:        now,
		Changes:   len(entries),
		Committed: false,
	}
	b.Set(tr, record)
	err := s.client.BatchWrite(ctx, len(entries), batchSize, maxOperationTime, b, func(b *firestore.WriteBatch, i int) error {
		entry := entries[i]
		e := s.expectationsCollection().Doc(entry.ID())
		b.Set(e, entry)

		tc := s.changesCollection().NewDoc()
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
	_, err = s.client.Set(ctx, tr, update, 10, maxOperationTime, firestore.MergeAll)
	return err
}

func (s *Store2) makeEntriesAndChanges(now time.Time, delta []expectations.Delta, firstIdx, lastIdx int) ([]expectationEntry2, []expectationChange) {
	s.entryCacheMutex.RLock()
	defer s.entryCacheMutex.RUnlock()
	var entries []expectationEntry2
	var changes []expectationChange

	for _, d := range delta {
		// Fetch what was there before - mainly to get the previous Ranges
		entry := s.entryCache[expectations.ID{
			Grouping: d.Grouping,
			Digest:   d.Digest,
		}]
		// These do nothing if the entry was already cached.
		entry.Grouping = d.Grouping
		entry.Digest = d.Digest
		// We intentionally do not set LastUsed here. It will be the zero value for time when
		// created, and we don't want to update LastUsed simply by triaging it.

		// Update Updated and Ranges with the new data.
		entry.Updated = now
		newRange := triageRange{
			FirstIndex: firstIdx,
			LastIndex:  lastIdx,
			Label:      d.Label,
		}
		previousLabel := expectations.Untriaged
		replacedRange := false
		// TODO(kjlubick): if needed, this could be a binary search, but since there will be < 20
		//   ranges for almost all entries, it probably doesn't matter.
		for i, r := range entry.Ranges {
			if r.FirstIndex == firstIdx && r.LastIndex == lastIdx {
				replacedRange = true
				previousLabel = r.Label
				entry.Ranges[i] = newRange
				break
			}
		}
		if !replacedRange {
			entry.Ranges = append(entry.Ranges, newRange)
			sort.Slice(entry.Ranges, func(i, j int) bool {
				return entry.Ranges[i].FirstIndex < entry.Ranges[j].FirstIndex
			})
		}

		entries = append(entries, entry)
		changes = append(changes, expectationChange{
			// RecordID will be filled out later
			Grouping:      d.Grouping,
			Digest:        d.Digest,
			AffectedRange: newRange,
			LabelBefore:   previousLabel,
		})
	}
	return entries, changes
}

func (s *Store2) Get(ctx context.Context) (expectations.ReadOnly, error) {
	if s.hasSnapshotsRunning {
		// If the snapshots are running, we first check to see if we have a fresh Expectations.
		s.returnCacheMutex.Lock()
		defer s.returnCacheMutex.Unlock()
		if s.returnCache != nil {
			return s.returnCache, nil
		}
		// At this point, we do not have a fresh expectation.Expectations (something has changed
		// since the last time we made it), so we assemble a new one from our in-RAM cache of
		// expectation entries (which we assume to be up to date courtesy of our running snapshots).
		e := s.assembleExpectations()
		s.returnCache = e
		return e, nil
	}
	// If the snapshots are not loaded, we assume we do not have a RAM cache and load the
	// expectations from Firestore.
	return s.loadExpectations(ctx)
}

func (s *Store2) loadExpectations(ctx context.Context) (*expectations.Expectations, error) {
	defer metrics2.FuncTimer().Stop()
	es := make([]*expectations.Expectations, s.numShards)
	queries := fs_utils.ShardOnDigest(s.expectationsCollection(), digestField, s.numShards)

	err := s.client.IterDocsInParallel(ctx, "loadExpectations", s.partition, queries, maxRetries, maxOperationTime, func(i int, doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		entry := expectationEntry2{}
		if err := doc.DataTo(&entry); err != nil {
			id := doc.Ref.ID
			return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal expectationEntry2 with id %s", id)
		}
		if len(entry.Ranges) == 0 {
			// This should never happen, but we'll ignore these malformed entries if they do.
			return nil
		}
		if es[i] == nil {
			es[i] = &expectations.Expectations{}
		}
		// TODO(kjlubick) If we decide to handle ranges of expectations, Get will need to take a
		//   parameter indicating the commit index for which we should return valid ranges.
		es[i].Set(entry.Grouping, entry.Digest, entry.Ranges[0].Label)
		return nil
	})

	if err != nil {
		return nil, skerr.Wrapf(err, "fetching expectations for partition %s", s.partition)
	}

	e := expectations.Expectations{}
	for _, ne := range es {
		e.MergeExpectations(ne)
	}
	return &e, nil
}

func (s *Store2) assembleExpectations() *expectations.Expectations {
	s.entryCacheMutex.RLock()
	defer s.entryCacheMutex.RUnlock()

	e := &expectations.Expectations{}
	for id, entry := range s.entryCache {
		if len(entry.Ranges) == 0 {
			sklog.Warningf("ignoring invalid entry for id %s", id)
			continue
		}
		// TODO(kjlubick) If we decide to handle ranges of expectations, Get will need to take a
		//   parameter indicating the commit index for which we should return valid ranges.
		e.Set(entry.Grouping, entry.Digest, entry.Ranges[0].Label)
	}
	return e
}

// GetTriageHistory implements the expectations.Store interface.
func (s *Store2) GetTriageHistory(ctx context.Context, grouping types.TestName, digest types.Digest) ([]expectations.TriageHistory, error) {
	defer metrics2.FuncTimer().Stop()
	q := s.changesCollection().Where(groupingField, "==", grouping).Where(digestField, "==", digest)
	entryID := fmt.Sprintf("%s-%s", grouping, digest)
	var recordsToFetch []*firestore.DocumentRef
	err := s.client.IterDocs(ctx, "triage_history", entryID, q, 3, maxOperationTime, func(doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		tc := triageChanges{}
		if err := doc.DataTo(&tc); err != nil {
			id := doc.Ref.ID
			return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal triage change with id %s", id)
		}
		recordsToFetch = append(recordsToFetch, s.recordsCollection().Doc(tc.RecordID))
		return nil
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "getting history for %s", entryID)
	}
	if len(recordsToFetch) == 0 {
		return nil, nil
	}
	records, err := s.client.GetAll(ctx, recordsToFetch)
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching %d records belonging to %s", len(recordsToFetch), entryID)
	}
	var rv []expectations.TriageHistory
	for _, doc := range records {
		if doc == nil {
			continue
		}
		tr := triageRecord2{}
		if err := doc.DataTo(&tr); err != nil {
			id := doc.Ref.ID
			return nil, skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal triage record with id %s", id)
		}
		rv = append(rv, expectations.TriageHistory{
			User: tr.UserName,
			TS:   tr.TS,
		})
	}
	sort.Slice(rv, func(i, j int) bool {
		return rv[i].TS.After(rv[j].TS)
	})
	return rv, nil
}

func (s *Store2) expectationsCollection() *firestore.CollectionRef {
	return s.client.Collection(partitions).Doc(s.partition).Collection(expectationEntries)
}

func (s *Store2) recordsCollection() *firestore.CollectionRef {
	return s.client.Collection(partitions).Doc(s.partition).Collection(recordEntries)
}

func (s *Store2) changesCollection() *firestore.CollectionRef {
	return s.client.Collection(partitions).Doc(s.partition).Collection(changeEntries)
}
