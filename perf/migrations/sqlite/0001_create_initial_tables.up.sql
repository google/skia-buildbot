CREATE TABLE IF NOT EXISTS TraceIDs  (
	trace_id INTEGER PRIMARY KEY,
	trace_name TEXT UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS Postings  (
	tile_number INTEGER,
	key_value text NOT NULL,
	trace_id INTEGER,
	PRIMARY KEY (tile_number, key_value, trace_id)
);

CREATE TABLE IF NOT EXISTS SourceFiles (
	source_file_id INTEGER PRIMARY KEY,
	source_file TEXT UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS TraceValues (
	trace_id INTEGER,
	commit_number INTEGER,
	val REAL,
	source_file_id INTEGER,
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