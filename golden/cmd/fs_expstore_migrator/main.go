package main

import (
	"context"
	"flag"
	"io/ioutil"
	"math"
	"os/exec"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"google.golang.org/api/iterator"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/fs_utils"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/types"
)

const (
	maxOperationTime = 2 * time.Minute
	maxRetries       = 5
)

func main() {
	var (
		fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
		fsNamespace = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'flutter', 'skia', etc")

		sqlDBName = flag.String("sql_db_name", "", "The name of the db that this data should be inserted into")

		groupingCSV = flag.String("grouping_csv", "", "A CSV file that has all known pairs of corpus + test name. First column is corpus, second is test name.")
	)
	flag.Parse()

	if *fsNamespace == "" {
		sklog.Fatalf("You must include fs_namespace")
	}

	if *sqlDBName == "" {
		sklog.Fatalf("You must include sql_db_name")
	}

	testNameToGrouping, err := parseGroupingCSV(*groupingCSV)
	if err != nil {
		sklog.Fatalf("processing testNameToGrouping from %s: %s", *groupingCSV, err)
	}

	fsClient, err := ifirestore.NewClient(context.Background(), *fsProjectID, "gold", *fsNamespace, nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}
	ctx := context.Background()
	v3 := v3Impl{client: fsClient}
	sqlDB := sqlDBV1{
		dbName:             *sqlDBName,
		testNameToGrouping: testNameToGrouping,
	}

	err = sqlDB.initExpectations(ctx)
	if err != nil {
		sklog.Fatalf("Could not set up sql to %s: %s", *sqlDBName, err)
	}

	// Fetch entries
	exp, err := v3.fetchExpectations(ctx)
	if err != nil {
		sklog.Fatalf("Fetching expectation entries: %s", err)
	}

	// Because there are such big changes in the layout for the SQL, it is not worth the effort to
	// maintain exactly all the triage history.
	if err := sqlDB.storeExpectationsAsOneCombinedRecord(ctx, exp); err != nil {
		sklog.Fatalf("Storing expectation entries: %s", err)
	}
}

type groupingIDBytes []byte

func parseGroupingCSV(csvFilePath string) (map[string]groupingIDBytes, error) {
	b, err := ioutil.ReadFile(csvFilePath)
	if err != nil {
		return nil, skerr.Wrapf(err, "reading file")
	}
	duplicateChecker := map[string]string{}

	rows := strings.Split(string(b), "\n")
	rv := make(map[string]groupingIDBytes, len(rows))
	for _, row := range rows {
		// Each row is corpus,testName
		ct := strings.Split(row, ",")
		if len(ct) != 2 {
			continue
		}
		corpus, testName := strings.TrimSpace(ct[0]), strings.TrimSpace(ct[1])
		if corpus == "corpus" {
			continue
		}
		if dup, ok := duplicateChecker[testName]; ok {
			sklog.Warningf("Duplicate test name %s, some expectations may be wrong. Previous corpus was %s, new one is %s", testName, dup, corpus)
		}
		duplicateChecker[testName] = corpus

		groupingMap := map[string]string{
			types.CorpusField:     corpus,
			types.PrimaryKeyField: testName,
		}
		_, groupingID, err := sql.SerializeMap(groupingMap)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv[testName] = groupingID
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

		sklog.Infof("Fetched %d entries for partition %s", len(combinedEntries), partition)
		if len(combinedEntries) != 0 {
			rv[partition] = combinedEntries
		}
	}
	if err != iterator.Done {
		return nil, skerr.Wrap(err)
	}

	return rv, nil
}

const (
	migrationUser = "sql_migration_user"
)

type sqlDBV1 struct {
	dbName string
	db     *pgxpool.Pool

	testNameToGrouping map[string]groupingIDBytes
}

