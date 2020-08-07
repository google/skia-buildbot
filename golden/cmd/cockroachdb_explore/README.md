Explore Gold's data organization
--------------------------------

Running `cockroachdb_explore` with the default arguments will store a sample set of data into
a local single-node CockroachDB instance. Users are then encouraged to open an SQL shell and run
some queries to explore.
```
$ cockroach sql --insecure --database demo_gold_db
root@:26257/demo_gold_db> SHOW TABLES;
```

See `cockroachdb_explore_test.go` for some example SQL queries.
And 

Example SQL Queries
-------------------
```sql
$ cockroach sql --insecure --database demo_gold_db

-- This double JOIN scenario returns all traces that have a negative digest some time after
-- commit_number 5 and match device=iPad6,3
> SELECT DISTINCT encode(TraceValues.digest, 'hex') AS digest, Traces.keys FROM
  (SELECT trace_id, keys FROM Traces 
   WHERE keys @> '{"device": "iPad6,3"}') AS Traces
JOIN
  (SELECT trace_id, digest, grouping_id FROM TraceValues 
   WHERE commit_number > 5) AS TraceValues
ON Traces.trace_id = TraceValues.trace_id
JOIN
  (SELECT grouping_id, digest FROM Expectations 
   WHERE label = 2) AS Expectations
ON TraceValues.grouping_id = Expectations.grouping_id
  AND TraceValues.digest = Expectations.digest;

-- Could one day add in the following clause
AND TraceValues.commit_number >= Expectations.start_index AND TraceValues.commit_number < Expectations.end_index

-- Select untriaged digests after commit_number 0 (i.e. digests that do not appear in expectations).
-- This accounts for the case that digests of different groupings may be triaged differently.
-- Note: for the demo case, this shows a FULL TABLE SCAN for Traces, but the cost-based
-- analyzer should correctly turn that into a "Lookup Join" with full-sized data.
> SELECT DISTINCT encode(TraceValues.digest, 'hex') AS digest, Traces.keys FROM
    Traces
  JOIN
    (SELECT trace_id, digest, grouping_id FROM TraceValues 
     WHERE commit_number > 0) AS TraceValues
  ON Traces.trace_id = TraceValues.trace_id
  JOIN
    (SELECT grouping_id, digest FROM Expectations 
     WHERE label = 0) AS Expectations
  ON TraceValues.grouping_id = Expectations.grouping_id
    AND TraceValues.digest = Expectations.digest;

-- Get the last 512 commit numbers where we have data. (i.e. get our Dense tile).
> SELECT DISTINCT TraceValues.commit_number FROM TraceValues 
  WHERE TraceValues.commit_number >= 0
  ORDER BY TraceValues.commit_number DESC LIMIT 512;

-- Get paramset of traces that have data before commit_number = 2
-- Note: requires full table scan of Traces and there's not much we can do about it, other
-- than caching.
> SELECT DISTINCT keys FROM
  Traces
JOIN
  TraceValues
ON Traces.trace_id = TraceValues.trace_id
  AND TraceValues.commit_number < 2;

-- Get all data from 3 specified traces.
> SELECT encode(digest, 'hex'), commit_number FROM TraceValues WHERE trace_id
IN (x'796f2cc3f33fa6a9a1f4bef3aa9c48c4', x'3b44c31afc832ef9d1a2d25a5b873152', x'47109b059f45e4f9d5ab61dd0199e2c9')
AND commit_number >= 0;

-- Get all unique digests in traces of a given search query
> SELECT DISTINCT encode(digest, 'hex') FROM
  TraceValues
JOIN
  Traces
ON Traces.trace_id = TraceValues.trace_id
  AND Traces.keys @> '{"color mode": "GREY","name":"triangle"}'
  AND commit_number >=0;

-- Get closest digests (positive and negative) to digest 000... and b02... searching in the entire
-- grouping with hash aa8d3... (this is {"name": "triangle", "source_type": "corners"})
> SELECT DISTINCT label, encode(left_digest, 'hex') as left, encode(right_digest, 'hex') as right,
  num_diff_pixels, max_rgba_diff, dimensions_differ
FROM
  (SELECT digest FROM TraceValues 
   WHERE TraceValues.commit_number > 0 
     AND TraceValues.grouping_id = x'aa8d3c14238a4f717b9a99f7fe3735a7') AS TraceValues
JOIN
  (SELECT * FROM DiffMetrics
   WHERE DiffMetrics.left_digest IN (x'00000000000000000000000000000000', x'b02b02b02b02b02b02b02b02b02b02b0')
     AND max_channel_diff >= 0 AND max_channel_diff <= 255) AS DiffMetrics
ON DiffMetrics.right_digest = TraceValues.digest
JOIN
  (SELECT digest, label FROM Expectations
   WHERE label > 0 
   AND Expectations.grouping_id = x'aa8d3c14238a4f717b9a99f7fe3735a7') AS Expectations
ON Expectations.digest = DiffMetrics.right_digest
ORDER BY 2, label, num_diff_pixels;

-- As previous example except restrict right-hand side to be "color mode": "GREY"
> SELECT DISTINCT label, encode(left_digest, 'hex') as left, encode(right_digest, 'hex') as right,
  num_diff_pixels, max_rgba_diff, dimensions_differ
FROM
  (SELECT digest, trace_id FROM TraceValues 
   WHERE TraceValues.commit_number > 0 
     AND TraceValues.grouping_id = x'aa8d3c14238a4f717b9a99f7fe3735a7') AS TraceValues
JOIN
  (SELECT trace_id FROM Traces
   WHERE keys @> '{"name": "triangle", "source_type": "corners", "color mode": "GREY"}') AS Traces
ON TraceValues.trace_id = Traces.trace_id
JOIN
  (SELECT * FROM DiffMetrics
   WHERE DiffMetrics.left_digest IN (x'00000000000000000000000000000000', x'b02b02b02b02b02b02b02b02b02b02b0')
     AND max_channel_diff >= 0 AND max_channel_diff <= 255) AS DiffMetrics
ON DiffMetrics.right_digest = TraceValues.digest
JOIN
  (SELECT digest, label FROM Expectations
   WHERE label > 0 
   AND Expectations.grouping_id = x'aa8d3c14238a4f717b9a99f7fe3735a7') AS Expectations
ON Expectations.digest = DiffMetrics.right_digest
ORDER BY 2, label, num_diff_pixels;


-- Get all digests broken down by test name.
> SELECT DISTINCT encode(TraceValues.digest, 'hex') AS digest,
  Traces.keys->>'source_type' AS corpus, Traces.keys->>'name' AS test_name
FROM
  TraceValues
JOIN
  Traces
ON TraceValues.trace_id = Traces.trace_id
  AND TraceValues.commit_number >= 0
ORDER BY corpus, test_name, digest;

-- Get all digests broken down by test name and color_mode (Future growth of specifying keys).
> SELECT DISTINCT encode(TraceValues.digest, 'hex') AS digest,
  Traces.keys->>'source_type' AS corpus, Traces.keys->>'name' AS test_name, Traces.keys->>'color mode' AS color_mode
FROM
  TraceValues
JOIN
  Traces
ON TraceValues.trace_id = Traces.trace_id
  AND TraceValues.commit_number >= 0
ORDER BY corpus, test_name, color_mode, digest;
```

Notes for performance:
INNER JOIN might need hints to be fast
for example, INNER LOOKUP JOIN is waaay faster on some of Perf's queries
https://www.cockroachlabs.com/docs/v20.1/cost-based-optimizer.html#join-hints

Doing the query in JSONB and not accidentally changing it to a string speeds it up
`TraceNames.params ->> 'arch' IN ('x86')`
is slower than
`TraceNames.params -> 'arch' IN ('"x86"'::JSONB)`


-- How to get ignored traces? Ingesters download the ignore rules when they see a new trace
-- (in a transaction) and then add the trace_keys to the Traces table with a true or false if it
-- is ignored. When ignore rules change, they modify that column for all traces.
--
-- Alternatively, we just have a process that searches for Traces in Traces where
-- matches_ignore_rule is NULL and then it updates those.

Select * from Traces 
WHERE (
    Traces.keys -> 'device' IN ('"taimen"'::JSONB) AND
    Traces.keys -> 'name' IN ('"square"'::JSONB, '"circle"'::JSONB)
  ) OR (
     Traces.keys -> 'device' IN ('"Nokia4"'::JSONB) AND
     Traces.keys -> 'source_type' IN ('"corners"'::JSONB)
  );
  
   
