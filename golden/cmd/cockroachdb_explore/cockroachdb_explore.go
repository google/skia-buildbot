// This executable initializes a cockroachdb instance with some Gold data. It is primarily for
// exploring SQL queries and schemas.
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

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/testutils/data_kitchen_sink"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"

	// Make sure the postgreSQL driver is loaded.
	_ "github.com/lib/pq"
)

func main() {
	local := flag.Bool("local", true, "Use a local instance of cockroachdb. If false, will connect to local port 26257.")

	flag.Parse()
	if *local {
		err := startLocalCockroachDB()
		if err != nil {
			sklog.Fatalf("Could not start local cockroachdb: %s", err)
		}
	} else {
		sklog.Infof("Not using local db. Make sure you ran kubectl port-forward gold-cockroachdb-0 26257:26234")
	}

	err := createDemoDBAndTables()
	if err != nil {
		sklog.Fatalf("Could not initialize db/tables: %s", err)
	}

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

	sklog.Infof("Done.")
}

func startLocalCockroachDB() error {
	err := exec.Command("killall", "-9", "cockroach").Run()
	if err != nil {
		sklog.Warning("Attempted to stop previous cockroach instances failed. Probably were none.")
	}

	out, err := exec.Command("cockroach", "version").CombinedOutput()
	if err != nil {
		return skerr.Wrapf(err, "Do you have 'cockroach' on your path? %s", out)
	}
	sklog.Infof("cockroach version: \n%s", out)

	tmpDir, err := ioutil.TempDir("", "cockroach-db")
	if err != nil {
		return skerr.Wrapf(err, "making tempdir")
	}

	err = exec.Command("cockroach",
		"start-single-node", "--insecure", "--listen-addr=localhost:26257",
		"--http-addr=localhost:8080", "--background",
		"--store="+tmpDir,
	).Start()

	if err != nil {
		return skerr.Wrapf(err, "starting local cockroach")
	}

	// Wait for DB to come up.
	time.Sleep(3 * time.Second)

	sklog.Infof("Check out localhost:8080 and %s for storage", tmpDir)
	return nil
}

func createDemoDBAndTables() error {
	out, err := exec.Command("cockroach", "sql", "--insecure", "--host=localhost:26257",
		`--execute=CREATE DATABASE IF NOT EXISTS demo_gold_db;`).CombinedOutput()
	if err != nil {
		return skerr.Wrapf(err, "creating database: %s", out)
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
	keys JSONB NOT NULL, -- The trace's keys, e.g. {"color mode":"RGB", "device":"walleye"}
	INVERTED INDEX trace_keys (keys)
);`,
		`--execute=CREATE TABLE IF NOT EXISTS OptionIDs  (
	options_hash BYTES PRIMARY KEY, -- MD5 hash of the options
	options JSONB NOT NULL -- The trace keys, e.g. {"ext":"png"}
);`,
		`--execute=CREATE TABLE IF NOT EXISTS SourceFiles (
	source_file_hash BYTES PRIMARY KEY, -- The MD5 hash of the source file name
	source_file STRING NOT NULL,  -- The full name of the source file, e.g. gs://bucket/2020/01/02/03/15/foo.json
	last_ingested TIMESTAMP WITH TIME ZONE NOT NULL
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
-- This is what the RGBAMinFilter and RGBAMaxFilter apply to. There does not appear to be a way to
-- do this via SQL statements (in a clean way).
	max_channel_diff INT2,
	max_rgba_diff INT2[], -- max delta in the red, green, blue, alpha channels.
	dimensions_differ BOOL,
	PRIMARY KEY (left_digest, right_digest)
);`, // TODO(kjlubick) tables for PS/CL/TJ etc
	).CombinedOutput()
	if err != nil {
		return skerr.Wrapf(err, "creating tables: %s", out)
	}
	sklog.Infof("cockroach command appears to have worked")
	return nil
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
				`UPSERT INTO SourceFiles (source_file_hash, source_file, last_ingested) VALUES
				  ($1, $2, $3)`,
				sourceFileHash[:], fakeFile, time.Now())
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
			`UPSERT INTO SourceFiles (source_file_hash, source_file, last_ingested) VALUES
				  ($1, $2, $3)`,
			sourceFileHash[:], fakeFile, time.Now())
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
