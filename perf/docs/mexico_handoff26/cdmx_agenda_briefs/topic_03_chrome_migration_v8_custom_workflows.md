# Topic 3: Completing Chrome Migration and Supporting V8's Custom Workflow

## Executive Summary
This brief analyzes the remaining dependencies, technical blockers, and architectural milestones required to complete the migration of Chrome and V8 performance infrastructure from legacy App Engine (`chromeperf.appspot.com` and Catapult Pinpoint) to Skia Perf and Temporal-based Pinpoint. It provides deep technical insight into V8's reliance on custom backend APIs, multi-repository roll bisections involving Profile Guided Optimization (PGO), and the critical path to decommission legacy Appspot services.

---

## 1. V8 Custom Backend APIs & Anomaly Page Reliance

The V8 performance monitoring workflow possesses specific requirements that distinguish it from standard Chromium waterfall benchmarks:

### 1.1 Custom Alert Routing & The `/a/` Page
- In standard Skia Perf instances, alert triage and regression views route to `/r2/` or the new triage UI.
- In V8 instances (both corp and public, defined in [`v8-internal.json`](../../configs/spanner/v8-internal.json) and [`v8-public.json`](../../configs/spanner/v8-public.json)), the instance configuration sets:
  ```json
  "new_alerts_page": true,
  "show_triage_link": false
  ```
