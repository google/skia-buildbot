// This file contains useful logic for maintenance tasks to migrate new schema
// changes.

package expectedschema

import (
	"context"

	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/sql/schema"
	"go.skia.org/infra/golden/go/config"
	golden_schema "go.skia.org/infra/golden/go/sql/schema"
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
`

// Same as above, but will be used when doing schema migration for spanner databases.
// Some statements can be different for CDB v/s Spanner, hence splitting into
// separate variables.
var FromLiveToNextSpanner = `
`

// ONLY DROP TABLE IF YOU JUST CREATED A NEW TABLE.
// FOR MODIFYING COLUMNS USE ADD/DROP COLUMN INSTEAD.
var FromNextToLive = `
`

// ONLY DROP TABLE IF YOU JUST CREATED A NEW TABLE.
// FOR MODIFYING COLUMNS USE ADD/DROP COLUMN INSTEAD.
var FromNextToLiveSpanner = `
`

// This function will check whether there's a new schema checked-in,
// and if so, migrate the schema in the given CockroachDB instance.
func ValidateAndMigrateNewSchema(ctx context.Context, db pool.Pool, datastoreType config.DatabaseType) error {
	sklog.Debugf("Starting validate and migrate. DatastoreType: %s", datastoreType)
	next, err := Load(datastoreType)
	if err != nil {
		return skerr.Wrap(err)
	}
	sklog.Debugf("Next schema: %s", next)
	prev, err := LoadPrev(datastoreType)
	if err != nil {
		return skerr.Wrap(err)
	}
	sklog.Debugf("Prev schema: %s", prev)
	actual, err := schema.GetDescription(ctx, db, golden_schema.Tables{}, string(datastoreType))
	if err != nil {
		return skerr.Wrap(err)
	}

	diffPrevActual := assertdeep.Diff(prev, *actual)
	diffNextActual := assertdeep.Diff(next, *actual)
	sklog.Debugf("Diff prev vs actual: %s", diffPrevActual)
	sklog.Debugf("Diff next vs actual: %s", diffNextActual)

	if diffNextActual != "" && diffPrevActual == "" {
		sklog.Debugf("Next is different from live schema. Will migrate. diffNextActual: %s", diffNextActual)
		fromLiveToNextStmt := FromLiveToNext
		if datastoreType == config.Spanner {
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