func (s *sqlDBV1) initExpectations(ctx context.Context) error {
	out, err := exec.Command("cockroach", "sql", "--insecure", "--host=localhost:26257",
		"--database="+s.dbName,
		`--execute=
-- Drop these tables because we'll be storing/creating new UUIDs
DROP TABLE IF EXISTS ExpectationRecords;
DROP TABLE IF EXISTS ExpectationDeltas;

CREATE TABLE IF NOT EXISTS Expectations (
  grouping_id BYTES, -- MD5 hash of the grouping JSON
  digest BYTES NOT NULL, -- MD5 hash of the pixel data
  label SMALLINT NOT NULL, -- 0 for untriaged, 1 for positive, 2 for negative
  start_index INT4, -- Reserved for future use with expectation ranges
  end_index INT4, -- Reserved for future use with expectation ranges
  expectation_record_id UUID, -- If not null, the record that set this value
  INDEX label_idx (label),
  INDEX group_label_idx (grouping_id, label) STORING (expectation_record_id),
  PRIMARY KEY (digest, grouping_id) -- start_index should be on primary key too eventually.
);

CREATE TABLE IF NOT EXISTS ChangelistExpectations (
  changelist_id STRING NOT NULL, -- e.g. "gerrit_12345"
  grouping_id BYTES, -- MD5 hash of the grouping JSON
  digest BYTES NOT NULL, -- MD5 hash of the pixel data
  label SMALLINT NOT NULL, -- 0 for untriaged, 1 for positive, 2 for negative
  start_index INT4, -- Reserved for future use with expectation ranges
  end_index INT4, -- Reserved for future use with expectation ranges
  expectation_record_id UUID, -- If not null, the record that set this value
  INDEX changelist_label_idx (changelist_id, label),
  PRIMARY KEY (digest, changelist_id, grouping_id) -- start_index should be on primary key too eventually.
);

CREATE TABLE IF NOT EXISTS ExpectationDeltas (
  expectation_delta_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  expectation_record_id UUID,
  grouping_id BYTES, -- MD5 hash of the grouping JSON
  digest BYTES NOT NULL, -- MD5 hash of the pixel data
  label_before SMALLINT, -- 0 for untriaged, 1 for positive, 2 for negative
  label_after SMALLINT, -- 0 for untriaged, 1 for positive, 2 for negative
  start_index INT4, -- Reserved for future use with expectation ranges
  end_index_before INT4, -- Reserved for future use with expectation ranges
  end_index_after INT4 -- Reserved for future use with expectation ranges
);

CREATE TABLE IF NOT EXISTS ExpectationRecords (
  expectation_record_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  changelist_id STRING, -- e.g. CodeReviewSystem and CL ID "gerrit_12345". Can be NULL
  user_name STRING,
  time TIMESTAMP WITH TIME ZONE,
  num_changes INT4
);
`,
	).CombinedOutput()
	if err != nil {
		return skerr.Wrapf(err, "creating tables: %s", out)
	}

	dbConnectionURL := "postgresql://root@localhost:26257/" + s.dbName + "?sslmode=disable"
	db, err := pgxpool.Connect(ctx, dbConnectionURL)
	if err != nil {
		return skerr.Wrapf(err, "connecting to the database")
	}
	c, err := db.Acquire(ctx)
	if err != nil {
		return skerr.Wrapf(err, "acquiring connection to %s", dbConnectionURL)
	}
	defer c.Release()
	if err = c.Conn().Ping(ctx); err != nil {
		return skerr.Wrapf(err, "connecting to database via ping %s", dbConnectionURL)
	}
	s.db = db

	sklog.Infof("SQL initialized")
	return nil
}

func (s *sqlDBV1) storeExpectationsAsOneCombinedRecord(ctx context.Context, entriesByPartition map[string][]v3ExpectationEntry) error {
	for partition, entries := range entriesByPartition {
		entries = s.filterObsoleteEntries(entries)

		recordID, err := s.storeTriageRecord(ctx, len(entries), partition)
		if err != nil {
			return skerr.Wrapf(err, "storing record")
		}

		if err := s.storeExpectationDeltas(ctx, recordID, entries); err != nil {
			return skerr.Wrapf(err, "storing deltas")
		}

		if partition == v3MasterPartition {
			if err := s.storePrimaryExpectations(ctx, recordID, entries); err != nil {
				return skerr.Wrapf(err, "storing primary branch expectations")
			}
		} else {
			if err := s.storeChangelistExpectations(ctx, recordID, partition, entries); err != nil {
				return skerr.Wrapf(err, "storing primary branch expectations")
			}
		}
		sklog.Infof("Stored %d entries for partition %s", len(entries), partition)
	}
	return nil
}

