package sql

//go:generate bazelisk run --config=mayberemote //:go -- run ./tosql

import (
	alertschema "go.skia.org/infra/perf/go/alerts/sqlalertstore/schema"
	gitschema "go.skia.org/infra/perf/go/git/schema"
	regressionschema "go.skia.org/infra/perf/go/regression/sqlregressionstore/schema"
	shortcutschema "go.skia.org/infra/perf/go/shortcut/sqlshortcutstore/schema"
	traceschema "go.skia.org/infra/perf/go/tracestore/sqltracestore/schema"
)

// Tables represents the full schema of the SQL database.
type Tables struct {
	Alerts      []alertschema.AlertSchema
	Commits     []gitschema.Commit
	ParamSets   []traceschema.ParamSetsSchema
	Postings    []traceschema.PostingsSchema
	Regressions []regressionschema.RegressionSchema
	Shortcuts   []shortcutschema.ShortcutSchema
	SourceFiles []traceschema.SourceFilesSchema
	TraceValues []traceschema.TraceValuesSchema
}
