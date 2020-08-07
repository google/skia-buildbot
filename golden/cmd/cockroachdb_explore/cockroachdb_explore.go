// This executable initializes a cockroachdb instance with some Gold data. It is primarily for
// exploring SQL queries and schemas.
package main

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"os/exec"
	"time"

	"github.com/jackc/pgx/v4" // This has better performance than database/sql
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/sqltest"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/testutils/data_kitchen_sink"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

func main() {
	local := flag.Bool("local", true, "Spin up a local instance of cockroachdb. If false, will connect to local port 26257.")
	fillData := flag.Bool("fill_data", true, "Fill in with sample data")

	flag.Parse()
	if *local {
		storageDir, err := sqltest.StartLocalCockroachDB()
		if err != nil {
			sklog.Fatalf("Could not start local cockroachdb: %s", err)
		}
		sklog.Infof("Check out localhost:8080 and %s for storage", storageDir)
	} else {
		sklog.Infof("Not using local db. Make sure you ran kubectl port-forward gold-cockroachdb-0 26257:26234")
	}

	err := createDemoDBAndTables()
	if err != nil {
		sklog.Fatalf("Could not initialize db/tables: %s", err)
	}
	if !*fillData {
		sklog.Infof("Done, not filling data as requested")
		return
	}

	ctx := context.Background()
	conf, err := pgx.ParseConfig("postgresql://root@localhost:26257/demo_gold_db?sslmode=disable")
	if err != nil {
		sklog.Fatalf("error getting postgress config: %s", err)
	}
	db, err := pgx.ConnectConfig(ctx, conf)
	if err != nil {
		sklog.Fatalf("error connecting to the database: %s", err)
	}
	defer db.Close(ctx)

	writeCommits(ctx, db)
	writeMasterBranchTraceData(ctx, db)
	writeCLData(ctx, db)
	writeMasterBranchExpectations(ctx, db)
	writeDiffMetrics(ctx, db)
	writeIgnoreRules(ctx, db)

	runTestQueries(ctx, db)

	sklog.Infof("Done.")
}

func createDemoDBAndTables() error {
	out, err := exec.Command("cockroach", "sql", "--insecure", "--host=localhost:26257",
		`--execute=CREATE DATABASE IF NOT EXISTS demo_gold_db;`).CombinedOutput()
	if err != nil {
		return skerr.Wrapf(err, "creating database: %s", out)
	}

	out, err = exec.Command("cockroach", "sql", "--insecure", "--host=localhost:26257",
		"--database=demo_gold_db", // Connect to demo_gold_db that we just made
		"--execute="+sql.CockroachDBSchema,
	).CombinedOutput()
	if err != nil {
		return skerr.Wrapf(err, "creating tables: %s", out)
	}
	sklog.Infof("cockroach command appears to have worked")
	return nil
}

func writeDiffMetrics(ctx context.Context, db *pgx.Conn) {
	for _, dbd := range data_kitchen_sink.MakePixelDiffsForCorpusNameGrouping() {
		leftDigestBytes, err := sql.DigestToBytes(dbd.LeftDigest)
		if err != nil {
			sklog.Fatalf("invalid digest %s: %s", dbd.LeftDigest, err)
		}
		rightDigestBytes, err := sql.DigestToBytes(dbd.RightDigest)
		if err != nil {
			sklog.Fatalf("invalid digest %s: %s", dbd.RightDigest, err)
		}

		// We insert all diffs twice, once with each digest taking turns in the "left" and "right"
		// position. This simplifies queries a lot (many fewer OR statements0.
		m := dbd.Metrics
		_, err = db.Exec(ctx,
			`UPSERT INTO DiffMetrics (left_digest, right_digest, num_diff_pixels, pixel_diff_percent,
         max_channel_diff, max_rgba_diff, dimensions_differ)
       VALUES ($1, $2, $3, $4, $5, $6, $7), ($2, $1, $3, $4, $5, $6, $7)`,
			leftDigestBytes, rightDigestBytes, m.NumDiffPixels, m.PixelDiffPercent,
			util.MaxInt(m.MaxRGBADiffs[:]...), m.MaxRGBADiffs[:], m.DimDiffer,
		)
		if err != nil {
			sklog.Fatalf("Could not add diff for %s-%s: %s", dbd.LeftDigest, dbd.RightDigest, err)
		}
	}
}

