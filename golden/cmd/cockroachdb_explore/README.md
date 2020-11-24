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


Current Goal
------------
- Import data at 1% scale (30 tests) to test all-TSV imports with almost no indexes. [done]
- While at 1%, write some go code that emulates a typical search flow for all untriaged digests
  in a given corpus, getting all the data we would need for the frontend UI. Check indexes
- Import at 10% scale (300 tests). Re-run search flow, and look at indexes again.
- Codify some of these steps (especially diff metrics) into cockroach explore tests.
- Reimport at 10% scale to make sure indexing works.
- Import at 100% scale (3000 tests). 
