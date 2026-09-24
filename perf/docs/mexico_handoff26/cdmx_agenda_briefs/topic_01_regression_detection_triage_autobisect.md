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

| Algorithm (`types.StepDetection`) | Calculation / Formula | Key Characteristics & Threshold Semantics | Code Reference |
| :--- | :--- | :--- | :--- |
| **`OriginalStep`** | $\text{StepSize} = y_0 - y_1$<br>$\text{LSE} = \frac{\sqrt{SSE_1 + SSE_2}}{N}$<br>$\text{Regression} = \frac{\text{StepSize}}{\max(\text{LSE}, \sigma_{\text{threshold}})}$ | Original recipe step detection. Normalizes step size by pooled least-squares error. If fit error is below noise threshold, caps divisor at $\sigma_{\text{threshold}}$. | [`stepfit.go:189-211`](../../../go/stepfit/stepfit.go#L189-L211) |
| **`AbsoluteStep`** | $\text{StepSize} = y_0 - y_1$<br>$\text{Regression} = y_0 - y_1$ | Checks if the raw delta between pre- and post-means exceeds an absolute value threshold. Useful for fixed-overhead bounds (e.g. memory). | [`stepfit.go:215-218`](../../../go/stepfit/stepfit.go#L215-L218) |
| **`Const`** | $\text{StepSize} = -1.0$<br>$\text{Regression} = \| \text{trace}[i] \|$ | Checks if the absolute magnitude of the point at the turning point exceeds a fixed constant threshold. | [`stepfit.go:222-229`](../../../go/stepfit/stepfit.go#L222-L229) |
| **`PercentStep`** | $\text{StepSize} = \frac{y_0 - y_1}{y_0}$<br>$\text{Regression} = \text{StepSize}$ | Checks relative percentage change from baseline mean $y_0$. Protects against division by zero ($\pm \infty \to \pm \text{MaxFloat32}$, $\text{NaN} \to 0$). | [`stepfit.go:233-245`](../../../go/stepfit/stepfit.go#L233-L245) |
| **`PercentMedianStep`** | Same as `PercentStep`, but $y_0 = \text{median}(\text{trace}[:i])$ and $y_1 = \text{median}(\text{trace}[i:])$ | Robust against single-point outlier spikes that distort sample means. | [`stepfit.go:95-97`](../../../go/stepfit/stepfit.go#L95-L97), [`233-245`](../../../go/stepfit/stepfit.go#L233-L245) |
| **`CohenStep`** | $\text{StepSize} = \frac{y_0 - y_1}{\max(s_p, \sigma_{\text{threshold}})}$<br>$s_p = \sqrt{\frac{(n_1-1)s_1^2 + (n_2-1)s_2^2}{n_1+n_2-2}}$ | Evaluates Cohen's $d$ effect size using pooled sample variance. Standardizes the difference relative to trace variance. Requires $N \ge 4$. | [`stepfit.go:249-287`](../../../go/stepfit/stepfit.go#L249-L287) |
| **`Stepiness`** | $\text{steppiness} = 1.0 - \sqrt{\frac{SSE_1 + SSE_2}{SSE_{\text{total}}}}$<br>$\text{Regression} = \pm \text{steppiness}$ | Normalized metric $[0, 1]$ measuring how well a binary step function accounts for total trace variance compared to the overall mean. Matches Catapult legacy formula. | [`stepfit.go:319-333`](../../../go/stepfit/stepfit.go#L319-L333) |
| **`MannWhitneyU`** | Calculates Mann-Whitney U test between `trace[:i]` and `trace[i:]`<br>$\text{Regression} = p\text{-value}$, $\text{LSE} = U$ | Non-parametric rank-sum hypothesis test. Triggers anomaly when $p \le \text{threshold}$ (e.g. $\alpha = 0.05$). Direction is determined by the sign of $(y_0 - y_1)$. | [`stepfit.go:150-157`](../../../go/stepfit/stepfit.go#L150-L157), [`290-299`](../../../go/stepfit/stepfit.go#L290-L299) |

---

## 2. Clustering and Multi-Trace Aggregation

In production, benchmarks frequently emit dozens or hundreds of sub-traces (e.g., individual tests within Speedometer, memory allocations across browser processes). Evaluating each trace in isolation can lead to alert storms. Perf uses clustering to consolidate co-moving traces into single regression events.

### 2.1 K-Means Clustering Pipeline
- Implementation: [`perf/go/kmeans/kmeans.go`](../../../go/kmeans/kmeans.go) and [`perf/go/clustering2/clustering.go`](../../../go/clustering2/clustering.go).
- Process:
  1. Traces matching an alert's query criteria are normalized and converted into `kmeans.Clusterable` vectors.
  2. K-Means runs up to `MAX_KMEANS_ITERATIONS = 100` ([`clustering.go:29`](../../../go/clustering2/clustering.go#L29)).
  3. For each cluster centroid, `stepfit.EvaluateRule` is executed to test whether the centroid itself exhibits a step function regression ([`clustering.go:200-205`](../../../go/clustering2/clustering.go#L200-L205)).
  4. Traces whose centroids cross the threshold are bundled into a `ClusterSummary` struct ([`clustering.go:38-60`](../../../go/clustering2/clustering.go#L38-L60)), recording `StepFit`, `Centroid`, `Keys`, and `ParamSummaries`.

### 2.2 StepFit Grouping (Individual Mode)
- Traces can also be evaluated individually without K-Means by setting `Algo: types.StepFitGrouping` in the alert configuration ([`perf/go/types/types.go:131`](../../../go/types/types.go#L131)).
- In this mode, `dfiter.NewStepFitDfTraceSlicer` ([`perf/go/dfiter/traceSlicer.go:95`](../../../go/dfiter/traceSlicer.go#L95)) runs a sliding window over every trace independently, feeding results to the regression refiners.

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
  - Note how `AndroidNotificationProvider` duplicates other providers (TODO: which one?)

---

## 6. Edge Cases in Large Commit Ranges & Dependency Rolls

1. **DEPS Roll Midpoint Ingestion**:
   - In [`pinpoint/go/midpoint/midpoint.go`](../../../../pinpoint/go/midpoint/midpoint.go), when bisecting a range where a dependency was rolled (e.g. Chromium updated V8 from revision $A$ to $B$), `FindMidCombinedCommit` detects adjacent parent commits and parses the `DEPS` file across Gitiles logs.
   - It creates a `CombinedCommit` pinning the parent Chromium commit and bisecting commits within the child V8 or WebRTC repository ([`midpoint.go:245-265`](../../../../pinpoint/go/midpoint/midpoint.go#L245-L265)).
2. **Missing Repository Backfilling**:
   - If one side of a bisection pair contains modified dependency overrides and the other does not, Pinpoint backfills the missing repository hashes by inspecting the `DEPS` file at that exact base revision.
3. **Flaky & Missing Runs**:
   - If Swarming benchmark runs fail, Pinpoint backdoors into **Functional Analysis** (`CompareFunctional`, [`pinpoint/go/compare/compare.go:251`](../../../../pinpoint/go/compare/compare.go#L251)), treating execution successes as $1$ and failures as $0$ to bisect the introduction of a crash or build error.
