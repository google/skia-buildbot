-- This is a useful file for playing around with SQL queries against a database
-- populated with Perf data. You can use this file by running:
--
--   cockroach sql --insecure --host=localhost < ./migrations/cdb.sql
--
-- Make sure to apply the migrations the first time:
--
--   cockroach sql --insecure --host=localhost < ./migrations/cockroachdb/0001_create_initial_tables.up.sql
--
-- You should be able to run this file against the same database more than once
-- w/o error.
--
-- For reference, the following trace names are used in this file
-- and these are their md5 hashes:
--   ,arch=x86,config=8888,   =>  fe385b159ff55dca481069805e5ff050
--   ,arch=x86,config=565,    =>  277262a9236d571883d47dab102070bc
--   ,arch=risc-v,config=565, =>  2f9eedb889a1af4e0cf5a76d29cf12b3
INSERT INTO
    SourceFiles (source_file, source_file_id)
VALUES
    (
        'gs://perf-bucket/2020/02/08/11/testdata.json',
        1
    ),
    (
        'gs://perf-bucket/2020/02/08/12/testdata.json',
        2
    ),
    (
        'gs://perf-bucket/2020/02/08/13/testdata.json',
        3
    ),
    (
        'gs://perf-bucket/2020/02/08/14/testdata.json',
        4
    ) ON CONFLICT DO NOTHING;

INSERT INTO
    TraceValues (trace_id, commit_number, val, source_file_id)
VALUES
    ('\xfe385b159ff55dca481069805e5ff050', 1, 1.2, 1),
    ('\xfe385b159ff55dca481069805e5ff050', 2, 1.3, 2),
    ('\xfe385b159ff55dca481069805e5ff050', 3, 1.4, 3),
    (
        '\x2f9eedb889a1af4e0cf5a76d29cf12b3',
        256,
        1.1,
        4
    ),
    ('\x277262a9236d571883d47dab102070bc', 1, 2.2, 1),
    ('\x277262a9236d571883d47dab102070bc', 2, 2.3, 2),
    ('\x277262a9236d571883d47dab102070bc', 3, 2.4, 3),
    (
        '\x2f9eedb889a1af4e0cf5a76d29cf12b3',
        256,
        2.1,
        4
    ) ON CONFLICT DO NOTHING;

INSERT INTO
    ParamSets (tile_number, param_key, param_value)
VALUES
    (0, 'config', '8888'),
    (0, 'config', '565'),
    (0, 'arch', 'x86'),
    (1, 'config', '565'),
    (1, 'arch', 'risc-v') ON CONFLICT DO NOTHING;

INSERT INTO
    Postings (tile_number, key_value, trace_id)
VALUES
    (
        0,
        'arch=x86',
        '\xfe385b159ff55dca481069805e5ff050'
    ),
    (
        0,
        'arch=x86',
        '\x277262a9236d571883d47dab102070bc'
    ),
    (
        0,
        'config=8888',
        '\xfe385b159ff55dca481069805e5ff050'
    ),
    (
        0,
        'config=565',
        '\x277262a9236d571883d47dab102070bc'
    ),
    (
        1,
        'config=565',
        '\x2f9eedb889a1af4e0cf5a76d29cf12b3'
    ),
    (
        1,
        'arch=risc-v',
        '\x2f9eedb889a1af4e0cf5a76d29cf12b3'
    ) ON CONFLICT DO NOTHING;

-- All trace_ids that match a particular key=value.
SELECT
    encode(trace_id, 'hex')
FROM
    Postings
WHERE
    key_value IN ('config=8888', 'config=565')
    AND tile_number = 0
ORDER BY
    trace_id ASC;

-- Retrieve matching values.
SELECT
    encode(trace_id, 'hex'),
    commit_number,
    val
FROM
    TraceValues
WHERE
    commit_number >= 0
    AND commit_number < 255
    AND trace_id IN (
        '\xfe385b159ff55dca481069805e5ff050',
        '\x277262a9236d571883d47dab102070bc'
    );

-- Compound queries.
SELECT
    encode(trace_id, 'hex')
FROM
    Postings
WHERE
    key_value IN ('config=8888', 'config=565')
    AND tile_number = 0
INTERSECT
SELECT
    encode(trace_id, 'hex')
FROM
    Postings
WHERE
    key_value IN ('arch=x86')
    AND tile_number = 0;

-- ParamSet for a tile.
SELECT
    param_key,
    param_value
FROM
    ParamSets
WHERE
    tile_number = 1;

-- Most recent tile.
SELECT
    tile_number
FROM
    ParamSets @by_tile_number
ORDER BY
    tile_number DESC
LIMIT
    1;

-- GetSource by trace_id.
SELECT
    SourceFiles.source_file
FROM
    TraceValues INNER LOOKUP
    JOIN SourceFiles ON TraceValues.source_file_id = SourceFiles.source_file_id
WHERE
    TraceValues.trace_id = '\xfe385b159ff55dca481069805e5ff050'
    AND TraceValues.commit_number = 1;

-- Count the number of matches to a query.
SELECT
    count(*)
FROM
    (
        SELECT
            trace_id
        FROM
            Postings
        WHERE
            key_value IN ('config=8888', 'config=565')
            AND tile_number = 0
        INTERSECT
        SELECT
            trace_id
        FROM
            Postings
        WHERE
            key_value IN ('arch=x86')
            AND tile_number = 0
    );

-- The first part of every query is to pull in the trace names
-- and trace_ids that match the given query for the given tile.
SELECT
    key_value,
    trace_id
FROM
    Postings @by_trace_id
WHERE
    tile_number = 0
    AND trace_id IN (
        SELECT
            trace_id
        FROM
            Postings
        WHERE
            key_value IN ('config=8888', 'config=565')
            AND tile_number = 0
        INTERSECT
        SELECT
            trace_id
        FROM
            Postings
        WHERE
            key_value IN ('arch=x86')
            AND tile_number = 0
    );

-- Then query for trace values in batches.
SELECT
    trace_id,
    commit_number,
    val
FROM
    TraceValues
WHERE
    tracevalues.commit_number >= 0
    AND tracevalues.commit_number < 256
    AND tracevalues.trace_id IN (
        '\xfe385b159ff55dca481069805e5ff050',
        '\x277262a9236d571883d47dab102070bc'
    );

-- GetLastNSources
SELECT SourceFiles.source_file
FROM
    TraceValues@primary INNER LOOKUP JOIN  SourceFiles@primary
    ON TraceValues.source_file_id = SourceFiles.source_file_id
WHERE
    TraceValues.trace_id='\xfe385b159ff55dca481069805e5ff050'
ORDER BY TraceValues.commit_number DESC
LIMIT 5;

-- GetTraceIDsBySource
-- Note that this requires the tile_number, which we won't have,
-- unless GetLastNSources also returns the commit_number.
SELECT Postings.key_value, Postings.trace_id
FROM
    SourceFiles@by_source_file
    INNER LOOKUP JOIN TraceValues@by_source_file_id
    ON TraceValues.source_file_id = SourceFiles.source_file_id
    INNER LOOKUP JOIN Postings@by_trace_id
    ON TraceValues.trace_id = Postings.trace_id
WHERE
    SourceFiles.source_file = 'gs://perf-bucket/2020/02/08/11/testdata.json'
    AND
    Postings.tile_number= 0
ORDER BY
    Postings.trace_id;
