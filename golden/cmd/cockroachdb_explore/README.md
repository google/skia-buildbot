

Example SQL Queries
-------------------
```sql
$ cockroach sql --insecure --database demo_gold_db
> SELECT encode(keys_hash, 'hex'), jsonb_pretty(keys) FROM KeyValueMaps where keys @> '{"color mode": "GREY", "name": "triangle"}';
> SELECT keys FROM KeyValueMaps WHERE keys_hash = x'47109b059f45e4f9d5ab61dd0199e2c9';
> SELECT commit_number, encode(digest, 'hex') FROM TraceValues WHERE trace_hash = x'47109b059f45e4f9d5ab61dd0199e2c9';

# Get trace data for grey triangle traces
> SELECT encode(TraceValues.trace_hash, 'hex') AS trace, TraceValues.commit_number, 
  encode(TraceValues.digest, 'hex') AS digest
FROM
  TraceValues
JOIN
  (SELECT keys_hash FROM KeyValueMaps 
   WHERE KeyValueMaps.keys @> '{"color mode": "GREY", "name": "triangle"}') AS KeyValueMaps
ON TraceValues.trace_hash = KeyValueMaps.keys_hash;

> SELECT KeyValueMaps.keys, encode(digest, 'hex') FROM 
  (SELECT digest, grouping_hash FROM Expectations 
   WHERE label = 2) AS Expectations -- 2 means negative
JOIN
  KeyValueMaps
ON Expectations.grouping_hash = KeyValueMaps.keys_hash;

-- This double JOIN scenario returns all traces that have a negative digest some time after
-- commit_number 5 and match device=iPad6,3
> SELECT DISTINCT encode(TraceValues.digest, 'hex') AS digest, KeyValueMaps.keys FROM
  (SELECT keys_hash, keys FROM KeyValueMaps 
   WHERE keys @> '{"device": "iPad6,3"}') AS KeyValueMaps
JOIN
  (SELECT trace_hash, digest, grouping_hash FROM TraceValues 
   WHERE commit_number > 5) AS TraceValues
ON KeyValueMaps.keys_hash = TraceValues.trace_hash
JOIN
  (SELECT grouping_hash, digest FROM Expectations 
   WHERE label = 2) AS Expectations
ON TraceValues.grouping_hash = Expectations.grouping_hash
  AND TraceValues.digest = Expectations.digest;

-- Could one day add in the following clause
AND TraceValues.commit_number >= Expectations.start_index AND TraceValues.commit_number < Expectations.end_index

-- Select untriaged digests after commit_number 0 (i.e. digests that do not appear in expectations).
-- This accounts for the case that digests of different groupings may be triaged differently.
-- Note: for the demo case, this shows a FULL TABLE SCAN for KeyValueMaps, but the cost-based
-- analyzer should correctly turn that into a "Lookup Join" with full-sized data.
> SELECT DISTINCT encode(TraceValues.digest, 'hex') AS digest, KeyValueMaps.keys FROM
    KeyValueMaps
  JOIN
    (SELECT trace_hash, digest, grouping_hash FROM TraceValues 
     WHERE commit_number > 0) AS TraceValues
  ON KeyValueMaps.keys_hash = TraceValues.trace_hash
  JOIN
    (SELECT grouping_hash, digest FROM Expectations 
     WHERE label = 0) AS Expectations
  ON TraceValues.grouping_hash = Expectations.grouping_hash
    AND TraceValues.digest = Expectations.digest;

-- Get the last 512 commit numbers where we have data. (i.e. get our Dense tile).
> SELECT DISTINCT TraceValues.commit_number from TraceValues WHERE
EXISTS (
  SELECT NULL
  FROM Commits
  WHERE TraceValues.commit_number = Commits.commit_number
    AND TraceValues.commit_number > 0
) ORDER BY TraceValues.commit_number DESC LIMIT 512;

-- Get paramset of traces that have data before commit_number = 2
-- Note: requires full table scan of KeyValueMaps and there's not much we can do about it, other
-- than caching.
> SELECT DISTINCT keys from
  KeyValueMaps
JOIN
  TraceValues
ON KeyValueMaps.keys_hash = TraceValues.trace_hash
  AND TraceValues.commit_number < 2;

-- Get all data from 3 specified traces.
> SELECT encode(digest, 'hex'), commit_number FROM TraceValues WHERE trace_hash
IN (x'796f2cc3f33fa6a9a1f4bef3aa9c48c4', x'3b44c31afc832ef9d1a2d25a5b873152', x'47109b059f45e4f9d5ab61dd0199e2c9')
AND commit_number >= 0;

-- Get all unique digests in traces of a given search query
> SELECT DISTINCT encode(digest, 'hex') FROM
  TraceValues
JOIN
  KeyValueMaps
ON KeyValueMaps.keys_hash = TraceValues.trace_hash
  AND KeyValueMaps.keys @> '{"color mode": "GREY","name":"triangle"}'
  AND commit_number >=0;

-- Get closest digests (positive and negative) to digest 000... and b02... searching in the entire
-- grouping with hash aa8d3... (this is {"name": "triangle", "source_type": "corners"})
> SELECT DISTINCT label, encode(left_digest, 'hex') as left, encode(right_digest, 'hex') as right,
  num_diff_pixels, max_rgba_diff, dimensions_differ
FROM
  (SELECT digest FROM TraceValues 
   WHERE TraceValues.commit_number > 0 
     AND TraceValues.grouping_hash = x'aa8d3c14238a4f717b9a99f7fe3735a7') AS TraceValues
JOIN
  (SELECT * FROM DiffMetrics
   WHERE DiffMetrics.left_digest IN (x'00000000000000000000000000000000', x'b02b02b02b02b02b02b02b02b02b02b0')
     AND max_channel_diff >= 0 AND max_channel_diff <= 255) AS DiffMetrics
ON DiffMetrics.right_digest = TraceValues.digest
JOIN
  (SELECT digest, label FROM Expectations
   WHERE label > 0 
   AND Expectations.grouping_hash = x'aa8d3c14238a4f717b9a99f7fe3735a7') AS Expectations
ON Expectations.digest = DiffMetrics.right_digest
ORDER BY 2, label, num_diff_pixels;

-- As previous example except restrict right-hand side to be "color mode": "GREY"
> SELECT DISTINCT label, encode(left_digest, 'hex') as left, encode(right_digest, 'hex') as right,
  num_diff_pixels, max_rgba_diff, dimensions_differ
FROM
  (SELECT digest, trace_hash FROM TraceValues 
   WHERE TraceValues.commit_number > 0 
     AND TraceValues.grouping_hash = x'aa8d3c14238a4f717b9a99f7fe3735a7') AS TraceValues
JOIN
  (SELECT keys_hash FROM KeyValueMaps
   WHERE keys @> '{"name": "triangle", "source_type": "corners", "color mode": "GREY"}') AS KeyValueMaps
ON TraceValues.trace_hash = KeyValueMaps.keys_hash
JOIN
  (SELECT * FROM DiffMetrics
   WHERE DiffMetrics.left_digest IN (x'00000000000000000000000000000000', x'b02b02b02b02b02b02b02b02b02b02b0')
     AND max_channel_diff >= 0 AND max_channel_diff <= 255) AS DiffMetrics
ON DiffMetrics.right_digest = TraceValues.digest
JOIN
  (SELECT digest, label FROM Expectations
   WHERE label > 0 
   AND Expectations.grouping_hash = x'aa8d3c14238a4f717b9a99f7fe3735a7') AS Expectations
ON Expectations.digest = DiffMetrics.right_digest
ORDER BY 2, label, num_diff_pixels;


-- Get all digests broken down by test name.
> SELECT DISTINCT encode(TraceValues.digest, 'hex') AS digest,
  KeyValueMaps.keys->>'source_type' AS corpus, KeyValueMaps.keys->>'name' AS test_name
FROM
  TraceValues
JOIN
  KeyValueMaps
ON TraceValues.trace_hash = KeyValueMaps.keys_hash
  AND TraceValues.commit_number >= 0
ORDER BY corpus, test_name, digest;

-- Get all digests broken down by test name and color_mode (Future growth of specifying keys).
> SELECT DISTINCT encode(TraceValues.digest, 'hex') AS digest,
  KeyValueMaps.keys->>'source_type' AS corpus, KeyValueMaps.keys->>'name' AS test_name, KeyValueMaps.keys->>'color mode' AS color_mode
FROM
  TraceValues
JOIN
  KeyValueMaps
ON TraceValues.trace_hash = KeyValueMaps.keys_hash
  AND TraceValues.commit_number >= 0
ORDER BY corpus, test_name, color_mode, digest;
```
