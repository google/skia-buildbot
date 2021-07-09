// The fsmigrator executable migrates various data from firestore to an SQL database.
// It uses port forwarding, as that is the simplest approach and there shouldn't be
// too much data.
package main

import (
	"context"
	"crypto/md5"
	"flag"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"google.golang.org/api/iterator"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/fs_utils"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/types"
)

func main() {
	var (
		fsProjectID    = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
		oldFSNamespace = flag.String("old_fs_namespace", "", "Typically the instance id. e.g. 'chrome-gpu', 'skia', etc")
		newSQLDatabase = flag.String("new_sql_db", "", "Something like the instance id (no dashes)")
	)
	flag.Parse()

	if *oldFSNamespace == "" {
		sklog.Fatalf("You must include fs_namespace")
	}

	if *newSQLDatabase == "" {
		sklog.Fatalf("You must include new_sql_db")
	}

	ctx := context.Background()
	fsClient, err := ifirestore.NewClient(ctx, *fsProjectID, "gold", *oldFSNamespace, nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}

	u := sql.GetConnectionURL("root@localhost:26234", *newSQLDatabase)
	conf, err := pgxpool.ParseConfig(u)
	if err != nil {
		sklog.Fatalf("error getting postgres config %s: %s", u, err)
	}
	conf.MaxConns = 16
	db, err := pgxpool.ConnectConfig(ctx, conf)
	if err != nil {
		sklog.Info("You must run\nkubectl port-forward gold-cockroachdb-0 26234:26234")
		sklog.Fatalf("error connecting to the database: %s", err)
	}

	oldNamespace := v3Impl{client: fsClient}

	// We need to look up the groupings by test name, because the old groupings were just the
	// test name and not combined with the corpus. As such, we fetch all the groupings from the SQL,
	// which has been filled through ingesting.
	nameToGroupings, err := fetchGroupings(ctx, db)
	if err != nil {
		sklog.Fatalf("Getting groupings from SQL: %s", err)
	}
	sklog.Infof("Fetched %d groupings from SQL DB", len(nameToGroupings))

	sklog.Infof("Fetching expectations")
	exp, err := oldNamespace.fetchExpectations(ctx)
	if err != nil {
		sklog.Fatalf("fetching expectations %s", err)
	}

	// pass in deltas so we can link in the triage record to the expectations.
	err = storeExpectations(ctx, db, exp, nameToGroupings)
	if err != nil {
		sklog.Fatalf("storing expectation deltas: %s", err)
	}
	//
	//	// Write the catchall expectation record. If somehow an expectation exists, but wasn't covered
	//	// by a delta (e.g. broken old data), we assign it to this catchall record.
	//	_, err = db.Exec(ctx, `UPSERT INTO ExpectationRecords
	//(expectation_record_id, user_name, triage_time, num_changes) VALUES ($1, $2, $3, $4)`,
	//		catchAllUUID, "sql_migrator", time.Now(), unowned)
	//	if err != nil {
	//		sklog.Fatalf("creating catch-all record: %s", err)
	//	}

	sklog.Info("done")
}

func fetchGroupings(ctx context.Context, db *pgxpool.Pool) (map[types.TestName][]schema.GroupingID, error) {
	rv := map[types.TestName][]schema.GroupingID{}
	rows, err := db.Query(ctx, `SELECT keys -> 'name', grouping_id FROM Groupings`)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name types.TestName
		var gID schema.GroupingID
		if err := rows.Scan(&name, &gID); err != nil {
			return nil, skerr.Wrap(err)
		}
		rv[name] = append(rv[name], gID)
	}
	count := 0
	for _, groupings := range rv {
		if len(groupings) > 1 {
			count++
		}
	}
	sklog.Infof("%d out of %d test names belonged to multiple corpora", count, len(rv))
	return rv, nil
}

var (
	catchAllUUID = uuid.MustParse("00000000-0000-0000-0000-000000000000")
)

func storeExpectations(ctx context.Context, db *pgxpool.Pool, toStore map[string][]v3ExpectationEntry, nameToGroupings map[types.TestName][]schema.GroupingID) error {
	sklog.Infof("have %d partitions", len(toStore))
	return skerr.Wrap(storePrimaryBranchExpectations(ctx, db, toStore[v3PrimaryPartition], nameToGroupings))
}