func writeCommits(ctx context.Context, db *pgx.Conn) {
	for i, c := range data_kitchen_sink.MakeCommits() {
		_, err := db.Exec(ctx,
			`INSERT INTO Commits (commit_number, git_hash, commit_time, author, subject)
VALUES ($1, $2, $3, $4, $5)`,
			i+1, c.Hash, c.CommitTime, c.Author, c.Subject,
		)
		if err != nil {
			sklog.Fatalf("Could not add commit %#v: %s", c, err)
		}
	}
}

func writeMasterBranchExpectations(ctx context.Context, db *pgx.Conn) {
	for _, tle := range data_kitchen_sink.MakeMasterBranchTriageHistory() {
		row := db.QueryRow(ctx,
			`INSERT INTO ExpectationsRecords (user_name, time, num_changes) VALUES ($1, $2, $3) RETURNING expectations_record_id`,
			tle.User, tle.TS, len(tle.Details))
		recordUUID := ""
		err := row.Scan(&recordUUID)
		if err != nil {
			sklog.Fatalf("Could not get new UUID: %s", err)
		}
		sklog.Infof("Wrote expectation record %s", recordUUID)

		for _, delta := range tle.Details {
			groupingJSON, groupingHash, err := sql.SerializeMap(delta.Grouping)
			if err != nil {
				sklog.Fatalf("Invalid grouping: %s", err)
			}
			digestBytes, err := sql.DigestToBytes(delta.Digest)
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

			_, err = db.Exec(ctx, `INSERT INTO KeyValueMaps (keys_hash, keys) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				groupingHash, groupingJSON)
			if err != nil {
				sklog.Fatalf("Could not insert keys for grouping %s: %s", groupingJSON, err)
			}

			_, err = db.Exec(ctx, `
INSERT INTO ExpectationsDeltas (expectations_record_id, grouping_hash, digest, label_before, label_after)
VALUES ($1, $2, $3, $4, $5)`,
				recordUUID, groupingHash, digestBytes, labelBeforeInt, labelAfterInt,
			)
			if err != nil {
				sklog.Fatalf("Could not write expectations delta %s: %s", delta, err)
			}

			_, err = db.Exec(ctx, `UPSERT INTO Expectations (grouping_hash, digest, label) VALUES ($1, $2, $3)`,
				groupingHash, digestBytes, labelAfterInt,
			)
			if err != nil {
				sklog.Fatalf("Could not write expectations %s: %s", delta, err)
			}
		}
	}
}

func writeMasterBranchTraceData(ctx context.Context, db *pgx.Conn) {
	const fakeFile = "gs://skia-gold-flutter/dm-json-v1/2020/03/31/23/d14a301e419af7f3eff7cc3a49bf936c75d2b2f0/waterfall/1585696758/dm-1585696758433097948.json"
	sourceFileHash := md5.Sum([]byte(fakeFile))

	for _, tp := range data_kitchen_sink.MakeTraces() {
		trace := tp.Trace
		keysJSON, keysHash, err := sql.SerializeMap(trace.Keys())
		if err != nil {
			sklog.Fatalf("Should never happen: %s", err)
		}
		optsJSON, optsHash, err := sql.SerializeMap(trace.Options())
		if err != nil {
			sklog.Fatalf("Should never happen: %s", err)
		}
		grouping := groupingFor(trace)
		groupingJSON, groupingHash, err := sql.SerializeMap(grouping)
		if err != nil {
			sklog.Fatalf("Should never happen: %s", err)
		}

		// This somewhat emulates that we ingest data by commit
		for commitNum := 0; commitNum < len(trace.Digests); commitNum++ {
			if trace.Digests[commitNum] == tiling.MissingDigest {
				continue // skip adding missing data (which is what we would expect in a real setting)
			}
			digestBytes, err := sql.DigestToBytes(trace.Digests[commitNum])
			if err != nil {
				sklog.Fatalf("Invalid digest: %s", err)
			}

			// In real ingestion, we don't have to insert this multiple times, but we do it here to
			// make sure storing multiple times doesn't break anything.
			_, err = db.Exec(ctx, `
INSERT INTO KeyValueMaps (keys_hash, keys) VALUES ($1, $2), ($3, $4), ($5, $6) 
ON CONFLICT DO NOTHING`,
				keysHash, keysJSON, optsHash, optsJSON, groupingHash, groupingJSON)
			if err != nil {
				sklog.Fatalf("Could not insert keys for trace %s: %s", tp.ID, err)
			}

			_, err = db.Exec(ctx, `
UPSERT INTO SourceFiles (source_file_hash, source_file, last_ingested) VALUES ($1, $2, $3)`,
				sourceFileHash[:], fakeFile, time.Now())
			if err != nil {
				sklog.Fatalf("Could not insert source file %s - %s: %s", sourceFileHash, fakeFile, err)
			}

			shard := sql.ComputeTraceValueShard(keysHash)

			_, err = db.Exec(ctx, `
UPSERT INTO TraceValues (trace_hash, shard, commit_number, grouping_hash, digest, 
  options_hash, source_file_hash)
VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				keysHash, shard, commitNum, groupingHash, digestBytes, optsHash, sourceFileHash[:])
			if err != nil {
				sklog.Fatalf("Could not insert data for trace %s commit %d: %s", tp.ID, commitNum, err)
			}

			// This lets us index by untriaged status and not have to scan over all TraceValues.
			_, err = db.Exec(ctx, `
INSERT INTO Expectations (grouping_hash, digest, label) VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING`,
				groupingHash, digestBytes, 0,
			)
			if err != nil {
				sklog.Fatalf("Could not write expectations (if needed) for %s: %s", tp.ID, err)
			}

		}
		sklog.Infof("Wrote trace %s (the long way)", tp.ID)
	}
}

