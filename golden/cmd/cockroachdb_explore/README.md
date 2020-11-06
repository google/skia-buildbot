Explore Gold's data organization
================================

Running `cockroachdb_explore` with the default arguments will store a sample set of data into
a local single-node CockroachDB instance. After, Users should open an SQL shell and run
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
UntriagedDigests AS (
    SELECT digest FROM Expectations
    WHERE grouping_id = x'0f2ffd3aef866dc6155bcbc5697b0604' AND label = 0
),
PositiveOrNegativeDigests AS (
    SELECT digest, expectation_record_id, label FROM Expectations
    WHERE grouping_id = x'0f2ffd3aef866dc6155bcbc5697b0604' AND label > 0
),
TracesOfInterest AS (
  SELECT trace_id FROM Traces
  WHERE Traces.keys @> '{"source_type": "corners", "name": "square"}'
   -- Traces.grouping_id = x'0f2ffd3aef866dc6155bcbc5697b0604'
   AND matches_any_ignore_rule = false
),
ObservedDigestsInTile AS (
    SELECT DISTINCT digest FROM TraceValues
    JOIN TracesOfInterest ON TraceValues.trace_id = TracesOfInterest.trace_id
    WHERE TraceValues.commit_id > 0
),
ComparisonBetweenUntriagedAndObserved AS (
    SELECT DiffMetrics.* FROM DiffMetrics
    JOIN UntriagedDigests on DiffMetrics.left_digest = UntriagedDigests.digest
    JOIN ObservedDigestsInTile ON DiffMetrics.right_digest = ObservedDigestsInTile.digest
    WHERE DiffMetrics.max_channel_diff >= 0 AND DiffMetrics.max_channel_diff <= 255
)
SELECT DISTINCT ON (left_digest, label)
  label, encode(left_digest, 'hex') as left_digest, encode(right_digest, 'hex') as right_digest,
  num_diff_pixels, max_rgba_diff, dimensions_differ, ExpectationRecords.user_name
FROM
  ComparisonBetweenUntriagedAndObserved
JOIN PositiveOrNegativeDigests
  ON ComparisonBetweenUntriagedAndObserved.right_digest = PositiveOrNegativeDigests.digest
INNER LOOKUP JOIN ExpectationRecords
ON ExpectationRecords.expectation_record_id = PositiveOrNegativeDigests.expectation_record_id
ORDER BY left_digest, label, num_diff_pixels ASC, max_channel_diff ASC, right_digest ASC;