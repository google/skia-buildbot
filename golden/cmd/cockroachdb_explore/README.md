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

WITH 
ExclusiveCLExpectations AS (
  SELECT ChangelistExpectations.grouping_id, ChangelistExpectations.digest,
  CASE WHEN ChangelistExpectations.label = -1 -- -1 is a value meaning "fallthrough"
    THEN
      0 -- report untriaged since we know from the join this is not on the primary branch.
    ELSE
      ChangelistExpectations.label
  END AS label
   FROM
   (SELECT grouping_id, digest, label FROM ChangelistExpectations@changelist_idx
    WHERE changelist_id = 'gerrit_internal_CL_new_tests' AND (label = 0 OR label = -1)) AS ChangelistExpectations
  LEFT LOOKUP JOIN
    Expectations
  ON Expectations.grouping_id = ChangelistExpectations.grouping_id AND Expectations.digest = ChangelistExpectations.digest
  WHERE Expectations.digest IS NULL -- remove expectations that are on primary branch
)
SELECT DISTINCT encode(ChangelistValues.digest, 'hex') AS digest, ChangelistTraces.keys FROM
  ExclusiveCLExpectations
JOIN
  (SELECT digest, grouping_id, changelist_trace_id FROM ChangelistValues
   WHERE changelist_id = 'gerrit_internal_CL_new_tests' AND patchset_id = 'gerrit_internal_PS_adds_new_corpus') AS ChangelistValues
ON ChangelistValues.grouping_id = ExclusiveCLExpectations.grouping_id
  AND ChangelistValues.digest = ExclusiveCLExpectations.digest
JOIN
  (SELECT keys, changelist_trace_id FROM ChangelistTraces
   WHERE matches_any_ignore_rule = false) AS ChangelistTraces
ON ChangelistValues.changelist_trace_id = ChangelistTraces.changelist_trace_id;
