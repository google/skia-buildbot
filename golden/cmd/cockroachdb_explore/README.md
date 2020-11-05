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


-- Searching for Traces
SELECT encode(trace_id, 'hex'), keys FROM Traces
WHERE keys @> '{"Wisconsin": "Wisconsin_North_Carolina", "Nebraska": "Nebraska_Georgia"}' ORDER BY 1;

-- searching for untriaged not at head traces
SELECT DISTINCT encode(TraceValues.digest, 'hex') AS digest, Traces.keys FROM
  (SELECT trace_id, keys FROM Traces
   WHERE Traces.keys @> '{"source_type": "gm", "Wisconsin": "Wisconsin_North_Carolina"}'
     AND Traces.matches_any_ignore_rule = false) AS Traces
JOIN
  (SELECT trace_id, digest, grouping_id FROM TraceValues
   WHERE commit_id >= 40500) AS TraceValues
ON Traces.trace_id = TraceValues.trace_id
JOIN
  (SELECT grouping_id, digest FROM Expectations
   WHERE label = 0) AS Expectations
ON TraceValues.grouping_id = Expectations.grouping_id
  AND TraceValues.digest = Expectations.digest;

-- searching for traces with negative digests not counting ignore rules.
SELECT DISTINCT encode(TraceValues.digest, 'hex') AS digest, Traces.keys FROM
  (SELECT trace_id, keys FROM Traces
   WHERE Traces.keys @> '{"source_type": "gm", "Wisconsin": "Wisconsin_North_Carolina"}') AS Traces
JOIN
  (SELECT trace_id, digest, grouping_id FROM TraceValues
   WHERE commit_id >= 40500) AS TraceValues
ON Traces.trace_id = TraceValues.trace_id
JOIN
  (SELECT grouping_id, digest FROM Expectations
   WHERE label = 2) AS Expectations
ON TraceValues.grouping_id = Expectations.grouping_id
  AND TraceValues.digest = Expectations.digest;

-- Search for all traces in range with an untriaged digest (VERY SLOW)
SELECT DISTINCT encode(Traces.trace_id, 'hex') AS trace_id FROM
  (SELECT grouping_id, digest FROM Expectations
   WHERE label = 0) AS Expectations
INNER HASH JOIN
  (SELECT trace_id, digest, grouping_id FROM TraceValues
   WHERE commit_id >= 40900) AS TraceValues
ON TraceValues.grouping_id = Expectations.grouping_id
  AND TraceValues.digest = Expectations.digest
JOIN
  (SELECT trace_id, keys FROM Traces
   WHERE Traces.keys @> '{"source_type": "gm"}' AND Traces.matches_any_ignore_rule = false) AS Traces
ON Traces.trace_id = TraceValues.trace_id;

-- Look up closest positive and negative digest (fast enough)
SELECT DISTINCT ON (left_digest, label)
  label, encode(left_digest, 'hex') as left_digest, encode(right_digest, 'hex') as right_digest,
  num_diff_pixels, max_rgba_diff, dimensions_differ, ExpectationRecords.user_name
FROM
  (SELECT DISTINCT digest FROM TraceValues@grouping_commit_digest_idx
   WHERE TraceValues.commit_id > 40500
     AND TraceValues.grouping_id = x'b68676ec89b7c0735b01ba0465ee561b') AS TraceValues
JOIN
  (SELECT * FROM DiffMetrics
   WHERE DiffMetrics.left_digest IN (x'f1221d72c93e285f32a1fcf08c5386be', x'db5c60b105d005c132a1fcf08c5386be', x'1234')
     AND max_channel_diff >= 0 AND max_channel_diff <= 255) AS DiffMetrics
ON DiffMetrics.right_digest = TraceValues.digest
JOIN
  (SELECT digest, label, expectation_record_id FROM Expectations@group_label_idx
   WHERE label > 0
   AND Expectations.grouping_id = x'b68676ec89b7c0735b01ba0465ee561b') AS Expectations
ON Expectations.digest = DiffMetrics.right_digest
INNER LOOKUP JOIN
  ExpectationRecords
ON ExpectationRecords.expectation_record_id = Expectations.expectation_record_id
ORDER BY left_digest, label, num_diff_pixels ASC, max_channel_diff ASC, right_digest ASC;

-- Searching untriaged at head (fast)
SELECT encode(ValuesAtHead.trace_id, 'hex') AS trace_id, encode(ValuesAtHead.digest, 'hex') AS digest FROM
  ValuesAtHead@commit_ignored_label_idx
WHERE ValuesAtHead.expectation_label = 0 AND ValuesAtHead.most_recent_commit_id > 0
  AND matches_any_ignore_rule = false
ORDER BY 1, 2;

SELECT count(DISTINCT TraceValues.trace_id) FROM
  (SELECT grouping_id, digest FROM Expectations
   WHERE label = 0) AS Expectations
INNER JOIN
  (SELECT trace_id, digest, grouping_id FROM TraceValues
   WHERE commit_id >= 40900) AS TraceValues