func (s *sqlDBV1) filterObsoleteEntries(entries []v3ExpectationEntry) []v3ExpectationEntry {
	entriesWithMatch := make([]v3ExpectationEntry, 0, len(entries))
	for _, entry := range entries {
		_, ok := s.testNameToGrouping[string(entry.Grouping)]
		if !ok {
			sklog.Warningf("Dropping entry with test %s", entry.Grouping)
			continue
		}
		entriesWithMatch = append(entriesWithMatch, entry)
	}
	return entriesWithMatch
}

// storeTriageRecords stores a single record and returns the ID of the record created.
func (s *sqlDBV1) storeTriageRecord(ctx context.Context, numChanges int, partition string) (string, error) {
	const primaryBranchStatement = `
INSERT INTO ExpectationRecords (user_name, time, num_changes) VALUES ($1, $2, $3)
RETURNING expectation_record_id`
	const changelistStatement = `
INSERT INTO ExpectationRecords (user_name, time, num_changes, changelist_id) VALUES ($1, $2, $3, $4)
RETURNING expectation_record_id`

	var row pgx.Row
	if partition == v3MasterPartition {
		row = s.db.QueryRow(ctx, primaryBranchStatement, migrationUser, time.Now(), numChanges)
	} else {
		row = s.db.QueryRow(ctx, changelistStatement, migrationUser, time.Now(), numChanges, partition)
	}

	var recordUUID string
	if err := row.Scan(&recordUUID); err != nil {
		return "", skerr.Wrapf(err, "storing record for partition %s", partition)
	}
	return recordUUID, nil
}

