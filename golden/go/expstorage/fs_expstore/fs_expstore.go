// Implements an ExpectationsStore based on Firestore. See FIRESTORE.md for the schema
// and design rationale.
package fs_expstore

import (
	"context"
	"errors"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/cenkalti/backoff"
	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/types"
)

// AccessMode indicates if this ExpectationsStore can update existing Expectations
// in the backing store or if if can only read them.
type AccessMode int

const (
	ReadOnly AccessMode = iota
	ReadWrite
)

const (
	MasterBranch = int64(0)
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
	issueCol = "issue"

	maxOperationTime = 2 * time.Minute
)

// Store implements expstorage.ExpectationsStore backed by
// Firestore. It has a write-through caching mechanism.
type Store struct {
	client *ifirestore.Client
	mode   AccessMode
	issue  int64 // Gerrit or GitHub issue, or MasterBranch

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

// New returns a new Store using the given firestore client. The issue param is used
// to indicate if this Store is configured to read/write the baselines for a given CL
// or if it is on MasterBranch.
func New(client *ifirestore.Client, issue int64, mode AccessMode) *Store {
	return &Store{
		client: client,
		issue:  issue,
		mode:   mode,
	}
}

// Get implements the ExpectationsStore interface.
func (f *Store) Get() (types.Expectations, error) {
	f.cacheMutex.RLock()
	defer f.cacheMutex.RUnlock()
	if f.cache == nil {
		c, err := f.loadExpectations()
		if err != nil {
			return nil, skerr.Fmt("could not load expectations from firestore: %s", err)
		}
		f.cache = c
	}
	return f.cache.DeepCopy(), nil
}

// loadExpectations reads the entire Expectations from the expectationsCollection,
// matching the configured branch.
func (f *Store) loadExpectations() (types.Expectations, error) {
	e := types.Expectations{}
	q := f.client.Collection(expectationsCollection).Where(issueCol, "==", MasterBranch)
	maxRetries := 3
	err := f.client.IterDocs("loadExpectations", "", q, maxRetries, maxOperationTime, func(doc *firestore.DocumentSnapshot) error {
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
func (f *Store) AddChange(ctx context.Context, newExp types.Expectations, userId string) error {
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
		UserName:  userId,
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
