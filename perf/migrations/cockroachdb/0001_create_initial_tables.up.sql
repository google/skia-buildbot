-- This table is used to store trace values. See go/tracestore/sqltracestore.
CREATE TABLE IF NOT EXISTS TraceValues (
	trace_id BYTES,
	-- Id of the trace name from TraceIDS.
	commit_number INT,
	-- A types.CommitNumber.
	val REAL,
	-- The floating point measurement.
	source_file_id INT,
	-- Id of the source filename, from SourceFiles.
	PRIMARY KEY (trace_id, commit_number)
);

-- This table is used to store source filenames. See go/tracestore/sqltracestore.
CREATE TABLE IF NOT EXISTS SourceFiles (
	source_file_id INT PRIMARY KEY DEFAULT unique_rowid(),
	-- The full name of the source file, e.g. gs://bucket/2020/01/02/03/15/foo.json
	source_file STRING UNIQUE NOT NULL
);

-- This table stores the ParamSet for each tile.
CREATE TABLE IF NOT EXISTS ParamSets (
	tile_number INT,
	param_key STRING,
	param_value STRING,
	-- The primary key is used to load a paramset for a given tile.
	PRIMARY KEY (tile_number, param_key, param_value),
	-- This secondary index is used to determine the most recent tile.
	INDEX by_tile_number (tile_number DESC)
);

-- This table is used to store an inverted index for trace names. See go/tracestore/sqltracestore.
CREATE TABLE IF NOT EXISTS Postings (
	-- A types.TileNumber.
	tile_number INT,
	-- A key value pair from a structured key, e.g. "config=8888".
	key_value STRING NOT NULL,
	-- md5(trace_name)
	trace_id BYTES,
	-- The primary index is used for looking up trace_ids based on queries for key=value pairs.
	PRIMARY KEY (tile_number, key_value, trace_id),
	-- This secondary index is used to reconstruct a trace's full name from a
	-- given trace_id.
	INDEX by_trace_id (tile_number, trace_id, key_value)
);

-- This table is used to store shortcuts. See go/shortcut/sqlshortcutstore.
CREATE TABLE IF NOT EXISTS Shortcuts (
	id TEXT UNIQUE NOT NULL PRIMARY KEY,
	trace_ids TEXT -- A shortcut.Shortcut serialized as JSON.
);

-- This table is used to store alerts. See go/alerts/sqlalertstore.
CREATE TABLE IF NOT EXISTS Alerts (
	id INT PRIMARY KEY DEFAULT unique_rowid(),
	-- alerts.Alert serialized as JSON.
	alert TEXT,
	-- The Alert.State which is an alerts.ConfigState value.
	config_state INT DEFAULT 0,
	last_modified INT -- Unix timestamp.
);

-- This table is used to store regressions. See go/regression/sqlregressionstore.
CREATE TABLE IF NOT EXISTS Regressions (
	-- The commit_number where the regression occurred.
	commit_number INT,
	-- The id of an Alert, i.e. the id from the Alerts table.
	alert_id INT,
	-- A regression.Regression serialized as JSON.
	regression TEXT,
	PRIMARY KEY (commit_number, alert_id)
);

-- This table is use to store commits. See go/git.
CREATE TABLE IF NOT EXISTS Commits (
	-- The commit_number.
	commit_number INT PRIMARY KEY,
	-- The git hash at that commit_number.
	git_hash TEXT UNIQUE NOT NULL,
	-- Commit time, as opposed to author time.
	commit_time INT,
	-- Author in the format of "Name <email>".
	author TEXT,
	-- The git commit subject.
	subject TEXT
);