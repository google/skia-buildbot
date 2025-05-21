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
var FromLiveToNext = `CREATE TABLE IF NOT EXISTS Changelists (
  changelist_id STRING PRIMARY KEY,
  system STRING NOT NULL,
  status STRING NOT NULL,
  owner_email STRING NOT NULL,
  subject STRING NOT NULL,
  last_ingested_data TIMESTAMP WITH TIME ZONE NOT NULL,
  INDEX system_status_ingested_idx (system, status, last_ingested_data),
  INDEX status_ingested_idx (status, last_ingested_data DESC)
);
CREATE TABLE IF NOT EXISTS CommitsWithData (
  commit_id STRING PRIMARY KEY,
  tile_id INT4 NOT NULL
);
CREATE TABLE IF NOT EXISTS DiffMetrics (
  left_digest BYTES,
  right_digest BYTES,
  num_pixels_diff INT4 NOT NULL,
  percent_pixels_diff FLOAT4 NOT NULL,
  max_rgba_diffs INT2[] NOT NULL,
  max_channel_diff INT2 NOT NULL,
  combined_metric FLOAT4 NOT NULL,
  dimensions_differ BOOL NOT NULL,
  ts TIMESTAMP WITH TIME ZONE NOT NULL,
  PRIMARY KEY (left_digest, right_digest)
);
CREATE TABLE IF NOT EXISTS ExpectationDeltas (
  expectation_record_id UUID,
  grouping_id BYTES,
  digest BYTES,
  label_before CHAR NOT NULL,
  label_after CHAR NOT NULL,
  PRIMARY KEY (expectation_record_id, grouping_id, digest)
);
CREATE TABLE IF NOT EXISTS ExpectationRecords (
  expectation_record_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  branch_name STRING,
  user_name STRING NOT NULL,
  triage_time TIMESTAMP WITH TIME ZONE NOT NULL,
  num_changes INT4 NOT NULL,
  INDEX branch_ts_idx (branch_name, triage_time)
);
CREATE TABLE IF NOT EXISTS Expectations (
  grouping_id BYTES,
  digest BYTES,
  label CHAR NOT NULL,
  expectation_record_id UUID,
  PRIMARY KEY (grouping_id, digest),
  INDEX label_idx (label)
);
CREATE TABLE IF NOT EXISTS GitCommits (
  git_hash STRING PRIMARY KEY,
  commit_id STRING NOT NULL,
  commit_time TIMESTAMP WITH TIME ZONE NOT NULL,
  author_email STRING NOT NULL,
  subject STRING NOT NULL,
  INDEX commit_idx (commit_id)
);
CREATE TABLE IF NOT EXISTS Groupings (
  grouping_id BYTES PRIMARY KEY,
  keys JSONB NOT NULL
);
CREATE TABLE IF NOT EXISTS IgnoreRules (
  ignore_rule_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  creator_email STRING NOT NULL,
  updated_email STRING NOT NULL,
  expires TIMESTAMP WITH TIME ZONE NOT NULL,
  note STRING,
  query JSONB NOT NULL
);
CREATE TABLE IF NOT EXISTS MetadataCommits (
  commit_id STRING PRIMARY KEY,
  commit_metadata STRING NOT NULL
);
CREATE TABLE IF NOT EXISTS Options (
  options_id BYTES PRIMARY KEY,
  keys JSONB NOT NULL
);
CREATE TABLE IF NOT EXISTS Patchsets (
  patchset_id STRING PRIMARY KEY,
  system STRING NOT NULL,
  changelist_id STRING NOT NULL REFERENCES Changelists (changelist_id),
  ps_order INT2 NOT NULL,
  git_hash STRING NOT NULL,
  commented_on_cl BOOL NOT NULL,
  created_ts TIMESTAMP WITH TIME ZONE,
  INDEX cl_order_idx (changelist_id, ps_order)
);
CREATE TABLE IF NOT EXISTS PrimaryBranchDiffCalculationWork (
  grouping_id BYTES PRIMARY KEY,
  last_calculated_ts TIMESTAMP WITH TIME ZONE NOT NULL,
  calculation_lease_ends TIMESTAMP WITH TIME ZONE NOT NULL,
  INDEX calculated_idx (last_calculated_ts)
);
CREATE TABLE IF NOT EXISTS PrimaryBranchParams (
  tile_id INT4,
  key STRING,
  value STRING,
  PRIMARY KEY (tile_id, key, value)
);
CREATE TABLE IF NOT EXISTS ProblemImages (
  digest STRING PRIMARY KEY,
  num_errors INT2 NOT NULL,
  latest_error STRING NOT NULL,
  error_ts TIMESTAMP WITH TIME ZONE NOT NULL
);
CREATE TABLE IF NOT EXISTS SecondaryBranchDiffCalculationWork (
  branch_name STRING,
  grouping_id BYTES,
  last_updated_ts TIMESTAMP WITH TIME ZONE NOT NULL,
  digests STRING[] NOT NULL,
  last_calculated_ts TIMESTAMP WITH TIME ZONE NOT NULL,
  calculation_lease_ends TIMESTAMP WITH TIME ZONE NOT NULL,
  PRIMARY KEY (branch_name, grouping_id),
  INDEX calculated_idx (last_calculated_ts)
);
CREATE TABLE IF NOT EXISTS SecondaryBranchExpectations (
  branch_name STRING,
  grouping_id BYTES,
  digest BYTES,
  label CHAR NOT NULL,
  expectation_record_id UUID NOT NULL,
  PRIMARY KEY (branch_name, grouping_id, digest)
);
CREATE TABLE IF NOT EXISTS SecondaryBranchParams (
  branch_name STRING,
  version_name STRING,
  key STRING,
  value STRING,
  PRIMARY KEY (branch_name, version_name, key, value)
);
CREATE TABLE IF NOT EXISTS SecondaryBranchValues (
  branch_name STRING,
  version_name STRING,
  secondary_branch_trace_id BYTES,
  digest BYTES NOT NULL,
  grouping_id BYTES NOT NULL,
  options_id BYTES NOT NULL,
  source_file_id BYTES NOT NULL,
  tryjob_id STRING,
  PRIMARY KEY (branch_name, version_name, secondary_branch_trace_id, source_file_id)
);
CREATE TABLE IF NOT EXISTS SourceFiles (
  source_file_id BYTES PRIMARY KEY,
  source_file STRING NOT NULL,
  last_ingested TIMESTAMP WITH TIME ZONE NOT NULL
);
CREATE TABLE IF NOT EXISTS TiledTraceDigests (
  trace_id BYTES,
  tile_id INT4,
  digest BYTES NOT NULL,
  grouping_id BYTES NOT NULL,
  PRIMARY KEY (trace_id, tile_id, digest),
  INDEX grouping_digest_idx (grouping_id, digest),
  INDEX tile_trace_idx (tile_id, trace_id)
);
CREATE TABLE IF NOT EXISTS TraceValues (
  shard INT2,
  trace_id BYTES,
  commit_id STRING,
  digest BYTES NOT NULL,
  grouping_id BYTES NOT NULL,
  options_id BYTES NOT NULL,
  source_file_id BYTES NOT NULL,
  PRIMARY KEY (shard, commit_id, trace_id),
  INDEX trace_commit_idx (trace_id, commit_id) STORING (digest, options_id, grouping_id)
);
CREATE TABLE IF NOT EXISTS Traces (
  trace_id BYTES PRIMARY KEY,
  corpus STRING AS (keys->>'source_type') STORED NOT NULL,
  grouping_id BYTES NOT NULL,
  keys JSONB NOT NULL,
  matches_any_ignore_rule BOOL,
  INDEX grouping_ignored_idx (grouping_id, matches_any_ignore_rule),
  INDEX ignored_grouping_idx (matches_any_ignore_rule, grouping_id),
  INVERTED INDEX keys_idx (keys)
);
CREATE TABLE IF NOT EXISTS TrackingCommits (
  repo STRING PRIMARY KEY,
  last_git_hash STRING NOT NULL
);
CREATE TABLE IF NOT EXISTS Tryjobs (
  tryjob_id STRING PRIMARY KEY,
  system STRING NOT NULL,
  changelist_id STRING NOT NULL REFERENCES Changelists (changelist_id),
  patchset_id STRING NOT NULL REFERENCES Patchsets (patchset_id),
  display_name STRING NOT NULL,
  last_ingested_data TIMESTAMP WITH TIME ZONE NOT NULL,
  INDEX cl_idx (changelist_id)
);
CREATE TABLE IF NOT EXISTS ValuesAtHead (
  trace_id BYTES PRIMARY KEY,
  most_recent_commit_id STRING NOT NULL,
  digest BYTES NOT NULL,
  options_id BYTES NOT NULL,
  grouping_id BYTES NOT NULL,
  corpus STRING AS (keys->>'source_type') STORED NOT NULL,
  keys JSONB NOT NULL,
  matches_any_ignore_rule BOOL,
  INDEX ignored_grouping_idx (matches_any_ignore_rule, grouping_id),
  INDEX corpus_commit_ignore_idx (corpus, most_recent_commit_id, matches_any_ignore_rule) STORING (grouping_id, digest),
  INVERTED INDEX keys_idx (keys)
);
CREATE TABLE IF NOT EXISTS DeprecatedIngestedFiles (
  source_file_id BYTES PRIMARY KEY,
  source_file STRING NOT NULL,
  last_ingested TIMESTAMP WITH TIME ZONE NOT NULL
);
CREATE TABLE IF NOT EXISTS DeprecatedExpectationUndos (
  id SERIAL PRIMARY KEY,
  expectation_id STRING NOT NULL,
  user_id STRING NOT NULL,
  ts TIMESTAMP WITH TIME ZONE NOT NULL
);
`

