package main

import (
	"context"
	"flag"
	"math"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"go.skia.org/infra/golden/go/fs_utils"
	"google.golang.org/api/iterator"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/types"
)

const (
	maxOperationTime = 2 * time.Minute
	maxRetries       = 5

	dryrun = true
)

func main() {
	var (
		fsProjectID    = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
		oldFSNamespace = flag.String("old_fs_namespace", "", "Typically the instance id. e.g. 'chrome-gpu', 'skia', etc")
		newFSNamespace = flag.String("new_fs_namespace", "", "Typically the instance id. e.g. 'chrome'")
	)
	flag.Parse()

	if *oldFSNamespace == "" {
		sklog.Fatalf("You must include fs_namespace")
	}

	if *newFSNamespace == "" {
		sklog.Fatalf("You must include sql_db_name")
	}

	fsClient, err := ifirestore.NewClient(context.Background(), *fsProjectID, "gold", *oldFSNamespace, nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}
	ctx := context.Background()
	oldNamespace := v3Impl{client: fsClient}

	fsClient, err = ifirestore.NewClient(context.Background(), *fsProjectID, "gold", *newFSNamespace, nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}
	newNamespace := v3Impl{client: fsClient}

	// Fetch triage records
	records, err := oldNamespace.fetchTriageRecords(ctx)
	if err != nil {
		sklog.Fatalf("Fetching triage records: %s", err)
	}
	sklog.Infof("Should migrate %d records", len(records))

	if err := newNamespace.storeRecords(ctx, records); err != nil {
		sklog.Fatalf("Storing triage records: %s", err)
	}

	// Fetch Deltas
	deltas, err := oldNamespace.fetchExpectationDeltas(ctx)
	if err != nil {
		sklog.Fatalf("Fetching expectation deltas: %s", err)
	}

	if err := newNamespace.storeExpectationChanges(ctx, deltas); err != nil {
		sklog.Fatalf("Storing triage records: %s", err)
	}

	// Fetch entries
	exp, err := oldNamespace.fetchExpectations(ctx)
	if err != nil {
		sklog.Fatalf("Fetching expectation entries: %s", err)
	}
	if err := newNamespace.storeEntries(ctx, exp); err != nil {
		sklog.Fatalf("Storing triage records: %s", err)
	}
	sklog.Infof("Done")
}

const (
	v3Partitions         = "expstore_partitions_v3"
	v3ExpectationEntries = "entries"
	v3RecordEntries      = "triage_records"
	v3ChangeEntries      = "triage_changes"

	v3MasterPartition = "master"
	v3BeginningOfTime = 0
	v3EndOfTime       = math.MaxInt32

	v3DigestField = "digest"
)

type v3Impl struct {
	client *ifirestore.Client
}

type v3ExpectationEntry struct {
	Grouping types.TestName  `firestore:"grouping"`
	Digest   types.Digest    `firestore:"digest"`
	Updated  time.Time       `firestore:"updated"`
	LastUsed time.Time       `firestore:"last_used"`
	Ranges   []v3TriageRange `firestore:"ranges"`
	NeedsGC  bool            `firestore:"needs_gc"`
}

func (e *v3ExpectationEntry) id() string {
	s := string(e.Grouping) + "|" + string(e.Digest)
	// firestore gets cranky if there are / in key names
	return strings.Replace(s, "/", "-", -1)
}

type v3TriageRange struct {
	FirstIndex int                   `firestore:"first_index"`
	LastIndex  int                   `firestore:"last_index"`
	Label      expectations.LabelInt `firestore:"label"`
}

type v3TriageRecord struct {
	ID        string
	UserName  string    `firestore:"user"`
	TS        time.Time `firestore:"ts"`
	Changes   int       `firestore:"changes"`
	Committed bool      `firestore:"committed"`
}

type v3ExpectationChange struct {
	// RecordID refers to a document in the records collection.
	RecordID      string                `firestore:"record_id"`
	Grouping      types.TestName        `firestore:"grouping"`
	Digest        types.Digest          `firestore:"digest"`
	AffectedRange v3TriageRange         `firestore:"affected_range"`
	LabelBefore   expectations.LabelInt `firestore:"label_before"`
}

func (v v3Impl) fetchTriageRecords(ctx context.Context) (map[string][]v3TriageRecord, error) {
	// maps partition to records
	rv := map[string][]v3TriageRecord{}
	partitionIterator := v.client.Collection(v3Partitions).DocumentRefs(ctx)
	p, err := partitionIterator.Next()
	for ; err == nil; p, err = partitionIterator.Next() {
		var records []v3TriageRecord
		partition := p.ID
		sklog.Infof("Partition %s", partition)
		recordIterator := v.client.Collection(v3Partitions).Doc(partition).Collection(v3RecordEntries).Documents(ctx)
		docs, err := recordIterator.GetAll()
		if err != nil {
			return nil, skerr.Wrapf(err, "getting records for %s", partition)
		}
		for _, doc := range docs {
			var r v3TriageRecord
			if err := doc.DataTo(&r); err != nil {
				if err != nil {
					sklog.Warning("Corrupt triage record with id %s", doc.Ref.ID)
					continue
				}
			}
			r.ID = doc.Ref.ID
			records = append(records, r)
		}
		rv[partition] = records
	}
	if err != iterator.Done {
		return nil, skerr.Wrap(err)
	}

	return rv, nil
}