func groupingFor(trace *tiling.Trace) map[string]string {
	return map[string]string{
		types.CorpusField:     trace.Corpus(),
		types.PrimaryKeyField: string(trace.TestName()),
	}
}

func writeCLData(ctx context.Context, db *pgx.Conn) {
	const fakeFile = "gs://skia-gold-flutter/trybot/dm-json-v1/2020/07/10/07/d108734f19645c8eb443f83ef6af6cdda78a3024/5390858176430080/1594365990/dm-1594365990150477392.json"
	sourceFileHash := md5.Sum([]byte(fakeFile))

	for _, trace := range data_kitchen_sink.MakeDataFromTryJobs() {
		keysJSON, keysHash, err := sql.SerializeMap(trace.Keys)
		if err != nil {
			sklog.Fatalf("Should never happen: %s", err)
		}
		optsJSON, optsHash, err := sql.SerializeMap(trace.Options)
		if err != nil {
			sklog.Fatalf("Should never happen: %s", err)
		}
		digestBytes, err := sql.DigestToBytes(trace.Digest)
		if err != nil {
			sklog.Fatalf("Invalid digest: %s", err)
		}

		_, err = db.Exec(ctx,
			`INSERT INTO Traces (trace_hash, keys) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			keysHash, keysJSON)
		if err != nil {
			sklog.Fatalf("Could not insert keys %s - %s: %s", keysHash, trace.Keys, err)
		}

		_, err = db.Exec(ctx,
			`INSERT INTO KeyValueMaps (keys_hash, keys) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			optsHash, optsJSON)
		if err != nil {
			sklog.Fatalf("Could not insert options %s - %s: %s", optsHash, trace.Options, err)
		}

		_, err = db.Exec(ctx,
			`UPSERT INTO SourceFiles (source_file_hash, source_file, last_ingested) VALUES ($1, $2, $3)`,
			sourceFileHash[:], fakeFile, time.Now())
		if err != nil {
			sklog.Fatalf("Could not insert source file %s - %s: %s", sourceFileHash, fakeFile, err)
		}

		crsCLID := fmt.Sprintf("%s_%s", trace.PatchSet.CRS, trace.PatchSet.CL)
		_, err = db.Exec(ctx,
			`UPSERT INTO TryJobValues (trace_hash, crs_cl_id, ps_id, digest, options_hash, 
         cis_tryjob_id, source_file_hash)
       VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			keysHash, crsCLID, trace.PatchSet.PS, digestBytes, optsHash, "TODO", sourceFileHash[:])
		if err != nil {
			sklog.Fatalf("Could not insert data for CL %#v: %s", trace.PatchSet, err)
		}
	}
}

func writeIgnoreRules(ctx context.Context, db *pgx.Conn) {
	for _, rule := range data_kitchen_sink.MakeIgnoreRules() {
		queryJSONBytes, err := json.Marshal(rule.Query)
		if err != nil {
			sklog.Fatalf("invalid JSON %s", err)
		}

		_, err = db.Exec(ctx, `
UPSERT INTO IgnoreRules (created_user, updated_user, expires, note, query) VALUES 
  ($1, $2, $3, $4, $5)`,
			rule.CreatedBy, rule.UpdatedBy, rule.Expires, rule.Note, string(queryJSONBytes))
		if err != nil {
			sklog.Fatalf("Could not insert rule %v: %s", rule, err)
		}
	}
}
