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
	// Should be used to create the firestore.NewClient that is passed into this struct.
	ExpectationStoreCollection = "expstore"

	expectationsCollection  = "expectations"
	triageRecordsCollection = "triage_records"
	triageChangesCollection = "triage_changes"

	// Columns in the Collections we query by
	issueCol = "issue"

	maxOperationTime = 2 * time.Minute
)

type ExpectationsFirestore struct {
	client *ifirestore.Client
	mode   AccessMode
	issue  int64 // Gerrit or GitHub issue, or MasterBranch

	cacheMutex sync.RWMutex
	cache      types.Expectations
}

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

type triageRecord struct {
	UserName  string    `firestore:"user"`
	TS        time.Time `firestore:"ts"`
	Issue     int64     `firestore:"issue"`
	Changes   int       `firestore:"changes"`
	Committed bool      `firestore:"committed"`
}

type triageChanges struct {
	RecordID    string         `firestore:"record_id"`
	Grouping    types.TestName `firestore:"grouping"`
	Digest      types.Digest   `firestore:"digest"`
	LabelBefore types.Label    `firestore:"before"`
	LabelAfter  types.Label    `firestore:"after"`
}

func New(client *ifirestore.Client, issue int64, mode AccessMode) *ExpectationsFirestore {
	return &ExpectationsFirestore{
		client: client,
		issue:  issue,
		mode:   mode,
	}
}

func (f *ExpectationsFirestore) Get() (types.Expectations, error) {
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

func (f *ExpectationsFirestore) loadExpectations() (types.Expectations, error) {
	e := types.Expectations{}
	q := f.client.Collection(expectationsCollection).Where(issueCol, "==", MasterBranch)
	err := f.client.IterDocs("loadExpectations", "", q, 3, maxOperationTime, func(doc *firestore.DocumentSnapshot) error {
		entry := expectationEntry{}
		if err := doc.DataTo(&entry); err != nil {
			return err
		}
		e.AddDigest(entry.Grouping, entry.Digest, entry.Label)
		return nil
	})
	return e, err
}

func (f *ExpectationsFirestore) AddChange(ctx context.Context, newExp types.Expectations, userId string) error {
	if f.mode != ReadWrite {
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

	tr := f.client.Collection(triageRecordsCollection).NewDoc()
	record := triageRecord{
		UserName:  userId,
		TS:        now,
		Issue:     f.issue,
		Changes:   len(entries),
		Committed: false,
	}
	b.Set(tr, record)

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

func (f *ExpectationsFirestore) flatten(now time.Time, newExp types.Expectations) ([]expectationEntry, []triageChanges) {
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
