-- This table is used to store trace names. See go/tracestore/sqltracestore.
CREATE TABLE IF NOT EXISTS TraceIDs  (
	trace_id INTEGER PRIMARY KEY,
	trace_name TEXT UNIQUE NOT NULL       -- The trace name as a structured key, e.g. ",arch=x86,config=8888,"
);

-- This table is used to store an inverted index for trace names. See go/tracestore/sqltracestore.
CREATE TABLE IF NOT EXISTS Postings  (
	tile_number INTEGER,                   -- A types.TileNumber.
	key_value text NOT NULL,               -- A key value pair from a structured key, e.g. "config=8888".
	trace_id INTEGER,                      -- Id of the trace name from TraceIDS.
	PRIMARY KEY (tile_number, key_value, trace_id)
);

-- This table is used to store source filenames. See go/tracestore/sqltracestore.
CREATE TABLE IF NOT EXISTS SourceFiles (
	source_file_id INTEGER PRIMARY KEY,
	source_file TEXT UNIQUE NOT NULL     -- The full name of the source file, e.g. gs://bucket/2020/01/02/03/15/foo.json
);

-- This table is used to store trace values. See go/tracestore/sqltracestore.
CREATE TABLE IF NOT EXISTS TraceValues (
	trace_id INTEGER,                    -- Id of the trace name from TraceIDS.
	commit_number INTEGER,               -- A types.CommitNumber.
	val REAL,                            -- The floating point measurement.
	source_file_id INTEGER,              -- Id of the source filename, from SourceFiles.
	PRIMARY KEY (trace_id, commit_number)
);

-- This table is used to store shortcuts. See go/shortcut/sqlshortcutstore.
CREATE TABLE IF NOT EXISTS Shortcuts (
	id TEXT UNIQUE NOT NULL PRIMARY KEY,
	trace_ids TEXT                       -- A shortcut.Shortcut serialized as JSON.
);

-- This table is used to store alerts. See go/alerts/sqlalertstore.
CREATE TABLE IF NOT EXISTS Alerts (
	id INTEGER PRIMARY KEY,
	alert TEXT,                      -- An alerts.Alert serialized as JSON.
	config_state INTEGER DEFAULT 0,  -- The Alert.State which is an alerts.ConfigState value.
	last_modified INTEGER            -- Unix timestamp.
);

-- This table is used to store regressions. See go/regression/sqlregressionstore.
CREATE TABLE IF NOT EXISTS Regressions (
	commit_number INTEGER,             -- The commit_number where the regression occurred.
	alert_id INTEGER,                  -- The id of an Alert, i.e. the id from the Alerts table.
	regression TEXT,                   -- A regression.Regression serialized as JSON.
	PRIMARY KEY (commit_number, alert_id)
);

-- This table is use to store commits. See go/git.
CREATE TABLE IF NOT EXISTS Commits (
  commit_number INTEGER PRIMARY KEY,  -- The commit_number.
  git_hash TEXT UNIQUE NOT NULL,      -- The git hash at that commit_number.
  commit_time INTEGER,                -- Commit time, as opposed to author time.
  author TEXT,                        -- Author in the format of "Name <email>".
  subject TEXT                        -- The git commit subject.
);