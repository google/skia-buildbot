# Architecture & Engineering Guide: Regressions2 Storage Indexing, K-Means Multi-Trace Incompatibility, and Sparse Anomaly Detection

## 1. Executive Summary

This reference guide addresses two critical architectural aspects of Skia Perf (`go.skia.org/infra/perf`):

1. **The K-Means Multi-Trace Incompatibility Trap in `regressions2store`**: How Cloud Spanner's `Regressions2.trace_id` indexing dramatically accelerates queries, why K-Means' multi-trace cluster regressions are architecturally incompatible with this indexing, and how this incompatibility causes silent query omissions (false negatives), heavy unindexed JSONB table scans, or write-path key overwrites.
2. **Sparse Traces & Wide Commit Range $[a; b]$ Anomaly Detection**: Why K-Means fundamentally fails on sparse traces and wide commit ranges (midpoint sentinel drops, `tooMuchMissingData` filtering, linear interpolation distortion via `vec32.Fill`, and dense matrix memory bloat), why `stepFitDfTraceSlicer` / `StepFit` is uniquely suited for sparse observations (dense observation filtering, 0% missing sentinels, exact commit boundary attribution, streaming memory), and an architectural blueprint for implementing targeted anomaly detection over wide commit ranges $[a; b]$.

---

## 2. Cloud Spanner `Regressions2` Schema & Indexing Mechanism

### 2.1 Schema Definition & Indices