func storePrimaryBranchExpectations(ctx context.Context, db *pgxpool.Pool, exps []v3ExpectationEntry, nameToGroupings map[types.TestName][]schema.GroupingID) error {
	const batchSize = 1000
	return util.ChunkIter(len(exps), batchSize, func(startIdx int, endIdx int) error {
		if err := ctx.Err(); err != nil {
			return skerr.Wrap(err)
		}
		batch := exps[startIdx:endIdx]
		statement := `INSERT INTO Expectations
(grouping_id, digest, label, expectation_record_id) VALUES `
		const valuesPerRow = 4
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, exp := range batch {
			groupings, ok := nameToGroupings[exp.Grouping]
			if !ok {
				continue
			}
			dBytes, err := sql.DigestToBytes(exp.Digest)
			if err != nil {
				sklog.Warningf("Corrupt digest %q on branch %s", exp.Digest, v3PrimaryPartition)
				continue
			}
			label := exp.Ranges[0].Label
			// some test names correspond to more than one grouping. We need to write the
			// expectations for both.
			for _, gID := range groupings {
				arguments = append(arguments, gID, dBytes, convertLabel(label), catchAllUUID)
			}
		}
		if len(arguments) == 0 {
			return nil
		}
		// Need to divide here to account for skipped rows.
		statement += sql.ValuesPlaceholders(valuesPerRow, len(arguments)/valuesPerRow)
		statement += `
ON CONFLICT (grouping_id, digest)
DO UPDATE SET (label, expectation_record_id) =
  (excluded.label, excluded.expectation_record_id)
WHERE Expectations.label = 'u' AND excluded.label != 'u'
`

		err := crdbpgx.ExecuteTx(ctx, db, pgx.TxOptions{}, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, statement, arguments...)
			return err // don't wrap - might get retried
		})
		return skerr.Wrap(err)
	})
}

// labelInt is the integer version of Label, used as the storage format in firestore
type labelInt int

const (
	// untriagedInt represents a previously unseen digest.
	untriagedInt labelInt = iota // == 0
	// positiveInt represents a known good digest.
	positiveInt
	// negativeInt represents a known bad digest.
	negativeInt
)

func convertLabel(label labelInt) schema.ExpectationLabel {
	switch label {
	case untriagedInt:
		return schema.LabelUntriaged
	case positiveInt:
		return schema.LabelPositive
	case negativeInt:
		return schema.LabelNegative
	}
	return schema.LabelUntriaged
}

func hash(id string) []byte {
	h := md5.Sum([]byte(id))
	return h[:]
}

const (
	v3Partitions         = "expstore_partitions_v3"
	v3ExpectationEntries = "entries"
	v3RecordEntries      = "triage_records"
	v3ChangeEntries      = "triage_changes"

	v3PrimaryPartition = "master"

	v3DigestField = "digest"

	maxRetries       = 10
	maxOperationTime = 10 * time.Minute
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
	FirstIndex int      `firestore:"first_index"`
	LastIndex  int      `firestore:"last_index"`
	Label      labelInt `firestore:"label"`
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
	RecordID      string         `firestore:"record_id"`
	Grouping      types.TestName `firestore:"grouping"`
	Digest        types.Digest   `firestore:"digest"`
	AffectedRange v3TriageRange  `firestore:"affected_range"`
	LabelBefore   labelInt       `firestore:"label_before"`
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
				sklog.Warning("Corrupt triage record with id %s", doc.Ref.ID)
				continue
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

func (v v3Impl) fetchExpectations(ctx context.Context) (map[string][]v3ExpectationEntry, error) {
	rv := map[string][]v3ExpectationEntry{} // Maps partition -> entries

	partitionIterator := v.client.Collection(v3Partitions).DocumentRefs(ctx)
	p, err := partitionIterator.Next()
	for ; err == nil; p, err = partitionIterator.Next() {
		partition := p.ID
		if partition != v3PrimaryPartition {
			continue
		}

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
