package main

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os/exec"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/testutils/data_kitchen_sink"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"

	// Make sure the postgreSQL driver is loaded.
	_ "github.com/lib/pq"
)

func main() {
	flag.Parse()
	err := exec.Command("killall", "-9", "cockroach").Run()
	if err != nil {
		sklog.Warning("Attempted to stop previous cockroach instances failed. Probably were none.")
	}

	out, err := exec.Command("cockroach", "version").CombinedOutput()
	if err != nil {
		sklog.Fatalf("Do you have 'cockroach' on your path? %s: %s", err, out)
	}
	sklog.Infof("cockroach version: \n%s", out)

	tmpDir, err := ioutil.TempDir("", "cockroach-db")
	if err != nil {
		sklog.Fatalf("Could not make tempdir: %s", err)
	}

	err = exec.Command("cockroach",
		"start-single-node", "--insecure", "--listen-addr=localhost:26257",
		"--http-addr=localhost:8080", "--background",
		"--store="+tmpDir,
	).Start()

	if err != nil {
		sklog.Fatalf("Could not start local cockroach version %s: %s", err)
	}

	// Wait for DB to come up.
	time.Sleep(3 * time.Second)

	sklog.Infof("Check out localhost:8080 and %s for storage", tmpDir)

	out, err = exec.Command("cockroach", "sql", "--insecure", "--host=localhost:26257",
		`--execute=CREATE DATABASE IF NOT EXISTS demo_gold_db;`).CombinedOutput()
	if err != nil {
		sklog.Fatalf("Could not create database: %s %s", err, out)
	}

	out, err = exec.Command("cockroach", "sql", "--insecure", "--host=localhost:26257",
		"--database=demo_gold_db", // Connect to demo_gold_db that we just made
		`--execute=
CREATE TABLE IF NOT EXISTS TraceValues (
	trace_hash BYTES, -- MD5 hash of the trace string
	commit_number INT4,
	digest BYTES NOT NULL, -- MD5 hash of the pixel data
	options_hash BYTES, -- MD5 hash of the options string
	source_file_hash BYTES NOT NULL, -- MD5 hash of the source file name
	PRIMARY KEY (trace_hash, commit_number)
);`,
		`--execute=CREATE TABLE IF NOT EXISTS Commits (
  commit_number INT4 PRIMARY KEY, -- The commit_number; a monotonically increasing number as we follow master branch through time.
  git_hash STRING NOT NULL,
  commit_time TIMESTAMP WITH TIME ZONE NOT NULL,
  author STRING,
  subject STRING
);`,
		`--execute=CREATE TABLE IF NOT EXISTS TraceIDs  (
	trace_hash BYTES PRIMARY KEY, -- MD5 hash of the keys (which defines the trace to be unique)
	keys JSONB NOT NULL -- The trace's keys, e.g. {"color mode":"RGB", "device":"walleye"}
);`,
		`--execute=CREATE TABLE IF NOT EXISTS OptionIDs  (
	options_hash BYTES PRIMARY KEY, -- MD5 hash of the options 
	options JSONB NOT NULL -- The trace keys, e.g. {"ext":"png"}
);`,
		`--execute=CREATE TABLE IF NOT EXISTS SourceFiles (
	source_file_hash BYTES PRIMARY KEY, -- The MD5 hash of the source file name
	source_file STRING NOT NULL  -- The full name of the source file, e.g. gs://bucket/2020/01/02/03/15/foo.json
);`,
		`--execute=CREATE TABLE IF NOT EXISTS Expectations (
	branch STRING, -- e.g. "master" or "gerrit_1234"
	grouping STRING, -- e.g. {"corpus": "round", "name": "circle"} (not JSONB because we want to use it as the primary key for updating)
	grouping_json JSONB, -- same as grouping, but in JSON form for querying. 
	digest BYTES NOT NULL, -- MD5 hash of the pixel data
	start_index INT4, -- Corresponds to commit number
	end_index INT4, -- Corresponds to commit number
	label SMALLINT, -- 0 for untriaged, 1 for positive, 2 for negative
	PRIMARY KEY (digest, branch, grouping, start_index)
);`,
		`--execute=CREATE TABLE IF NOT EXISTS ExpectationsDeltas (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	record_id UUID, -- matches primary key of ExpectationRecords table
	branch STRING, -- e.g. "master" or "gerrit_1234"
	grouping STRING, -- e.g. {"corpus": "round", "name": "circle"}
	digest BYTES NOT NULL, -- MD5 hash of the pixel data
	start_index INT4, -- Corresponds to commit number
	label_before SMALLINT, -- 0 for untriaged, 1 for positive, 2 for negative
	label_after SMALLINT -- 0 for untriaged, 1 for positive, 2 for negative
);`,
		`--execute=CREATE TABLE IF NOT EXISTS ExpectationsRecords (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	user_name STRING,
	time TIMESTAMP WITH TIME ZONE,
	num_changes INT4
);`,
	).CombinedOutput()
	if err != nil {
		sklog.Fatalf("Could not create tables: %s %s", err, out)
	}

	// TODO(kjlubick) https://www.cockroachlabs.com/docs/stable/comment-on.html#add-a-comment-to-a-column

	db, err := sql.Open("postgres",
		"postgresql://root@localhost:26257/demo_gold_db?sslmode=disable")
	if err != nil {
		log.Fatal("error connecting to the database: ", err)
	}
	defer db.Close()

	ctx := context.Background()
	writeTraceData(ctx, db)

	for _, tle := range data_kitchen_sink.MakeTriageHistory() {
		row := db.QueryRowContext(ctx,
			`INSERT INTO ExpectationsRecords (user_name, time, num_changes) VALUES ($1, $2, $3) RETURNING id`,
			tle.User, tle.TS, len(tle.Details))
		rowUUID := ""
		err := row.Scan(&rowUUID)
		if err != nil {
			sklog.Fatalf("Could not get new UUID: %s", err)
		}
		sklog.Infof("Wrote expectation record %s", rowUUID)

		for _, delta := range tle.Details {
			// TODO(kjlubick) transactions to write the data for undoing

			corpus := data_kitchen_sink.RoundCorpus
			if delta.Grouping != data_kitchen_sink.CircleTest {
				corpus = data_kitchen_sink.CornersCorpus
			}
			groupJSON, err := json.Marshal(map[string]string{
				types.CorpusField:     corpus,
				types.PrimaryKeyField: string(delta.Grouping),
			})
			if err != nil {
				sklog.Fatalf("Invalid grouping: %s", err)
			}
			digestBytes, err := digestToBytes(delta.Digest)
			if err != nil {
				sklog.Fatalf("Invalid digest: %s", err)
			}
			labelInt := 0
			if delta.Label == expectations.Positive {
				labelInt = 1
			} else if delta.Label == expectations.Negative {
				labelInt = 2
			}

			// NOTE for easier querying, should delete rows from Expectations when marking something
			// back to untriaged.

			_, err = db.ExecContext(ctx,
				`UPSERT INTO Expectations (branch, grouping, grouping_json, digest, start_index, end_index, label)
VALUES ($1, $2, $3, $4, 0, 2147483647, $5)`,
				"master", groupJSON, groupJSON, digestBytes, labelInt,
			)
			if err != nil {
				sklog.Fatalf("Could not write expectations delta %s: %s", delta, err)
			}
		}
	}

	sklog.Infof(`Done.
Try out the following commands in the terminal to test the live db:
$ cockroach sql --insecure --database demo_gold_db
> SELECT encode(trace_hash, 'hex'), jsonb_pretty(keys) FROM TraceIDs;
> SELECT encode(trace_hash, 'hex'), jsonb_pretty(keys) FROM TraceIDs where keys @> '{"color mode": "GREY", "name": "triangle"}';
> SELECT keys FROM traceids WHERE trace_hash = x'47109b059f45e4f9d5ab61dd0199e2c9';
> SELECT commit_number, encode(digest, 'hex') FROM TraceValues WHERE trace_hash = x'47109b059f45e4f9d5ab61dd0199e2c9';

# Get trace data for grey triangle traces
> SELECT encode(trace_hash, 'hex'), commit_number, encode(digest, 'hex') FROM TraceValues values
JOIN (SELECT trace_hash th FROM TraceIDs where keys @> '{"color mode": "GREY", "name": "triangle"}') as matchingTraces
ON values.trace_hash = matchingTraces.th;
# This inverted index will prevent a full table scan for the above query
> CREATE INVERTED INDEX ON TraceIDs (keys)

> select * from ExpectationsRecords order by time desc;

> select grouping_json, encode(digest, 'hex') from Expectations where label = 2;

# This triple JOIN scenario returns all traces that have a negative digest some time after 
# commit_number 5 and match device=iPad6,3 [Needs some indexing because currently requires 3
# FULL_SCANs.]
> SELECT DISTINCT encode(tracesThatMatchKeys.trace_hash, 'hex'), tracesThatMatchKeys.keys FROM (
		(SELECT digest neg_digest, start_index, end_index FROM Expectations WHERE label = 2)
	JOIN
		(SELECT digest, trace_hash th, commit_number FROM TraceValues WHERE commit_number > 5) as tracesWithNegatives
	ON tracesWithNegatives.digest = neg_digest AND commit_number >= start_index AND commit_number < end_index
)
JOIN (
		(SELECT grouping_json FROM Expectations WHERE label=2)
	JOIN
		(SELECT trace_hash, keys from TraceIDs where keys @> '{"device": "iPad6,3"}') as tracesThatMatchKeys
	ON tracesThatMatchKeys.keys @> grouping_json
)
ON tracesThatMatchKeys.trace_hash = tracesWithNegatives.th;

# Select all untriaged digests (i.e. digests that do not appear in expectations). [needs some fine
# tuning to make sure grouping matches)
# See https://stackoverflow.com/a/2973582
> SELECT DISTINCT encode(TraceValues.digest, 'hex') from TraceValues WHERE NOT EXISTS (
	SELECT NULL
	FROM Expectations
	WHERE TraceValues.digest = Expectations.digest
);

`)
}