The `Regressions2` schema is defined in [`perf/go/regression/sqlregression2store/schema/schema.go:10-92`](../../go/regression/sqlregression2store/schema/schema.go#L10-L92) and initialized in Cloud Spanner via [`perf/go/sql/spanner/schema_spanner.go:107-127, 210-216`](../../go/sql/spanner/schema_spanner.go#L107-L127):

```sql
CREATE TABLE IF NOT EXISTS Regressions2 (
  id TEXT PRIMARY KEY DEFAULT spanner.generate_uuid(),
  commit_number INT,
  prev_commit_number INT,
  display_commit_number INT,
  alert_id INT,
  sub_name TEXT,
  bug_id INT,
  creation_time TIMESTAMPTZ DEFAULT now(),
  median_before REAL,
  median_after REAL,
  is_improvement BOOL,
  cluster_type TEXT,
  cluster_summary JSONB,
  frame JSONB,
  legacy_key TEXT,
  trace_id BYTEA,
  triage_status TEXT,
  triage_message TEXT,
  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) TTL INTERVAL '1095 days' ON createdat;

-- Dedicated Secondary Indices:
CREATE INDEX IF NOT EXISTS by_trace_id_and_commit on Regressions2 (trace_id, commit_number);
CREATE INDEX IF NOT EXISTS by_alert_id on Regressions2 (alert_id);
CREATE INDEX IF NOT EXISTS by_commit_alert on Regressions2 (commit_number, alert_id);
CREATE INDEX IF NOT EXISTS by_commit_and_prev_commit on Regressions2 (commit_number, prev_commit_number);
CREATE INDEX IF NOT EXISTS by_sub_name_creation_time on Regressions2 (sub_name, creation_time DESC);
CREATE INDEX IF NOT EXISTS by_sub_name_triage_status_creation_time_asc on Regressions2 (sub_name, triage_status, creation_time ASC);
CREATE INDEX IF NOT EXISTS by_legacy_key on Regressions2 (legacy_key);
```

### 2.2 Trace ID Encoding & Index Architecture

- **`trace_id BYTEA` Column**: Represents the 16-byte MD5 cryptographic hash of a single trace name. Computed via [`types.TraceIDForSQLInBytesFromTraceName(traceName)`](../../go/types/types.go#L105):
  $$\text{trace\_id} = \text{MD5}(\text{traceName}) \in \mathbb{B}^{16}$$
- **Composite B-Tree Index `by_trace_id_and_commit`**: Keyed on `(trace_id, commit_number)`. This index allows Cloud Spanner to perform logarithmic seeks ($O(\log N)$) directly to the exact trace and commit slice.

### 2.3 How `regressions2store` Depends on `trace_id` for High Performance

Controlled by configuration flag `instanceConfig.Experiments.RegressionsTraceIdField` ([`perf/go/config/config.go:1031`](../../go/config/config.go#L1031)), [`perf/go/regression/sqlregression2store/sqlregression2store.go`](../../go/regression/sqlregression2store/sqlregression2store.go) branches between indexed SQL operations and unindexed JSONB operations:

| Query Method                                                                                                                                                   | Fast Indexed Path (`RegressionsTraceIdField = true`)                                                                                    | Legacy Path (`RegressionsTraceIdField = false`)                                                                       | Performance Difference                                                                             |
| :------------------------------------------------------------------------------------------------------------------------------------------------------------- | :-------------------------------------------------------------------------------------------------------------------------------------- | :-------------------------------------------------------------------------------------------------------------------- | :------------------------------------------------------------------------------------------------- | ----------------------------- |
| **`RangeFiltered`** ([`sqlregression2store.go:478-521`](../../go/regression/sqlregression2store/sqlregression2store.go#L478-L521))                             | `WHERE commit_number >= $1 AND commit_number <= $2 AND trace_id = ANY($3)` (`readRangeFilteredByTraceId`)                               | `WHERE commit_number >= $1 AND commit_number <= $2 AND (frame->'dataframe'->'traceset') ?\| $3` (`readRangeFiltered`) | **Sub-5ms B-Tree seek** vs. unindexed table scan evaluating JSONB `?                               | ` across every candidate row. |
| **`GetRegressionsBefore`** ([`sqlregression2store.go:730-749`](../../go/regression/sqlregression2store/sqlregression2store.go#L730-L749))                      | `WHERE commit_number <= $1 AND trace_id = $2 AND sub_name = $4 ORDER BY commit_number DESC LIMIT $3` (`readRegressionsBeforeByTraceId`) | `WHERE commit_number <= $1 AND (frame->'dataframe'->'traceset') ? $2 AND sub_name = $4 ...` (`readRegressionsBefore`) | **Single index lookup** vs. scanning entire history of sub_name for JSONB key existence.           |
| **`GetRegressionsBeforeBatch`** ([`sqlregression2store.go:1441-1479`](../../go/regression/sqlregression2store/sqlregression2store.go#L1441-L1479))             | `JOIN UNNEST($1::bytea[]) WITH ORDINALITY AS t1(trace_id, i) ON r2.trace_id = t1.trace_id` (`readRegressionsBeforeBatchByTraceId`)      | `JOIN UNNEST($1::text[]) ... ON (r2.frame->'dataframe'->'traceset') ? t1.trace_name` (`readRegressionsBeforeBatch`)   | **Index nested-loop join** vs. Cartesian cross-product JSONB search.                               |
| **Write Lookups (`readModifyWriteCompat`)** ([`sqlregression2store.go:1089-1116`](../../go/regression/sqlregression2store/sqlregression2store.go#L1089-L1116)) | `WHERE prev_commit_number < $1 AND commit_number > $2 AND alert_id=$3 AND trace_id=$4`                                                  | `WHERE commit_number=$1 AND alert_id=$2` (`readCompat`)                                                               | Clean per-trace regression isolation vs. collapsing all regressions under `(commit_id, alert_id)`. |

---

## 3. The K-Means Multi-Trace Incompatibility Trap

### 3.1 Trace Set Cardinality: StepFit vs. K-Means

- **StepFit (`types.StepFitGrouping`)**: `stepFitDfTraceSlicer` ([`perf/go/dfiter/traceSlicer.go:83-85`](../../go/dfiter/traceSlicer.go#L83-L85)) slices the input DataFrame into single-trace sub-DataFrames where `len(df.TraceSet) == 1`. Each regression is tied to exactly one trace name.
- **K-Means (`types.KMeansGrouping`)**: Lloyd's algorithm clusters all matching traces into $K$ centroids. When a centroid regresses, its member traces are bundled into `ClusterSummary.Keys` ([`perf/go/clustering2/clustering.go:219-221`](../../go/clustering2/clustering.go#L219-L221), up to `MaxSampleTracesPerCluster = 50` traces). In `perf/go/regression/continuous/continuous.go:154-163`, the stored DataFrame's `TraceSet` is populated with all traces from `cl.Keys`.

### 3.2 The Write-Path Degeneration: `getTraceIdFromTraceSet`

When persisting a regression in `sqlregression2store.go`, `populateRegression2Fields` ([`sqlregression2store.go:1249-1258`](../../go/regression/sqlregression2store/sqlregression2store.go#L1249-L1258)) executes:

```go
// Note for K-means detection, traceID doesn't make sense (since there are multiple traces).
traceID, err := getTraceIdFromTraceSet(regression.Frame.DataFrame.TraceSet)
```

In `getTraceIdFromTraceSet` ([`sqlregression2store.go:1282-1300`](../../go/regression/sqlregression2store/sqlregression2store.go#L1282-L1300)):

```go
func getTraceIdFromTraceSet(traceset types.TraceSet) (string, error) {
	if len(traceset) > 1 {
		text := "Modern regression detection uses just single-trace detection, but traceset has len %d. " +
			"Selecting just one traceset for the trace_id field."
		sklog.Warningf(text, len(traceset))
	}
	if len(traceset) == 0 {
		return "", skerr.Fmt("Regression requires to be detected on some trace, but the traceset is empty?")
	}
	var traceName string
	for name := range traceset {
		traceName = name
		// One tracename is present in case of single-trace detection.
		// For K-means, we select just one trace, but the flag should not be enabled anyway -
		// - we have to stick with the old query.
		break
	}
	return string(types.TraceIDForSQLFromTraceName(traceName)), nil
}
```

### 3.3 The Three Catastrophic Failure Modes of K-Means Under `regressions2store`

1. **Silent Query Omissions (False Negatives)**:
   When a K-Means regression affects 50 traces ($T_1, T_2, \dots, T_{50}$), Go map iteration non-deterministically selects **only one trace** (e.g. $T_1$) to write into `Regressions2.trace_id`. Traces $T_2$ through $T_{50}$ are **not written to `trace_id`**.
   When the UI, triage view, or Pinpoint bisection queries for regressions on trace $T_2$ with `RegressionsTraceIdField = true`, Cloud Spanner evaluates `WHERE trace_id = $T_2` using index `by_trace_id_and_commit`. Spanner returns **0 rows**. The regression on $T_2$ is completely lost!
2. **Forced Fallback to Heavy Unindexed Scans**:
   To find all 50 traces, the instance must disable `RegressionsTraceIdField` and fall back to `(frame->'dataframe'->'traceset') ? $trace_name`. This eliminates the performance advantage of the `by_trace_id_and_commit` index and forces Spanner into unindexed table scans across millions of rows.
   As explicitly noted in the code comment ([`sqlregression2store.go:1295-1296`](../../go/regression/sqlregression2store/sqlregression2store.go#L1295-L1296)):
   > _"For K-means, we select just one trace, but the flag should not be enabled anyway - we have to stick with the old query."_
3. **Write Overwrite Collisions**:
   In `Write` / `updateBasedOnAlertAlgo` ([`sqlregression2store.go:1029-1035`](../../go/regression/sqlregression2store/sqlregression2store.go#L1029-L1035)), K-Means passes `traceName = ""`. In `readModifyWriteCompat` ([`sqlregression2store.go:1088-1116`](../../go/regression/sqlregression2store/sqlregression2store.go#L1088-L1116)), an empty `traceName` falls back to `readCompat`: `WHERE commit_number = $1 AND alert_id = $2`. If an alert produces multiple distinct cluster regressions at the same commit, they overwrite each other!
   Conversely, StepFit passes the exact `traceName`, cleanly isolating each regression under `(commit_number, alert_id, trace_id)`.

---

## 4. Sparse Traces & Wide Commit Range $[a; b]$: Why K-Means Fails and StepFit is Mandatory

### 4.1 Four Mathematical & Structural Failure Modes of K-Means on Sparse Traces

When querying anomaly detection over wide commit spans $[a; b]$ (e.g. 5,000 commits) where benchmark observations are sparse (e.g. 30 real points):

1. **Midpoint Sentinel Rejection**:
   `kmeansDataframeSlicer` ([`perf/go/dfiter/traceSlicer.go:130-158`](../../go/dfiter/traceSlicer.go#L130-L158)) creates sliding windows of size $W = 2 \cdot \text{radius} + 1$ across raw commit indices.
   In [`perf/go/regression/detector.go:303-305`](../../go/regression/detector.go#L303-L305):
   ```go
   // If the midpoint is missing, we don't detect for this dataframe's trace.
   if tr[n] == vec32.MissingDataSentinel {
       return true
   }
   ```
   If a benchmark runs once every 50 commits, 49 out of 50 sliding windows have `tr[n] == MissingDataSentinel` at the turning point and are immediately discarded.
2. **Asymmetric Missing Data Filtering (`tooMuchMissingData`)**:
   In `perf/go/regression/detector.go:284-308`:
   ```go
   return missing(tr[:n]) || missing(tr[len(tr)-n:])
   ```
   `tooMuchMissingData` rejects any trace where $>50\%$ of values on either side of the midpoint are missing. For sparse traces where data points are spaced across commits, the missing data rate routinely exceeds $80\%$. In `detector.go:331-338`, **100% of traces are filtered out**, detecting 0 anomalies.
3. **Linear Interpolation Distortion (`vec32.Fill`)**:
   In `ctrace2.NewFullTrace` ([`perf/go/ctrace2/ctrace.go:53-63`](../../go/ctrace2/ctrace.go#L53-L63)), missing values are filled via `vec32.Fill(norm)` before Z-score normalization. Over wide commit intervals, linear interpolation transforms an abrupt, sharp step into a slow, gradual linear ramp. This suppresses the calculated step size below the `Interesting` threshold or generates false-positive drift artifacts.
4. **Dense Memory & I/O Explosion**:
   `sqltracestore.go:1149` allocates `ret[name] = vec32.New(len(commits))`. Slicing and holding an $M \times 5,000$ matrix consisting of $99\%$ `vec32.MissingDataSentinel` floats causes severe GC pressure and memory consumption.

### 4.2 Why StepFit + `stepFitDfTraceSlicer` Excels on Sparse Traces

`StepFitGrouping` with `stepFitDfTraceSlicer` ([`perf/go/dfiter/traceSlicer.go:13-128`](../../go/dfiter/traceSlicer.go#L13-L128)) was specifically engineered for sparse, asynchronous time series:

1. **Upfront Sentinel Elimination**:
   `NewStepFitDfTraceSlicer` ([`traceSlicer.go:101-117`](../../go/dfiter/traceSlicer.go#L101-L117)) strips `vec32.MissingDataSentinel` values upfront:
   ```go
   validPoints := []float32{}
   validHeaders := []*dataframe.ColumnHeader{}
   for i, v := range trace {
       if v != vec32.MissingDataSentinel {
           validPoints = append(validPoints, v)
           validHeaders = append(validHeaders, df.Header[i])
       }
   }
   filteredTraces[k] = validPoints
   filteredHeaders[k] = validHeaders
   ```
2. **Dense Observation Sliding Window**:
   Sliding windows of size $W = 2 \cdot \text{radius} + 1$ move over `validPoints` (the sequence of real measurements), **never** over empty commits. Every window has $0\%$ missing data, and candidate turning point `tr[n]` is guaranteed to be a real measurement. `tooMuchMissingData` never drops these traces.
3. **Exact Commit Boundary Attribution**:
   Commit metadata is preserved in `validHeaders[start:end]`. When `stepfit.EvaluateRule` finds a turning point at index $i$, the turning point maps directly to `validHeaders[i].Offset` and `validHeaders[i-1].Offset`, providing the exact commit range $(c_{\text{prev}}, c_{\text{curr}}]$ spanning the step.
4. **Streaming Flat Memory Footprint**:
   Sub-DataFrames are emitted 1 trace at a time (`types.TraceSet{traceID: slicedTrace}`), maintaining flat memory usage.

---

## 5. Architectural Specification: Targeted Anomaly Detection on Wide Commit Range `[a; b]`

### 5.1 End-to-End Execution Flow

```
[ User Request: Range [a; b], Query Q, (Optional TraceIDs) ]
                            |
                            v
   [ 1. Query Sparse TraceValues / TraceValues2 ]
   (SELECT trace_id, commit_number, val FROM TraceValues2
    WHERE commit_number BETWEEN a AND b AND trace_id IN (...))
                            |
                            v
   [ 2. Enforce types.StepFitGrouping ]
   (Reject KMeansGrouping: prevents missing-data drops and interpolation ramps)
                            |
                            v
   [ 3. Dense Observation Construction & Adaptive Radius Guard ]
   - Filter out MissingDataSentinel -> validPoints[], validHeaders[]
   - Adaptive Window Guard: if len(validPoints) < windowSize (2 * radius + 1),
     dynamically adjust radius = max(3, len(validPoints)/2 - 1)
                            |
                            v
   [ 4. stepFitDfTraceSlicer Sliding Window ]
   - Slides window over validPoints (0% missing data)
                            |
                            v
   [ 5. stepfit.EvaluateRule ]
   (Evaluates raw metric steps across dense observation window)
                            |
                            v
   [ 6. Anomaly Bounds Extraction ]
   (PrevCommitNumber = validHeaders[i-1].Offset, CommitNumber = validHeaders[i].Offset)
                            |
                            v
   [ 7. Persist to Regressions2 ]
   (Key: (commit_number, alert_id, trace_id BYTEA) -> Indexed by by_trace_id_and_commit)
```

### 5.2 Architectural Requirements & Implementation Guidelines

1. **Extend Domain Representation**:
   Currently, [`types.Domain`](../../go/types/types.go#L235-L245) only supports `Offset` (single commit mode) or `N` + `End` time (trailing commit mode). To support range $[a; b]$, `types.Domain` must be extended with explicit commit bounds:
   ```go
   type Domain struct {
       N           int32              `json:"n"`
       End         time.Time          `json:"end"`
       Offset      int32              `json:"offset"`
       BeginCommit types.CommitNumber `json:"begin_commit,omitempty"`
       EndCommit   types.CommitNumber `json:"end_commit,omitempty"`
   }
   ```
2. **Mandate `types.StepFitGrouping`**:
   The feature must strictly enforce `alert.Algo = types.StepFitGrouping` and reject `KMeansGrouping`. K-Means will fail due to `tooMuchMissingData` filtering, linear interpolation distortion (`vec32.Fill`), and `regressions2store` multi-trace indexing incompatibilities.
3. **Sparse Commit Range Query Optimization**:
   Do not load full tiles or allocate a dense $(b - a + 1)$ slice of `MissingDataSentinel` floats via `vec32.New(len(commits))`. Instead, query `TraceValues` / `TraceValues2` restricted to `WHERE commit_number >= a AND commit_number <= b` for the selected traces, directly returning only the existing observations.
4. **Adaptive Observation Window Guard**:
   `stepFitDfTraceSlicer` requires `len(validPoints) >= windowSize` ($W = 2 \cdot \text{radius} + 1$, typically 21 commits). If a sparse trace in $[a; b]$ contains fewer than $W$ valid observations, the slicer produces 0 windows and detects nothing. The feature should:
   - Adaptively scale down the radius: `radius = max(3, len(validPoints)/2 - 1)` when $7 \le \text{len}(validPoints) < 21$, OR
   - Query additional context points outside $[a; b]$ (e.g., $k$ points before $a$ and $k$ points after $b$) to establish stable baseline and post-step statistics, OR
   - Return an explicit validation error: `"Insufficient data points in range [a; b]: found %d observations, required minimum %d"`.
5. **Accurate Anomaly Bounding for Pinpoint**:
   Because commits between sparse points $P_{i-1}$ and $P_i$ have no test data, the anomaly cannot be pinned to a single commit. The feature must populate `PrevCommitNumber = validHeaders[i-1].Offset` and `CommitNumber = validHeaders[i].Offset`. Downstream Pinpoint bisection will bisect across $(P_{i-1}, P_i]$.
6. **Storage & Fast Indexing**:
   Because StepFit produces single-trace anomalies, each detected regression is stored with its exact `trace_id BYTEA`. This ensures all subsequent queries (UI graphs, alert history, bisection tracking) leverage the `by_trace_id_and_commit` B-tree index in Cloud Spanner.
