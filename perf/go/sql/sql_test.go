package sql_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/sql/schema"
	"go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/sql/expectedschema"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

const DropTables = `
	DROP TABLE IF EXISTS Alerts;
	DROP TABLE IF EXISTS AnomalyGroups;
	DROP TABLE IF EXISTS Commits;
	DROP TABLE IF EXISTS Culprits;
	DROP TABLE IF EXISTS GraphsShortcuts;
	DROP TABLE IF EXISTS ParamSets;
	DROP TABLE IF EXISTS Postings;
	DROP TABLE IF EXISTS Regressions;
	DROP TABLE IF EXISTS Regressions2;
	DROP TABLE IF EXISTS Shortcuts;
	DROP TABLE IF EXISTS SourceFiles;
	DROP TABLE IF EXISTS Subscriptions;
	DROP TABLE IF EXISTS TraceValues;
`

// LiveSchema has to reflect what's live in prod right now
const LiveSchema = `
CREATE TABLE IF NOT EXISTS Alerts (
	id INT PRIMARY KEY DEFAULT unique_rowid(),
	alert TEXT,
	config_state INT DEFAULT 0,
	last_modified INT,
	sub_name STRING,
	sub_revision STRING
  );
  CREATE TABLE IF NOT EXISTS AnomalyGroups (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	creation_time TIMESTAMPTZ DEFAULT now(),
	anomaly_ids UUID ARRAY,
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
  CREATE TABLE IF NOT EXISTS Commits (
	commit_number INT PRIMARY KEY,
	git_hash TEXT UNIQUE NOT NULL,
	commit_time INT,
	author TEXT,
	subject TEXT
  );
  CREATE TABLE IF NOT EXISTS Culprits (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	host STRING,
	project STRING,
	ref STRING,
	revision STRING,
	last_modified INT,
	anomaly_group_ids STRING ARRAY,
	issue_ids STRING ARRAY,
	group_issue_map JSONB,
	UNIQUE INDEX by_revision (revision, host, project, ref)
  );
  CREATE TABLE IF NOT EXISTS Favorites (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	user_id STRING NOT NULL,
	name STRING,
	url STRING NOT NULL,
	description STRING,
	last_modified INT,
	INDEX by_user_id (user_id)
  );
  CREATE TABLE IF NOT EXISTS GraphsShortcuts (
	id TEXT UNIQUE NOT NULL PRIMARY KEY,
	graphs TEXT
  );
  CREATE TABLE IF NOT EXISTS ParamSets (
	tile_number INT,
	param_key STRING,
	param_value STRING,
	PRIMARY KEY (tile_number, param_key, param_value),
	INDEX by_tile_number (tile_number DESC)
  );
  CREATE TABLE IF NOT EXISTS Postings (
	tile_number INT,
	key_value STRING NOT NULL,
	trace_id BYTES,
	PRIMARY KEY (tile_number, key_value, trace_id),
	INDEX by_trace_id (tile_number, trace_id, key_value),
	INDEX by_key_value (tile_number, key_value)
  );
  CREATE TABLE IF NOT EXISTS Regressions (
	commit_number INT,
	alert_id INT,
	regression TEXT,
	migrated BOOL,
	regression_id TEXT,
	PRIMARY KEY (commit_number, alert_id)
  );
  CREATE TABLE IF NOT EXISTS Regressions2 (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	commit_number INT,
	prev_commit_number INT,
	alert_id INT,
	creation_time TIMESTAMPTZ DEFAULT now(),
	median_before REAL,
	median_after REAL,
	is_improvement BOOL,
	cluster_type TEXT,
	cluster_summary JSONB,
	frame JSONB,
	triage_status TEXT,
	triage_message TEXT,
	INDEX by_alert_id (alert_id),
	INDEX by_commit_alert (commit_number, alert_id)
  );
  CREATE TABLE IF NOT EXISTS Shortcuts (
	id TEXT UNIQUE NOT NULL PRIMARY KEY,
	trace_ids TEXT
  );
  CREATE TABLE IF NOT EXISTS SourceFiles (
	source_file_id INT PRIMARY KEY DEFAULT unique_rowid(),
	source_file STRING UNIQUE NOT NULL,
	INDEX by_source_file (source_file, source_file_id)
  );
  CREATE TABLE IF NOT EXISTS Subscriptions (
	name STRING UNIQUE NOT NULL,
	revision STRING NOT NULL,
	bug_labels STRING ARRAY,
	hotlists STRING ARRAY,
	bug_component STRING,
	bug_priority INT,
	bug_severity INT,
	bug_cc_emails STRING ARRAY,
	contact_email STRING,
	PRIMARY KEY(name, revision)
  );
  CREATE TABLE IF NOT EXISTS TraceValues (
	trace_id BYTES,
	commit_number INT,
	val REAL,
	source_file_id INT,
	PRIMARY KEY (trace_id, commit_number),
	INDEX by_source_file_id (source_file_id, trace_id)
  );
  `

func getSchema(t *testing.T, db pool.Pool) *schema.Description {
	ret, err := schema.GetDescription(context.Background(), db, sql.Tables{})
	require.NoError(t, err)
	require.NotEmpty(t, ret.ColumnNameAndType)
	return ret
}

func Test_LiveToNextSchemaMigration(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(t, "desc")

	expectedSchema := getSchema(t, db)

	_, err := db.Exec(ctx, DropTables)
	require.NoError(t, err)
	_, err = db.Exec(ctx, LiveSchema)
	require.NoError(t, err)
	_, err = db.Exec(ctx, expectedschema.FromLiveToNext)
	require.NoError(t, err)

	migratedSchema := getSchema(t, db)
	assertdeep.Equal(t, expectedSchema, migratedSchema)
	// Test the test, make sure at least one known column is present.
	require.Equal(t, "text def: nullable:NO", migratedSchema.ColumnNameAndType["graphsshortcuts.id"])
}
