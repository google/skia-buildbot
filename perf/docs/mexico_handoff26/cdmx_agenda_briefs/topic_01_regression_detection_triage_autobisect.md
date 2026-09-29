# Topic 1: How Regressions Are Detected, Triaged, and Automatically Bisected Across Perf and Pinpoint

## Executive Summary

This brief provides an end-to-end technical breakdown of the anomaly detection, clustering, triaging, and automatic bisection subsystems spanning Skia Perf and Pinpoint. It details how time-series benchmark traces are evaluated for statistically significant shifts, grouped into actionable clusters, forwarded through anomaly group services, and dispatched to Temporal-based Pinpoint bisection workflows to identify culprit commits.

---

## 1. Step Detection Algorithms in Skia Perf

The core algorithmic logic for identifying performance regressions in individual benchmark traces lives in [`perf/go/stepfit/stepfit.go`](../../../go/stepfit/stepfit.go) and [`perf/go/stepfit/rules.go`](../../../go/stepfit/rules.go).

### 1.1 Detection Window and Turning Point Analysis

When evaluating an individual trace or centroid over a sliding window of length $N$:

- The mid-point index $i = \lfloor N / 2 \rfloor$ serves as the candidate turning point separating the baseline (before) values `trace[:i]` from the treatment (after) values `trace[i:]` ([`stepfit.go:91`](../../../go/stepfit/stepfit.go#L91)).
- Depending on the configured step algorithm (`simpleRule.Step`), baseline and treatment summaries ($y_0, y_1$) are computed as either means ([`stepfit.go:99-100`](../../../go/stepfit/stepfit.go#L99-L100)) or medians (for `PercentMedianStep`, [`stepfit.go:96-97`](../../../go/stepfit/stepfit.go#L96-L97)).

### 1.2 Mathematical Formulations of Supported Algorithms

| Algorithm (`types.StepDetection`) | Calculation / Formula                                                                                                                                                       | Key Characteristics & Threshold Semantics                                                                                                                               | Code Reference                                                                                                          |
| :-------------------------------- | :-------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :---------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :---------------------------------------------------------------------------------------------------------------------- |
| **`OriginalStep`**                | $\text{StepSize} = y_0 - y_1$<br>$\text{LSE} = \frac{\sqrt{SSE_1 + SSE_2}}{N}$<br>$\text{Regression} = \frac{\text{StepSize}}{\max(\text{LSE}, \sigma_{\text{threshold}})}$ | Original recipe step detection. Normalizes step size by pooled least-squares error. If fit error is below noise threshold, caps divisor at $\sigma_{\text{threshold}}$. | [`stepfit.go:189-211`](../../../go/stepfit/stepfit.go#L189-L211)                                                        |
| **`AbsoluteStep`**                | $\text{StepSize} = y_0 - y_1$<br>$\text{Regression} = y_0 - y_1$                                                                                                            | Checks if the raw delta between pre- and post-means exceeds an absolute value threshold. Useful for fixed-overhead bounds (e.g. memory).                                | [`stepfit.go:215-218`](../../../go/stepfit/stepfit.go#L215-L218)                                                        |
| **`Const`**                       | $\text{StepSize} = -1.0$<br>$\text{Regression} = \| \text{trace}[i] \|$                                                                                                     | Checks if the absolute magnitude of the point at the turning point exceeds a fixed constant threshold.                                                                  | [`stepfit.go:222-229`](../../../go/stepfit/stepfit.go#L222-L229)                                                        |
| **`PercentStep`**                 | $\text{StepSize} = \frac{y_0 - y_1}{y_0}$<br>$\text{Regression} = \text{StepSize}$                                                                                          | Checks relative percentage change from baseline mean $y_0$. Protects against division by zero ($\pm \infty \to \pm \text{MaxFloat32}$, $\text{NaN} \to 0$).             | [`stepfit.go:233-245`](../../../go/stepfit/stepfit.go#L233-L245)                                                        |
| **`PercentMedianStep`**           | Same as `PercentStep`, but $y_0 = \text{median}(\text{trace}[:i])$ and $y_1 = \text{median}(\text{trace}[i:])$                                                              | Robust against single-point outlier spikes that distort sample means.                                                                                                   | [`stepfit.go:95-97`](../../../go/stepfit/stepfit.go#L95-L97), [`233-245`](../../../go/stepfit/stepfit.go#L233-L245)     |
| **`CohenStep`**                   | $\text{StepSize} = \frac{y_0 - y_1}{\max(s_p, \sigma_{\text{threshold}})}$<br>$s_p = \sqrt{\frac{(n_1-1)s_1^2 + (n_2-1)s_2^2}{n_1+n_2-2}}$                                  | Evaluates Cohen's $d$ effect size using pooled sample variance. Standardizes the difference relative to trace variance. Requires $N \ge 4$.                             | [`stepfit.go:249-287`](../../../go/stepfit/stepfit.go#L249-L287)                                                        |
| **`Stepiness`**                   | $\text{steppiness} = 1.0 - \sqrt{\frac{SSE_1 + SSE_2}{SSE_{\text{total}}}}$<br>$\text{Regression} = \pm \text{steppiness}$                                                  | Normalized metric $[0, 1]$ measuring how well a binary step function accounts for total trace variance compared to the overall mean. Matches Catapult legacy formula.   | [`stepfit.go:319-333`](../../../go/stepfit/stepfit.go#L319-L333)                                                        |
| **`MannWhitneyU`**                | Calculates Mann-Whitney U test between `trace[:i]` and `trace[i:]`<br>$\text{Regression} = p\text{-value}$, $\text{LSE} = U$                                                | Non-parametric rank-sum hypothesis test. Triggers anomaly when $p \le \text{threshold}$ (e.g. $\alpha = 0.05$). Direction is determined by the sign of $(y_0 - y_1)$.   | [`stepfit.go:150-157`](../../../go/stepfit/stepfit.go#L150-L157), [`290-299`](../../../go/stepfit/stepfit.go#L290-L299) |

---

## 2. Clustering and Multi-Trace Aggregation: K-Means vs. StepFit Grouping

In production, benchmarks frequently emit dozens or hundreds of sub-traces (e.g., individual tests within Speedometer, memory allocations across browser processes). Skia Perf provides two fundamentally different architectural modes for regression analysis, governed by `Alert.Algo` ([`perf/go/types/types.go:123-132`](../../../go/types/types.go#L123-L132)): **`KMeansGrouping`** (cluster shape aggregation) and **`StepFitGrouping`** (individual trace evaluation, presented in the Web UI as **"Individual"**; [`algo-select-sk.ts:60-69`](../../../modules/algo-select-sk/algo-select-sk.ts#L60-L69)).

### 2.1 Conceptual Clarification: Grouping Strategy vs. Step Detection Function

A frequent source of confusion is the distinction between _Stepfit_ and _K-Means_:

- **Stepfit** ([`perf/go/stepfit/stepfit.go:90`](../../../go/stepfit/stepfit.go#L90)) is the **underlying mathematical step-detection function**. It evaluates a _single_ 1D time-series vector (either an individual trace or a cluster centroid) across a sliding window and returns a `StepFit` struct containing step size, regression score, turning point, and status (`HIGH`, `LOW`, or `UNINTERESTING`).
- **Grouping Algorithms** (`types.RegressionDetectionGrouping`) define the **meta-strategy** for organizing trace data before step detection is executed:
  - **`types.KMeansGrouping ("kmeans")`**: Multi-trace clustering meta-algorithm. Collects all matching traces into a unified DataFrame, standardizes their shapes into Z-scores ($\mu = 0, \sigma = 1$), clusters them via Lloyd's algorithm into $K$ centroids, and then runs Stepfit once per _cluster centroid_. Traces are evaluated in standardized $\sigma$-units.
  - **`types.StepFitGrouping ("stepfit")`**: Individual trace mode. Bypasses K-Means clustering entirely. Each trace is sliced into a 1-trace DataFrame and evaluated independently through Stepfit on its raw metric units (milliseconds, bytes, score).

```
                         [ Trace Store / Spanner Data ]
                                       |
                   +-------------------+-------------------+
                   |                                       |
         [ types.KMeansGrouping ]                [ types.StepFitGrouping ]
                   |                                       |
      kmeansDataframeSlicer (All Traces)        stepFitDfTraceSlicer (Per-Trace Dense)
                   |                                       |
          vec32.Norm (Z-score)                   Filters MissingDataSentinel
                   |                                       |
       Lloyd's K-Means (K Centroids)              Direct Trace Slices (1 Trace/DF)
                   |                                       |
    +--------------v--------------+         +--------------v--------------+
    |  stepfit.EvaluateRule()     |         |  stepfit.EvaluateRule()     |
    |  (Evaluated ONCE per        |         |  (Evaluated for EVERY       |
    |   cluster centroid)         |         |   individual trace)         |
    +--------------+--------------+         +--------------+--------------+
                   |                                       |
           ClusterSummaries                        ClusterSummaries
        (Keys = cluster members)                 (High / Low partitions)
                   |                                       |
        Regressions2 Table                      Regressions2 Table
     Key: (commit, alert, "")                Key: (commit, alert, trace_name)
```

---

### 2.2 K-Means Clustering Pipeline (`types.KMeansGrouping`)

- **Implementation**: [`perf/go/kmeans/kmeans.go`](../../../go/kmeans/kmeans.go), [`perf/go/ctrace2/ctrace.go`](../../../go/ctrace2/ctrace.go), and [`perf/go/clustering2/clustering.go`](../../../go/clustering2/clustering.go).
- **Execution Pipeline**:
  1. **Dataframe Slicing**: `kmeansDataframeSlicer` ([`perf/go/dfiter/traceSlicer.go:130-158`](../../../go/dfiter/traceSlicer.go#L130-L158)) slices across all $M$ traces for a window of size $W = 2 \cdot \text{radius} + 1$.
  2. **Data Cleansing & Z-Score Normalization**:
     - Traces with $>50\%$ missing data on either side of the midpoint commit are filtered out (`tooMuchMissingData`, [`perf/go/regression/detector.go:331-333`](../../../go/regression/detector.go#L331-L333)).
     - Remaining traces are converted to `kmeans.Clusterable` vectors via `ctrace2.NewFullTrace` ([`ctrace.go:53-63`](../../../go/ctrace2/ctrace.go#L53-L63)).
     - Missing points are linearly interpolated via `vec32.Fill(norm)`.
     - Traces are normalized to mean $\mu = 0$ and standard deviation $\sigma = 1$ via `vec32.Norm(norm, minStdDev)`:
       $$\tilde{x}_i = \frac{x_i - \mu}{\max(\sigma, \sigma_{\min})}$$
     - Distance between trace $\mathbf{u}$ and centroid $\mathbf{c}$ is Euclidean distance in standardized shape space:
       $$D(\mathbf{u}, \mathbf{c}) = \sqrt{\sum_{i=1}^{W} (\tilde{u}_i - \tilde{c}_i)^2}$$
  3. **Adaptive Centroid Selection**:
     - If alert `K <= 0`, Skia Perf dynamically selects $K$ based on total traces $M$ ([`detector.go:343-352`](../../../go/regression/detector.go#L343-L352)):
       $$K = \left\lfloor \frac{40}{30,000} \cdot M + 10 \right\rfloor \quad \text{constrained to } 10 \le K \le 50$$
     - Initial centroids are picked via random reservoir sampling (`chooseK`, [`clustering.go:97-104`](../../../go/clustering2/clustering.go#L97-L104)).
  4. **Lloyd's Iteration**:
     - Runs up to `MAX_KMEANS_ITERATIONS = 100` ([`clustering.go:29`](../../../go/clustering2/clustering.go#L29)).
     - Each iteration assigns traces to their nearest centroid (`kmeans.Do`, [`kmeans.go:49-74`](../../../go/kmeans/kmeans.go#L49-L74)) and recomputes centroids as component-wise arithmetic means (`ctrace2.CalculateCentroid`, [`ctrace.go:66-83`](../../../go/ctrace2/ctrace.go#L66-L83)).
     - Terminates early if change in total distortion error $\Delta E < \text{KMEAN\_EPSILON} = 1.0$ ([`clustering.go:34, 256`](../../../go/clustering2/clustering.go#L34)).
  5. **Centroid Step Detection**:
     - `stepfit.EvaluateRule` is called **only on the $K$ calculated centroids** ([`clustering.go:200`](../../../go/clustering2/clustering.go#L200)), **not** on individual traces!
     - If a centroid exhibits a statistically significant step, its member traces (ordered by proximity to centroid, up to `MaxSampleTracesPerCluster = 50`) are packaged into `ClusterSummary.Keys` ([`clustering.go:210-222`](../../../go/clustering2/clustering.go#L210-L222)).
     - Because centroids are in Z-score units, step detection evaluates change relative to trace variance rather than absolute metric delta.

---

### 2.3 StepFit Grouping Pipeline (`types.StepFitGrouping`)

- **Implementation**: [`perf/go/dfiter/traceSlicer.go`](../../../go/dfiter/traceSlicer.go) and [`perf/go/regression/stepfit.go`](../../../go/regression/stepfit.go).
- **Execution Pipeline**:
  1. **Dense Trace Slicing (`stepFitDfTraceSlicer`)**:
     - Instead of matrix-slicing all traces together, `stepFitDfTraceSlicer` ([`traceSlicer.go:13-32`](../../../go/dfiter/traceSlicer.go#L13-L32)) loops over traces individually.
     - For each trace, it filters out all `vec32.MissingDataSentinel` values without interpolation, creating a compact sequence of valid observations along with their commit headers ([`traceSlicer.go:101-117`](../../../go/dfiter/traceSlicer.go#L101-L117)).
     - It yields sub-DataFrames containing **exactly 1 trace** (`ret.TraceSet = types.TraceSet{traceID: slicedTrace}`, [`traceSlicer.go:83-85`](../../../go/dfiter/traceSlicer.go#L83-L85)).
  2. **Direct Trace Evaluation**:
     - `regression.StepFit` ([`perf/go/regression/stepfit.go:16-57`](../../../go/regression/stepfit.go#L16-57)) evaluates `stepfit.EvaluateRule(trace, stddevThreshold, rule)` on the raw trace data.
  3. **High / Low Partitioning**:
     - An anomalous trace is assigned to either `low` (step down) or `high` (step up) ([`stepfit.go:30-56`](../../../go/regression/stepfit.go#L30-L56)).
     - The "centroid" is simply a copy of the trace itself (`low.Centroid = vec32.Dup(trace)`).
  4. **Single-Trace Identity Attribution**:
     - In `detectRegressionsOnDataFrame` ([`perf/go/regression/detector.go:369-377`](../../../go/regression/detector.go#L369-L377)), the singular trace name is extracted (`cr.TraceName = string(k)`), preserving per-trace attribution for downstream refiners.

---

### 2.4 Continuous & Event-Driven Pipeline Divergence

The choice of grouping algorithm deeply alters database queries, concurrency, and persistence in the continuous regression pipeline:

| Dimension                          | `KMeansGrouping`                                                                                                            | `StepFitGrouping`                                                                                                                                               | Code Reference                                                                                                      |
| :--------------------------------- | :-------------------------------------------------------------------------------------------------------------------------- | :-------------------------------------------------------------------------------------------------------------------------------------------------------------- | :------------------------------------------------------------------------------------------------------------------ |
| **Event-Driven PubSub Handling**   | Must execute full query (`ProcessAlertConfig`) over the **entire trace universe** to recompute geometric cluster centroids. | Executes targeted `ProcessAlertConfigForTraces`, querying **only incoming trace IDs** matching the file ingestion event.                                        | [`continuous.go:560-577`](../../../go/regression/continuous/continuous.go#L560-L577)                                |
| **Worker Concurrency & Chunking**  | Processes 1 alert config per worker thread across all traces simultaneously.                                                | Chunks trace IDs (default 50 traces per chunk when `DfIterTraceSlicer` is enabled) across a worker pool.                                                        | [`continuous.go:624-637`](../../../go/regression/continuous/continuous.go#L624-L637)                                |
| **Spanner `Regressions2` Storage** | Keyed by `(commit_id, alert_id)` with `trace_name = ""`. Represents a multi-trace cluster regression.                       | Keyed by `(commit_id, alert_id, traceName)`. Stores fine-grained per-trace regressions. Prevents overwriting if regression already recorded (`r.Frame != nil`). | [`sqlregression2store.go:1029-1057`](../../../go/regression/sqlregression2store/sqlregression2store.go#L1029-L1057) |
| **Refiner Compatibility**          | Incompatible with `AnomalyBoundsRefiner` / `ImprovedAnomalyBoundsRefiner` (fails validation).                               | **Mandatory** for `AnomalyBoundsRefiner` and `ImprovedAnomalyBoundsRefiner`. Invariant requires `len(clusters) <= 1` and `len(Keys) == 1`.                      | [`anomaly_bounds_refiner.go:60-87`](../../../go/regression/refiner/anomaly_bounds_refiner.go#L60-L87)               |
| **`MinimumNum` Semantics**         | Filters clusters with fewer than `MinimumNum` co-moving traces (`len(cl.Keys) >= cfg.MinimumNum`).                          | **Operational Trap**: Because `len(cl.Keys) == 1`, setting `MinimumNum > 1` silently suppresses all regressions under `DefaultRegressionRefiner`.               | [`default_regression_refiner.go:57`](../../../go/regression/refiner/default_regression_refiner.go#L57)              |
| **Notification Metadata**          | Aggregates parameter keys across cluster members (`GetParamSummariesForKeys`).                                              | Attaches explicit `metadata.TraceID = cl.Keys[0]` and populates direct commit links.                                                                            | [`notify.go:298-319`](../../../go/notify/notify.go#L298-L319)                                                       |

---

### 2.5 Algorithmic Complexity, Memory, and Database Load

Let $M$ be the total number of traces matching the alert query, $N$ be the total commits in the continuous scan range, $W = 2 \cdot \text{radius} + 1$ be the sliding window length, $K$ be the number of centroids ($10 \le K \le 50$), and $I$ be Lloyd's iterations ($I \le 100$):

```
+---------------------------------------------------------------------------------------------------------+
| Metric                 | KMeansGrouping                                 | StepFitGrouping               |
+---------------------------------------------------------------------------------------------------------+
| Time Complexity        | O((N - W) * (I * K * M * W + K * W_stepfit))   | O(M * (N - W) * W_stepfit)    |
| Memory Footprint       | O(M * W) (Dense matrix of all traces)         | O(chunk_size * W) (Streaming) |
| Event Ingestion DB I/O | Full scan of M_total traces in alert           | Point scan of M_event traces  |
| Stepfit Invocations    | Exactly (N - W + 1) * K                        | Exactly sum_{m=1}^M (L_m - W) |
+---------------------------------------------------------------------------------------------------------+
```

- **Computational Scaling**:
  - In `KMeansGrouping`, pairwise distance calculations dominate: each iteration evaluates $M \times K$ Euclidean vector distances of length $W$, yielding $O(I \cdot K \cdot M \cdot W)$. However, `stepfit.EvaluateRule` is executed **only $K$ times per window** ($K \le 50$).
  - In `StepFitGrouping`, Lloyd's clustering is skipped entirely ($0$ vector distance calculations), but `stepfit.EvaluateRule` is executed **for every window of every trace**. If using computationally heavier step algorithms like `MannWhitneyU` ($O(W \log W)$ due to rank sorting), CPU cost scales strictly linearly with total trace observations.
- **Database & Memory Scaling**:
  - In `KMeansGrouping`, Skia Perf must hold the full $M \times W$ float32 matrix in memory to compute centroids. In high-cardinality suites ($M > 50,000$), memory consumption and tile deserialization latency become significant.
  - In `StepFitGrouping` with `experiments.df_iter_trace_slicer = true` ([`perf/configs/spanner/chrome-internal.json:43`](../../../configs/spanner/chrome-internal.json#L43)), traces are sliced and queried in parallel chunks of 50 (or in a single targeted query via `UseRecursiveLoader`), maintaining a flat, predictable memory footprint.

---

### 2.6 Practical Heuristics: When to Use Which?

```
                                  [ Select Grouping Algorithm ]
                                                |
                 +------------------------------+------------------------------+
                 |                                                             |
   Does the alert monitor a wide suite                            Is the metric a dedicated KPI,
   with co-moving sub-tests?                                      North Star, or isolated metric?
   (e.g., Speedometer, JetStream, Memory)                         (e.g., FCP, LCP, Startup Latency)
                 |                                                             |
                 v                                                             v
       [ Use KMeansGrouping ]                                       [ Use StepFitGrouping ]
                 |                                                             |
   - Reduces alert storms (500 traces -> 1 bug)                   - Detects isolated regressions
   - Suppresses uncorrelated test noise                           - Preserves exact anomaly bounds
   - Finds systemic architectural regressions                     - Mandatory for AnomalyBoundsRefiner
   - Evaluates in standardized sigma-units                        - Evaluates in raw metric units
   - Caveat: Dilutes single-test drops                            - Caveat: Alert storms if query too broad
```

#### Choose `KMeansGrouping` When:

1. **High-Dimensional Test Suites with Correlated Sub-Traces**:
   - Benchmarks like **Speedometer 3.0**, **JetStream 2**, or **Blink Layout Tests** emit hundreds of sub-scores (e.g. `Speedometer3/TodoMVC-React`, `Speedometer3/CodeMirror`). A compiler or engine optimization typically shifts all sub-tests in unison.
   - K-Means aggregates these into a single cluster summary, preventing sheriff alert storms (1 consolidated bug instead of 400 redundant bugs).
2. **Noise Suppression via Centroid Averaging**:
   - For noisy microbenchmarks, averaging $M$ co-moving traces into a centroid attenuates uncorrelated Gaussian noise by approximately $\frac{1}{\sqrt{M}}$, making subtle true steps clearly detectable above `MinStdDev`.
3. **Broad Multi-Metric Architectural Regressions**:
   - Ideal for monitoring system-wide resource allocations (e.g., total browser memory across renderers, GPU memory footprint), where individual trace variance is high but the aggregate vector movement is statistically clear.
4. **Drawback — Signal Dilution**:
   - If an optimization regresses only _one specific API_ (e.g., `Canvas2D.arc` regresses 20% while 99 other canvas benchmarks are flat), K-Means will absorb that trace into a flat cluster centroid, diluting the step below the `Interesting` threshold and missing the regression.

#### Choose `StepFitGrouping` When:

1. **North Star & High-Priority KPI Metrics**:
   - Critical user-centric metrics such as **First Contentful Paint (FCP)**, **Largest Contentful Paint (LCP)**, **Cumulative Layout Shift (CLS)**, **Cold Startup Time**, or **APK Binary Size**.
   - These metrics must never be averaged with other metrics; every regression requires immediate, isolated detection and bisection.
2. **Isolated or Narrow Regressions**:
   - Targeted unit benchmarks or subsystem microbenchmarks where regressions occur independently (e.g., Skia path rendering or V8 RegEx execution).
3. **Sparse or Asynchronously Ingested Traces**:
   - Datasets where different bots report at different cadences or have missing data. `StepFitDfTraceSlicer` strips missing sentinels per trace without fabricating interpolated values via `vec32.Fill`, preventing spurious drops caused by missing data interpolation.
4. **Integration with Advanced Refiners & Auto-Bisection**:
   - **`AnomalyBoundsRefiner`** and **`ImprovedAnomalyBoundsRefiner`** ([`anomaly_bounds_refiner.go:60-87`](../../../go/regression/refiner/anomaly_bounds_refiner.go#L60-L87)) **strictly require `StepFitGrouping`**. They track previous change points and calculate precise anomaly bounding intervals (`StartCommit`, `EndCommit`) for Pinpoint bisection.
5. **Event-Driven Low-Latency Detection**:
   - Production instances leveraging PubSub file ingestion (`chrome-internal`, `android`) can execute fast point-queries for newly landed commits via `ProcessAlertConfigForTraces`, avoiding heavy full-table queries.

---

### 2.7 Cloud Spanner `Regressions2` Schema, `trace_id` Indexing, and the K-Means Incompatibility Trap

The transition from the legacy `Regressions` table to `Regressions2` in Cloud Spanner is central to Skia Perf's scalability. However, this storage architecture creates a hard incompatibility with K-Means clustering.

#### 2.7.1 `Regressions2` Table Schema & Secondary Indices

Defined in [`perf/go/regression/sqlregression2store/schema/schema.go:10-92`](../../../go/regression/sqlregression2store/schema/schema.go#L10-L92) and generated in [`perf/go/sql/spanner/schema_spanner.go:107-127, 210-216`](../../../go/sql/spanner/schema_spanner.go#L107-L127):

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

- **`trace_id BYTEA`**: A 16-byte cryptographic MD5 hash of a single trace name (`types.TraceIDForSQLInBytesFromTraceName(traceName)`, [`perf/go/types/types.go:105`](../../../go/types/types.go#L105)).
- **Primary Secondary Index**: `by_trace_id_and_commit (trace_id, commit_number)` enables sub-millisecond point seeks when looking up regressions for specific benchmark traces across commit ranges.

#### 2.7.2 How `regressions2store` Leverages `trace_id` for High Performance

Controlled by `instanceConfig.Experiments.RegressionsTraceIdField` ([`perf/go/config/config.go:1031`](../../../go/config/config.go#L1031)), [`perf/go/regression/sqlregression2store/sqlregression2store.go`](../../../go/regression/sqlregression2store/sqlregression2store.go) provides dual query paths:

1. **`RangeFiltered`** ([`sqlregression2store.go:478-521`](../../../go/regression/sqlregression2store/sqlregression2store.go#L478-L521)):
   - **Fast Path (`readRangeFilteredByTraceId:184-193`)**:
     ```sql
     SELECT {{ .Columns }} FROM Regressions2
     WHERE commit_number >= $1 AND commit_number <= $2 AND trace_id = ANY($3)
     ```
     Executes a fast index seek across `by_trace_id_and_commit (trace_id, commit_number)` in milliseconds.
   - **Legacy Path (`readRangeFiltered:174-183`)**:
     ```sql
     SELECT {{ .Columns }} FROM Regressions2
     WHERE commit_number >= $1 AND commit_number <= $2
       AND (frame->'dataframe'->'traceset') ?| $3
     ```
     Evaluates the JSONB key-existence operator `?|` across every candidate row. Because JSONB attributes cannot use B-Tree seeks, this causes an unindexed table scan, causing high Spanner CPU and query timeouts.
2. **`GetRegressionsBefore`** ([`sqlregression2store.go:730-749`](../../../go/regression/sqlregression2store/sqlregression2store.go#L730-L749)):
   - Fast path (`readRegressionsBeforeByTraceId:207-219`): `WHERE commit_number <= $1 AND trace_id = $2 AND sub_name = $4 ORDER BY commit_number DESC LIMIT $3` (Indexed).
   - Legacy path (`readRegressionsBefore:194-206`): `WHERE commit_number <= $1 AND (frame->'dataframe'->'traceset') ? $2 AND sub_name = $4 ...` (Unindexed JSONB evaluation).
3. **`GetRegressionsBeforeBatch`** ([`sqlregression2store.go:1441-1479`](../../../go/regression/sqlregression2store/sqlregression2store.go#L1441-L1479)):
   - Fast path (`readRegressionsBeforeBatchByTraceId:245-260`): Joins unnested `traceIDs` array on `ON r2.trace_id = t1.trace_id`. Spanner performs index nested-loop joins.
   - Legacy path (`readRegressionsBeforeBatch:220-244`): Joins on `ON (r2.frame->'dataframe'->'traceset') ? t1.trace_name`, requiring an exhaustive cross-product JSONB evaluation.
4. **Write-Path Lookups (`readModifyWriteCompat`)** ([`sqlregression2store.go:1089-1116`](../../../go/regression/sqlregression2store/sqlregression2store.go#L1089-L1116)):
   - When updating or inserting a regression, if `traceName != ""`, it uses `readRegressionsByCommitRangeAlertAndTraceId` (`AND trace_id=$4`).
   - If `traceName == ""` (as in K-Means), it falls back to `readCompat`: `WHERE commit_number=$1 AND alert_id=$2`.

#### 2.7.3 The K-Means Multi-Trace Incompatibility Trap

In K-Means clustering, an alert groups multiple co-moving traces into a single regression cluster (`ClusterSummary.Keys` contains up to `MaxSampleTracesPerCluster = 50` member traces; [`perf/go/clustering2/clustering.go:195-221`](../../../go/clustering2/clustering.go#L195-L221)). When reporting regressions in `perf/go/regression/continuous/continuous.go:154-163`, `resp.Frame.DataFrame` is built containing all keys from `cl.Keys`:

```go
for _, key := range cl.Keys {
    df.TraceSet[key] = originalDataFrame.TraceSet[key]
}
```

When saving the regression, `populateRegression2Fields` ([`sqlregression2store.go:1249-1258`](../../../go/regression/sqlregression2store/sqlregression2store.go#L1249-L1258)) attempts to populate `Regressions2.trace_id`:

```go
// Note for K-means detection, traceID doesn't make sense (since there are multiple traces).
traceID, err := getTraceIdFromTraceSet(regression.Frame.DataFrame.TraceSet)
```

In `getTraceIdFromTraceSet` ([`sqlregression2store.go:1282-1300`](../../../go/regression/sqlregression2store/sqlregression2store.go#L1282-L1300)):

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

**Why K-Means Breaks `regressions2store`:**

1. **Silent Query Omissions (False Negatives)**:
   If a K-Means regression affects 50 traces ($T_1, T_2, \dots, T_{50}$), `getTraceIdFromTraceSet` nondeterministically picks **only one trace** (e.g. $T_1$, whatever Go map iteration returns first) to store in `Regressions2.trace_id`. Traces $T_2$ through $T_{50}$ are **not** stored in `trace_id`.
   When the UI, triage view, or autobisection queries for regressions on trace $T_2$ with `RegressionsTraceIdField = true`, Spanner searches `trace_id = $T_2` using the index. It finds **no rows**! The regression on $T_2$ is completely invisible.
2. **Forced Fallback to Slow Queries**:
   To find all 50 traces, the instance must disable `RegressionsTraceIdField` and query using the legacy JSONB containment operator `(frame->'dataframe'->'traceset') ? $trace_name`. This eliminates the performance advantage of the `by_trace_id_and_commit` index and brings back heavy Spanner table scans. As documented in the code comment:
   `"For K-means, we select just one trace, but the flag should not be enabled anyway - we have to stick with the old query."`
3. **Key Collisions in Write Paths**:
   In `Write` / `updateBasedOnAlertAlgo` ([`sqlregression2store.go:1029-1035`](../../../go/regression/sqlregression2store/sqlregression2store.go#L1029-L1035)), K-Means passes `traceName = ""`. In `readModifyWriteCompat` ([`sqlregression2store.go:1088-1116`](../../../go/regression/sqlregression2store/sqlregression2store.go#L1088-L1116)), an empty `traceName` forces the lookup to match solely on `(commit_number, alert_id)` via `readCompat`. If an alert produces multiple distinct cluster regressions at the same commit, they overwrite each other!
   Conversely, StepFit passes the exact `traceName`, cleanly isolating each regression under `(commit_number, alert_id, trace_id)`.

**Architectural Takeaway**: K-Means multi-trace clustering is fundamentally incompatible with `regressions2store`'s high-performance `trace_id` B-Tree indexing. Fast, index-accelerated regression lookups require single-trace regressions produced by `StepFitGrouping`.

---

### 2.8 Sparse Traces & Wide Commit Range `[a; b]` Anomaly Detection: Why StepFit is Mandatory

Detecting regressions on sparse traces or querying anomaly detection across wide commit ranges $[a; b]$ (e.g., spans of 1,000–10,000 commits that contain only a handful of actual data points) fundamentally breaks K-Means assumptions.

#### 2.8.1 Why K-Means Fails on Sparse or Wide Ranges

1. **Midpoint Sentinel Rejection (`tooMuchMissingData`)**:
   In K-Means, `kmeansDataframeSlicer` ([`perf/go/dfiter/traceSlicer.go:130-158`](../../../go/dfiter/traceSlicer.go#L130-L158)) creates sliding windows of size $W = 2 \cdot \text{radius} + 1$ over the raw commit indices in `df.Header`.
   In [`perf/go/regression/detector.go:303-305`](../../../go/regression/detector.go#L303-L305):
   ```go
   // If the midpoint is missing, we don't detect for this dataframe's trace.
   if tr[n] == vec32.MissingDataSentinel {
       return true
   }
   ```
   If a benchmark runs sparsely (e.g. once every 20 commits), 19 out of 20 sliding windows have `tr[n] == MissingDataSentinel` at the candidate turning point and are immediately discarded.
2. **Asymmetric Missing Data Filtering**:
   `tooMuchMissingData` ([`detector.go:284-308`](../../../go/regression/detector.go#L284-L308)) checks whether either side of the midpoint has $>50\%$ missing values:
   ```go
   return missing(tr[:n]) || missing(tr[len(tr)-n:])
   ```
   For sparse traces where data is collected intermittently across a wide commit range, missing data rates routinely exceed $80\%$. Consequently, `detector.go:331-333` discards $100\%$ of candidate windows, detecting **zero** anomalies.
3. **Linear Interpolation Artifacts (`vec32.Fill`)**:
   Even if a window survives missing-data filters, `ctrace2.NewFullTrace` ([`perf/go/ctrace2/ctrace.go:53-63`](../../../go/ctrace2/ctrace.go#L53-L63)) runs `vec32.Fill(norm)` to linearly interpolate missing values before computing Euclidean distances and Z-scores.
   Across a wide commit gap (e.g. 100 commits without data), linear interpolation constructs artificial linear slopes. This dilutes sudden, sharp step transitions into gradual ramps, reducing calculated step sizes below the `Interesting` threshold or generating phantom false-positive steps.
4. **Memory & I/O Explosion**:
   Constructing a full DataFrame spanning thousands of commits across $M$ traces builds an enormous $M \times |b - a|$ matrix of `float32` values in `readTracesByChannelForCommitRange` (`ret[name] = vec32.New(len(commits))`, [`sqltracestore.go:1149`](../../../go/tracestore/sqltracestore/sqltracestore.go#L1149)), almost all of which are `vec32.MissingDataSentinel`. Slicing and holding this sparse matrix causes excessive GC pressure and memory consumption.

#### 2.8.2 Why StepFit + `stepFitDfTraceSlicer` Excels on Sparse Traces

`StepFitGrouping` with `stepFitDfTraceSlicer` ([`perf/go/dfiter/traceSlicer.go:13-128`](../../../go/dfiter/traceSlicer.go#L13-L128)) was specifically designed to handle sparse, asynchronous time series:

1. **Upfront Sentinel Elimination**:
   `NewStepFitDfTraceSlicer` ([`traceSlicer.go:101-117`](../../../go/dfiter/traceSlicer.go#L101-L117)) filters out all `vec32.MissingDataSentinel` values upfront without any interpolation:
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
2. **Dense Observation Windowing**:
   The sliding window of size $W$ moves over `validPoints` (the sequence of real observations), **not** over sparse commit indices! Every window evaluated has $0\%$ missing data, and the candidate turning point `tr[n]` is guaranteed to be a real, observed measurement. `tooMuchMissingData` never drops these traces.
3. **Exact Commit Boundary Attribution**:
   The associated commit metadata is preserved in `validHeaders[start:end]`. When `stepfit.EvaluateRule` finds a turning point at index $i$, the turning point maps directly to `validHeaders[i].Offset` and `validHeaders[i-1].Offset`, providing the exact commit range spanning the step, regardless of how many empty commits lie between them.
4. **Streaming Flat Memory Footprint**:
   Sub-DataFrames are emitted one trace at a time (`types.TraceSet{traceID: slicedTrace}`), eliminating dense matrix overhead.

---

### 2.9 Feature Architecture: Detecting Anomalies on Exact, Wide Commit Ranges `[a; b]` with Sparse Observations

When building a feature to run ad-hoc or targeted anomaly detection on an exact commit range $[a; b]$ where the range is wide but data points are sparse:

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

#### Architectural Requirements & Implementation Guidelines:

1. **Extend Domain Representation**:
   Currently, [`types.Domain`](../../../go/types/types.go#L235-L245) only supports `Offset` (single commit mode) or `N` + `End` time (trailing commit mode). To support range $[a; b]$, `types.Domain` must be extended with explicit commit bounds (e.g. `BeginCommit` and `EndCommit`), or a dedicated range detection request struct must be introduced.
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

---

## 3. Auto-Bisection Architecture Across Perf and Pinpoint

When a regression cluster or anomaly is detected and an alert action specifies `Action = BISECT` ([`perf/go/types/types.go:272`](../../../go/types/types.go#L272)), Skia Perf triggers automatic bisection via Temporal workflows.

### 3.1 Perf Orchestrator: `MaybeTriggerBisectionWorkflow`

Location: [`perf/go/workflows/internal/maybe_trigger_bisection.go:51`](../../../go/workflows/internal/maybe_trigger_bisection.go#L51).

1. **Clustering Window Delay**:
   - The workflow waits for `WaitTimeForAnomalyClusteringWindow` (default 30 minutes, [`maybe_trigger_bisection.go:61`](../../../go/workflows/internal/maybe_trigger_bisection.go#L61)) so newly arriving test runs can cluster with the detected anomaly before bisection begins.
2. **Action Dispatch**:
   - Queries `AnomalyGroupServiceUrl` for the anomaly group metadata ([`maybe_trigger_bisection.go:68-72`](../../../go/workflows/internal/maybe_trigger_bisection.go#L68-L72)).
   - If `GroupAction == ag_pb.GroupActionType_BISECT`, it checks `isBisectionAllowed(ctx)` ([`maybe_trigger_bisection.go:91`](../../../go/workflows/internal/maybe_trigger_bisection.go#L91)).
3. **Pinpoint Job Creation**:
   - Finds top anomaly from the group (`findTopAnomalies`, [`maybe_trigger_bisection.go:123`](../../../go/workflows/internal/maybe_trigger_bisection.go#L123)).
   - Converts integer commit positions to Git hashes (`getCommitHashes`, [`maybe_trigger_bisection.go:134-138`](../../../go/workflows/internal/maybe_trigger_bisection.go#L134-L138)).
   - Calls Pinpoint client `createBisectJob` ([`maybe_trigger_bisection.go:143-151`](../../../go/workflows/internal/maybe_trigger_bisection.go#L143-L151)), returning `jobId`.
   - Records `jobId` into the anomaly group via `UpdateAnomalyGroupRequest` ([`maybe_trigger_bisection.go:155-161`](../../../go/workflows/internal/maybe_trigger_bisection.go#L155-L161)).
4. **Polling and Completion**:
   - Polls Pinpoint job status via `waitPinpointJobCompletion` ([`maybe_trigger_bisection.go:167`](../../../go/workflows/internal/maybe_trigger_bisection.go#L167)).
   - Invokes `postBisectionProcessing`: extracts culprits, stores results into `Autobisections` table via `AutobisectionService` ([`maybe_trigger_bisection.go:183-193`](../../../go/workflows/internal/maybe_trigger_bisection.go#L183-L193)), and files or updates IssueTracker issues.

### 3.2 Culprit Persistence & Notification Subsystem (`perf/go/culprit`)

The extraction and alerting of culprit commits is handled by the dedicated `perf/go/culprit` service:

- **`ProcessCulpritWorkflow`** ([`perf/go/workflows/internal/process_culprit.go:17`](../../../go/workflows/internal/process_culprit.go#L17)): Spares child workflows per detected culprit commit.
- **`CulpritService`** ([`perf/go/culprit/service/service.go:23`](../../../go/culprit/service/service.go#L23)):
  - Implements `CulpritServiceServer` gRPC interface.
  - `PersistCulprit`: Upserts culprit commits into the Cloud Spanner `Culprits` table via `Store` ([`perf/go/culprit/store.go:10`](../../../go/culprit/store.go#L10)) and binds `CulpritIDs` to the parent `AnomalyGroup`.
  - `NotifyUserOfCulprit`: Looks up subscriptions and invokes `notifier.NotifyCulpritFound` to automatically file or update Buganizer / IssueTracker issues with Markdown templates.
  - **Subscription Allowlist Defense**: Uses `SheriffConfigsToNotify` ([`service.go:168-177`](../../../go/culprit/service/service.go#L168-L177)) as a safety firewall. Subscriptions not explicitly allowlisted have bug filing redirected to sandbox component `1325852` with empty CCs, preventing accidental notifications during testing.
  - **Repo Mapping Scope**: [`maybe_trigger_bisection.go:600-604`](../../../go/workflows/internal/maybe_trigger_bisection.go#L600-L604) contains an active `TODO(mordeckimarcin) support other repos` where only `chromium` is currently mapped to `common.ChromiumSrcGit`. Support for external sub-repos (V8, WebRTC) requires expanding this repository translation map.

---

## 4. Pinpoint Bisection Execution Workflow

Location: [`pinpoint/go/workflows/internal/bisect.go`](../../../../pinpoint/go/workflows/internal/bisect.go).

### 4.1 Sample Sizing and Adaptive Iterations

- `benchmarkRunIterations = [...]int32{10, 20, 40, 80, 160}` ([`bisect.go:20`](../../../../pinpoint/go/workflows/internal/bisect.go#L20)).
- Bisection starts at initial sample size (default 10, [`bisect.go:180`](../../../../pinpoint/go/workflows/internal/bisect.go#L180)).

### 4.2 Statistical Comparison Engine

Location: [`pinpoint/go/compare/compare.go`](../../../../pinpoint/go/compare/compare.go).

- Combines two non-parametric tests: Kolmogorov-Smirnov (`PValueKS`) and Mann-Whitney U (`PValueMWU`) ([`compare.go:338-343`](../../../../pinpoint/go/compare/compare.go#L338-L343)):
  $$P = \min(P_{\text{KS}}, P_{\text{MWU}})$$
- Verdict logic ([`compare.go:346-359`](../../../../pinpoint/go/compare/compare.go#L346-L359)):
  - $P \le \text{LowThreshold}$ (0.01): `Different` (reject null hypothesis; samples come from different distributions).
  - $P \le \text{HighThreshold}$: `Unknown` (suspicious difference; gather more data).
  - $P > \text{HighThreshold}$: `Same` (statistically indistinguishable).

### 4.3 Recursive Midpoint Search & Culprit Determination

Handled in the workflow selector loop ([`bisect.go:268-350`](../../../../pinpoint/go/workflows/internal/bisect.go#L268-L350)):

- **If Verdict is `Unknown`**:
  - If sample size has reached max (`160`), bisection halts to avoid unbounded cost and assumes statistical equivalence ([`bisect.go:276-280`](../../../../pinpoint/go/workflows/internal/bisect.go#L276-L280)).
  - Otherwise, calls `schedulePairRuns` to double the sample size (e.g. $10 \to 20 \to 40 \to 80 \to 160$) and re-compares ([`bisect.go:282-305`](../../../../pinpoint/go/workflows/internal/bisect.go#L282-L305)).
- **If Verdict is `Different`**:
  - Executes `FindMidCommitActivity` ([`bisect.go:309`](../../../../pinpoint/go/workflows/internal/bisect.go#L309)).
  - Checks if `mid == lower` using `CheckCombinedCommitEqualActivity` ([`bisect.go:315`](../../../../pinpoint/go/workflows/internal/bisect.go#L315)).
    - **Culprit Found**: If `equal == true` (commits are adjacent), the culprit is precisely **`higher`** ([`bisect.go:322-330`](../../../../pinpoint/go/workflows/internal/bisect.go#L322-L330)), because `lower` was verified baseline and no intermediate commits exist. It records `be.Culprits` and `be.DetailedCulprits` (`Prior: lower, Culprit: higher`).
    - If not equal, it schedules benchmark runs for `mid` and bisects subranges `[lower, mid]` and `[mid, higher]` concurrently via the Temporal selector.
- **If Verdict is `Same`**:
  - Terminates further exploration of that commit range.

### 4.4 Large Commit Ranges & DEPS Unravelling Edge Cases

Implemented in [`pinpoint/go/midpoint/midpoint.go`](../../../../pinpoint/go/midpoint/midpoint.go) and documented in [`pinpoint/go/midpoint/doc.go:64-91`](../../../../pinpoint/go/midpoint/doc.go#L64-L91):

1. **Multi-Dependency Rolls**: When a single Chromium commit rolls multiple dependencies (e.g. both V8 and WebRTC), `diffUrl` ([`midpoint.go:185`](../../../../pinpoint/go/midpoint/midpoint.go#L185)) breaks on the first matched dependency. Regressions introduced by subsequent dependencies in that same roll are currently masked.
2. **Depth-1 Nesting Limit**: Bisection unrolls top-level DEPS into child repositories, but nested rolls within dependencies (e.g. V8 rolling an internal third-party dependency) are treated as adjacent and cannot be bisected further.
3. **Non-Linear Git History**: Pinpoint relies on linear `git rev-list` ordering; branched or non-linear history breaks midpoint positioning.
4. **Gitiles Latency / Memory Scale Limits**: Enormous commit ranges generate large Gitiles commit logs via `LogFirstParent`, increasing RPC latency and memory pressure on the midpoint service.

---

## 5. Partner Triage Workflows: V8 vs. Android

Triage workflows diverge significantly based on whether repositories use linear Git commits or synthetic build metadata:

### 5.1 V8 Triage Workflow

- **Sheriff Config Alerting**: V8 uses alerts driven by `sheriff_config` protobuf definitions ([`perf/go/sheriffconfig/proto/v1/sheriff_config.proto`](../../../go/sheriffconfig/proto/v1/sheriff_config.proto)). In instance configs (`v8-internal.json`, [`perf/configs/spanner/v8-internal.json:105`](../../../configs/spanner/v8-internal.json#L105)), `keys_for_commit_range` includes `["V8", "WebRTC", "V8 Git Hash", "WebRTC Git Hash"]`.
- **UI Restrictions & Custom Routing**:
  - `perf/configs/spanner/v8-internal.json` explicitly sets `"show_bisect_btn": false` and `"show_triage_link": false` ([`v8-internal.json:112-113`](../../../configs/spanner/v8-internal.json#L112-L113)) and `"landing_page_rel_path": "/m/"` (multigraph).
  - Pinpoint auto-bisection does not directly bisect V8 standalone builds without Chromium PGO profiles, so UI bisection actions are restricted in favor of V8 rotation triage.
- **Direct Git Commit Mapping**: Git revisions link directly to `https://chromium.googlesource.com/v8/v8/+/...` via `point-links-sk` ([`perf/modules/point-links-sk/point-links-sk.ts:120-125`](../../../modules/point-links-sk/point-links-sk.ts#L120-L125)).

### 5.2 Android / AndroidX Triage Workflow

- **Synthetic Build IDs**: Android benchmark traces are indexed against internal Android build numbers rather than standard Git commit hashes.
- **`AndroidNotificationProvider`**:
  - Implemented in [`perf/go/notify/android_notification_provider.go`](../../../go/notify/android_notification_provider.go).
  - Implements `GetBuildIdUrlDiff()` ([`android_notification_provider.go:68-80`](../../../go/notify/android_notification_provider.go#L68-L80)) extracting `"Build ID"` metadata and generating range search URLs:
    `https://android-build.corp.google.com/range_search/cls/from_id/{from}/to_id/{to}/?s=menu&includeTo=0&includeFrom=1`
  - Automatically formats affected tests in notification bug templates as:
    `{{test_class}}#{{test_method}} ({{device_name}} {{os_version}})` ([`android_notification_provider.go:63`](../../../go/notify/android_notification_provider.go#L63)).
  - Directs sheriffs to partner playbook: `http://go/androidx-bench-triage` ([`perf/configs/spanner/android.json:17`](../../../configs/spanner/android.json#L17)).

---

## 6. Edge Cases in Large Commit Ranges & Dependency Rolls

1. **DEPS Roll Midpoint Ingestion**:
   - In [`pinpoint/go/midpoint/midpoint.go`](../../../../pinpoint/go/midpoint/midpoint.go), when bisecting a range where a dependency was rolled (e.g. Chromium updated V8 from revision $A$ to $B$), `FindMidCombinedCommit` detects adjacent parent commits and parses the `DEPS` file across Gitiles logs.
   - It creates a `CombinedCommit` pinning the parent Chromium commit and bisecting commits within the child V8 or WebRTC repository ([`midpoint.go:245-265`](../../../../pinpoint/go/midpoint/midpoint.go#L245-L265)).
2. **Missing Repository Backfilling**:
   - If one side of a bisection pair contains modified dependency overrides and the other does not, Pinpoint backfills the missing repository hashes by inspecting the `DEPS` file at that exact base revision.
3. **Flaky & Missing Runs**:
   - If Swarming benchmark runs fail, Pinpoint backdoors into **Functional Analysis** (`CompareFunctional`, [`pinpoint/go/compare/compare.go:251`](../../../../pinpoint/go/compare/compare.go#L251)), treating execution successes as $1$ and failures as $0$ to bisect the introduction of a crash or build error.