func (s *sqlDBV1) storeExpectationDeltas(ctx context.Context, recordID string, entries []v3ExpectationEntry) error {
	sklog.Infof("Storing %d deltas", len(entries))

	const batchSize = 1000
	err := util.ChunkIter(len(entries), batchSize, func(startIdx int, endIdx int) error {
		batch := entries[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}

		statement := `INSERT INTO ExpectationDeltas (expectation_record_id, grouping_id, digest, label_before, label_after) VALUES`
		const valuesPerRow = 5

		vp, err := sql.ValuesPlaceholders(valuesPerRow, len(batch))
		if err != nil {
			return skerr.Wrap(err)
		}
		statement = statement + vp

		arguments := make([]interface{}, 0, len(batch)*valuesPerRow)
		for _, value := range batch {
			arguments = append(arguments, recordID)

			groupingHash, ok := s.testNameToGrouping[string(value.Grouping)]
			if !ok {
				return skerr.Fmt("unknown grouping %s", value.Grouping)
			}
			arguments = append(arguments, groupingHash)

			b, err := sql.DigestToBytes(value.Digest)
			if err != nil {
				sklog.Fatalf("Invalid Digest: %s", value.Digest)
			}
			arguments = append(arguments, b)
			arguments = append(arguments, sql.LabelUntriaged)
			arguments = append(arguments, value.Ranges[0].Label)
		}
		_, err = s.db.Exec(ctx, statement, arguments...)
		return err
	})
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func (s *sqlDBV1) storePrimaryExpectations(ctx context.Context, recordID string, entries []v3ExpectationEntry) error {
	const batchSize = 200
	err := util.ChunkIter(len(entries), batchSize, func(startIdx int, endIdx int) error {
		sklog.Infof("Storing entries %d:%d", startIdx, endIdx)
		batch := entries[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}

		statement := `UPSERT INTO Expectations (grouping_id, digest, label, expectation_record_id) VALUES`
		const valuesPerRow = 4

		vp, err := sql.ValuesPlaceholders(valuesPerRow, len(batch))
		if err != nil {
			return skerr.Wrap(err)
		}
		statement += vp
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, value := range batch {
			groupingHash, ok := s.testNameToGrouping[string(value.Grouping)]
			if !ok {
				return skerr.Fmt("unknown grouping %s", value.Grouping)
			}
			arguments = append(arguments, groupingHash)

			b, err := sql.DigestToBytes(value.Digest)
			if err != nil {
				sklog.Fatalf("Invalid Digest: %s", value.Digest)
			}
			arguments = append(arguments, b)
			arguments = append(arguments, value.Ranges[0].Label)
			arguments = append(arguments, recordID)
		}
		_, err = s.db.Exec(ctx, statement, arguments...)
		if err != nil {
			return skerr.Wrapf(err, "storing primary branch expectations "+statement)
		}

		// TODO(kjlubick) use Temporary Tables when cockroachdb supports them.
		_, err = s.db.Exec(ctx, `CREATE TABLE IF NOT EXISTS TemporaryExpectations (
  grouping_id BYTES NOT NULL, -- MD5 hash of the grouping JSON
  digest BYTES NOT NULL, -- MD5 hash of the pixel data
  new_label SMALLINT NOT NULL, -- 0 for untriaged, 1 for positive, 2 for negative
  new_expectation_record_id UUID NOT NULL,
  PRIMARY KEY (digest, grouping_id)
);`)
		if err != nil {
			return skerr.Wrapf(err, "creating temp table")
		}

		insertToTempStatement := `INSERT INTO TemporaryExpectations (grouping_id, digest, new_label, new_expectation_record_id) VALUES ` + vp
		_, err = s.db.Exec(ctx, insertToTempStatement, arguments...)
		if err != nil {
			return skerr.Wrapf(err, "inserting into temp table %s", insertToTempStatement)
		}

		updateUsingTempTable := `
UPDATE ValuesAtHead
SET expectation_label = new_label, expectation_record_id = new_expectation_record_id
FROM TemporaryExpectations
WHERE ValuesAtHead.grouping_id = TemporaryExpectations.grouping_id and ValuesAtHead.digest = TemporaryExpectations.digest
RETURNING NOTHING`

		_, err = s.db.Exec(ctx, updateUsingTempTable)
		if err != nil {
			return skerr.Wrapf(err, "updating from temp table %s", updateUsingTempTable)
		}

		// Deleting all rows from the temporary table is quicker than dropping the table.
		_, err = s.db.Exec(ctx, `DELETE FROM TemporaryExpectations RETURNING NOTHING;`)
		if err != nil {
			return skerr.Wrapf(err, "deleting rows from temp table")
		}
		return nil
	})
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func (s *sqlDBV1) storeChangelistExpectations(ctx context.Context, recordID, changelistID string, entries []v3ExpectationEntry) error {
	const batchSize = 1000
	err := util.ChunkIter(len(entries), batchSize, func(startIdx int, endIdx int) error {
		batch := entries[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}

		statement := `UPSERT INTO ChangelistExpectations (changelist_id, grouping_id, digest, label, expectation_record_id) VALUES`
		const valuesPerRow = 5

		vp, err := sql.ValuesPlaceholders(valuesPerRow, len(batch))
		if err != nil {
			return skerr.Wrap(err)
		}
		statement += vp
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, value := range batch {
			arguments = append(arguments, changelistID)
			groupingHash, ok := s.testNameToGrouping[string(value.Grouping)]
			if !ok {
				return skerr.Fmt("unknown grouping %s", value.Grouping)
			}
			arguments = append(arguments, groupingHash)

			b, err := sql.DigestToBytes(value.Digest)
			if err != nil {
				sklog.Fatalf("Invalid Digest: %s", value.Digest)
			}
			arguments = append(arguments, b)
			// As a special case, if expectations have been set back to 0, we need to use -1 to fallback
			// to the primary branch if it has one.
			label := sql.ExpectationsLabel(value.Ranges[0].Label)
			if label == sql.LabelUntriaged {
				label = sql.FallbackToPrimaryBranch
			}
			arguments = append(arguments, label)
			arguments = append(arguments, recordID)
		}
		_, err = s.db.Exec(ctx, statement, arguments...)
		if err != nil {
			return skerr.Wrapf(err, "storing changelist expectations "+statement)
		}
		return nil
	})
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}
