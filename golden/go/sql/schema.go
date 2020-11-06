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
  INDEX commit_grouping_digest_idx (commit_id DESC, grouping_id, digest),
-- This index provides easier/faster joins with DiffMetrics.
  INDEX grouping_commit_digest_idx (grouping_id, commit_id DESC, digest),
-- This index allows us to look up data just by trace and commit.
  INDEX trace_commit_idx (trace_id, commit_id) STORING (digest),
-- TODO (kjlubick) might want a index on digest, commit_id, trace to look up "who drew this?"
  PRIMARY KEY (shard, commit_id, trace_id) -- gives some locality for both commits and traces
);
-- Pre-split this table so to avoid the one range it starts on from being hot during initial
-- ingestion.
ALTER TABLE TraceValues SPLIT AT VALUES ('\x02', 0), ('\x04', 0), ('\x06', 0);

-- This table exists to make queries against HEAD faster.
CREATE TABLE IF NOT EXISTS ValuesAtHead (
  trace_id BYTES NOT NULL PRIMARY KEY, -- MD5 hash of the key/values
  most_recent_commit_id INT4,
  digest BYTES, -- MD5 hash of the pixel data
  --options_id BYTES NOT NULL, -- This should be updated with digest
-- denormalized data below, stored here to avoid joins
  grouping_id BYTES NOT NULL, -- MD5 hash of the key/values belonging to the grouping.
  -- These keys are here to make searches quicker and to make it easier to update ignore rules.
  keys JSONB NOT NULL, -- The trace's keys, e.g. {"color mode":"RGB", "device":"walleye", etc}
  expectation_label SMALLINT NOT NULL, -- 0 for untriaged, 1 for positive, 2 for negative
  expectation_record_id UUID,
  matches_any_ignore_rule BOOL,

  INVERTED INDEX keys_idx (keys),
  INDEX commit_ignored_label_idx (most_recent_commit_id desc, matches_any_ignore_rule, expectation_label),
  -- This index is used for updating expectations quickly.
  INDEX grouping_digest_idx (grouping_id, digest),

  -- These column families break the keys into three groups based on frequency of updates. This
  -- helps keep the table small (and fit on fewer ranges). The groups are f1=often, f2=rarely,
  -- f3=never. 
  FAMILY f1 (most_recent_commit_id, digest),
  FAMILY f2 (matches_any_ignore_rule, expectation_label, expectation_record_id),
  FAMILY f3 (keys, grouping_id)
);

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
  matches_any_ignore_rule BOOL, -- IDEA, list the ignore rules that do match this, so we can 
-- selectively turn those on or off?
  INVERTED INDEX keys_idx (keys),
  INDEX ignored_idx (matches_any_ignore_rule) STORING (keys)
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
  commit_id INT4 NOT NULL, -- TODO(kjlubick) maybe this should be bucketed into 50 commit sections?
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
  grouping_id BYTES NOT NULL, -- MD5 hash of the grouping JSON
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
  changelist_id STRING, -- e.g. CodeReviewSystem and CL ID "gerrit_12345". Can be NULL for primary branch.
  user_name STRING,
  time TIMESTAMP WITH TIME ZONE,
  num_changes INT4
);

CREATE TABLE IF NOT EXISTS DiffMetrics (
  left_digest BYTES NOT NULL, -- MD5 hash of the pixel data.
  right_digest BYTES NOT NULL, -- MD5 hash of the pixel data.
  -- TODO(kjlubick) The combined diff metric
  num_diff_pixels INT4,
  pixel_diff_percent FLOAT4,
-- This is what the RGBAMinFilter and RGBAMaxFilter apply to. There does not appear to be a way to
-- do this via SQL statements (in a clean way).
  max_channel_diff INT2,
  max_rgba_diff INT2[], -- max delta in the red, green, blue, alpha channels.
  dimensions_differ BOOL,
  -- TODO(kjlubick) The created date, so we can occasionally prune old data. That or with a join on TraceValues.
  PRIMARY KEY (left_digest, right_digest)
);

ALTER TABLE DiffMetrics SPLIT AT VALUES ('\x22', '\x00'), ('\x44', '\x00'), ('\x66', '\x00'), 
('\x88', '\x00'), ('\xaa', '\x00'), ('\xcc', '\x00'), ('\xee', '\x00');

-- This table should be filled in by a task running in the background using window functions against
-- DiffMetrics to precompute this.
CREATE TABLE IF NOT EXISTS DiffMetricsClosestView (
  left_digest BYTES NOT NULL, -- MD5 hash of the pixel data.
  closest_rank INT2,  -- This entry is the nth closest diff against left_digest
  right_digest BYTES NOT NULL, -- MD5 hash of the pixel data.
  -- TODO(kjlubick) The combined diff metric
  num_diff_pixels INT4,
  pixel_diff_percent FLOAT4,
  max_channel_diff INT2,
  max_rgba_diff INT2[], -- max delta in the red, green, blue, alpha channels.
  dimensions_differ BOOL,
  PRIMARY KEY (left_digest, closest_rank)
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

type TraceID []byte
type GroupingID []byte
type OptionsID []byte
type Digest []byte

func (t TraceID) Equals(other []byte) bool {
	if len(t) != len(other) {
		return false
	}
	for i := 0; i < len(t); i++ {
		if t[i] != other[i] {
			return false
		}
	}
	return true
}
