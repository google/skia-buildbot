

Example SQL Queries
-------------------
```sql
$ cockroach sql --insecure --database demo_gold_db
> SELECT encode(trace_hash, 'hex'), jsonb_pretty(keys) FROM TraceIDs;
> SELECT encode(trace_hash, 'hex'), jsonb_pretty(keys) FROM TraceIDs where keys @> '{"color mode": "GREY", "name": "triangle"}';
> SELECT keys FROM traceids WHERE trace_hash = x'47109b059f45e4f9d5ab61dd0199e2c9';
> SELECT commit_number, encode(digest, 'hex') FROM TraceValues WHERE trace_hash = x'47109b059f45e4f9d5ab61dd0199e2c9';

# Get trace data for grey triangle traces
> SELECT encode(TraceValues.trace_hash, 'hex') AS trace, commit_number, encode(TraceValues.digest, 'hex') AS digest
FROM
  TraceValues
JOIN
  TraceIDs
ON TraceValues.trace_hash = TraceIDs.trace_hash
AND TraceIDs.keys @> '{"color mode": "GREY", "name": "triangle"}';

> SELECT grouping_json, encode(digest, 'hex') from Expectations where label = 2;

# This triple JOIN scenario returns all traces that have a negative digest some time after
# commit_number 5 and match device=iPad6,3 [Needs some indexing because currently requires 3
# FULL_SCANs.]
> SELECT DISTINCT encode(TraceIDs.trace_hash, 'hex'), TraceIDs.keys FROM
  TraceIDs
JOIN
  TraceValues
ON TraceIDs.trace_hash = TraceValues.trace_hash
  AND TraceIDs.keys @> '{"device": "iPad6,3"}'
  AND TraceValues.commit_number > 5
JOIN
  Expectations
ON TraceIDs.keys @> Expectations.grouping_json
  AND Expectations.digest = TraceValues.digest
  AND Expectations.label = 2;
// Could one day add in the following clause
/*AND TraceValues.commit_number >= Expectations.start_index AND TraceValues.commit_number < Expectations.end_index*

# Select untriaged digests after commit_number 0 (i.e. digests that do not appear in expectations).
# This accounts for the case that digests of different groupings may be triaged differently.
# See https://stackoverflow.com/a/2973582
> SELECT DISTINCT encode(TraceValues.digest, 'hex') AS digest, TraceIDs.keys FROM
  TraceIDs
JOIN
  TraceValues
ON TraceIDs.trace_hash = TraceValues.trace_hash
  AND TraceIDs.keys @> '{}'
  AND TraceValues.commit_number > 0
JOIN
  Expectations
ON TraceIDs.keys @> Expectations.grouping_json
  AND NOT EXISTS (
  SELECT NULL
  FROM Expectations
  WHERE TraceIDs.keys @> Expectations.grouping_json AND TraceValues.digest = Expectations.digest
) OR (TraceIDs.keys @> Expectations.grouping_json
    AND TraceValues.digest = Expectations.digest
    AND Expectations.label = 0);

# Get the last 512 commit numbers where we have data. (i.e. get our Dense tile).
> SELECT DISTINCT TraceValues.commit_number from TraceValues WHERE
NOT EXISTS (
  SELECT NULL
  FROM Commits
  WHERE TraceValues.commit_number = Commits.commit_number
) ORDER BY TraceValues.commit_number DESC LIMIT 512;

# Get paramset of traces that have data before commit_number = 2
> SELECT DISTINCT keys from
  TraceIDs
JOIN
  TraceValues
ON TraceIDs.trace_hash = TraceValues.trace_hash
  AND TraceValues.commit_number < 2;

# Get all data from 3 specified traces.
> SELECT encode(digest, 'hex'), commit_number FROM TraceValues WHERE trace_hash
IN (x'796f2cc3f33fa6a9a1f4bef3aa9c48c4', x'3b44c31afc832ef9d1a2d25a5b873152', x'47109b059f45e4f9d5ab61dd0199e2c9')
AND commit_number >= 0;

# Get all unique digests in traces of a given grouping
> SELECT DISTINCT encode(digest, 'hex') FROM
  TraceValues
JOIN
  TraceIDs
ON TraceIDs.trace_hash = TraceValues.trace_hash
  AND TraceIDs.keys @> '{"color mode": "GREY","name":"triangle"}'
  AND commit_number >=0;

# Get closest positive digest to b01 (closest being defined by smallest num_diff_pixels)
# Could we get both positive and negative in one query? Sure, but it's a real pain:
# https://stackoverflow.com/a/7745635
> SELECT DISTINCT encode(left_digest, 'hex') as left, encode(right_digest, 'hex') as right,
  num_diff_pixels, max_rgba_diff, dimensions_differ
FROM
  TraceIDs
JOIN
  TraceValues
ON TraceIDs.trace_hash = TraceValues.trace_hash
  AND TraceIDs.keys @> '{}'
  AND TraceValues.commit_number > 0
JOIN
  DiffMetrics
ON (DiffMetrics.left_digest = TraceValues.digest
  OR DiffMetrics.right_digest = TraceValues.digest)
  AND (left_digest = x'b01b01b01b01b01b01b01b01b01b01b0'
  OR right_digest = x'b01b01b01b01b01b01b01b01b01b01b0')
  AND max_channel_diff >= 0 AND max_channel_diff <= 255
JOIN
  Expectations
ON TraceIDs.keys @> Expectations.grouping_json
  -- need to make sure the expectation label applies to the other digest (not the one we are
  -- looking for comparison to.
  AND Expectations.digest != x'b01b01b01b01b01b01b01b01b01b01b0'
  AND (Expectations.digest = DiffMetrics.left_digest
  OR Expectations.digest = DiffMetrics.right_digest)
  AND Expectations.label = 1
ORDER BY DiffMetrics.num_diff_pixels LIMIT 1;

# Get all digests broken down by test name.
> SELECT DISTINCT encode(TraceValues.digest, 'hex') AS digest,
  TraceIDs.keys->>'source_type' AS corpus, TraceIDs.keys->>'name' AS test_name
FROM
  TraceValues
JOIN
  TraceIDs
ON TraceValues.trace_hash = TraceIDs.trace_hash
  AND TraceValues.commit_number >= 0
ORDER BY corpus, test_name, digest;

# Get all digests broken down by test name and color_mode (Future growth of specifying keys).
> SELECT DISTINCT encode(TraceValues.digest, 'hex') AS digest,
  TraceIDs.keys->>'source_type' AS corpus, TraceIDs.keys->>'name' AS test_name, TraceIDs.keys->>'color mode' AS color_mode
FROM
  TraceValues
JOIN
  TraceIDs
ON TraceValues.trace_hash = TraceIDs.trace_hash
  AND TraceValues.commit_number >= 0
ORDER BY corpus, test_name, color_mode, digest;

SELECT commit_number, encode(TraceValues.digest, 'hex') AS digest, TraceIDs.keys ->> 'cpu_or_gpu'
FROM
  TraceValues
JOIN
  TraceIDs
ON TraceValues.trace_hash = TraceIDs.trace_hash
AND TraceIDs.keys @> '{"source_type": "canvaskit", "name": "1x4_from_scratch"}';

```
