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
    tile_number INTEGER,
    trace_id INTEGER,
    commit_number INTEGER,
    val REAL,
    source_file_id INTEGER,
    PRIMARY KEY (tile_number, trace_id, commit_number)
);

INSERT INTO SourceFiles (source_file)
VALUES
  ("gs://perf-bucket/2020/02/08/11/testdata.json"),
  ("gs://perf-bucket/2020/02/08/12/testdata.json"),
  ("gs://perf-bucket/2020/02/08/13/testdata.json"),
  ("gs://perf-bucket/2020/02/08/14/testdata.json");

INSERT INTO TraceIDs (trace_name)
VALUES
 (",arch=x86,config=8888"),
 (",arch=x86,config=565"),
 (",arch=arm,config=8888"),
 (",arch=arm,config=565");

 SELECT trace_id, trace_name FROM TraceIDs;

 INSERT OR REPLACE INTO TraceValues (tile_number, trace_id, commit_number, val, source_file_id)
 VALUES
   (0, 1, 1,   1.2, 1),
   (0, 1, 2,   1.3, 2),
   (0, 1, 3,   1.4, 3),
   (1, 1, 256, 1.1, 4),
   (0, 2, 1,   2.2, 1),
   (0, 2, 2,   2.3, 2),
   (0, 2, 3,   2.4, 3),
   (1, 2, 256, 2.1, 4);

INSERT OR REPLACE INTO Postings (tile_number, key_value, trace_id)
VALUES
   (0, "arch=x86", 1),
   (0, "arch=x86", 2),
   (0, "arch=arm", 3),
   (0, "arch=arm", 4),
   (0, "config=8888", 1),
   (0, "config=8888", 3),
   (0, "config=565", 2),
   (0, "config=565", 4);

-- All trace_ids that match a particular key=value.
SELECT tile_number, key_value, trace_id FROM Postings
WHERE tile_number=0 AND key_value="arch=x86"
ORDER BY trace_id;

-- Retrieve matching values. Note that sqlite querys are limited to 1MB,
-- so we might need to break up the trace_ids if the query is too long.
SELECT trace_id, commit_number, val FROM TraceValues
WHERE tile_number=0 AND (trace_id=1 OR trace_id=2);

-- Retrieve source file.
SELECT source_file_id, source_file from SourceFiles
WHERE source_file_id=1;
