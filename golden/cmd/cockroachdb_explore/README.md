Explore Gold's data organization
================================

Running `cockroachdb_explore` with the default arguments will store a sample set of data into
a local single-node CockroachDB instance. Users are then encouraged to open an SQL shell and run
some queries to explore.
```
$ cockroach sql --insecure --database demo_gold_db
root@:26257/demo_gold_db> SHOW TABLES;
```

See `cockroachdb_explore_test.go` for some example SQL queries.

Notes for performance
---------------------
INNER JOIN might need hints to be fast
for example, INNER LOOKUP JOIN is waaay faster on some of Perf's queries
https://www.cockroachlabs.com/docs/v20.1/cost-based-optimizer.html#join-hints

Doing the query in JSONB and not accidentally changing it to a string speeds it up
`TraceNames.params ->> 'arch' IN ('x86')`
is slower than
`TraceNames.params -> 'arch' IN ('"x86"'::JSONB)`

OR queries on JSONB columns are always full table scans (probably better to trigger multiple
in parallel, or use UNION) https://github.com/cockroachdb/cockroach/issues/47340

SELECT DISTINCT encode(grouping_id, 'hex'), encode(digest, 'hex'),
    COUNT(*) OVER (PARTITION BY grouping_id, digest) AS unique_digests
FROM TraceValues
WHERE commit_id > 40990
LIMIT 100;

WITH 
GroupingsAndDigests AS (
    SELECT DISTINCT grouping_id, digest
    FROM TraceValues@commit_grouping_digest_idx
    WHERE commit_id > 40500
),
LabeledDigests AS (
    SELECT GroupingsAndDigests.*, Expectations.label
    FROM GroupingsAndDigests
    JOIN Expectations
    ON GroupingsAndDigests.digest = Expectations.digest 
      AND GroupingsAndDigests.grouping_id = Expectations.grouping_id
)
SELECT DISTINCT grouping_id, label,
    COUNT(*) OVER(PARTITION by grouping_id, label) 
    FROM LabeledDigests
LIMIT 100;