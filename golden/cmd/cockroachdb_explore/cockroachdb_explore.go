package main

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os/exec"
	"time"

	"github.com/lib/pq"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/testutils/data_kitchen_sink"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"

	// Make sure the postgreSQL driver is loaded.
	_ "github.com/lib/pq"
)

const simulateSparseData = true

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
		`--execute=
CREATE TABLE IF NOT EXISTS TryJobValues (
	trace_hash BYTES, -- MD5 hash of the trace string
	crs_cl_id STRING, -- e.g. CodeReviewSystem and CL ID "gerrit_12345"
	ps_id STRING, -- PatchSet id
	digest BYTES NOT NULL, -- MD5 hash of the pixel data
	options_hash BYTES, -- MD5 hash of the options string
	source_file_hash BYTES NOT NULL, -- MD5 hash of the source file name TODO(kjlubick) should this be tryjob id? should tryjob id be here?
	PRIMARY KEY (trace_hash, crs_cl_id, ps_id)
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
		`--execute=CREATE TABLE IF NOT EXISTS DiffMetrics (
	left_digest BYTES NOT NULL, -- MD5 hash of the pixel data
	right_digest BYTES NOT NULL, -- MD5 hash of the pixel data
	num_diff_pixels INT4,
	pixel_diff_percent FLOAT4,
	max_channel_diff INT2, -- This is what the RGBAMinFilter and RGBAMaxFilter apply to
	max_rgba_diff INT2[], -- max delta in the red, green, blue, alpha channels. 
	dimensions_differ BOOL,
	PRIMARY KEY (left_digest, right_digest)
);`, // TODO(kjlubick) tables for PS/CL/TJ etc
	).CombinedOutput()
	if err != nil {
		sklog.Fatalf("Could not create tables: %s %s", err, out)
	}

	// TODO(kjlubick) https://www.cockroachlabs.com/docs/stable/comment-on.html#add-a-comment-to-a-column

	db, err := sql.Open("postgres",
		"postgresql://root@localhost:26257/demo_gold_db?sslmode=disable")
	if err != nil {
		sklog.Fatalf("error connecting to the database: ", err)
	}
	defer db.Close()

	ctx := context.Background()
	writeCommits(ctx, db)
	writeMasterBranchTraceData(ctx, db)
	writeCLData(ctx, db)
	writeMasterBranchExpectations(ctx, db)
	writeDiffMetrics(ctx, db)

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

# Select untriaged digests after commit_number 2 (i.e. digests that do not appear in expectations).
# This accounts for the case that digests may be triaged differently.
# See https://stackoverflow.com/a/2973582
> SELECT encode(TraceValues.digest, 'hex'), TraceValues.commit_number FROM
	TraceValues
JOIN
	TraceIDs
ON TraceValues.trace_hash = TraceIDs.trace_hash
WHERE
TraceValues.commit_number > 2 AND
NOT EXISTS (
	SELECT NULL
	FROM Expectations
	WHERE TraceIDs.keys @> Expectations.grouping_json AND TraceValues.digest = Expectations.digest
);

# Get the last 512 commit numbers where we have data. (i.e. get our Dense tile).
> SELECT DISTINCT TraceValues.commit_number from TraceValues WHERE
NOT EXISTS (
	SELECT NULL
	FROM Commits
	WHERE TraceValues.commit_number = Commits.commit_number
) ORDER BY TraceValues.commit_number DESC LIMIT 512;

# SELECT traces that match device = "iPad6,3" and have at least one untriaged digest.
SELECT DISTINCT encode(TraceIDs.trace_hash, 'hex'), TraceIDs.keys FROM
	TraceValues
JOIN
	(SELECT * FROM TraceIDs where TraceIDs.keys @> '{"device": "iPad6,3"}') as TraceIDs
ON TraceValues.trace_hash = TraceIDs.trace_hash
WHERE
TraceValues.commit_number > 2 AND
NOT EXISTS (
	SELECT NULL
	FROM Expectations
	WHERE TraceIDs.keys @> Expectations.grouping_json AND TraceValues.digest = Expectations.digest
);

# SELECT traces that match device = "iPad6,3" and have at least one negative digest.
SELECT DISTINCT encode(TraceIDs.trace_hash, 'hex'), TraceIDs.keys FROM
	TraceValues
JOIN
	(SELECT * FROM TraceIDs where TraceIDs.keys @> '{"device": "iPad6,3"}') as TraceIDs
ON TraceValues.trace_hash = TraceIDs.trace_hash
WHERE
TraceValues.commit_number > 2 AND
EXISTS (
	SELECT NULL
	FROM Expectations
	WHERE TraceIDs.keys @> Expectations.grouping_json AND TraceValues.digest = Expectations.digest AND
	Expectations.label = 2
);

# Get paramset of traces that have data before commit_number = 2
> SELECT DISTINCT keys from
	TraceIDs
JOIN
	TraceValues
ON TraceIDs.trace_hash = TraceValues.trace_hash
WHERE TraceValues.commit_number < 2;

# Get all data from 3 specified traces.
> SELECT encode(digest, 'hex'), commit_number FROM TraceValues WHERE trace_hash
IN (x'796f2cc3f33fa6a9a1f4bef3aa9c48c4', x'3b44c31afc832ef9d1a2d25a5b873152', x'47109b059f45e4f9d5ab61dd0199e2c9')
AND commit_number >= 0;

# Get all unique digests in traces of a given grouping
> SELECT DISTINCT encode(digest, 'hex') FROM
	TraceValues
JOIN
	(SELECT * FROM TraceIDs WHERE TraceIDs.keys @> '{"color mode": "GREY","name":"triangle"}') as TraceIDs
ON TraceIDs.trace_hash = TraceValues.trace_hash
where commit_number >=0;

# Get closest positive digest c03c (compared to digests on traces that have "color mode": "GREY")
> SELECT encode(left_digest, 'hex'), encode(right_digest, 'hex'), num_diff_pixels, pixel_diff_percent, max_rgba_diff, dimensions_differ FROM
  DiffMetrics
JOIN (
    (SELECT * FROM TraceValues where commit_number >=0) as TraceValues
  JOIN
    (SELECT * FROM TraceIDs WHERE TraceIDs.keys @> '{"color mode": "GREY"}') as TraceIDs
  ON TraceValues.trace_hash = TraceIDs.trace_hash
)
ON DiffMetrics.left_digest = TraceValues.digest OR DiffMetrics.right_digest = TraceValues.digest
WHERE (left_digest = x'c03c03c03c03c03c03c03c03c03c03c0' OR right_digest = x'c03c03c03c03c03c03c03c03c03c03c0')
AND EXISTS (
	SELECT NULL
	FROM Expectations
	WHERE TraceIDs.keys @> Expectations.grouping_json AND TraceValues.digest = Expectations.digest AND
	Expectations.label = 1
) ORDER BY pixel_diff_percent, max_channel_diff ASC LIMIT 1;

# all positive digests for two digests...might need to practice exists queries.
> SELECT DISTINCT encode(left_digest, 'hex'), encode(right_digest, 'hex'), num_diff_pixels, pixel_diff_percent, max_rgba_diff, dimensions_differ FROM
  DiffMetrics
JOIN (
    (SELECT * FROM TraceValues where commit_number >=0) as TraceValues
  JOIN
    (SELECT * FROM TraceIDs WHERE TraceIDs.keys @> '{"color mode": "GREY"}') as TraceIDs
  ON TraceValues.trace_hash = TraceIDs.trace_hash
)
ON DiffMetrics.left_digest = TraceValues.digest OR DiffMetrics.right_digest = TraceValues.digest
WHERE (
  left_digest IN (x'c03c03c03c03c03c03c03c03c03c03c0', x'00000000000000000000000000000000') OR
  right_digest IN (x'c03c03c03c03c03c03c03c03c03c03c0', x'00000000000000000000000000000000'))
AND EXISTS (
	SELECT NULL
	FROM Expectations
	WHERE TraceIDs.keys @> Expectations.grouping_json AND TraceValues.digest = Expectations.digest AND
	Expectations.label = 1
) ORDER BY pixel_diff_percent;
`)
}

