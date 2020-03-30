-- This table is used to store trace names. See go/tracestore/sqltracestore.
CREATE TABLE IF NOT EXISTS TraceIDs  (
	trace_id INT PRIMARY KEY DEFAULT unique_rowid(),
	trace_name STRING UNIQUE NOT NULL
);

-- This table is used to store an inverted index for trace names. See go/tracestore/sqltracestore.
CREATE TABLE IF NOT EXISTS Postings  (
	tile_number INT,              -- A types.TileNumber.
	key_value STRING NOT NULL,    -- A key value pair from a structured key, e.g. "config=8888".
	trace_id INT,                 -- Id of the trace name from TraceIDS.
	PRIMARY KEY (tile_number, key_value, trace_id)
);

-- This table is used to store source filenames. See go/tracestore/sqltracestore.
CREATE TABLE IF NOT EXISTS SourceFiles (
	source_file_id INT PRIMARY KEY DEFAULT unique_rowid(),
	source_file STRING UNIQUE NOT NULL     -- The full name of the source file, e.g. gs://bucket/2020/01/02/03/15/foo.json
);

-- This table is used to store trace values. See go/tracestore/sqltracestore.
CREATE TABLE IF NOT EXISTS TraceValues (
	trace_id INT,                        -- Id of the trace name from TraceIDS.
	commit_number INT,                   -- A types.CommitNumber.
	val REAL,                            -- The floating point measurement.
	source_file_id INT,                  -- Id of the source filename, from SourceFiles.
	PRIMARY KEY (trace_id, commit_number)
);

-- This table is used to store shortcuts. See go/shortcut/sqlshortcutstore.
CREATE TABLE IF NOT EXISTS Shortcuts (
	id TEXT UNIQUE NOT NULL PRIMARY KEY,
	trace_ids TEXT                       -- A shortcut.Shortcut serialized as JSON.
);

-- This table is used to store alerts. See go/alerts/sqlalertstore.
CREATE TABLE IF NOT EXISTS Alerts (
	id INT PRIMARY KEY DEFAULT unique_rowid(),
	alert TEXT,                                -- alerts.Alert serialized as JSON.
	config_state INT DEFAULT 0,                -- The Alert.State which is an alerts.ConfigState value.
	last_modified INT                          -- Unix timestamp.
);

-- This table is used to store regressions. See go/regression/sqlregressionstore.
CREATE TABLE IF NOT EXISTS Regressions (
	commit_number INT,                 -- The commit_number where the regression occurred.
	alert_id INT,                      -- The id of an Alert, i.e. the id from the Alerts table.
	regression TEXT,                   -- A regression.Regression serialized as JSON.
	PRIMARY KEY (commit_number, alert_id)
);

-- This table is use to store commits. See go/git.
CREATE TABLE IF NOT EXISTS Commits (
  commit_number INT PRIMARY KEY,  -- The commit_number.
  git_hash TEXT UNIQUE NOT NULL,      -- The git hash at that commit_number.
  commit_time INT,                -- Commit time, as opposed to author time.
  author TEXT,                        -- Author in the format of "Name <email>".
  subject TEXT                        -- The git commit subject.
);