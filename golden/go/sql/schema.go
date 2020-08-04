package sql

// CockroachDBSchema is the schema for all tables used to store Gold data in CockroachDB.
const CockroachDBSchema = `
CREATE TABLE IF NOT EXISTS TraceValues (
  trace_hash BYTES, -- MD5 hash of the key/values
  shard BYTES, -- The first N bytes of trace_hash; N is currently 1
  commit_number INT4,
  grouping_hash BYTES, -- MD5 hash of the key/values belonging to the grouping. If the grouping 
                       -- changes, this would require altering the table (should be done very rarely).
  digest BYTES NOT NULL, -- MD5 hash of the pixel data
  options_hash BYTES, -- MD5 hash of the options string
  source_file_hash BYTES NOT NULL, -- MD5 hash of the source file name
  INDEX (commit_number, grouping_hash, digest), -- Allows for easier joins with Expectations
  INDEX (trace_hash, commit_number), 
-- Could add an index on just trace_hash
  PRIMARY KEY (shard, commit_number, trace_hash) -- gives some locality for both commits and traces
);
-- Pre-split this table so to avoid the one range it starts on from being hot during initial
-- ingestion.
ALTER TABLE TraceValues SPLIT AT VALUES ('\x03', 0), ('\x07', 0), ('\x0b', 0);

CREATE TABLE IF NOT EXISTS TryJobValues (
  trace_hash BYTES, -- MD5 hash of the trace string
  crs_cl_id STRING, -- CodeReviewSystem and CL ID e.g. "gerrit_12345"
  ps_id STRING, -- PatchSet id
  digest BYTES NOT NULL, -- MD5 hash of the pixel data
  options_hash BYTES, -- MD5 hash of the options string
  cis_tryjob_id STRING NOT NULL, -- ContinuousIntegrationSystem and ID e.g. "buildbucket_12345"
  source_file_hash BYTES NOT NULL, -- MD5 hash of the source file name
  PRIMARY KEY (trace_hash, crs_cl_id, ps_id)
);

CREATE TABLE IF NOT EXISTS Commits (
  commit_number INT4 PRIMARY KEY, -- The commit_number; a monotonically increasing number as we follow master branch through time.
  git_hash STRING NOT NULL,
  commit_time TIMESTAMP WITH TIME ZONE NOT NULL,
  author STRING,
  subject STRING
);

CREATE TABLE IF NOT EXISTS KeyValueMaps ( -- contains trace keys, option keys, etc
  keys_hash BYTES PRIMARY KEY, -- MD5 hash of the stringified JSON.
  keys JSONB NOT NULL, -- The trace's keys, e.g. {"color mode":"RGB", "device":"walleye"}
  INVERTED INDEX (keys)
);

CREATE TABLE IF NOT EXISTS SourceFiles (
  source_file_hash BYTES PRIMARY KEY, -- The MD5 hash of the source file name
  source_file STRING NOT NULL,  -- The full name of the source file, e.g. gs://bucket/2020/01/02/03/15/foo.json
  last_ingested TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE TABLE IF NOT EXISTS Expectations (
  grouping_hash BYTES, -- MD5 hash of the grouping JSON
  digest BYTES NOT NULL, -- MD5 hash of the pixel data
  label SMALLINT, -- 0 for untriaged, 1 for positive, 2 for negative
  start_index INT4, -- Reserved for future use with expectation ranges
  end_index INT4, -- Reserved for future use with expectation ranges
  INDEX (label),
  PRIMARY KEY (digest, grouping_hash) -- start_index should be on primary key too eventually.
);

CREATE TABLE IF NOT EXISTS CLExpectations (
  crs_cl_id STRING, -- e.g. CodeReviewSystem and CL ID "gerrit_12345"
  grouping_hash BYTES, -- MD5 hash of the grouping JSON
  digest BYTES NOT NULL, -- MD5 hash of the pixel data
  label SMALLINT, -- 0 for untriaged, 1 for positive, 2 for negative
  start_index INT4, -- Reserved for future use with expectation ranges
  end_index INT4, -- Reserved for future use with expectation ranges
  PRIMARY KEY (digest, crs_cl_id, grouping_hash) -- start_index should be on primary key too eventually.
);

CREATE TABLE IF NOT EXISTS ExpectationsDeltas (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  record_id UUID, -- matches primary key of ExpectationRecords table
  grouping_hash BYTES, -- MD5 hash of the grouping JSON
  digest BYTES NOT NULL, -- MD5 hash of the pixel data
  label_before SMALLINT, -- 0 for untriaged, 1 for positive, 2 for negative
  label_after SMALLINT, -- 0 for untriaged, 1 for positive, 2 for negative
  start_index INT4, -- Reserved for future use with expectation ranges
  end_index_before INT4, -- Reserved for future use with expectation ranges
  end_index_after INT4 -- Reserved for future use with expectation ranges
);

CREATE TABLE IF NOT EXISTS ExpectationsRecords (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  crs_cl_id STRING, -- e.g. CodeReviewSystem and CL ID "gerrit_12345". Can be empty string.
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
`

// TODO(kjlubick) tables for PS/CL/TJ etc
