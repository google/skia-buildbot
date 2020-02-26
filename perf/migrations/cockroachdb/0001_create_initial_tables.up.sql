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

CREATE TABLE IF NOT EXISTS Shortcuts (
	id TEXT UNIQUE NOT NULL PRIMARY KEY,
	trace_ids TEXT
);