func (v v3Impl) storeRecords(ctx context.Context, recordsByPartition map[string][]v3TriageRecord) error {
	if dryrun {
		sklog.Infof("Would store %d partition of records", len(recordsByPartition))
		return nil
	}
	for partition, records := range recordsByPartition {
		sklog.Infof("Writing %d records for partition %s", len(records), partition)
		entryCollection := v.client.Collection(v3Partitions).Doc(partition).Collection(v3RecordEntries)

		err := v.client.BatchWrite(ctx, len(records), ifirestore.MAX_TRANSACTION_DOCS, maxOperationTime, nil, func(b *firestore.WriteBatch, i int) error {
			record := records[i]
			doc := entryCollection.Doc(record.ID)
			b.Set(doc, record)
			return nil
		})
		if err != nil {
			return skerr.Wrapf(err, "storing to partition %s", partition)
		}
	}
	return nil
}

func (v v3Impl) fetchExpectationDeltas(ctx context.Context) (map[string][]v3ExpectationChange, error) {
	rv := map[string][]v3ExpectationChange{} // Maps partition -> entries

	partitionIterator := v.client.Collection(v3Partitions).DocumentRefs(ctx)
	p, err := partitionIterator.Next()
	for ; err == nil; p, err = partitionIterator.Next() {
		partition := p.ID
		const numShards = 16
		base := v.client.Collection(v3Partitions).Doc(partition).Collection(v3ChangeEntries)

		queries := fs_utils.ShardOnDigest(base, v3DigestField, numShards)
		shardedEntries := make([][]v3ExpectationChange, numShards)

		err := v.client.IterDocsInParallel(ctx, "loadExpectationDeltas", partition, queries, maxRetries, maxOperationTime, func(i int, doc *firestore.DocumentSnapshot) error {
			if doc == nil {
				return nil
			}
			entry := v3ExpectationChange{}
			if err := doc.DataTo(&entry); err != nil {
				id := doc.Ref.ID
				return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal expectationChange with id %s", id)
			}

			shardedEntries[i] = append(shardedEntries[i], entry)
			return nil
		})

		if err != nil {
			return nil, skerr.Wrapf(err, "fetching expectation deltas for partition %s", partition)
		}

		var combinedEntries []v3ExpectationChange
		for _, shard := range shardedEntries {
			combinedEntries = append(combinedEntries, shard...)
		}

		rv[partition] = combinedEntries
	}
	return rv, nil
}

func (v v3Impl) storeExpectationChanges(ctx context.Context, changesByPartition map[string][]v3ExpectationChange) error {
	if dryrun {
		sklog.Infof("Would store %d partition of changes", len(changesByPartition))
		return nil
	}
	for partition, changes := range changesByPartition {
		sklog.Infof("Writing %d changes for partition %s", len(changes), partition)
		changesCollection := v.client.Collection(v3Partitions).Doc(partition).Collection(v3ChangeEntries)

		err := v.client.BatchWrite(ctx, len(changes), ifirestore.MAX_TRANSACTION_DOCS, maxOperationTime, nil, func(b *firestore.WriteBatch, i int) error {
			change := changes[i]
			doc := changesCollection.NewDoc()
			b.Set(doc, change)
			return nil
		})
		if err != nil {
			return skerr.Wrapf(err, "storing to partition %s", partition)
		}
	}
	return nil
}

func (v v3Impl) fetchExpectations(ctx context.Context) (map[string][]v3ExpectationEntry, error) {
	rv := map[string][]v3ExpectationEntry{} // Maps partition -> entries

	partitionIterator := v.client.Collection(v3Partitions).DocumentRefs(ctx)
	p, err := partitionIterator.Next()
	for ; err == nil; p, err = partitionIterator.Next() {
		partition := p.ID

		const numShards = 16
		base := v.client.Collection(v3Partitions).Doc(partition).Collection(v3ExpectationEntries)
		queries := fs_utils.ShardOnDigest(base, v3DigestField, numShards)
		shardedEntries := make([][]v3ExpectationEntry, numShards)

		err := v.client.IterDocsInParallel(ctx, "loadExpectations", partition, queries, maxRetries, maxOperationTime, func(i int, doc *firestore.DocumentSnapshot) error {
			if doc == nil {
				return nil
			}
			entry := v3ExpectationEntry{}
			if err := doc.DataTo(&entry); err != nil {
				id := doc.Ref.ID
				return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal expectationEntry with id %s", id)
			}
			if len(entry.Ranges) == 0 {
				// This should never happen, but we'll ignore these malformed entries if they do.
				return nil
			}
			shardedEntries[i] = append(shardedEntries[i], entry)
			return nil
		})

		if err != nil {
			return nil, skerr.Wrapf(err, "fetching expectations for partition %s", partition)
		}

		var combinedEntries []v3ExpectationEntry
		for _, shard := range shardedEntries {
			combinedEntries = append(combinedEntries, shard...)
		}

		rv[partition] = combinedEntries
		sklog.Infof("Fetched %d entries for partition %s", len(combinedEntries), partition)
	}
	return rv, nil
}

func (v v3Impl) storeEntries(ctx context.Context, entriesByPartition map[string][]v3ExpectationEntry) error {
	if dryrun {
		sklog.Infof("Would store %d partition of entries", len(entriesByPartition))
		return nil
	}
	for partition, entries := range entriesByPartition {
		sklog.Infof("Writing %d entries for partition %s", len(entries), partition)
		entryCollection := v.client.Collection(v3Partitions).Doc(partition).Collection(v3ExpectationEntries)

		err := v.client.BatchWrite(ctx, len(entries), ifirestore.MAX_TRANSACTION_DOCS, maxOperationTime, nil, func(b *firestore.WriteBatch, i int) error {
			entry := entries[i]
			doc := entryCollection.Doc(entry.id())
			b.Set(doc, entry)
			return nil
		})
		if err != nil {
			return skerr.Wrapf(err, "storing to partition %s", partition)
		}
	}
	return nil
}
