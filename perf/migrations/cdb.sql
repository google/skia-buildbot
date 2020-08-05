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
UPSERT INTO TraceNames (trace_id, params)
VALUES
    (
        '\xfe385b159ff55dca481069805e5ff050',
        '{"arch": "x86",   "config":"8888"}'
    ),
    (
        '\x277262a9236d571883d47dab102070bc',
        '{"arch":"x86",   "config": "565"}'
    ),
    (
        '\x0f17700460ee99c6488c2f6130804de5',
        '{"arch":"arm",   "config":"8888"}'
    ),
    (
        '\x6a5622e86c6059d74373f6a79df96054',
        '{"arch":"arm",   "config":"565"}'
    ),
    (
        '\x0d1f35f01672b2105bbc3f19adfcef67',
        '{"arch":"riscv",   "config":"565"}'
    );

UPSERT INTO TraceValues2 (trace_id, commit_number, val, source_file_id)
VALUES
    ('\xfe385b159ff55dca481069805e5ff050', 1, 1.2, 1),
    ('\xfe385b159ff55dca481069805e5ff050', 2, 1.3, 2),
    ('\xfe385b159ff55dca481069805e5ff050', 3, 1.4, 3),
    (
        '\x0d1f35f01672b2105bbc3f19adfcef67',
        256,
        1.1,
        4
    ),
    ('\x277262a9236d571883d47dab102070bc', 1, 2.2, 1),
    ('\x277262a9236d571883d47dab102070bc', 2, 2.3, 2),
    ('\x277262a9236d571883d47dab102070bc', 3, 2.4, 3),
    (
        '\x0d1f35f01672b2105bbc3f19adfcef67',
        256,
        2.1,
        4
    );

-- All trace_ids that match a particular key=value.
SELECT
    encode(trace_id, 'hex'),
    params
FROM
    TraceNames
WHERE
    params ->> 'arch' IN ('x86');

-- Retrieve matching values.
SELECT
    encode(trace_id, 'hex'),
    commit_number,
    val
FROM
    TraceValues2
WHERE
    commit_number >= 0
    AND commit_number < 255
    AND trace_id IN (
        '\xfe385b159ff55dca481069805e5ff050',
        '\x277262a9236d571883d47dab102070bc'
    );

-- Compound queries.
SELECT
    params
FROM
    TraceNames
WHERE
    params ->> 'arch' IN ('x86', 'arm')
    AND params ->> 'config' IN ('8888');

-- ParamSet for a tile.
SELECT
    DISTINCT TraceNames.params
FROM
    TraceNames INNER LOOKUP
    JOIN Tiles ON TraceNames.trace_id = Tiles.trace_id
WHERE
    Tiles.tile_number = 2
LIMIT
    10;

-- Count traces per tile.
SELECT
    COUNT(trace_id)
FROM
    Tiles
WHERE
    tile_number = 3;

-- Most recent commit.
SELECT
    commit_number
FROM
    TraceValues2
ORDER BY
    commit_number DESC
LIMIT
    1;

-- GetSource by trace_id.
SELECT
    SourceFiles.source_file
FROM
    TraceNames
    INNER JOIN TraceValues2 ON TraceValues2.trace_id = TraceNames.trace_id
    INNER JOIN SourceFiles ON SourceFiles.source_file_id = TraceValues2.source_file_id
WHERE
    TraceNames.trace_id = '\xfe385b159ff55dca481069805e5ff050'
    AND TraceValues2.commit_number = 3;

-- Count the number of matches to a query.
SELECT
    COUNT(*)
FROM
    TraceNames
    INNER JOIN TraceValues2 ON TraceNames.trace_id = TraceValues2.trace_id
WHERE
    TraceNames.params -> 'arch' IN ('"riscv"' :: JSONB)
    AND TraceValues2.commit_number >= 256
    AND TraceValues2.commit_number < 512;

-- Fully query traces from tile based on query plan w/o the sub-select.
SELECT
    TraceNames.params,
    TraceValues2.commit_number,
    TraceValues2.val
FROM
    TraceNames
    INNER JOIN TraceValues2 ON TraceValues2.trace_id = TraceNames.trace_id
WHERE
    TraceValues2.commit_number >= 0
    AND TraceValues2.commit_number < 255
    AND TraceNames.params -> 'arch' IN ('"x86"' :: JSONB)
    AND TraceNames.params -> 'config' IN ('"565"' :: JSONB, '"8888"' :: JSONB);

-- This is fast with PRIMARY KEY (trace_id, commit_number)
SELECT
    tracenames.params,
    tracevalues2.commit_number,
    tracevalues2.val
FROM
    TraceValues2 INNER LOOKUP
    JOIN TraceNames ON tracevalues2.trace_id = tracenames.trace_id
WHERE
    tracevalues2.commit_number >= 47920
    AND tracevalues2.commit_number < 49950
    AND tracevalues2.trace_id IN (
        SELECT
            DISTINCT trace_id
        FROM
            tracenames
        WHERE
            params -> 'name' = '"AndroidCodec_01_original.jpg_SampleSize2"' :: JSONB
    );

-- Create the Tile table on the fly if we haven't ingested it.
INSERT INTO
    Tiles (tile_number, trace_id)
SELECT
    DISTINCT mod(commit_number, 256),
    trace_id
FROM
    tracevalues2 ON CONFLICT DO NOTHING;