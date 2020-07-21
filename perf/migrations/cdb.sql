
CREATE TABLE IF NOT EXISTS traceids  (
	trace_id INT PRIMARY KEY DEFAULT unique_rowid(),
	trace_name STRING UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS postings (
    tile_number INT,
    trace_id INT,
    key_values STRING[],
    PRIMARY KEY(tile_number, trace_id)
);

INSERT INTO
    traceids (trace_id, trace_name)
VALUES
    (0, ',arch=x86,config=8888,'),
    (1, ',arch=arm,config=8888,')
ON CONFLICT
DO NOTHING;

INSERT INTO
   postings (tile_number, trace_id, key_values)
VALUES
  (0, 0, ARRAY['arch=x86', 'config=8888']),
  (0, 1, ARRAY['arch=arm', 'config=8888'])
ON CONFLICT
DO NOTHING;

SELECT
    *
FROM
    postings
WHERE
    key_values @> ARRAY['config=8888'];