ON TraceValues.grouping_id = Expectations.grouping_id
  AND TraceValues.digest = Expectations.digest;


-- Get all the closest digests for all untriaged digest in the given grouping
-- With about 100 digests, this takes about 12 seconds.
WITH
UntriagedDigests AS (
    SELECT digest FROM Expectations
    WHERE grouping_id = x'b68676ec89b7c0735b01ba0465ee561b' AND label = 0
),
PositiveOrNegativeDigests AS (
    SELECT digest, expectation_record_id, label FROM Expectations
    WHERE grouping_id = x'b68676ec89b7c0735b01ba0465ee561b' AND label > 0
),
TracesOfInterest AS (
  SELECT trace_id FROM Traces
  -- this is x'b68676ec89b7c0735b01ba0465ee561b' (should we have grouping_id) in Traces?
  WHERE Traces.keys @> '{"source_type": "gm", "name": "blend_modes_3347691812853273713"}'
    AND matches_any_ignore_rule = false
),
ObservedDigestsInTile AS (
    SELECT DISTINCT digest FROM TraceValues
    JOIN TracesOfInterest ON TraceValues.trace_id = TracesOfInterest.trace_id
    WHERE TraceValues.commit_id > 40500
),
ComparisonBetweenUntriagedAndObserved AS (
    SELECT DiffMetrics.* FROM DiffMetrics
    JOIN UntriagedDigests on DiffMetrics.left_digest = UntriagedDigests.digest
    JOIN ObservedDigestsInTile ON DiffMetrics.right_digest = ObservedDigestsInTile.digest
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
WHERE max_channel_diff >= 0 AND max_channel_diff <= 255
ORDER BY left_digest, label, num_diff_pixels ASC, max_channel_diff ASC, right_digest ASC;



-- Use the closest view table
WITH
UntriagedDigests AS (
    SELECT digest FROM Expectations
    WHERE grouping_id = x'b68676ec89b7c0735b01ba0465ee561b' AND label = 0
),
PositiveOrNegativeDigests AS (
    SELECT digest, expectation_record_id, label FROM Expectations
    WHERE grouping_id = x'b68676ec89b7c0735b01ba0465ee561b' AND label > 0
),
TracesOfInterest AS (
  SELECT trace_id FROM Traces
  -- this is x'b68676ec89b7c0735b01ba0465ee561b' (should we have grouping_id) in Traces?
  WHERE Traces.keys @> '{"source_type": "gm", "name": "blend_modes_3347691812853273713"}'
    AND matches_any_ignore_rule = false
),
ObservedDigestsInTile AS (
    SELECT DISTINCT digest FROM TraceValues
    JOIN TracesOfInterest ON TraceValues.trace_id = TracesOfInterest.trace_id
    WHERE TraceValues.commit_id > 40500
),
ComparisonBetweenUntriagedAndObserved AS (
    SELECT DiffMetricsClosestView.* FROM DiffMetricsClosestView
    JOIN UntriagedDigests on DiffMetricsClosestView.left_digest = UntriagedDigests.digest
    JOIN ObservedDigestsInTile ON DiffMetricsClosestView.right_digest = ObservedDigestsInTile.digest
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
WHERE max_channel_diff >= 0 AND max_channel_diff <= 255
ORDER BY left_digest, label, num_diff_pixels ASC, max_channel_diff ASC, right_digest ASC;


WITH
InitialRankings AS (
	SELECT DiffMetrics.*, dense_rank() over (
		PARTITION BY left_digest
		ORDER BY num_diff_pixels ASC, pixel_diff_percent ASC, max_channel_diff ASC
	) AS initialRank
	FROM DiffMetrics
	where left_digest = x'190851ccb5a7cac332a1fcf08c5386be'
),
DiffsWithLabels AS (
    SELECT DISTINCT InitialRankings.*, max(label) OVER(PARTITION BY right_digest) AS max_label
    FROM InitialRankings
    JOIN Expectations ON Expectations.digest = InitialRankings.right_digest
    WHERE initialRank < 100
),
RankedDiffs AS (
	SELECT DiffsWithLabels.*, dense_rank() over (
		PARTITION BY left_digest, max_label
		ORDER BY num_diff_pixels ASC, pixel_diff_percent ASC, max_channel_diff ASC
	) AS diffRank
	from DiffsWithLabels
),
TopOfEachLabel AS (
	SELECT RankedDiffs.*, dense_rank() OVER (
		PARTITION BY left_digest
		ORDER BY num_diff_pixels ASC, pixel_diff_percent ASC, max_channel_diff ASC
	) AS overallRank
	FROM RankedDiffs
	WHERE (RankedDiffs.max_label = 0 AND RankedDiffs.diffRank <= 3) OR
	      (RankedDiffs.max_label = 1 AND RankedDiffs.diffRank <= 5) OR
	      (RankedDiffs.max_label = 2 AND RankedDiffs.diffRank <= 2)
)
select * from TopOfEachLabel;
