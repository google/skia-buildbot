package sql_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/sql/schema"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/sql/expectedschema"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

const DropTables = `
	DROP TABLE IF EXISTS Alerts;
	DROP TABLE IF EXISTS AnomalyGroups;
	DROP TABLE IF EXISTS Commits;
	DROP TABLE IF EXISTS Culprits;
	DROP TABLE IF EXISTS Favorites;
	DROP TABLE IF EXISTS GraphsShortcuts;
  DROP TABLE IF EXISTS Metadata;
	DROP TABLE IF EXISTS ParamSets;
	DROP TABLE IF EXISTS Postings;
	DROP TABLE IF EXISTS Regressions;
	DROP TABLE IF EXISTS Regressions2;
	DROP TABLE IF EXISTS ReverseKeyMap;
	DROP TABLE IF EXISTS Shortcuts;
	DROP TABLE IF EXISTS SourceFiles;
	DROP TABLE IF EXISTS Subscriptions;
  DROP TABLE IF EXISTS TraceParams;
	DROP TABLE IF EXISTS TraceValues;
	DROP TABLE IF EXISTS TraceValues2;
	DROP TABLE IF EXISTS UserIssues;
`

const DropSpannerIndices = `
  DROP INDEX IF EXISTS by_revision;
  DROP INDEX IF EXISTS by_user_id;
  DROP INDEX IF EXISTS by_tile_number;
  DROP INDEX IF EXISTS by_trace_id;
  DROP INDEX IF EXISTS by_trace_id2;
  DROP INDEX IF EXISTS by_key_value;
  DROP INDEX IF EXISTS by_alert_id;
  DROP INDEX IF EXISTS by_commit_alert;
  DROP INDEX IF EXISTS by_source_file;
  DROP INDEX IF EXISTS by_source_file_id;
  DROP INDEX IF EXISTS by_trace_id_tv2;
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
CREATE TABLE IF NOT EXISTS Metadata (
  source_file_id INT PRIMARY KEY,
  links JSONB
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
  INDEX by_trace_id2 (tile_number, trace_id),
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
CREATE TABLE IF NOT EXISTS ReverseKeyMap (
  modified_value TEXT,
  param_key TEXT,
  original_value TEXT,
  PRIMARY KEY(modified_value, param_key)
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
  name STRING NOT NULL,
  revision STRING NOT NULL,
  bug_labels STRING ARRAY,
  hotlists STRING ARRAY,
  bug_component STRING,
  bug_priority INT,
  bug_severity INT,
  bug_cc_emails STRING ARRAY,
  contact_email STRING,
  is_active BOOL,
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
CREATE TABLE IF NOT EXISTS UserIssues (
  user_id TEXT NOT NULL,
  trace_key TEXT NOT NULL,
  commit_position INT NOT NULL,
  issue_id INT NOT NULL,
  last_modified TIMESTAMPTZ DEFAULT now(),
  PRIMARY KEY(trace_key, commit_position)
);
  `