// Same as above, but will be used when doing schema migration for spanner databases.
// Some statements can be different for CDB v/s Spanner, hence splitting into
// separate variables.
var FromLiveToNextSpanner = `CREATE TABLE IF NOT EXISTS Changelists (
  changelist_id TEXT PRIMARY KEY,
  system TEXT NOT NULL,
  status TEXT NOT NULL,
  owner_email TEXT NOT NULL,
  subject TEXT NOT NULL,
  last_ingested_data TIMESTAMP WITH TIME ZONE NOT NULL,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS CommitsWithData (
  commit_id TEXT PRIMARY KEY,
  tile_id INT8 NOT NULL,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS DiffMetrics (
  left_digest BYTEA,
  right_digest BYTEA,
  num_pixels_diff INT8 NOT NULL,
  percent_pixels_diff FLOAT4 NOT NULL,
  max_rgba_diffs INT8[] NOT NULL,
  max_channel_diff INT8 NOT NULL,
  combined_metric FLOAT4 NOT NULL,
  dimensions_differ BOOL NOT NULL,
  ts TIMESTAMP WITH TIME ZONE NOT NULL,
  PRIMARY KEY (left_digest, right_digest),
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS ExpectationDeltas (
  expectation_record_id TEXT,
  grouping_id BYTEA,
  digest BYTEA,
  label_before VARCHAR(1) NOT NULL,
  label_after VARCHAR(1) NOT NULL,
  PRIMARY KEY (expectation_record_id, grouping_id, digest),
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS ExpectationRecords (
  expectation_record_id TEXT PRIMARY KEY DEFAULT spanner.generate_uuid(),
  branch_name TEXT,
  user_name TEXT NOT NULL,
  triage_time TIMESTAMP WITH TIME ZONE NOT NULL,
  num_changes INT8 NOT NULL,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS Expectations (
  grouping_id BYTEA,
  digest BYTEA,
  label VARCHAR(1) NOT NULL,
  expectation_record_id TEXT,
  PRIMARY KEY (grouping_id, digest),
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS GitCommits (
  git_hash TEXT PRIMARY KEY,
  commit_id TEXT NOT NULL,
  commit_time TIMESTAMP WITH TIME ZONE NOT NULL,
  author_email TEXT NOT NULL,
  subject TEXT NOT NULL,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS Groupings (
  grouping_id BYTEA PRIMARY KEY,
  keys JSONB NOT NULL,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS IgnoreRules (
  ignore_rule_id TEXT PRIMARY KEY DEFAULT spanner.generate_uuid(),
  creator_email TEXT NOT NULL,
  updated_email TEXT NOT NULL,
  expires TIMESTAMP WITH TIME ZONE NOT NULL,
  note TEXT,
  query JSONB NOT NULL,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS MetadataCommits (
  commit_id TEXT PRIMARY KEY,
  commit_metadata TEXT NOT NULL,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS Options (
  options_id BYTEA PRIMARY KEY,
  keys JSONB NOT NULL,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS Patchsets (
  patchset_id TEXT PRIMARY KEY,
  system TEXT NOT NULL,
  changelist_id TEXT NOT NULL REFERENCES Changelists (changelist_id),
  ps_order INT8 NOT NULL,
  git_hash TEXT NOT NULL,
  commented_on_cl BOOL NOT NULL,
  created_ts TIMESTAMP WITH TIME ZONE,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS PrimaryBranchDiffCalculationWork (
  grouping_id BYTEA PRIMARY KEY,
  last_calculated_ts TIMESTAMP WITH TIME ZONE NOT NULL,
  calculation_lease_ends TIMESTAMP WITH TIME ZONE NOT NULL,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS PrimaryBranchParams (
  tile_id INT8,
  key TEXT,
  value TEXT,
  PRIMARY KEY (tile_id, key, value),
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS ProblemImages (
  digest TEXT PRIMARY KEY,
  num_errors INT8 NOT NULL,
  latest_error TEXT NOT NULL,
  error_ts TIMESTAMP WITH TIME ZONE NOT NULL,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS SecondaryBranchDiffCalculationWork (
  branch_name TEXT,
  grouping_id BYTEA,
  last_updated_ts TIMESTAMP WITH TIME ZONE NOT NULL,
  digests TEXT[] NOT NULL,
  last_calculated_ts TIMESTAMP WITH TIME ZONE NOT NULL,
  calculation_lease_ends TIMESTAMP WITH TIME ZONE NOT NULL,
  PRIMARY KEY (branch_name, grouping_id),
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS SecondaryBranchExpectations (
  branch_name TEXT,
  grouping_id BYTEA,
  digest BYTEA,
  label VARCHAR(1) NOT NULL,
  expectation_record_id TEXT NOT NULL,
  PRIMARY KEY (branch_name, grouping_id, digest),
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS SecondaryBranchParams (
  branch_name TEXT,
  version_name TEXT,
  key TEXT,
  value TEXT,
  PRIMARY KEY (branch_name, version_name, key, value),
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS SecondaryBranchValues (
  branch_name TEXT,
  version_name TEXT,
  secondary_branch_trace_id BYTEA,
  digest BYTEA NOT NULL,
  grouping_id BYTEA NOT NULL,
  options_id BYTEA NOT NULL,
  source_file_id BYTEA NOT NULL,
  tryjob_id TEXT,
  PRIMARY KEY (branch_name, version_name, secondary_branch_trace_id, source_file_id),
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS SourceFiles (
  source_file_id BYTEA PRIMARY KEY,
  source_file TEXT NOT NULL,
  last_ingested TIMESTAMP WITH TIME ZONE NOT NULL,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS TiledTraceDigests (
  trace_id BYTEA,
  tile_id INT8,
  digest BYTEA NOT NULL,
  grouping_id BYTEA NOT NULL,
  PRIMARY KEY (trace_id, tile_id, digest),
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS TraceValues (
  shard INT8,
  trace_id BYTEA,
  commit_id TEXT,
  digest BYTEA NOT NULL,
  grouping_id BYTEA NOT NULL,
  options_id BYTEA NOT NULL,
  source_file_id BYTEA NOT NULL,
  PRIMARY KEY (shard, commit_id, trace_id),
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS Traces (
  trace_id BYTEA PRIMARY KEY,
  corpus TEXT GENERATED ALWAYS AS (keys->>'source_type') STORED NOT NULL,
  grouping_id BYTEA NOT NULL,
  keys JSONB NOT NULL,
  matches_any_ignore_rule BOOL,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS TrackingCommits (
  repo TEXT PRIMARY KEY,
  last_git_hash TEXT NOT NULL,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS Tryjobs (
  tryjob_id TEXT PRIMARY KEY,
  system TEXT NOT NULL,
  changelist_id TEXT NOT NULL REFERENCES Changelists (changelist_id),
  patchset_id TEXT NOT NULL REFERENCES Patchsets (patchset_id),
  display_name TEXT NOT NULL,
  last_ingested_data TIMESTAMP WITH TIME ZONE NOT NULL,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS ValuesAtHead (
  trace_id BYTEA PRIMARY KEY,
  most_recent_commit_id TEXT NOT NULL,
  digest BYTEA NOT NULL,
  options_id BYTEA NOT NULL,
  grouping_id BYTEA NOT NULL,
  corpus TEXT GENERATED ALWAYS AS (keys->>'source_type') STORED NOT NULL,
  keys JSONB NOT NULL,
  matches_any_ignore_rule BOOL,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS DeprecatedIngestedFiles (
  source_file_id BYTEA PRIMARY KEY,
  source_file TEXT NOT NULL,
  last_ingested TIMESTAMP WITH TIME ZONE NOT NULL,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE TABLE IF NOT EXISTS DeprecatedExpectationUndos (
  id INT8 PRIMARY KEY,
  expectation_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  ts TIMESTAMP WITH TIME ZONE NOT NULL,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;
CREATE INDEX IF NOT EXISTS system_status_ingested_idx on Changelists (system, status, last_ingested_data);
CREATE INDEX IF NOT EXISTS status_ingested_idx on Changelists (status, last_ingested_data DESC);
CREATE INDEX IF NOT EXISTS branch_ts_idx on ExpectationRecords (branch_name, triage_time);
CREATE INDEX IF NOT EXISTS label_idx on Expectations (label);
CREATE INDEX IF NOT EXISTS commit_idx on GitCommits (commit_id);
CREATE INDEX IF NOT EXISTS cl_order_idx on Patchsets (changelist_id, ps_order);
CREATE INDEX IF NOT EXISTS calculated_idx on PrimaryBranchDiffCalculationWork (last_calculated_ts);
CREATE INDEX IF NOT EXISTS calculated_idx_1 on SecondaryBranchDiffCalculationWork (last_calculated_ts);
CREATE INDEX IF NOT EXISTS grouping_digest_idx on TiledTraceDigests (grouping_id, digest);
CREATE INDEX IF NOT EXISTS tile_trace_idx on TiledTraceDigests (tile_id, trace_id);
CREATE INDEX IF NOT EXISTS trace_commit_idx on TraceValues (trace_id, commit_id) INCLUDE (digest, options_id, grouping_id);
CREATE INDEX IF NOT EXISTS grouping_ignored_idx on Traces (grouping_id, matches_any_ignore_rule);
CREATE INDEX IF NOT EXISTS ignored_grouping_idx on Traces (matches_any_ignore_rule, grouping_id);
CREATE INDEX IF NOT EXISTS cl_idx on Tryjobs (changelist_id);
CREATE INDEX IF NOT EXISTS ignored_grouping_idx_1 on ValuesAtHead (matches_any_ignore_rule, grouping_id);
CREATE INDEX IF NOT EXISTS corpus_commit_ignore_idx on ValuesAtHead (corpus, most_recent_commit_id, matches_any_ignore_rule) INCLUDE (grouping_id, digest);
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
		/*
			    // TODO(pasthana): Uncomment once https://skia-review.googlesource.com/c/buildbot/+/992260
			    // is merged, and schema.jsons have been validated to be equal to current
			    // prod db
					fromLiveToNextStmt := FromLiveToNext
					if datastoreType == config.Spanner {
						fromLiveToNextStmt = FromLiveToNextSpanner
					}
					_, err = db.Exec(ctx, fromLiveToNextStmt)
					if err != nil {
						sklog.Errorf("Failed to migrate Schema from prev to next. Prev: %s, Next: %s.", prev, next)
						return skerr.Wrapf(err, "Failed to migrate Schema")
					}
		*/
	} else if diffNextActual != "" && diffPrevActual != "" {
		sklog.Errorf("Live schema doesn't match next or previous checked-in schema. diffNextActual: %s, diffPrevActual: %s.", diffNextActual, diffPrevActual)
		return skerr.Fmt("Live schema doesn't match next or previous checked-in schema.")
	}

	return nil
}