func writeDiffMetrics(ctx context.Context, db *sql.DB) {
	for _, dbd := range data_kitchen_sink.MakePixelDiffsForCorpusNameGrouping() {
		leftDigestBytes, err := digestToBytes(dbd.LeftDigest)
		if err != nil {
			sklog.Fatalf("invalid digest %s: %s", dbd.LeftDigest, err)
		}
		rightDigestBytes, err := digestToBytes(dbd.RightDigest)
		if err != nil {
			sklog.Fatalf("invalid digest %s: %s", dbd.RightDigest, err)
		}

		m := dbd.Metrics
		_, err = db.ExecContext(ctx,
			`UPSERT INTO DiffMetrics (left_digest, right_digest, num_diff_pixels, pixel_diff_percent,
				 max_channel_diff, max_rgba_diff, dimensions_differ)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			leftDigestBytes, rightDigestBytes, m.NumDiffPixels, m.PixelDiffPercent,
			util.MaxInt(m.MaxRGBADiffs[:]...), pq.Array(m.MaxRGBADiffs), m.DimDiffer,
		)
		if err != nil {
			sklog.Fatalf("Could not add diff for %s-%s: %s", dbd.LeftDigest, dbd.RightDigest, err)
		}
	}
}

func writeCommits(ctx context.Context, db *sql.DB) {
	for i, c := range data_kitchen_sink.MakeCommits() {
		_, err := db.ExecContext(ctx,
			`INSERT INTO Commits (commit_number, git_hash, commit_time, author, subject)
VALUES ($1, $2, $3, $4, $5)`,
			i+1, c.Hash, c.CommitTime, c.Author, c.Subject,
		)
		if err != nil {
			sklog.Fatalf("Could not add commit %#v: %s", c, err)
		}
	}
}

func writeMasterBranchExpectations(ctx context.Context, db *sql.DB) {
	for _, tle := range data_kitchen_sink.MakeMasterBranchTriageHistory() {
		row := db.QueryRowContext(ctx,
			`INSERT INTO ExpectationsRecords (user_name, time, num_changes) VALUES ($1, $2, $3) RETURNING id`,
			tle.User, tle.TS, len(tle.Details))
		recordUUID := ""
		err := row.Scan(&recordUUID)
		if err != nil {
			sklog.Fatalf("Could not get new UUID: %s", err)
		}
		sklog.Infof("Wrote expectation record %s", recordUUID)

		for _, delta := range tle.Details {
			// TODO(kjlubick) transactions to write the data for undoing

			groupJSON, err := json.Marshal(delta.Grouping)
			if err != nil {
				sklog.Fatalf("Invalid grouping: %s", err)
			}
			digestBytes, err := digestToBytes(delta.Digest)
			if err != nil {
				sklog.Fatalf("Invalid digest: %s", err)
			}
			labelAfterInt := 0
			if delta.LabelAfter == expectations.Positive {
				labelAfterInt = 1
			} else if delta.LabelAfter == expectations.Negative {
				labelAfterInt = 2
			}
			labelBeforeInt := 0
			if delta.LabelBefore == expectations.Positive {
				labelBeforeInt = 1
			} else if delta.LabelBefore == expectations.Negative {
				labelBeforeInt = 2
			}

			_, err = db.ExecContext(ctx,
				`INSERT INTO ExpectationsDeltas (record_id, grouping, digest, label_before, label_after)
VALUES ($1, $2, $3, $4, $5)`,
				recordUUID, groupJSON, digestBytes, labelBeforeInt, labelAfterInt,
			)
			if err != nil {
				sklog.Fatalf("Could not write expectations delta %s: %s", delta, err)
			}

			// NOTE for easier querying, should delete rows from Expectations when marking something
			// back to untriaged.

			_, err = db.ExecContext(ctx,
				`UPSERT INTO Expectations (grouping, grouping_json, digest, label)
VALUES ($1, $2, $3, $4)`,
				groupJSON, groupJSON, digestBytes, labelAfterInt,
			)
			if err != nil {
				sklog.Fatalf("Could not write expectations %s: %s", delta, err)
			}
		}
	}
}

func writeMasterBranchTraceData(ctx context.Context, db *sql.DB) {
	const fakeFile = "gs://skia-gold-flutter/dm-json-v1/2020/03/31/23/d14a301e419af7f3eff7cc3a49bf936c75d2b2f0/waterfall/1585696758/dm-1585696758433097948.json"
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

		const commitNumOffset = 0

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

			modifiedCommitNum := commitNumOffset + commitNum

			_, err = db.ExecContext(ctx,
				`UPSERT INTO TraceValues (trace_hash, commit_number, digest, options_hash, source_file_hash)
VALUES ($1, $2, $3, $4, $5)`,
				keysHash, modifiedCommitNum, digestBytes, optsHash, sourceFileHash[:])
			if err != nil {
				sklog.Fatalf("Could not insert data for trace %s commit %d: %s", tp.ID, commitNum, err)
			}
		}
		sklog.Infof("Wrote trace %s (the long way)", tp.ID)
	}
}

func writeCLData(ctx context.Context, db *sql.DB) {
	const fakeFile = "gs://skia-gold-flutter/trybot/dm-json-v1/2020/07/10/07/d108734f19645c8eb443f83ef6af6cdda78a3024/5390858176430080/1594365990/dm-1594365990150477392.json"
	sourceFileHash := md5.Sum([]byte(fakeFile))

	for _, trace := range data_kitchen_sink.MakeDataFromTryJobs() {
		keysHash, keysJSON, err := serializeMap(trace.Keys)
		if err != nil {
			sklog.Fatalf("Should never happen: %s", err)
		}
		optsHash, optsJSON, err := serializeMap(trace.Options)
		if err != nil {
			sklog.Fatalf("Should never happen: %s", err)
		}
		digestBytes, err := digestToBytes(trace.Digest)
		if err != nil {
			sklog.Fatalf("Invalid digest: %s", err)
		}

		_, err = db.ExecContext(ctx,
			`INSERT INTO TraceIDs (trace_hash, keys) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			keysHash, keysJSON)
		if err != nil {
			sklog.Fatalf("Could not insert keys %s - %s: %s", keysHash, trace.Keys, err)
		}

		_, err = db.ExecContext(ctx,
			`INSERT INTO OptionIDs (options_hash, options) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			optsHash, optsJSON)
		if err != nil {
			sklog.Fatalf("Could not insert options %s - %s: %s", optsHash, trace.Options, err)
		}

		_, err = db.ExecContext(ctx,
			`INSERT INTO SourceFiles (source_file_hash, source_file) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			sourceFileHash[:], fakeFile)
		if err != nil {
			sklog.Fatalf("Could not insert source file %s - %s: %s", sourceFileHash, fakeFile, err)
		}

		crsCLID := fmt.Sprintf("%s_%s", trace.PatchSet.CRS, trace.PatchSet.CL)
		_, err = db.ExecContext(ctx,
			`UPSERT INTO TryJobValues (trace_hash, crs_cl_id, ps_id, digest, options_hash, source_file_hash)
VALUES ($1, $2, $3, $4, $5, $6)`,
			keysHash, crsCLID, trace.PatchSet.PS, digestBytes, optsHash, sourceFileHash[:])
		if err != nil {
			sklog.Fatalf("Could not insert data for CL %#v: %s", trace.PatchSet, err)
		}
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
