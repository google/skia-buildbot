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
	"bytes"
	"context"
	"slices"
	"text/template"

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
var FromLiveToNextSpanner = `
	CREATE INDEX idx_alerts_subname ON Alerts(sub_name);
`

// ONLY DROP TABLE IF YOU JUST CREATED A NEW TABLE.
// FOR MODIFYING COLUMNS USE ADD/DROP COLUMN INSTEAD.
var FromNextToLiveSpanner = `
	DROP INDEX IF EXISTS idx_alerts_subname;
`

// This function will check whether there's a new schema checked-in,
// and if so, migrate the schema in the given Spanner instance.
func ValidateAndMigrateNewSchema(ctx context.Context, db pool.Pool) error {
	sklog.Debugf("Starting validate and migrate.")
	next, err := Load()
	if err != nil {
		return skerr.Wrap(err)
	}

	prev, err := LoadPrev()
	if err != nil {
		return skerr.Wrap(err)
	}

	actual, err := schema.GetDescription(ctx, db, sql.Tables{}, "spanner")
	if err != nil {
		return skerr.Wrap(err)
	}

	diffPrevActual := assertdeep.Diff(prev, *actual)
	diffNextActual := assertdeep.Diff(next, *actual)

	if diffNextActual != "" && diffPrevActual == "" {
		sklog.Debugf("Next is different from live schema. Will migrate. diffNextActual: %s", diffNextActual)

		_, err = db.Exec(ctx, FromLiveToNextSpanner)
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

func Difference(as []string, bs []string) ([]string, []string) {
	var aNotB, bNotA []string
	for _, a := range as {
		if !slices.Contains(bs, a) {
			aNotB = append(aNotB, a)
		}
	}
	for _, b := range bs {
		if !slices.Contains(as, b) {
			bNotA = append(bNotA, b)
		}
	}
	return aNotB, bNotA
}

type TraceParamsUpdateContext struct {
	AddCols  []string
	DropCols []string
	AddIdxs  []string
	DropIdxs []string
}

var traceParamsUpdateTemplate = `
{{ range $i, $idx := .DropIdxs -}}
DROP INDEX {{ $idx }};
{{ end -}}

{{ range $i, $col := .DropCols -}}
ALTER TABLE traceparams
	DROP COLUMN {{ $col }};
{{ end -}}

{{ range $i, $col := .AddCols -}}
ALTER TABLE traceparams
	ADD COLUMN {{ $col }} character varying GENERATED ALWAYS AS (((params ->> '{{ $col }}'::text))::character varying) VIRTUAL;
{{ end -}}

{{ range $i, $idx := .AddIdxs -}}
CREATE INDEX idx_traceparams_{{ $idx }} ON traceparams ({{ $idx }});
{{ end -}}
 `

// This function updates the traceparams table schema so that it has generated columns
// for each param key in the paramsets table (last 2 tiles) and indexes for each column
// named in the instance config, and will drop any surplus columns/indexes it finds in the
// schema.
func UpdateTraceParamsSchema(ctx context.Context, db pool.Pool, datastoreType config.DataStoreType, traceParamsIndexes []string) error {
	sklog.Debugf("Starting update traceparams schema. DatastoreType: %s", datastoreType)

	requiredCols, err := GetParams(ctx, db, string(datastoreType))
	if err != nil {
		return skerr.Wrap(err)
	}
	requiredIdxs := traceParamsIndexes
	actualCols, actualIdxs, err := GetTraceParamsGeneratedColsAndIdxs(ctx, db, string(datastoreType))
	if err != nil {
		return skerr.Wrap(err)
	}
	extraCols, missingCols := Difference(actualCols, requiredCols)
	extraIdxs, missingIdxs := Difference(actualIdxs, requiredIdxs)
	if len(extraCols) == 0 && len(missingCols) == 0 &&
		len(extraIdxs) == 0 && len(missingIdxs) == 0 {
		sklog.Debugf("No changes needed for traceparams generated columns/indexes.")
		return nil
	}
	context := TraceParamsUpdateContext{
		AddCols: missingCols, DropCols: extraCols,
		AddIdxs: missingIdxs, DropIdxs: extraIdxs}
	t, err := template.New("").Parse(traceParamsUpdateTemplate)
	if err != nil {
		return skerr.Wrapf(err, "parsing template %q", traceParamsUpdateTemplate)
	}
	var b bytes.Buffer
	if err := t.Execute(&b, context); err != nil {
		sklog.Errorf("Failed to execute template=%v, context=%v.", t, context)
		return skerr.Wrapf(err, "Failed to migrate Schema")
	}
	traceParamsUpdateStatement := b.String()
	_, err = db.Exec(ctx, traceParamsUpdateStatement)
	if err != nil {
		sklog.Errorf("Failed to update traceparams schema (statement='%s')", traceParamsUpdateStatement)
		return skerr.Wrapf(err, "Failed to migrate Schema")
	}

	return nil
}
