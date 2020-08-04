package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os/exec"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/iterator"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/types"
)

const (
	maxOperationTime = 2 * time.Minute
)

func main() {
	var (
		fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
		fsNamespace = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'flutter', 'skia', etc")

		sqlDBName = flag.String("sql_db_name", "", "The name of the db that this data should be inserted into")
	)
	flag.Parse()

	if *fsNamespace == "" {
		sklog.Fatalf("You must include fs_namespace")
	}

	if *sqlDBName == "" {
		sklog.Fatalf("You must include sql_db_name")
	}

	fsClient, err := ifirestore.NewClient(context.Background(), *fsProjectID, "gold", *fsNamespace, nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}
	ctx := context.Background()
	v3 := v3Impl{client: fsClient}
	sqlDB := sqlDBV1{
		dbName: *sqlDBName,
	}

	err = sqlDB.initExpectations(ctx)
	if err != nil {
		sklog.Fatalf("Could not set up sql to %s: %s", *sqlDBName, err)
	}

	// Fetch triage records
	records, err := v3.fetchTriageRecords(ctx)
	if err != nil {
		sklog.Fatalf("Fetching triage records: %s", err)
	}
	sklog.Infof("Should migrate %d records", len(records))

	if err := sqlDB.storeTriageRecords(ctx, records); err != nil {
		sklog.Fatalf("Storing triage records: %s", err)
	}

	sklog.Infof("Records written %v", sqlDB.oldRecordIDToNewRecordID)

	// Fetch Deltas
	deltas, err := v3.fetchExpectationDeltas(ctx)
	if err != nil {
		sklog.Fatalf("Fetching expectation deltas: %s", err)
	}

	if err := sqlDB.storeExpectationDeltas(ctx, deltas); err != nil {
		sklog.Fatalf("Storing expectation deltas: %s", err)
	}

	// Fetch entries
	exp, err := v3.fetchExpectations(ctx)
	if err != nil {
		sklog.Fatalf("Fetching expectation entries: %s", err)
	}

	if err := sqlDB.storeExpectations(ctx, exp); err != nil {
		sklog.Fatalf("Storing expectation entries: %s", err)
	}
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

func (v v3Impl) fetchExpectationDeltas(ctx context.Context) ([]v3ExpectationChange, error) {
	// TODO(kjlubick) handle CL expectations
	deltaIterator := v.client.Collection(v3Partitions).Doc(v3MasterPartition).Collection(v3ChangeEntries).Documents(ctx)

	var rv []v3ExpectationChange
	docs, err := deltaIterator.GetAll() FIXME this times out
	if err != nil {
		return nil, skerr.Wrapf(err, "getting changes for %s", v3MasterPartition)
	}
	for _, doc := range docs {
		var c v3ExpectationChange
		if err := doc.DataTo(&c); err != nil {
			if err != nil {
				sklog.Warning("Corrupt expectation change with id %s", doc.Ref.ID)
				continue
			}
		}
		rv = append(rv, c)
	}
	return rv, nil
}

func (v v3Impl) fetchExpectations(ctx context.Context) ([]v3ExpectationEntry, error) {
	// TODO(kjlubick) handle CL expectations
	entryIterator := v.client.Collection(v3Partitions).Doc(v3MasterPartition).Collection(v3ExpectationEntries).Documents(ctx)

	var rv []v3ExpectationEntry
	docs, err := entryIterator.GetAll()  FIXME this will time out
	if err != nil {
		return nil, skerr.Wrapf(err, "getting changes for %s", v3MasterPartition)
	}
	for _, doc := range docs {
		var e v3ExpectationEntry
		if err := doc.DataTo(&e); err != nil {
			if err != nil {
				sklog.Warning("Corrupt expectation change with id %s", doc.Ref.ID)
				continue
			}
		}
		rv = append(rv, e)
	}
	return rv, nil
}

type sqlDBV1 struct {
	dbName string
	db     *pgx.Conn

	oldRecordIDToNewRecordID map[string]string
}

func (s *sqlDBV1) initExpectations(ctx context.Context) error {
	out, err := exec.Command("cockroach", "sql", "--insecure", "--host=localhost:26257",
		"--database="+s.dbName,
		`--execute=DROP TABLE IF EXISTS Expectations`,
		`--execute=DROP TABLE IF EXISTS CLExpectations`,
		`--execute=DROP TABLE IF EXISTS ExpectationsDeltas`,
		`--execute=DROP TABLE IF EXISTS ExpectationsRecords`,
		`--execute=CREATE TABLE IF NOT EXISTS Expectations (
	grouping STRING, -- e.g. {"corpus": "round", "name": "circle"} (not JSONB because we want to use it as the primary key for updating)
	grouping_json JSONB, -- same as grouping, but in JSON form for querying.
	digest BYTES NOT NULL, -- MD5 hash of the pixel data
	label SMALLINT, -- 0 for untriaged, 1 for positive, 2 for negative
	start_index INT4, -- Reserved for future use with expectation ranges
	end_index INT4, -- Reserved for future use with expectation ranges
	PRIMARY KEY (digest, grouping) -- start_index should be on primary key too eventually.
);`,
		`--execute=CREATE TABLE IF NOT EXISTS CLExpectations (
	crs_cl_id STRING, -- e.g. CodeReviewSystem and CL ID "gerrit_12345"
	grouping STRING, -- e.g. {"corpus": "round", "name": "circle"} (not JSONB because we want to use it as the primary key for updating)
	grouping_json JSONB, -- same as grouping, but in JSON form for querying.
	digest BYTES NOT NULL, -- MD5 hash of the pixel data
	label SMALLINT, -- 0 for untriaged, 1 for positive, 2 for negative
	start_index INT4, -- Reserved for future use with expectation ranges
	end_index INT4, -- Reserved for future use with expectation ranges
	PRIMARY KEY (digest, crs_cl_id, grouping) -- start_index should be on primary key too eventually.
);`,
		`--execute=CREATE TABLE IF NOT EXISTS ExpectationsDeltas (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	record_id UUID, -- matches primary key of ExpectationRecords table
	grouping STRING, -- e.g. {"corpus": "round", "name": "circle"}
	digest BYTES NOT NULL, -- MD5 hash of the pixel data
	label_before SMALLINT, -- 0 for untriaged, 1 for positive, 2 for negative
	label_after SMALLINT, -- 0 for untriaged, 1 for positive, 2 for negative
	start_index INT4, -- Reserved for future use with expectation ranges
	end_index_before INT4, -- Reserved for future use with expectation ranges
	end_index_after INT4 -- Reserved for future use with expectation ranges
);`,
		`--execute=CREATE TABLE IF NOT EXISTS ExpectationsRecords (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	crs_cl_id STRING, -- e.g. CodeReviewSystem and CL ID "gerrit_12345". Can be empty string.
	user_name STRING,
	time TIMESTAMP WITH TIME ZONE,
	num_changes INT4
);`,
	).CombinedOutput()
	if err != nil {
		return skerr.Wrapf(err, "creating tables: %s", out)
	}

	dbConnectionURL := "postgresql://root@localhost:26257/" + s.dbName + "?sslmode=disable"
	conf, err := pgx.ParseConfig(dbConnectionURL)
	if err != nil {
		sklog.Fatalf("error getting postgress config: %s", err)
	}
	db, err := pgx.ConnectConfig(ctx, conf)
	if err != nil {
		sklog.Fatalf("error connecting to the database: %s", err)
	}
	if err = db.Ping(ctx); err != nil {
		return skerr.Wrapf(err, "connecting to database via ping %s", dbConnectionURL)
	}
	s.db = db

	sklog.Infof("SQL initializized")
	return nil
}

func (s *sqlDBV1) storeTriageRecords(ctx context.Context, records map[string][]v3TriageRecord) error {
	// TODO(kjlubick) migrate CL expectations too
	mRecords := records[v3MasterPartition]
	s.oldRecordIDToNewRecordID = map[string]string{}

	// We can't upload these in batches (easily) because of the returning statement
	 FIXME this is slow, use go routines
	for _, record := range mRecords {
		row := s.db.QueryRow(ctx,
			`INSERT INTO ExpectationsRecords (user_name, time, num_changes) VALUES ($1, $2, $3) RETURNING id`,
			record.UserName, record.TS, record.Changes)
		recordUUID := ""
		err := row.Scan(&recordUUID)
		if err != nil {
			return skerr.Wrap(err)
		}
		s.oldRecordIDToNewRecordID[record.ID] = recordUUID
	}
	return nil
}

func (s *sqlDBV1) storeExpectationDeltas(ctx context.Context, deltas []v3ExpectationChange) error {
	const batchSize = 100
	err := util.ChunkIter(len(deltas), batchSize, func(startIdx int, endIdx int) error {
		batch := deltas[startIdx:endIdx]

		statement := `INSERT INTO ExpectationsDeltas (record_id, grouping, digest, label_before, label_after) VALUES`

		grouping := map[string]string{
			types.CorpusField: "TODO", // probably need to include a map on the input that maps test name -> corpus
		}

		var arguments []interface{}
		argumentIdx := 1
		for i, value := range batch {
			if i != 0 {
				statement += ","
			}
			statement += fmt.Sprintf("($%d, $%d, $%d, $%d, $%d)",
				argumentIdx, argumentIdx+1, argumentIdx+2, argumentIdx+3, argumentIdx+4)
			argumentIdx += 5

			arguments = append(arguments, s.oldRecordIDToNewRecordID[value.RecordID])

			grouping[types.PrimaryKeyField] = string(value.Grouping)
			jb, err := json.Marshal(grouping)
			if err != nil {
				sklog.Fatalf("Invalid JSON: %s", err)
			}
			arguments = append(arguments, string(jb))

			b, err := digestToBytes(value.Digest)
			if err != nil {
				sklog.Fatalf("Invalid Digest: %s", value.Digest)
			}
			arguments = append(arguments, b)
			arguments = append(arguments, value.LabelBefore)
			arguments = append(arguments, value.AffectedRange.Label)
		}
		_, err := s.db.Exec(ctx, statement, arguments...)
		return err
	})
	if err != nil {
		return skerr.Wrap(err)
	}
	sklog.Infof("Stored %d deltas", len(deltas))
	return nil
}

func (s *sqlDBV1) storeExpectations(ctx context.Context, entries []v3ExpectationEntry) error {
	const batchSize = 100
	err := util.ChunkIter(len(entries), batchSize, func(startIdx int, endIdx int) error {
		batch := entries[startIdx:endIdx]

		statement := `INSERT INTO Expectations (grouping, grouping_json, digest, label) VALUES`

		grouping := map[string]string{
			types.CorpusField: "TODO", // probably need to include a map on the input that maps test name -> corpus
		}

		var arguments []interface{}
		argumentIdx := 1
		for i, value := range batch {
			if i != 0 {
				statement += ","
			}
			statement += fmt.Sprintf("($%d, $%d, $%d, $%d)",
				argumentIdx, argumentIdx+1, argumentIdx+2, argumentIdx+3)
			argumentIdx += 4

			grouping[types.PrimaryKeyField] = string(value.Grouping)
			jb, err := json.Marshal(grouping)
			if err != nil {
				sklog.Fatalf("Invalid JSON: %s", err)
			}
			arguments = append(arguments, string(jb))
			arguments = append(arguments, string(jb))

			b, err := digestToBytes(value.Digest)
			if err != nil {
				sklog.Fatalf("Invalid Digest: %s", value.Digest)
			}
			arguments = append(arguments, b)
			arguments = append(arguments, value.Ranges[0].Label)
		}
		_, err := s.db.Exec(ctx, statement, arguments...)
		return err
	})
	if err != nil {
		return skerr.Wrap(err)
	}
	sklog.Infof("Stored %d entries", len(entries))
	return nil
}

func digestToBytes(d types.Digest) ([]byte, error) {
	return hex.DecodeString(string(d))
}
