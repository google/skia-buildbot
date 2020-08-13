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
COALESCE(ChangelistExpectations.digest, Expectations.digest) AS digest,
COALESCE(ChangelistExpectations.label, Expectations.label) AS label FROM
  (SELECT grouping_id, digest, label FROM ChangelistExpectations
   WHERE changelist_id = 'gerrit_internal_CL_new_tests') AS ChangelistExpectations
FULL OUTER JOIN
  Expectations
ON ChangelistExpectations.grouping_id = Expectations.grouping_id 
  AND ChangelistExpectations.digest = Expectations.digest
;

-- This is better than doing a WHERE label = 0 or label = 0 because it can use the indexes.
SELECT COALESCE(ChangelistExpectations.grouping_id, Expectations.grouping_id) AS grouping_id,
COALESCE(ChangelistExpectations.digest, Expectations.digest) AS digest FROM
  (SELECT grouping_id, digest, label FROM ChangelistExpectations
   WHERE changelist_id = 'gerrit_internal_CL_new_tests' AND label = 0) AS ChangelistExpectations
FULL OUTER JOIN
  (SELECT grouping_id, digest, label FROM Expectations@label_idx WHERE label = 0) AS Expectations
ON ChangelistExpectations.grouping_id = Expectations.grouping_id 
  AND ChangelistExpectations.digest = Expectations.digest;
