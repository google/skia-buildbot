DROP TABLE IF EXISTS TraceIDs;
DROP TABLE IF EXISTS SourceFiles;
DROP TABLE IF EXISTS Postings;

CREATE TABLE IF NOT EXISTS TraceIDs  (
    trace_id BYTES PRIMARY KEY DEFAULT uuid_v4(),
    trace_name STRING UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS SourceFiles (
    source_file_id BYTES PRIMARY KEY DEFAULT uuid_v4(),
    source_file STRING UNIQUE NOT NULL
);

SHOW INDEX FROM SourceFiles;

CREATE TABLE IF NOT EXISTS Postings  (
    tile_number INTEGER,
    key_value STRING NOT NULL,
    trace_id BYTES,
    PRIMARY KEY (tile_number, key_value, trace_id)
);

INSERT INTO TraceIDs (trace_name)
VALUES
  (',arch=x86,config=8888,'),
  (',arch=x86,config=565,'),
  (',arch=arm,config=8888,'),
  (',arch=arm,config=565,')
ON CONFLICT
DO NOTHING;

SELECT trace_id, trace_name FROM TraceIDs;

SHOW COLUMNS FROM TraceIDs;

SHOW COLUMNS FROM SourceFiles;

INSERT INTO SourceFiles (source_file)
VALUES
  ('gs://perf-bucket/2020/02/08/11/testdata.json'),
  ('gs://perf-bucket/2020/02/08/12/testdata.json'),
  ('gs://perf-bucket/2020/02/08/13/testdata.json'),
  ('gs://perf-bucket/2020/02/08/14/testdata.json')
ON CONFLICT
DO NOTHING;

