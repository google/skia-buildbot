CREATE TABLE IF NOT EXISTS TraceIDs  (
	trace_id INT PRIMARY KEY DEFAULT unique_rowid(),
	trace_name STRING UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS SourceFiles (
	source_file_id INT PRIMARY KEY DEFAULT unique_rowid(),
	source_file STRING UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS Postings  (
	tile_number INT,
	key_value STRING NOT NULL,
	trace_id INT,
	PRIMARY KEY (tile_number, key_value, trace_id)
);

CREATE TABLE IF NOT EXISTS TraceValues (
	trace_id INT,
	commit_number INT,
	val REAL,
	source_file_id INT,
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
