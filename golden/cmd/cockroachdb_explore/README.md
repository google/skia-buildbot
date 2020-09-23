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

WITH ToUpdate AS (
SELECT decode(key, 'hex') AS trace_id, decode(value, 'hex') AS new_digest
FROM json_each_text('{"c02c02c02c02c02c02c02c02c02c02c0": "b03b03b03b03b03b03b03b03b03b03b0",
                      "c07c07c07c07c07c07c07c07c07c07c0": "a08a08a08a08a08a08a08a08a08a08a0"}')
)
UPDATE ValuesAtHead
SET
  digest = CASE
WHEN (ValuesAtHead.most_recent_commit_id < 37) THEN
  new_digest
ELSE
  digest
END,
  most_recent_commit_id = CASE
WHEN (most_recent_commit_id < 37) THEN
  37
ELSE
  most_recent_commit_id
END
FROM ToUpdate
WHERE ValuesAtHead.trace_id = ToUpdate.trace_id
RETURNING NOTHING;
