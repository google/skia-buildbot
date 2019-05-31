// Implements an ExpectationsStore based on Firestore. See FIRESTORE.md for the schema
// and design rationale.
package fs_expstore

import (
	"context"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
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

const (
	// Should be used to create the firestore.NewClient that is passed into this struct.
	ExpectationStoreCollection = "expstore"

	expectationsCollection  = "expectations"
	triageRecordsCollection = "triage_records"
	triageChangesCollection = "triage_changes"

	// Columns in expectationEntry document
	groupingCol = "grouping"
	digestCol   = "digest"
	labelCol    = "label"
	updatedCol  = "updated"
	issueCol    = "issue"

	maxLoadTime = 2 * time.Minute
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

type triageRecord struct {
	UserName string    `firestore:"user"`
	TS       time.Time `firestore:"ts"`
	Issue    int64     `firestore:"issue"`
	Changes  int       `firestore:"changes"`
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
	if f.cache != nil {
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
	err := f.client.IterDocs("loadExpectations", "", q, 3, maxLoadTime, func(doc *firestore.DocumentSnapshot) error {
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
	// Create the entries that we want to write (using the previous values)
	now := time.Now()
	entries, changes := f.flatten(now, newExp)

	// Nothing to add
	if len(entries) == 0 {
		return nil
	}

	// firestore can do up to 500 writes at once, we have 2 writes per entry, plus 1 triageRecord
	batchSize := (500 / 2) - 1

	b := f.client.Batch()

	tr := f.client.Collection(triageRecordsCollection).NewDoc()
	record := triageRecord{
		UserName: userId,
		TS:       now,
		Issue:    f.issue,
		Changes:  len(entries),
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
		if _, err := b.Commit(ctx); err != nil {
			return skerr.Fmt("problem writing entries[%d, %d]: %s", i, stop, err)
		}
	}
	// Write the changes to the locale cache.
	f.cacheMutex.Lock()
	defer f.cacheMutex.Unlock()
	if f.cache == nil {
		f.cache = newExp.DeepCopy()
	} else {
		f.cache.MergeExpectations(newExp)
	}
	return nil
}

func (f *ExpectationsFirestore) flatten(now time.Time, newExp types.Expectations) ([]expectationEntry, []triageChanges) {
	f.cacheMutex.RLock()
	defer f.cacheMutex.RUnlock()
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

func (e *expectationEntry) ID() string {
	return string(e.Grouping) + "|" + string(e.Digest)
}
