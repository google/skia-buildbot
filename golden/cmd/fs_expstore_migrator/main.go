package main

import (
	"context"
	"flag"
	"math"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/fs_utils"
	"go.skia.org/infra/golden/go/types"
)

const (
	maxOperationTime = 2 * time.Minute
)

func main() {
	var (
		fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
		fsNamespace = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'flutter', 'skia', etc")
	)
	flag.Parse()

	if *fsNamespace == "" {
		sklog.Fatalf("You must include namespace")
	}

	fsClient, err := ifirestore.NewClient(context.Background(), *fsProjectID, "gold", *fsNamespace, nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}
	ctx := context.Background()
	v2 := v2Impl{client: fsClient}
	v3 := v3Impl{client: fsClient}

	records, err := v2.loadTriageRecords(ctx)
	if err != nil {
		sklog.Fatalf("loading v2 of records : %s", err)
	}

	sklog.Infof("%d triage records retrieved - storing them", len(records))

	if err := v3.migrateAndStoreRecords(ctx, records); err != nil {
		sklog.Fatalf("storing v3 of records", err)
	}

	sklog.Infof("All %d triage records migrated", len(records))

	panic("stop for now")
	//entries, err := v2.loadExpectationEntries(ctx)
	//if err != nil {
	//	sklog.Fatalf("loading v2 of expectations : %s", err)
	//}
	//
	//sklog.Infof("%d expectation entries retrieved - storing them", len(entries))
	//
	//if err := v3.migrateAndStoreEntries(ctx, entries); err != nil {
	//	sklog.Fatalf("storing v3 of expectations : %s", err)
	//}
	//sklog.Infof("All %d expectation entries migrated", len(entries))
}

const (
	v2ExpectationsCollection  = "expstore_expectations_v2"
	v2TriageRecordsCollection = "expstore_triage_records_v2"
	v2TriageChangesCollection = "expstore_triage_changes_v2"
)

type v2Impl struct {
	client *ifirestore.Client
}

type v2ExpectationEntry struct {
	Grouping   types.TestName     `firestore:"grouping"`
	Digest     types.Digest       `firestore:"digest"`
	Label      expectations.Label `firestore:"label"`
	Updated    time.Time          `firestore:"updated"`
	CRSAndCLID string             `firestore:"crs_cl_id"`
	LastUsed   time.Time          `firestore:"last_used"`
}

type v2TriageRecord struct {
	UserName   string    `firestore:"user"`
	TS         time.Time `firestore:"ts"`
	CRSAndCLID string    `firestore:"crs_cl_id"`
	Changes    int       `firestore:"changes"`
	Committed  bool      `firestore:"committed"`
}

func (v v2Impl) loadExpectationEntries(ctx context.Context) ([]v2ExpectationEntry, error) {
	const shards = 16
	const shardField = "digest"
	q := fs_utils.ShardOnDigest(v.client.Collection(v2ExpectationsCollection), shardField, shards)
	shardedEntries := make([][]v2ExpectationEntry, shards)
	err := v.client.IterDocsInParallel(ctx, "v2 expectation entries", "", q, 3, maxOperationTime, func(shard int, doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		entry := v2ExpectationEntry{}
		if err := doc.DataTo(&entry); err != nil {
			id := doc.Ref.ID
			return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal expectationEntry with id %s", id)
		}
		shardedEntries[shard] = append(shardedEntries[shard], entry)
		return nil
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	rv := make([]v2ExpectationEntry, 0, shards*len(shardedEntries[0]))
	for _, entries := range shardedEntries {
		for _, entry := range entries {
			rv = append(rv, entry)
		}
	}
	return rv, nil
}

// The returned map has the id as the key. That way, the triageChanges don't have have their
// RecordID changed.
func (v v2Impl) loadTriageRecords(ctx context.Context) (map[string]v2TriageRecord, error) {
	rv := map[string]v2TriageRecord{}

	q := v.client.Collection(v2TriageRecordsCollection).OrderBy("ts", firestore.Desc)
	err := v.client.IterDocs(ctx, "getting records", "", q, 3, maxOperationTime, func(doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		id := doc.Ref.ID
		tr := v2TriageRecord{}
		if err := doc.DataTo(&tr); err != nil {
			return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal triageRecord with id %s", id)
		}
		rv[id] = tr
		return nil
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}

const (
	v3Partitions         = "expstore_partitions_v3"
	v3ExpectationEntries = "entries"
	v3RecordEntries      = "triage_records"
	v3ChangeEntries      = "triage_changes"

	v3MasterPartition = "master"
	v3BeginningOfTime = 0
	v3EndOfTime       = math.MaxInt32
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
	FirstIndex int                `firestore:"first_index"`
	LastIndex  int                `firestore:"last_index"`
	Label      expectations.Label `firestore:"label"`
}

type v3TriageRecord struct {
	UserName  string    `firestore:"user"`
	TS        time.Time `firestore:"ts"`
	Changes   int       `firestore:"changes"`
	Committed bool      `firestore:"committed"`
}

func (v v3Impl) migrateAndStoreEntries(ctx context.Context, oldEntries []v2ExpectationEntry) error {
	entriesByPartition := map[string][]v3ExpectationEntry{}

	for _, oldEntry := range oldEntries {
		partition := oldEntry.CRSAndCLID
		if partition == "" {
			partition = v3MasterPartition
		}
		entriesByPartition[partition] = append(entriesByPartition[partition], v3ExpectationEntry{
			Grouping: oldEntry.Grouping,
			Digest:   oldEntry.Digest,
			Updated:  oldEntry.Updated,
			LastUsed: oldEntry.LastUsed,
			Ranges: []v3TriageRange{
				{
					FirstIndex: v3BeginningOfTime,
					LastIndex:  v3EndOfTime,
					Label:      oldEntry.Label,
				},
			},
			NeedsGC: false,
		})
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

func (v v3Impl) migrateAndStoreRecords(ctx context.Context, oldRecords map[string]v2TriageRecord) error {
	for id, oldRecord := range oldRecords {
		partition := oldRecord.CRSAndCLID
		if partition == "" {
			partition = v3MasterPartition
		}
		recordCollection := v.client.Collection(v3Partitions).Doc(partition).Collection(v3RecordEntries)
		_, err := recordCollection.Doc(id).Set(ctx, v3TriageRecord{
			UserName:  oldRecord.UserName,
			TS:        oldRecord.TS,
			Changes:   oldRecord.Changes,
			Committed: oldRecord.Committed,
		})
		if err != nil {
			return skerr.Wrapf(err, "writing record with id %s", id)
		}
	}
	return nil
}
