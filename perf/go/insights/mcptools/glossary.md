# Skia Perf glossary (perfinsights)

**Trace**: one time series of measurements, identified by a trace key of key=value params, e.g.
`,benchmark=speedometer3,bot=mac-m1,master=ChromiumPerf,test=Score,units=score,`.
`test.test_path` is master/bot/benchmark/test/subtests.

**Commit number**: Perf's sequential index of the commits on the tracked branch (for Chromium,
the commit position). Measurements are recorded per commit number.

**Anomaly**: a step change in a trace, or in a cluster of traces, that an alert config detected.
Perf stores anomalies as "regressions", even when the change is an improvement. Each anomaly has
an `id` (a UUID). `num_traces` > 1 means the anomaly covers a cluster of traces.

**direction**: `regression` (the metric got worse) or `improvement` (it got better), as decided
when the anomaly was detected.

**median_before, median_after, pct_change, abs_delta**: the medians of the trace before and after
the step. `pct_change` = (after - before) / |before| \* 100, or null when before is 0. Its sign is
the sign of the raw change, so whether a positive value is good depends on the metric: higher is
worse for times and sizes, better for scores. Use `direction` to know which it is.

**commit_range**: `[start, end]`, the commits that may contain the change. `start` is the commit
after the last data point before the step and `end` is the first data point after it. Any commit
in the range may be the cause; a wide range means sparse data. `display` is the commit the Perf UI
highlights.

**Sheriff (subscription)**: a team's sheriff config. Alert configs belong to a sheriff and
anomalies are reported to it, with its bug component. Use list_sheriffs for the exact names.

**Alert config**: the detection rule that flagged an anomaly: a trace `query`, an algorithm
(`stepfit` checks each trace; `kmeans` clusters traces first), the step detection method (`step`,
e.g. absolute, percent or cohen), the `threshold` (Perf's "interesting" value), the `radius` (the
number of points on each side of the step) and the `direction` to alert on (UP, DOWN or BOTH).

**Triage status**:

- `pending`: not grouped yet, the group can still grow, or auto-bisection is running.
- `untriaged`: needs the sheriff's attention.
- `triaged`: a bug was filed (manually or automatically) or bisection found a culprit.
- `ignored`: the sheriff decided it is not worth investigating (noise or expected).
- `none`: the anomaly has no triage state.

**bug_ids**: the issues filed for the anomaly, manually or by auto-triage and auto-bisection.

**Anomaly group**: anomalies that Perf grouped because they likely share a cause (same sheriff and
benchmark, overlapping commit ranges). Its `action` is NOACTION, REPORT (file a bug) or BISECT
(run a Pinpoint bisection).

**Culprit**: a commit that bisection identified as the cause of an anomaly group's regressions.

**window**: the data points around the anomaly, as stored when it was detected. When `normalized`
is true (k-means alerts) the values are a normalized cluster centroid, not raw measurements:
compare their shape, not their magnitude.

**overlapping_ranges** (in summaries): regressions whose commit ranges intersect. They may share
one root cause in the intersected range; this is a hint, not a proof.

**visibility**: `internal` data must not be shared outside the audience allowed for the instance.
