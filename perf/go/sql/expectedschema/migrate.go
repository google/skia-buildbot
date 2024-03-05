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
var FromLiveToNext = `
	DROP TABLE IF EXISTS AnomalyGroups;
	CREATE TABLE IF NOT EXISTS AnomalyGroups (
	  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	  creation_time TIMESTAMPTZ DEFAULT now(),
	  anomalies JSONB,
	  group_meta_data JSONB,
	  common_rev_start INT,
	  common_rev_end INT,
	  action TEXT,
	  action_time TIMESTAMPTZ,
	  bisection_id TEXT,
	  reported_issue_id TEXT,
	  culprit_ids UUID ARRAY,
	  last_modified_time TIMESTAMPTZ
	);
`

var FromNextToLive = `
	DROP TABLE IF EXISTS AnomalyGroups;
	CREATE TABLE IF NOT EXISTS AnomalyGroups (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		action TEXT,
		action_time TIMESTAMPTZ,
		bisection_id TEXT,
		reported_issue_id TEXT,
		anomalies JSONB,
		creation_time TIMESTAMPTZ DEFAULT now(),
		culprit_ids UUID ARRAY,
		common_rev_start INT,
		common_rev_end INT,
		last_modified_time TIMESTAMPTZ,
		subscription_name TEXT
	);
`

// This function will check whether there's a new schema checked-in,
// and if so, migrate the schema in the given CockroachDB instance.
func ValidateAndMigrateNewSchema(ctx context.Context, db pool.Pool) error {
	next, err := Load()
	if err != nil {
		return skerr.Wrap(err)
	}

	prev, err := LoadPrev()
	if err != nil {
		return skerr.Wrap(err)
	}

	actual, err := schema.GetDescription(ctx, db, sql.Tables{})
	if err != nil {
		return skerr.Wrap(err)
	}

	diffPrevActual := assertdeep.Diff(prev, *actual)
	diffNextActual := assertdeep.Diff(next, *actual)

	if diffNextActual != "" && diffPrevActual == "" {
		_, err = db.Exec(ctx, FromLiveToNext)
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
