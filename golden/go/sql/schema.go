package sql

// CockroachDBSchema is the schema for all tables used to store Gold data in CockroachDB.
const CockroachDBSchema = `
CREATE TABLE IF NOT EXISTS TraceValues (
  trace_id BYTES NOT NULL, -- MD5 hash of the key/values
  shard BYTES NOT NULL, -- A small piece of the trace id to slightly break up trace data.
  commit_id INT4,
  grouping_id BYTES NOT NULL, -- MD5 hash of the key/values belonging to the grouping. If the grouping 
                     -- changes, this would require altering the table (should be done very rarely).
  digest BYTES NOT NULL, -- MD5 hash of the pixel data
  options_id BYTES NOT NULL, -- MD5 hash of the options string
  source_file_id BYTES NOT NULL, -- MD5 hash of the source file name
-- This index provides easier/faster joins with Expectations.
  INDEX grouping_digest_commit_idx (grouping_id, digest, commit_id DESC),
-- This index provides easier/faster joins with DiffMetrics.
  INDEX grouping_commit_digest_idx (grouping_id, commit_id DESC, digest),
-- This index allows us to look up data just by trace and commit, it is particularly handy for
-- getting data at HEAD.
  INDEX trace_commit_idx (trace_id, commit_id) STORING (grouping_id, digest),
  PRIMARY KEY (shard, commit_id, trace_id) -- gives some locality for both commits and traces
);
-- Pre-split this table so to avoid the one range it starts on from being hot during initial
-- ingestion.
ALTER TABLE TraceValues SPLIT AT VALUES ('\x02', 0), ('\x04', 0), ('\x06', 0);

CREATE TABLE IF NOT EXISTS ChangelistValues (
  changelist_trace_id BYTES NOT NULL, -- MD5 hash of the key/values
  changelist_id STRING NOT NULL, -- e.g. "gerrit_12345"
  patchset_id STRING NOT NULL,
  digest BYTES NOT NULL, -- MD5 hash of the pixel data
  grouping_id BYTES NOT NULL,
  options_id BYTES NOT NULL, -- MD5 hash of the options string
  source_file_id BYTES NOT NULL, -- MD5 hash of the source file name
  tryjob_id STRING NOT NULL, -- e.g. "buildbucket_12345"
  PRIMARY KEY (changelist_id, patchset_id, changelist_trace_id)
);

CREATE TABLE IF NOT EXISTS Commits (
-- A monotonically increasing number as we follow primary branch through time.
  commit_id INT4 PRIMARY KEY, 
  git_hash STRING NOT NULL,
  commit_time TIMESTAMP WITH TIME ZONE NOT NULL,
  author STRING,
  subject STRING,
-- This is set the first time data lands on the primary branch for this commit number. We use this
-- to determine the dense tile of data. Previously, we had tried to determine this with a DISTINCT
-- search over TraceValues, but that takes several minutes when there are 1M+ traces per commit.
  has_data BOOL,
  INDEX has_data_idx (has_data DESC, commit_id DESC), -- Used for finding the dense tile
  INDEX git_hash_idx (git_hash)
);

 -- Assumption: a trace_id is in this table if and only if it is on the primary branch.
CREATE TABLE IF NOT EXISTS Traces (
  trace_id BYTES PRIMARY KEY, -- MD5 hash of the stringified JSON of the keys
  keys JSONB NOT NULL, -- The trace's keys, e.g. {"color mode":"RGB", "device":"walleye", etc}
-- This should be unset (e.g. NULL) when a trace is added for the first time. It will get updated
-- 1) by an automated request that searches for Traces with "matches_any_ignore_rule IS NULL" and
-- sets them according to the latest rules; and 2) whenever ignore rules are changed all traces
-- will have this value set.
  matches_any_ignore_rule BOOL,
-- This will be conditionally updated at ingestion time. Having this here saves a big self join
-- on the TraceValues table
  most_recent_commit_id INT4 NOT NULL,
  INVERTED INDEX keys_idx (keys),
  INDEX ignored_idx (matches_any_ignore_rule) STORING (keys),
  INDEX head_idx (most_recent_commit_id DESC, matches_any_ignore_rule) STORING (keys)
);

CREATE TABLE IF NOT EXISTS ChangelistTraces (
  changelist_trace_id BYTES PRIMARY KEY, -- MD5 hash of the stringified JSON of the keys
  keys JSONB NOT NULL, -- The trace's keys, e.g. {"color mode":"RGB", "device":"walleye", etc}
  matches_any_ignore_rule BOOL,
  INVERTED INDEX keys_idx (keys),
  INDEX ignored_idx (matches_any_ignore_rule) STORING (keys)
);

-- Originally, we had done a join between Traces and TraceValues to find the params where 
-- commit_id was in a given range. However, this took several minutes when there are 1M+ traces
-- per commit. This table is effectively an index for getting just that data. Note, this contains
-- Keys and Options. The number of rows per commit in this table is much much less than the number
-- of rows in TraceValues per commit and thus faster to query.
CREATE TABLE IF NOT EXISTS PrimaryBranchParams (
  key STRING NOT NULL,
  value STRING NOT NULL,
  commit_id INT4 NOT NULL,
  PRIMARY KEY (commit_id, key, value)
);

CREATE TABLE IF NOT EXISTS ChangelistParams (
  key STRING NOT NULL,
  value STRING NOT NULL,
  changelist_id STRING NOT NULL,
  patchset_id STRING NOT NULL,
  PRIMARY KEY (changelist_id, patchset_id, key, value)
);

CREATE TABLE IF NOT EXISTS Options ( -- contains option keys (for either CLs or primary branch)
  options_id BYTES PRIMARY KEY, -- MD5 hash of the stringified JSON.
  keys JSONB NOT NULL, -- keys, e.g. {"ext": "png"}
  INVERTED INDEX keys_idx (keys)
);

CREATE TABLE IF NOT EXISTS Groupings ( -- contains groupings (for either CLs or primary branch)
  grouping_id BYTES PRIMARY KEY, -- MD5 hash of the stringified JSON.
  keys JSONB NOT NULL, -- keys, e.g. {"source_type":"round","name":"circle"}
  INVERTED INDEX keys_idx (keys)
);

CREATE TABLE IF NOT EXISTS SourceFiles (
  source_file_id BYTES PRIMARY KEY, -- The MD5 hash of the source file name
  source_file STRING NOT NULL,  -- The full name of the source file, e.g. gs://bucket/2020/01/02/03/15/foo.json
  last_ingested TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE TABLE IF NOT EXISTS Expectations (
  grouping_id BYTES, -- MD5 hash of the grouping JSON
  digest BYTES NOT NULL, -- MD5 hash of the pixel data
  label SMALLINT NOT NULL, -- 0 for untriaged, 1 for positive, 2 for negative
  start_index INT4, -- Reserved for future use with expectation ranges
  end_index INT4, -- Reserved for future use with expectation ranges
  expectation_record_id UUID, -- If not null, the record that set this value
  INDEX label_idx (label),
  INDEX group_label_idx (grouping_id, label) STORING (expectation_record_id),
  PRIMARY KEY (digest, grouping_id) -- start_index should be on primary key too eventually.
);

CREATE TABLE IF NOT EXISTS ChangelistExpectations (
  changelist_id STRING NOT NULL, -- e.g. "gerrit_12345"
  grouping_id BYTES, -- MD5 hash of the grouping JSON
  digest BYTES NOT NULL, -- MD5 hash of the pixel data
  label SMALLINT NOT NULL, -- 0 for untriaged, 1 for positive, 2 for negative
  start_index INT4, -- Reserved for future use with expectation ranges
  end_index INT4, -- Reserved for future use with expectation ranges
  expectation_record_id UUID, -- If not null, the record that set this value
  INDEX changelist_label_idx (changelist_id, label),
  PRIMARY KEY (digest, changelist_id, grouping_id) -- start_index should be on primary key too eventually.
);

CREATE TABLE IF NOT EXISTS ExpectationDeltas (
  expectation_delta_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  expectation_record_id UUID,
  grouping_id BYTES, -- MD5 hash of the grouping JSON
  digest BYTES NOT NULL, -- MD5 hash of the pixel data
  label_before SMALLINT, -- 0 for untriaged, 1 for positive, 2 for negative
  label_after SMALLINT, -- 0 for untriaged, 1 for positive, 2 for negative
  start_index INT4, -- Reserved for future use with expectation ranges
  end_index_before INT4, -- Reserved for future use with expectation ranges
  end_index_after INT4 -- Reserved for future use with expectation ranges
);

CREATE TABLE IF NOT EXISTS ExpectationRecords (
  expectation_record_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  changelist_id STRING, -- e.g. CodeReviewSystem and CL ID "gerrit_12345". Can be NULL
  user_name STRING,
  time TIMESTAMP WITH TIME ZONE,
  num_changes INT4
);

CREATE TABLE IF NOT EXISTS DiffMetrics (
  left_digest BYTES NOT NULL, -- MD5 hash of the pixel data.
  right_digest BYTES NOT NULL, -- MD5 hash of the pixel data
  num_diff_pixels INT4,
  pixel_diff_percent FLOAT4,
-- This is what the RGBAMinFilter and RGBAMaxFilter apply to. There does not appear to be a way to
-- do this via SQL statements (in a clean way).
  max_channel_diff INT2,
  max_rgba_diff INT2[], -- max delta in the red, green, blue, alpha channels.
  dimensions_differ BOOL,
  PRIMARY KEY (left_digest, right_digest)
);

CREATE TABLE IF NOT EXISTS IgnoreRules (
  ignore_rule_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  created_user STRING NOT NULL, 
  updated_user STRING NOT NULL, 
  expires TIMESTAMP WITH TIME ZONE,
  note STRING,
  query JSONB -- a map of key => values
);

CREATE TABLE IF NOT EXISTS Changelists (
  changelist_id STRING PRIMARY KEY, -- Includes system (e.g. "gerrit_1234") for easy joins.
  system STRING NOT NULL, -- e.g. "gerrit", "github"
  status INT2,
  owner STRING,
  updated TIMESTAMP WITH TIME ZONE, -- This is updated if data is ingested into this CL.
  subject STRING
);

CREATE TABLE IF NOT EXISTS Patchsets (
  patchset_id STRING PRIMARY KEY, -- Includes system (e.g. "gerrit_abcd") for easy joins.
  system STRING NOT NULL, -- e.g. "gerrit", "github"
  changelist_id STRING, -- parent CL this belongs to
  ps_order INT2, -- a 1 indexed number telling us where this PS fits in time.
  git_hash STRING,
  commented_on_parent_cl BOOL DEFAULT false
);

CREATE TABLE IF NOT EXISTS Tryjobs (
  tryjob_id STRING PRIMARY KEY, -- Includes system (e.g. "buildbucket_1234") for easy joins.
  system STRING NOT NULL, -- e.g. "buildbucket"
  changelist_id STRING,
  patchset_id STRING,
  display_name STRING, -- human readable name
  updated TIMESTAMP WITH TIME ZONE
);
`
