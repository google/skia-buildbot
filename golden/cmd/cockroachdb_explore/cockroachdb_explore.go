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
	"go.skia.org/infra/go/paramtools"
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

	must(writeCommits(ctx, db))
	must(writePrimaryBranchTraceData(ctx, db))
	must(writePrimaryBranchExpectations(ctx, db))
	must(writeTryjobsChangelistsAndPatchsets(ctx, db))
	must(writeChangelistExpectations(ctx, db))
	must(writeTryjobData(ctx, db))
	must(writeDiffMetrics(ctx, db))
	must(writeIgnoreRules(ctx, db))

	sklog.Infof("Done.")
}

func must(err error) {
	if err != nil {
		sklog.Fatal(err)
	}
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

func writeDiffMetrics(ctx context.Context, db *pgx.Conn) error {
	for _, dbd := range data_kitchen_sink.MakePixelDiffsForCorpusNameGrouping() {
		leftDigestBytes, err := sql.DigestToBytes(dbd.LeftDigest)
		if err != nil {
			return skerr.Wrap(err)
		}
		rightDigestBytes, err := sql.DigestToBytes(dbd.RightDigest)
		if err != nil {
			return skerr.Wrap(err)
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
			return skerr.Wrapf(err, "adding diff for %s-%s", dbd.LeftDigest, dbd.RightDigest)
		}
	}
	return nil
}

func writeCommits(ctx context.Context, db *pgx.Conn) error {
	for i, c := range data_kitchen_sink.MakeCommits() {
		_, err := db.Exec(ctx,
			`INSERT INTO Commits (commit_id, git_hash, commit_time, author, subject)
VALUES ($1, $2, $3, $4, $5)`,
			i+1, c.Hash, c.CommitTime, c.Author, c.Subject,
		)
		if err != nil {
			return skerr.Wrapf(err, "adding commit %#v", c)
		}
	}
	return nil
}

func writePrimaryBranchExpectations(ctx context.Context, db *pgx.Conn) error {
	for _, tle := range data_kitchen_sink.MakePrimaryBranchTriageHistory() {
		row := db.QueryRow(ctx,
			`INSERT INTO ExpectationsRecords (user_name, time, num_changes) VALUES ($1, $2, $3) RETURNING expectations_record_id`,
			tle.User, tle.TS, len(tle.Details))
		recordUUID := ""
		err := row.Scan(&recordUUID)
		if err != nil {
			return skerr.Wrapf(err, "getting new UUID")
		}
		sklog.Infof("Wrote expectation record %s", recordUUID)

		for _, delta := range tle.Details {
			groupingJSON, groupingHash, err := sql.SerializeMap(delta.Grouping)
			if err != nil {
				return skerr.Wrap(err)
			}
			digestBytes, err := sql.DigestToBytes(delta.Digest)
			if err != nil {
				return skerr.Wrap(err)
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

			_, err = db.Exec(ctx, `INSERT INTO Groupings (grouping_id, keys) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				groupingHash, groupingJSON)
			if err != nil {
				return skerr.Wrapf(err, "inserting keys for grouping %s", groupingJSON)
			}

			_, err = db.Exec(ctx, `
INSERT INTO ExpectationsDeltas (expectations_record_id, grouping_id, digest, label_before, label_after)
VALUES ($1, $2, $3, $4, $5)`,
				recordUUID, groupingHash, digestBytes, labelBeforeInt, labelAfterInt,
			)
			if err != nil {
				return skerr.Wrapf(err, "writing expectations delta %s", delta)
			}

			_, err = db.Exec(ctx, `UPSERT INTO Expectations (grouping_id, digest, label) VALUES ($1, $2, $3)`,
				groupingHash, digestBytes, labelAfterInt,
			)
			if err != nil {
				return skerr.Wrapf(err, "writing expectations %s", delta)
			}
		}
	}
	return nil
}

func writeChangelistExpectations(ctx context.Context, db *pgx.Conn) error {
	for cID, entries := range data_kitchen_sink.MakeChangelistsTriageHistory() {
		crsCLID := qualifyIDWithSystem(cID.CRS, cID.CL)
		for _, tle := range entries {
			row := db.QueryRow(ctx, `
INSERT INTO ExpectationsRecords (changelist_id, user_name, time, num_changes) 
  VALUES ($1, $2, $3, $4) RETURNING expectations_record_id`,
				crsCLID, tle.User, tle.TS, len(tle.Details))
			recordUUID := ""
			err := row.Scan(&recordUUID)
			if err != nil {
				return skerr.Wrapf(err, "getting new UUID")
			}
			sklog.Infof("Wrote expectation record %s", recordUUID)

			for _, delta := range tle.Details {
				groupingJSON, groupingHash, err := sql.SerializeMap(delta.Grouping)
				if err != nil {
					return skerr.Wrap(err)
				}
				digestBytes, err := sql.DigestToBytes(delta.Digest)
				if err != nil {
					return skerr.Wrap(err)
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

				_, err = db.Exec(ctx, `INSERT INTO Groupings (grouping_id, keys) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
					groupingHash, groupingJSON)
				if err != nil {
					return skerr.Wrapf(err, "inserting keys for grouping %s", groupingJSON)
				}

				_, err = db.Exec(ctx, `
INSERT INTO ExpectationsDeltas (expectations_record_id, grouping_id, digest, label_before, label_after)
VALUES ($1, $2, $3, $4, $5)`,
					recordUUID, groupingHash, digestBytes, labelBeforeInt, labelAfterInt,
				)
				if err != nil {
					return skerr.Wrapf(err, "writing expectations delta %s", delta)
				}

				_, err = db.Exec(ctx, `
UPSERT INTO ChangelistExpectations (changelist_id, grouping_id, digest, label)
VALUES ($1, $2, $3, $4)`,
					crsCLID, groupingHash, digestBytes, labelAfterInt,
				)
				if err != nil {
					return skerr.Wrapf(err, "writing expectations %s", delta)
				}
			}
		}
	}
	return nil
}

func writePrimaryBranchTraceData(ctx context.Context, db *pgx.Conn) error {
	const fakeFile = "gs://skia-gold-flutter/dm-json-v1/2020/03/31/23/d14a301e419af7f3eff7cc3a49bf936c75d2b2f0/waterfall/1585696758/dm-1585696758433097948.json"
	sourceFileHash := md5.Sum([]byte(fakeFile))

	for _, tp := range data_kitchen_sink.MakeTraces() {
		trace := tp.Trace
		keysJSON, traceHash, err := sql.SerializeMap(trace.Keys())
		if err != nil {
			return skerr.Wrap(err)
		}
		optsJSON, optsHash, err := sql.SerializeMap(trace.Options())
		if err != nil {
			return skerr.Wrap(err)
		}
		grouping := groupingFor(trace.Keys())
		groupingJSON, groupingHash, err := sql.SerializeMap(grouping)
		if err != nil {
			return skerr.Wrap(err)
		}

		// It's important to set most_recent_commit_id to 0 so we can compare against it later.
		// Comparison to NULL is problematic https://www.xaprb.com/blog/2006/05/18/why-null-never-compares-false-to-anything-in-sql/
		_, err = db.Exec(ctx, `
INSERT INTO Traces (trace_id, keys, most_recent_commit_id) VALUES 
  ($1, $2, 0) ON CONFLICT DO NOTHING`,
			traceHash, keysJSON)
		if err != nil {
			return skerr.Wrapf(err, "inserting keys for trace %s", tp.ID)
		}

		_, err = db.Exec(ctx, `INSERT INTO Options (options_id, keys) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			optsHash, optsJSON)
		if err != nil {
			return skerr.Wrapf(err, "inserting options for trace %s", tp.ID)
		}

		_, err = db.Exec(ctx, `INSERT INTO Groupings (grouping_id, keys) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			groupingHash, groupingJSON)
		if err != nil {
			return skerr.Wrapf(err, "inserting grouping for trace %s", tp.ID)
		}

		_, err = db.Exec(ctx, `
UPSERT INTO SourceFiles (source_file_id, source_file, last_ingested) VALUES ($1, $2, $3)`,
			sourceFileHash[:], fakeFile, time.Now())
		if err != nil {
			return skerr.Wrapf(err, "inserting source file %s - %s", sourceFileHash, fakeFile)
		}

		// This somewhat emulates that we ingest data by commit
		for i := 0; i < len(trace.Digests); i++ {
			if trace.Digests[i] == tiling.MissingDigest {
				continue // skip adding missing data (which is what we would expect in a real setting)
			}
			digestBytes, err := sql.DigestToBytes(trace.Digests[i])
			if err != nil {
				return skerr.Wrapf(err, "invalid digest: %s", trace.Digests[i])
			}

			commitID := i + 1
			shard := sql.ComputeTraceValueShard(traceHash)

			_, err = db.Exec(ctx, `
UPSERT INTO TraceValues (trace_id, shard, commit_id, grouping_id, digest, options_id, source_file_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				traceHash, shard, commitID, groupingHash, digestBytes, optsHash, sourceFileHash[:])
			if err != nil {
				return skerr.Wrapf(err, "inserting data for trace %s commit %d", tp.ID, i)
			}

			// This lets us index by untriaged status and not have to scan over all TraceValues.
			_, err = db.Exec(ctx, `
INSERT INTO Expectations (grouping_id, digest, label) VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING`,
				groupingHash, digestBytes, sql.LabelUntriaged,
			)
			if err != nil {
				return skerr.Wrapf(err, "writing expectations (if needed) for %s", tp.ID)
			}

			_, err = db.Exec(ctx, `UPDATE Commits SET has_data = true WHERE commit_id = $1`,
				commitID,
			)
			if err != nil {
				return skerr.Wrapf(err, "writing commit denseness")
			}

			err = writePrimaryParams(ctx, db, trace.Keys(), trace.Options(), commitID)
			if err != nil {
				return skerr.Wrapf(err, "writing PrimaryBranchParams")
			}

			_, err = db.Exec(ctx, `
UPDATE Traces 
SET most_recent_commit_id = CASE
WHEN (most_recent_commit_id < $2) THEN 
  $2
ELSE 
  most_recent_commit_id
END
WHERE trace_id IN ($1)`,
				traceHash, commitID,
			)
			if err != nil {
				return skerr.Wrapf(err, "writing most recent commit_id %d to trace %s", commitID, keysJSON)
			}
		}
		sklog.Infof("Wrote trace %s (the long way)", tp.ID)
	}
	return nil
}

func writePrimaryParams(ctx context.Context, db *pgx.Conn, keys, options map[string]string, commitID int) error {
	insert := "INSERT INTO PrimaryBranchParams (key, value, commit_id) VALUES "
	const valuesPerRow = 3
	numRows := len(keys) + len(options)
	placeholders, err := sql.ValuesPlaceholders(valuesPerRow, numRows)
	if err != nil {
		return skerr.Wrap(err)
	}
	insert += placeholders
	insert += " ON CONFLICT DO NOTHING"

	arguments := make([]interface{}, 0, valuesPerRow*numRows)
	for key, value := range keys {
		arguments = append(arguments, key, value, commitID)
	}
	for option, value := range options {
		arguments = append(arguments, option, value, commitID)
	}

	_, err = db.Exec(ctx, insert, arguments...)
	if err != nil {
		return skerr.Wrapf(err, "inserting params for primary branch")
	}
	return nil
}

func writeTryjobParams(ctx context.Context, db *pgx.Conn, keys, options map[string]string, clID, psID string) error {
	insert := "INSERT INTO TryjobParams (key, value, changelist_id, patchset_id) VALUES "
	const valuesPerRow = 4
	numRows := len(keys) + len(options)
	placeholders, err := sql.ValuesPlaceholders(valuesPerRow, numRows)
	if err != nil {
		return skerr.Wrap(err)
	}
	insert += placeholders
	insert += " ON CONFLICT DO NOTHING"

	arguments := make([]interface{}, 0, valuesPerRow*numRows)
	for key, value := range keys {
		arguments = append(arguments, key, value, clID, psID)
	}
	for option, value := range options {
		arguments = append(arguments, option, value, clID, psID)
	}

	_, err = db.Exec(ctx, insert, arguments...)
	if err != nil {
		return skerr.Wrapf(err, "inserting params for primary branch")
	}
	return nil
}

func groupingFor(keys map[string]string) map[string]string {
	return map[string]string{
		types.CorpusField:     keys[types.CorpusField],
		types.PrimaryKeyField: keys[types.PrimaryKeyField],
	}
}

func qualifyIDWithSystem(system, id string) string {
	return fmt.Sprintf("%s_%s", system, id)
}

func writeTryjobsChangelistsAndPatchsets(ctx context.Context, db *pgx.Conn) error {
	for crs, cls := range data_kitchen_sink.MakeChangelists() {
		for _, cl := range cls {
			crsCLID := qualifyIDWithSystem(crs, cl.SystemID)
			_, err := db.Exec(ctx, `
INSERT INTO Changelists (changelist_id, system, status, owner, updated, subject) VALUES
  ($1, $2, $3, $4, $5, $6)`,
				crsCLID, crs, cl.Status, cl.Owner, cl.Updated, cl.Subject)
			if err != nil {
				return skerr.Wrapf(err, "inserting cl %v", cl)
			}
		}
	}

	for combinedID, ps := range data_kitchen_sink.MakePatchsets() {
		crsCLID := qualifyIDWithSystem(combinedID.CRS, combinedID.CL)
		crsPSID := qualifyIDWithSystem(combinedID.CRS, ps.SystemID)
		_, err := db.Exec(ctx, `
INSERT INTO Patchsets (patchset_id, system, changelist_id, ps_order, git_hash) VALUES
  ($1, $2, $3, $4, $5)`,
			crsPSID, combinedID.CRS, crsCLID, ps.Order, ps.GitHash)
		if err != nil {
			return skerr.Wrapf(err, "inserting ps %v", ps)
		}
	}

	for combinedID, tryjobs := range data_kitchen_sink.MakeTryjobs() {
		crsCLID := qualifyIDWithSystem(combinedID.CRS, combinedID.CL)
		crsPSID := qualifyIDWithSystem(combinedID.CRS, combinedID.PS)

		for _, tj := range tryjobs {
			cisTJID := qualifyIDWithSystem(tj.System, tj.SystemID)
			_, err := db.Exec(ctx, `
INSERT INTO Tryjobs (tryjob_id, system, changelist_id, patchset_id, display_name, updated) VALUES
  ($1, $2, $3, $4, $5, $6)`,
				cisTJID, tj.System, crsCLID, crsPSID, tj.DisplayName, tj.Updated)
			if err != nil {
				return skerr.Wrapf(err, "inserting tryjob %v", tj)
			}
		}

	}
	return nil
}

func writeTryjobData(ctx context.Context, db *pgx.Conn) error {
	const fakeFile = "gs://skia-gold-flutter/trybot/dm-json-v1/2020/07/10/07/d108734f19645c8eb443f83ef6af6cdda78a3024/5390858176430080/1594365990/dm-1594365990150477392.json"
	sourceFileHash := md5.Sum([]byte(fakeFile))

	for _, tryjobDataPoint := range data_kitchen_sink.MakeDataFromTryJobs() {
		keysJSON, traceHash, err := sql.SerializeMap(tryjobDataPoint.Keys)
		if err != nil {
			return skerr.Wrap(err)
		}
		optsJSON, optsHash, err := sql.SerializeMap(tryjobDataPoint.Options)
		if err != nil {
			return skerr.Wrap(err)
		}
		grouping := groupingFor(tryjobDataPoint.Keys)
		groupingJSON, groupingHash, err := sql.SerializeMap(grouping)
		if err != nil {
			return skerr.Wrap(err)
		}
		digestBytes, err := sql.DigestToBytes(tryjobDataPoint.Digest)
		if err != nil {
			return skerr.Wrap(err)
		}

		_, err = db.Exec(ctx,
			`INSERT INTO TryjobTraces (tryjob_trace_id, keys) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			traceHash, keysJSON)
		if err != nil {
			return skerr.Wrapf(err, "inserting keys %s - %s", traceHash, tryjobDataPoint.Keys)
		}

		_, err = db.Exec(ctx,
			`INSERT INTO Options (options_id, keys) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			optsHash, optsJSON)
		if err != nil {
			return skerr.Wrapf(err, "inserting options %s - %s", optsHash, tryjobDataPoint.Options)
		}

		_, err = db.Exec(ctx,
			`INSERT INTO Groupings (grouping_id, keys) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			groupingHash, groupingJSON)
		if err != nil {
			return skerr.Wrapf(err, "inserting grouping %s - %s", groupingHash, grouping)
		}

		_, err = db.Exec(ctx,
			`UPSERT INTO SourceFiles (source_file_id, source_file, last_ingested) VALUES ($1, $2, $3)`,
			sourceFileHash[:], fakeFile, time.Now())
		if err != nil {
			return skerr.Wrapf(err, "inserting source file %s - %s", sourceFileHash, fakeFile)
		}

		crsCLID := qualifyIDWithSystem(tryjobDataPoint.PatchSet.CRS, tryjobDataPoint.PatchSet.CL)
		crsPSID := qualifyIDWithSystem(tryjobDataPoint.PatchSet.CRS, tryjobDataPoint.PatchSet.PS)
		_, err = db.Exec(ctx,
			`UPSERT INTO TryJobValues (trace_id, tryjob_id, digest, grouping_id, options_id, 
         changelist_id, patchset_id, source_file_id)
       VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			traceHash, tryjobDataPoint.TryJobID, digestBytes, groupingHash, optsHash,
			crsCLID, crsPSID, sourceFileHash[:])
		if err != nil {
			return skerr.Wrapf(err, "inserting data for CL/PS %#v", tryjobDataPoint.PatchSet)
		}

		// This lets us index by untriaged status and not have to scan over all TraceValues.
		_, err = db.Exec(ctx, `
INSERT INTO ChangelistExpectations (changelist_id, grouping_id, digest, label)
VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
			crsCLID, groupingHash, digestBytes, sql.LabelUntriaged,
		)
		if err != nil {
			return skerr.Wrapf(err, "writing CL expectations (if needed) for %s", keysJSON)
		}

		err = writeTryjobParams(ctx, db, tryjobDataPoint.Keys, tryjobDataPoint.Options, crsCLID, crsPSID)
		if err != nil {
			return skerr.Wrapf(err, "inserting params for CL/PS %#v", tryjobDataPoint.PatchSet)
		}
	}
	return nil
}

func writeIgnoreRules(ctx context.Context, db *pgx.Conn) error {
	rules := data_kitchen_sink.MakeIgnoreRules()
	insert := `INSERT INTO IgnoreRules (created_user, updated_user, expires, note, query) VALUES `
	const valuesPerRow = 5
	placeholders, err := sql.ValuesPlaceholders(valuesPerRow, len(rules))
	if err != nil {
		return skerr.Wrap(err)
	}
	insert += placeholders

	arguments := make([]interface{}, 0, valuesPerRow*len(rules))
	for _, rule := range rules {
		queryJSONBytes, err := json.Marshal(rule.Query)
		if err != nil {
			return skerr.Wrap(err)
		}
		arguments = append(arguments, rule.CreatedBy, rule.UpdatedBy, rule.Expires, rule.Note, string(queryJSONBytes))
	}
	_, err = db.Exec(ctx, insert, arguments...)
	if err != nil {
		return skerr.Wrapf(err, "storing ignore rules")
	}

	return skerr.Wrap(updateIgnoredTraces(ctx, db))
}

func updateIgnoredTraces(ctx context.Context, db *pgx.Conn) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	// Rollback is safe to call even if the tx is already closed, so if
	// the tx commits successfully, this is a no-op
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, "SELECT query FROM IgnoreRules;")
	if err != nil {
		return skerr.Wrap(err)
	}
	var ignoreRules []paramtools.ParamSet
	for rows.Next() {
		rule := paramtools.ParamSet{}
		err := rows.Scan(&rule)
		if err != nil {
			return skerr.Wrap(err)
		}
		ignoreRules = append(ignoreRules, rule)
	}
	sklog.Infof("SELECTED rules: %s", ignoreRules)

	condition, arguments := sql.ConvertIgnoreRules(ignoreRules)

	statement := `UPDATE Traces SET matches_any_ignore_rule = `
	statement += condition
	statement += `WHERE matches_any_ignore_rule IS NULL;`
	_, err = tx.Exec(ctx, statement, arguments...)
	if err != nil {
		return skerr.Wrapf(err, "Updating with statement: %s", statement)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return skerr.Wrapf(err, "updating traces ignored status")
	}
	return nil
}
