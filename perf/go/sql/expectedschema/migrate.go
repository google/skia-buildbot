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

// The two vars below should be updated everytime there's a schema change.
var FromLiveToNext = `
	CREATE INDEX IF NOT EXISTS by_key_value
	ON postings	(tile_number, key_value);
`

var FromNextToLive = `
	DROP INDEX IF EXISTS postings@by_key_value;
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