- This forces routing to the dedicated `/a/` anomaly page ([`perf/go/config/config.go:1160-1175`](../../go/config/config.go#L1160-L1175)). The V8 sheriff team relies directly on this view to monitor subscriptions defined in `sheriff_config` protobuf specs ([`perf/go/sheriffconfig/proto/v1/sheriff_config.proto`](../../go/sheriffconfig/proto/v1/sheriff_config.proto)).
- The backend serves these alerts through `SheriffConfigService` ([`perf/go/sheriffconfig/service/service.go`](../../go/sheriffconfig/service/service.go)), mapping subscriptions to individual alerts and anomaly groups.

### 1.2 Multi-Commit Range Resolution in UI
- Unlike single-repository instances that monitor only Chromium or Skia, V8 benchmark traces reflect multiple interrelated codebases:
  ```json
  "keys_for_commit_range": ["V8", "WebRTC", "V8 Git Hash", "WebRTC Git Hash"]
  ```
  ([`perf/configs/spanner/v8-internal.json:105`](../../configs/spanner/v8-internal.json#L105)).
- The frontend web component [`perf/modules/point-links-sk/point-links-sk.ts`](../../modules/point-links-sk/point-links-sk.ts) contains explicit logic for V8 commit hashing ([`point-links-sk.ts:120-125`](../../modules/point-links-sk/point-links-sk.ts#L120-L125), [`445-450`](../../modules/point-links-sk/point-links-sk.ts#L445-L450)):
  - It extracts `V8 Git Hash` from the ingestion payload at commit $n$ and commit $n-1$.
  - It constructs a Gitiles range comparison URL:
    `https://chromium.googlesource.com/v8/v8/+log/{prev_hash}..{curr_hash}`
  - If the hash has not changed between commits, it renders a direct link to the singular commit.
- In [`perf/go/dataframe/metadata.go:137-142`](../../go/dataframe/metadata.go#L137-L142), backend metadata generators resolve commit keys to upstream Gitiles endpoints specifically for `"V8"` and `"WebRTC"`.

---

## 2. Trace Handling and Telemetry Formatting

The legacy Chromeperf pipeline relied on Catapult's Telemetry framework and its JSON-based histogram format.

### 2.1 Ingestion Format Parity
- Ingestion parsers in [`perf/go/ingest/parser/parser.go`](../../go/ingest/parser/parser.go) and [`perf/go/perfresults/perf_results_parser.go`](../../go/perfresults/perf_results_parser.go) process both modern Skia formats and Catapult Histogram-Set JSONs.
- Each trace key is encoded as an inverted parameter map:
  `,benchmark=v8.browsing_mobile,master=ChromiumPerf,test=memory:chrome:all_processes:reported_by_chrome:v8:heap:code_space:effective_size,`
- Telemetry trace logs contain custom measurement units (e.g. `ms_smallerIsBetter`, `sizeInBytes_smallerIsBetter`). The refiner logic in [`perf/go/regression/refiner/default_regression_refiner.go:40-65`](../../go/regression/refiner/default_regression_refiner.go#L40-L65) uses these units to determine regression directionality (`UP`, `DOWN`, or `BOTH`).

---

## 3. V8 Pinpoint Blocker: Multi-Repo & PGO Combinations

A major blocker preventing full transition of V8 bisections to modern Pinpoint stems from the complexity of Chromium DEPS rolls and Profile Guided Optimization (PGO) builds.

### 3.1 The Multi-Repo DEPS Roll Problem
- V8 performance tests in Chrome frequently run inside a Chromium browser shell rather than standalone `d8` binaries.
- When an anomaly is detected on Chromium waterfall traces that originate from a V8 DEPS roll:
  1. The Chromium commit range might span a single commit: `Roll V8 from a1b2c3 to d4e5f6 (15 commits)`.
  2. Pinpoint must bisect the 15 commits *inside* the V8 repository ([`pinpoint/go/midpoint/midpoint.go:245-265`](../../../../pinpoint/go/midpoint/midpoint.go#L245-L265)).
  3. To build a testable Chromium binary for intermediate V8 commit $V_{\text{mid}}$, Pinpoint must compile Chromium with a modified `DEPS` file pointing `src/v8` to $V_{\text{mid}}$.

### 3.2 The PGO Profile Mismatch
- Modern Chrome release and performance builds are compiled with Profile Guided Optimization (PGO) profiles to optimize hot code paths.
- Problem: Intermediate commits synthesized during a bisection (e.g., Chromium $C_1$ combined with an unreleased V8 commit $V_{\text{mid}}$) do not have pre-computed PGO profiles.
- If a benchmark bot configuration uses a PGO builder target (e.g. `mac-m1_mini_2020-perf-pgo`, `linux-perf-pgo`), compilation with an incompatible or missing profile can either:
  1. Fail the build entirely due to missing profile artifacts in CIPD/CAS.
  2. Produce a binary with unpredictable compiler optimizations, introducing artificial noise into the benchmark that dwarfs the regression being measured.
- Mitigation & Architecture:
  - As implemented in [`pinpoint/go/workflows/internal/cbb_runner.go`](../../../../pinpoint/go/workflows/internal/cbb_runner.go) and [`pinpoint/go/bot_configs/isolate_targets.yaml`](../../../../pinpoint/go/bot_configs/isolate_targets.yaml), Pinpoint must explicitly map bot configurations to non-PGO fallback targets or fetch nearest-matching profile CAS digests when executing sub-repo bisections.

---

## 4. Path to Appspot Shutdown

Decommissioning `chromeperf.appspot.com` and `pinpoint-dot-chromeperf.appspot.com` requires eliminating all bridge components that route traffic back to App Engine:

```
[Legacy State]
Perf/Pinpoint (Go/Temporal) ──► CatapultClient.WriteBisectToCatapult()
                                            │
                                            ▼
                               [pinpoint-dot-chromeperf.appspot.com]
                                            │
                                            ▼
                                  [App Engine Datastore]
                                            │
                                            ▼
                                   [Catapult Web UI]

[Target Migrated State]
Perf/Pinpoint (Go/Temporal) ──► Spanner Database (Autobisections, Culprits)
                                            │
                                            ▼
                               [Pinpoint Go Web Service]
                                            │
                                            ▼
                               [New Pinpoint Angular WebUI]
```

### 4.1 Step 1: Replace UI Writeback with Spanner Storage
- In the transition architecture, when `CatapultBisectWorkflow` finishes, it converts its results and makes an HTTP POST request to `catapultBisectPostUrl = "https://pinpoint-dot-chromeperf.appspot.com/api/job"` ([`pinpoint/go/workflows/catapult/write.go:19`](../../../../pinpoint/go/workflows/catapult/write.go#L19), [`58-75`](../../../../pinpoint/go/workflows/catapult/write.go#L58-L75)) so legacy Catapult can render the job.
- **Action**: Fully enable `databaseWriteback` flag in Pinpoint worker ([`pinpoint/go/workflows/worker/main.go:36`](../../../../pinpoint/go/workflows/worker/main.go#L36)), writing bisection state directly to Cloud Spanner `Autobisections` and `Culprits` tables.

### 4.2 Step 2: Cut Over the Frontend UI
- Complete the new Pinpoint WebUI located in [`pinpoint/webui/`](../../../../pinpoint/webui/) (built on modern Angular with zoneless change detection, [`pinpoint/webui/app/app.config.ts:1`](../../../../pinpoint/webui/app/app.config.ts#L1)).
- Serve the UI via [`pinpoint/go/webui/main.go`](../../../../pinpoint/go/webui/main.go), pointing Chrome and V8 sheriffs to the new dashboard.

### 4.3 Step 3: Decommission `skia-bridge` and Appspot Instances
- Remove `chromePerfClientImpl` calls to `skia-bridge-dot-chromeperf.appspot.com` ([`perf/go/chromeperf/chromeperfClient.go:26`](../../go/chromeperf/chromeperfClient.go#L26)).
- Shut down App Engine services and redirect legacy URLs to Skia Perf.