// LiveSchemaSpanner has to reflect what's live in prod right now in spanner
const LiveSchemaSpanner = `
CREATE SEQUENCE IF NOT EXISTS Alerts_seq bit_reversed_positive;
CREATE SEQUENCE IF NOT EXISTS SourceFiles_seq bit_reversed_positive;
CREATE TABLE IF NOT EXISTS Alerts (
  id INT DEFAULT nextval('Alerts_seq'),
  alert TEXT,
  config_state INT DEFAULT 0,
  last_modified INT,
  sub_name TEXT,
  sub_revision TEXT,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id)
);
CREATE TABLE IF NOT EXISTS AnomalyGroups (
  id TEXT PRIMARY KEY DEFAULT spanner.generate_uuid(),
  creation_time TIMESTAMPTZ DEFAULT now(),
  anomaly_ids TEXT ARRAY,
  group_meta_data JSONB,
  common_rev_start INT,
  common_rev_end INT,
  action TEXT,
  action_time TIMESTAMPTZ,
  bisection_id TEXT,
  reported_issue_id TEXT,
  culprit_ids TEXT ARRAY,
  last_modified_time TIMESTAMPTZ,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS Commits (
  commit_number INT PRIMARY KEY,
  git_hash TEXT  NOT NULL,
  commit_time INT,
  author TEXT,
  subject TEXT,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS Culprits (
  id TEXT PRIMARY KEY DEFAULT spanner.generate_uuid(),
  host TEXT,
  project TEXT,
  ref TEXT,
  revision TEXT,
  last_modified INT,
  anomaly_group_ids TEXT ARRAY,
  issue_ids TEXT ARRAY,
  group_issue_map JSONB,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS Favorites (
  id TEXT PRIMARY KEY DEFAULT spanner.generate_uuid(),
  user_id TEXT NOT NULL,
  name TEXT,
  url TEXT NOT NULL,
  description TEXT,
  last_modified INT,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS GraphsShortcuts (
  id TEXT  NOT NULL PRIMARY KEY,
  graphs TEXT,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS Metadata (
  source_file_id INT PRIMARY KEY,
  links JSONB,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS ParamSets (
  tile_number INT,
  param_key TEXT,
  param_value TEXT,
  PRIMARY KEY (tile_number, param_key, param_value),
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS Postings (
  tile_number INT,
  key_value TEXT NOT NULL,
  trace_id BYTEA,
  PRIMARY KEY (tile_number, key_value, trace_id),
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS Regressions (
  commit_number INT,
  alert_id INT,
  regression TEXT,
  migrated BOOL,
  regression_id TEXT,
  PRIMARY KEY (commit_number, alert_id),
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS Regressions2 (
  id TEXT PRIMARY KEY DEFAULT spanner.generate_uuid(),
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
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS ReverseKeyMap (
  modified_value TEXT,
  param_key TEXT,
  original_value TEXT,
  PRIMARY KEY(modified_value, param_key),
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS Shortcuts (
  id TEXT  NOT NULL PRIMARY KEY,
  trace_ids TEXT,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS SourceFiles (
  source_file_id INT DEFAULT nextval('SourceFiles_seq'),
  source_file TEXT  NOT NULL,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (source_file_id)
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS Subscriptions (
  name TEXT NOT NULL,
  revision TEXT NOT NULL,
  bug_labels TEXT ARRAY,
  hotlists TEXT ARRAY,
  bug_component TEXT,
  bug_priority INT,
  bug_severity INT,
  bug_cc_emails TEXT ARRAY,
  contact_email TEXT,
  is_active BOOL,
  PRIMARY KEY(name, revision),
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS TraceValues (
  trace_id BYTEA,
  commit_number INT,
  val REAL,
  source_file_id INT,
  PRIMARY KEY (trace_id, commit_number),
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
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
CREATE TABLE IF NOT EXISTS UserIssues (
  user_id TEXT NOT NULL,
  trace_key TEXT NOT NULL,
  commit_position INT NOT NULL,
  issue_id INT NOT NULL,
  last_modified TIMESTAMPTZ DEFAULT now(),
  PRIMARY KEY(trace_key, commit_position),
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE INDEX IF NOT EXISTS by_revision on Culprits (revision, host, project, ref);
CREATE INDEX IF NOT EXISTS by_user_id on Favorites (user_id);
CREATE INDEX IF NOT EXISTS by_tile_number on ParamSets (tile_number DESC);
CREATE INDEX IF NOT EXISTS by_trace_id on Postings (tile_number, trace_id, key_value);
CREATE INDEX IF NOT EXISTS by_trace_id2 on Postings (tile_number, trace_id);
CREATE INDEX IF NOT EXISTS by_key_value on Postings (tile_number, key_value);
CREATE INDEX IF NOT EXISTS by_alert_id on Regressions2 (alert_id);
CREATE INDEX IF NOT EXISTS by_commit_alert on Regressions2 (commit_number, alert_id);
CREATE INDEX IF NOT EXISTS by_source_file on SourceFiles (source_file, source_file_id);
CREATE INDEX IF NOT EXISTS by_source_file_id on TraceValues (source_file_id, trace_id);
CREATE INDEX IF NOT EXISTS by_trace_id_tv2 on TraceValues2 (trace_id, benchmark, bot, test, subtest_1, subtest_2, subtest_3);
`

func getSchema(t *testing.T, db pool.Pool, dbtype config.DataStoreType) *schema.Description {
	ret, err := schema.GetDescription(context.Background(), db, sql.Tables{}, string(dbtype))
	require.NoError(t, err)
	require.NotEmpty(t, ret.ColumnNameAndType)
	return ret
}

func Test_LiveToNextSchemaMigration(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(t, "desc")

	expectedSchema := getSchema(t, db, config.CockroachDBDataStoreType)

	_, err := db.Exec(ctx, DropTables)
	require.NoError(t, err)
	_, err = db.Exec(ctx, LiveSchema)
	require.NoError(t, err)
	_, err = db.Exec(ctx, expectedschema.FromLiveToNext)
	require.NoError(t, err)

	migratedSchema := getSchema(t, db, config.CockroachDBDataStoreType)
	assertdeep.Equal(t, expectedSchema, migratedSchema)
	// Test the test, make sure at least one known column is present.
	require.Equal(t, "text def: nullable:NO", migratedSchema.ColumnNameAndType["graphsshortcuts.id"])
}

func Test_LiveToNextSchemaMigration_Spanner(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewSpannerDBForTests(t, "desc")

	expectedSchema := getSchema(t, db, config.SpannerDataStoreType)

	_, err := db.Exec(ctx, DropSpannerIndices)
	require.NoError(t, err)
	_, err = db.Exec(ctx, DropTables)
	require.NoError(t, err)
	_, err = db.Exec(ctx, LiveSchemaSpanner)
	require.NoError(t, err)
	_, err = db.Exec(ctx, expectedschema.FromLiveToNextSpanner)
	require.NoError(t, err)

	migratedSchema := getSchema(t, db, config.SpannerDataStoreType)
	assertdeep.Equal(t, expectedSchema, migratedSchema)
	// Test the test, make sure at least one known column is present.
	require.Equal(t, "character varying def: nullable:NO", migratedSchema.ColumnNameAndType["graphsshortcuts.id"])
}
