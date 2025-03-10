// This file contains useful logic for maintenance tasks to migrate new schema
// changes.
//
// Maintenance tasks (see //perf/go/maintenance/maintenance.go) use the same
// executable as the perf frontend and ingesters, so when Louhi does an update
// all of them will be deployed at the same time. Since frontend and ingesters
// check for a correct schema they will panic on startup, so the old
// instances of those apps will continue to run.
//
// The maintenance task will run and thus will upgrade the schema in the
// database. After that completes when frontend and ingesters are retried
// (k8s does that automatically), then they will start successfully. This
// means that any change in schema must be compatible with both the current
// and previous version of the perfserver executable.
package expectedschema

import (
	"context"

	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/sql/schema"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/sql"
)

// The two vars below should be updated everytime there's a schema change:
//   - FromLiveToNext tells the SQL to execute to apply the change
//   - FromNextToLive tells the SQL to revert the change
//
// Also we need to update LiveSchema schema and DropTables in sql_test.go:
//   - DropTables deletes all tables *including* the new one in the change.
//   - LiveSchema creates all existing tables *without* the new one in the
//     change.
//
// DO NOT DROP TABLES IN VAR BELOW.
// FOR MODIFYING COLUMNS USE ADD/DROP COLUMN INSTEAD.
var FromLiveToNext = `
	CREATE TABLE IF NOT EXISTS TraceValues2 (
		trace_id BYTES,
		commit_number INT,
		val REAL,
		source_file_id INT,
		benchmark STRING,
		bot STRING,
		test STRING,
		subtest_1 STRING,
		subtest_2 STRING,
		subtest_3 STRING,
		PRIMARY KEY (trace_id, commit_number),
		INDEX by_trace_id_tv2 (trace_id, benchmark, bot, test, subtest_1, subtest_2, subtest_3)
  	);
`

// Same as above, but will be used when doing schema migration for spanner databases.
// Some statements can be different for CDB v/s Spanner, hence splitting into
// separate variables.
var FromLiveToNextSpanner = `
	CREATE TABLE IF NOT EXISTS TraceValues2 (
		trace_id BYTEA,
		commit_number INT,
		val REAL,
		source_file_id INT,
		benchmark TEXT,
		bot TEXT,
		test TEXT,
		subtest_1 TEXT,
		subtest_2 TEXT,
		subtest_3 TEXT,
		PRIMARY KEY (trace_id, commit_number),
		createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	) TTL INTERVAL '1095 days' ON createdat;
	CREATE INDEX IF NOT EXISTS by_trace_id_tv2 on TraceValues2 (trace_id, benchmark, bot, test, subtest_1, subtest_2, subtest_3);
`

// ONLY DROP TABLE IF YOU JUST CREATED A NEW TABLE.
// FOR MODIFYING COLUMNS USE ADD/DROP COLUMN INSTEAD.
var FromNextToLive = `
	DROP INDEX IF EXISTS by_trace_id_tv2;
	DROP TABLE IF EXISTS TraceValues2;
`

// This function will check whether there's a new schema checked-in,
// and if so, migrate the schema in the given CockroachDB instance.
func ValidateAndMigrateNewSchema(ctx context.Context, db pool.Pool, datastoreType config.DataStoreType) error {
	sklog.Debugf("Starting validate and migrate. DatastoreType: %s", datastoreType)
	next, err := Load(datastoreType)
	if err != nil {
		return skerr.Wrap(err)
	}

	prev, err := LoadPrev(datastoreType)
	if err != nil {
		return skerr.Wrap(err)
	}

	actual, err := schema.GetDescription(ctx, db, sql.Tables{}, string(datastoreType))
	if err != nil {
		return skerr.Wrap(err)
	}

	diffPrevActual := assertdeep.Diff(prev, *actual)
	diffNextActual := assertdeep.Diff(next, *actual)

	if diffNextActual != "" && diffPrevActual == "" {
		sklog.Debugf("Next is different from live schema. Will migrate. diffNextActual: %s", diffNextActual)
		fromLiveToNextStmt := FromLiveToNext
		if datastoreType == config.SpannerDataStoreType {
			fromLiveToNextStmt = FromLiveToNextSpanner
		}
		_, err = db.Exec(ctx, fromLiveToNextStmt)
		if err != nil {
			sklog.Errorf("Failed to migrate Schema from prev to next. Prev: %s, Next: %s.", prev, next)
			return skerr.Wrapf(err, "Failed to migrate Schema")
		}
	} else if diffNextActual != "" && diffPrevActual != "" {
		sklog.Errorf("Live schema doesn't match next or previous checked-in schema. diffNextActual: %s, diffPrevActual: %s.", diffNextActual, diffPrevActual)
		return skerr.Fmt("Live schema doesn't match next or previous checked-in schema.")
	}

	return nil
}