func writeTraceData(ctx context.Context, db *sql.DB) {
	const fakeFile = "skia-gold-flutter/dm-json-v1/2020/03/31/23/d14a301e419af7f3eff7cc3a49bf936c75d2b2f0/waterfall/1585696758/dm-1585696758433097948.json"
	sourceFileHash := md5.Sum([]byte(fakeFile))

	for _, tp := range data_kitchen_sink.MakeTraces() {
		trace := tp.Trace
		keysHash, keysJSON, err := serializeMap(trace.Keys())
		if err != nil {
			sklog.Fatalf("Should never happen: %s", err)
		}
		optsHash, optsJSON, err := serializeMap(trace.Options())
		if err != nil {
			sklog.Fatalf("Should never happen: %s", err)
		}

		for commitNum := 0; commitNum < len(trace.Digests); commitNum++ {
			if trace.Digests[commitNum] == tiling.MissingDigest {
				continue // skip adding missing data (which is what we would expect in a real setting)
			}
			digestBytes, err := digestToBytes(trace.Digests[commitNum])
			if err != nil {
				sklog.Fatalf("Invalid digest: %s", err)
			}

			// Could maybe use md5 function

			_, err = db.ExecContext(ctx,
				`INSERT INTO TraceIDs (trace_hash, keys) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				keysHash, keysJSON)
			if err != nil {
				sklog.Fatalf("Could not insert keys %s - %s: %s", keysHash, trace.Keys(), err)
			}

			_, err = db.ExecContext(ctx,
				`INSERT INTO OptionIDs (options_hash, options) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				optsHash, optsJSON)
			if err != nil {
				sklog.Fatalf("Could not insert options %s - %s: %s", optsHash, trace.Options(), err)
			}

			_, err = db.ExecContext(ctx,
				`INSERT INTO SourceFiles (source_file_hash, source_file) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				sourceFileHash[:], fakeFile)
			if err != nil {
				sklog.Fatalf("Could not insert source file %s - %s: %s", sourceFileHash, fakeFile, err)
			}

			_, err = db.ExecContext(ctx,
				`UPSERT INTO TraceValues (trace_hash, commit_number, digest, options_hash, source_file_hash)
VALUES ($1, $2, $3, $4, $5)`,
				keysHash, commitNum, digestBytes, optsHash, sourceFileHash[:])
			if err != nil {
				sklog.Fatalf("Could not insert data for trace %s commit %d: %s", tp.ID, commitNum, err)
			}
		}
		sklog.Infof("Wrote trace %s (the long way)", tp.ID)
	}
}

func serializeMap(m map[string]string) ([]byte, string, error) {
	str, err := json.Marshal(m)
	if err != nil {
		return nil, "", err
	}
	h := md5.Sum(str)
	return h[:], string(str), err
}

func digestToBytes(d types.Digest) ([]byte, error) {
	return hex.DecodeString(string(d))
}
