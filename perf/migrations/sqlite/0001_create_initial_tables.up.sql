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

CREATE TABLE IF NOT EXISTS Shortcuts (
	id TEXT UNIQUE NOT NULL PRIMARY KEY,
	trace_ids TEXT
);