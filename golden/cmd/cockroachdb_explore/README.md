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

WITH 
CombinedExpectations AS (
    -- Any triaging in the SecondaryBranch overrides the primary branch's expectations
    SELECT coalesce(SecondaryBranchExpectations.grouping_id, Expectations.grouping_id) AS grouping_id,
      coalesce(SecondaryBranchExpectations.digest, Expectations.digest) AS digest,
      coalesce(SecondaryBranchExpectations.label, Expectations.label, 0) AS label
    FROM 
      Expectations
    FULL OUTER JOIN -- Could be a MERGE join if needed for speed
      (SELECT * FROM SecondaryBranchExpectations
      WHERE SecondaryBranchExpectations.branch_name = 'gerrit_1000') AS SecondaryBranchExpectations
    ON SecondaryBranchExpectations.grouping_id = Expectations.grouping_id
      AND SecondaryBranchExpectations.digest = Expectations.digest
),
ValuesAndExpectations AS (
	-- Probably beneficial to also return trace values here
    SELECT DISTINCT SecondaryBranchValues.grouping_id, SecondaryBranchValues.digest,
                    coalesce(CombinedExpectations.label, 0) AS label
    FROM
       SecondaryBranchValues
    LEFT JOIN
       CombinedExpectations
    ON SecondaryBranchValues.grouping_id = CombinedExpectations.grouping_id 
      AND SecondaryBranchValues.digest = CombinedExpectations.digest
    WHERE SecondaryBranchValues.branch_name = 'gerrit_1000'
      AND SecondaryBranchValues.version_name = 'ps_1'
)
-- label being NULL here means it wasn't seen on the primary branch.
-- label being zero here means it was.
SELECT keys, encode(digest, 'hex') FROM 
  ValuesAndExpectations
JOIN
  Groupings
ON ValuesAndExpectations.grouping_id = Groupings.grouping_id
WHERE label = 0;
