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

SELECT DISTINCT encode(TryJobValues.digest, 'hex') AS digest, TryjobTraces.keys FROM
  (SELECT grouping_id, digest FROM Expectations 
   WHERE label = 0) AS Expectations -- 0 means untriaged
ON TraceValues.grouping_id = Expectations.grouping_id
  AND TraceValues.digest = Expectations.digest
  
  (SELECT trace_id, keys FROM Traces 
   WHERE Traces.keys @> '{"source_type": "round", "color mode": "RGB"}' 
     AND Traces.matches_any_ignore_rule = false) AS Traces
JOIN
  (SELECT trace_id, digest, grouping_id FROM TraceValues@trace_and_commit_idx 
   WHERE commit_id >= 1) AS TraceValues -- This range is just to show it possible
ON Traces.trace_id = TraceValues.trace_id
JOIN
  (SELECT grouping_id, digest FROM Expectations 
   WHERE label = 0) AS Expectations -- 0 means untriaged
ON TraceValues.grouping_id = Expectations.grouping_id
  AND TraceValues.digest = Expectations.digest;

-- Combining a CL's expectations with primary branch
SELECT COALESCE(ChangelistExpectations.grouping_id, Expectations.grouping_id) AS grouping_id,
encode(COALESCE(ChangelistExpectations.digest, Expectations.digest), 'hex') AS digest,
CASE WHEN ChangelistExpectations.label = -1
  THEN
    COALESCE(Expectations.label, 0)
  ELSE
    COALESCE(ChangelistExpectations.label, Expectations.label)
END AS label FROM
  (SELECT grouping_id, digest, label FROM ChangelistExpectations
   WHERE changelist_id = 'gerrit_internal_CL_new_tests') AS ChangelistExpectations
FULL OUTER JOIN
  Expectations
ON ChangelistExpectations.grouping_id = Expectations.grouping_id 
  AND ChangelistExpectations.digest = Expectations.digest;


-- A better way to get untriaged grouping and digests from both the CL expectations and the
-- primary branch
WITH 
  ProbablyUntriagedFromCL AS (
    SELECT ChangelistExpectations.grouping_id, ChangelistExpectations.digest,
    CASE WHEN ChangelistExpectations.label = -1
      THEN
        COALESCE(Expectations.label, 0)
      ELSE
        COALESCE(ChangelistExpectations.label, Expectations.label)
    END AS label
     FROM
     (SELECT grouping_id, digest, label FROM ChangelistExpectations@changelist_idx
      WHERE changelist_id = 'gerrit_CL_fix_ios' AND (label = 0 OR label = -1)) AS ChangelistExpectations
    LEFT LOOKUP JOIN
      Expectations
    ON Expectations.grouping_id = ChangelistExpectations.grouping_id AND Expectations.digest = ChangelistExpectations.digest
  )
SELECT grouping_id, digest FROM
  (SELECT grouping_id, digest FROM ProbablyUntriagedFromCL
   WHERE label = 0) AS ChangelistExpectations
UNION
  SELECT grouping_id, digest FROM Expectations@label_idx WHERE label = 0;

SELECT Groupings.keys, encode(JoinedExpectations.digest, 'hex') FROM
  (WITH 
    ProbablyUntriagedFromCL AS (
      SELECT ChangelistExpectations.grouping_id, ChangelistExpectations.digest,
      CASE WHEN ChangelistExpectations.label = -1
        THEN
          COALESCE(Expectations.label, 0)
        ELSE
          COALESCE(ChangelistExpectations.label, Expectations.label)
      END AS label
       FROM
       (SELECT grouping_id, digest, label FROM ChangelistExpectations@changelist_idx
        WHERE changelist_id = 'gerrit_CL_fix_ios' AND (label = 0 OR label = -1)) AS ChangelistExpectations
      LEFT LOOKUP JOIN
        Expectations
      ON Expectations.grouping_id = ChangelistExpectations.grouping_id AND Expectations.digest = ChangelistExpectations.digest
    )
  SELECT grouping_id, digest FROM
    (SELECT grouping_id, digest FROM ProbablyUntriagedFromCL
     WHERE label = 0) AS ChangelistExpectations
  UNION
    SELECT grouping_id, digest FROM Expectations@label_idx WHERE label = 0
  ) AS JoinedExpectations
JOIN
  Groupings
ON JoinedExpectations.grouping_id = Groupings.grouping_id;
