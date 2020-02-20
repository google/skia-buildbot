DROP TABLE IF EXISTS TraceIDs;
DROP TABLE IF EXISTS SourceFiles;
DROP TABLE IF EXISTS Postings;

CREATE TABLE IF NOT EXISTS TraceIDs  (
    trace_id INT PRIMARY KEY DEFAULT unique_rowid(),
    trace_name STRING UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS SourceFiles (
    source_file_id INT PRIMARY KEY DEFAULT unique_rowid(),
    source_file STRING UNIQUE NOT NULL
);

SHOW INDEX FROM SourceFiles;

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

INSERT INTO TraceIDs (trace_id, trace_name)
VALUES
  (1, ',arch=x86,config=8888,'),
  (2, ',arch=x86,config=565,'),
  (3, ',arch=arm,config=8888,'),
  (4, ',arch=arm,config=565,')
ON CONFLICT
DO NOTHING;

SELECT trace_id, trace_name FROM TraceIDs;

SHOW COLUMNS FROM TraceIDs;

SHOW COLUMNS FROM SourceFiles;

INSERT INTO SourceFiles (source_file_id, source_file)
VALUES
  (1, 'gs://perf-bucket/2020/02/08/11/testdata.json'),
  (2, 'gs://perf-bucket/2020/02/08/12/testdata.json'),
  (3, 'gs://perf-bucket/2020/02/08/13/testdata.json'),
  (4, 'gs://perf-bucket/2020/02/08/14/testdata.json')
ON CONFLICT
DO NOTHING;

UPSERT INTO TraceValues (trace_id, commit_number, val, source_file_id)
 VALUES
   (1, 1,   1.2, 1),
   (1, 2,   1.3, 2),
   (1, 3,   1.4, 3),
   (1, 256, 1.1, 4),
   (2, 1,   2.2, 1),
   (2, 2,   2.3, 2),
   (2, 3,   2.4, 3),
   (2, 256, 2.1, 4);

UPSERT INTO Postings (tile_number, key_value, trace_id)
VALUES
   (2, 'config=565', 4),
   (0, 'arch=x86', 1),
   (0, 'arch=x86', 2),
   (0, 'arch=arm', 3),
   (0, 'arch=arm', 4),
   (0, 'config=8888', 1),
   (0, 'config=8888', 3),
   (0, 'config=565', 2),
   (0, 'config=565', 4);

-- All trace_ids that match a particular key=value.
SELECT tile_number, key_value, trace_id FROM Postings
WHERE tile_number=0 AND key_value='arch=x86'
ORDER BY trace_id;

-- Retrieve matching values.
SELECT trace_id, commit_number, val FROM TraceValues
WHERE commit_number>=0 AND commit_number<255 AND trace_id IN (1,2);

-- Build traces using a JOIN.
SELECT TraceIDs.trace_name, TraceValues.commit_number, TraceValues.val FROM TraceIDs
INNER JOIN TraceValues ON TraceValues.trace_id = TraceIDs.trace_id
WHERE TraceIDs.trace_name=',arch=x86,config=8888,' OR TraceIDs.trace_name=',arch=x86,config=565,';

-- Retrieve source file.
SELECT source_file_id, source_file from SourceFiles
WHERE source_file_id=1;

-- Create ParamSet for Tile.
SELECT DISTINCT key_value FROM Postings
WHERE tile_number=0;

-- Most recent tile.
SELECT tile_number FROM Postings ORDER BY tile_number DESC LIMIT 1;

-- Count indices for Tile.
SELECT COUNT(*) FROM Postings WHERE tile_number=0;

-- GetSource by trace name.
SELECT SourceFiles.source_file FROM TraceIDs
INNER JOIN TraceValues ON TraceValues.trace_id = TraceIDs.trace_id
INNER JOIN SourceFiles ON SourceFiles.source_file_id = TraceValues.source_file_id
WHERE TraceIDs.trace_name=',arch=x86,config=8888,' AND TraceValues.commit_number=256;


-- Fully query traces from tile based on query plan.
SELECT TraceIDs.trace_name, TraceValues.commit_number, TraceValues.val FROM TraceIDs
INNER JOIN TraceValues ON TraceValues.trace_id = TraceIDs.trace_id
WHERE
  TraceValues.trace_id IN (
    SELECT trace_id FROM Postings WHERE key_value IN ('arch=x86', 'arch=arm')
  )
  AND TraceValues.trace_id IN (
    SELECT trace_id FROM Postings WHERE key_value IN ('config=8888')
  );

-- Count the traces that would be returned from a query.
SELECT COUNT(DISTINCT trace_id) FROM TraceValues
WHERE
  commit_number > 0
  AND commit_number < 8;
