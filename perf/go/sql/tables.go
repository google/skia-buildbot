package sql

//go:generate bazelisk run --config=mayberemote //:go -- run ./tosql

import (
	alertschema "go.skia.org/infra/perf/go/alerts/sqlalertstore/schema"
	anomalygroupschema "go.skia.org/infra/perf/go/anomalygroup/sqlanomalygroupstore/schema"
	culpritschema "go.skia.org/infra/perf/go/culprit/sqlculpritstore/schema"
	gitschema "go.skia.org/infra/perf/go/git/schema"
	graphsshortcutschema "go.skia.org/infra/perf/go/graphsshortcut/graphsshortcutstore/schema"
	regression2schema "go.skia.org/infra/perf/go/regression/sqlregression2store/schema"
	regressionschema "go.skia.org/infra/perf/go/regression/sqlregressionstore/schema"
	shortcutschema "go.skia.org/infra/perf/go/shortcut/sqlshortcutstore/schema"
	subscriptionschema "go.skia.org/infra/perf/go/subscription/sqlsubscriptionstore/schema"
	traceschema "go.skia.org/infra/perf/go/tracestore/sqltracestore/schema"
)

// Tables represents the full schema of the SQL database.
type Tables struct {
	Alerts          []alertschema.AlertSchema
	AnomalyGroups   []anomalygroupschema.AnomalyGroupSchema
	Commits         []gitschema.Commit
	Culprits        []culpritschema.CulpritSchema
	GraphsShortcuts []graphsshortcutschema.GraphsShortcutSchema
	ParamSets       []traceschema.ParamSetsSchema
	Postings        []traceschema.PostingsSchema
	Regressions     []regressionschema.RegressionSchema
	Regressions2    []regression2schema.Regression2Schema
	Shortcuts       []shortcutschema.ShortcutSchema
	SourceFiles     []traceschema.SourceFilesSchema
	Subscriptions   []subscriptionschema.SubscriptionSchema
	TraceValues     []traceschema.TraceValuesSchema
}
