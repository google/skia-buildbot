# Topic 7: Operational Summaries: CBB, Bot Fleet Capacity, Task Backlog Audit & Skia Gold

## Executive Summary
This brief synthesizes operational topics from the CDMX working agenda, focusing on Comparative Browser Benchmarking (CBB), Swarming bot fleet capacity management, task backlog auditing in Temporal/Pinpoint, and the architectural parallels with Skia Gold.

---

## 1. Comparative Browser Benchmarking (CBB)

CBB is an automated benchmarking subsystem within Pinpoint designed to track Chrome's performance relative to rival browsers (Apple Safari, Microsoft Edge) on identical reference hardware.

### 1.1 Architecture & Workflow Orchestration
- Workflow definitions: [`pinpoint/go/workflows/internal/cbb_new_release_detector.go`](../../../../pinpoint/go/workflows/internal/cbb_new_release_detector.go) and [`pinpoint/go/workflows/internal/cbb_runner.go`](../../../../pinpoint/go/workflows/internal/cbb_runner.go).
- `CbbNewReleaseDetectorWorkflow` runs periodically via a Temporal schedule (`"CBB Schedule"`, [`cbb_new_release_detector.go:9-21`](../../../../pinpoint/go/workflows/internal/cbb_new_release_detector.go#L9-L21)):
  1. Detects new browser release versions for Chrome, Safari, and Edge (`CbbGetBrowserVersionsWorkflow`, [`cbb_new_release_detector.go:254`](../../../../pinpoint/go/workflows/internal/cbb_new_release_detector.go#L254)).
  2. For Safari, downloads the latest Safari Technology Preview (STP) packages and uploads them to CIPD via `DownloadSafariTPWorkflow` ([`pinpoint/go/workflows/internal/stp_downloader.go`](../../../../pinpoint/go/workflows/internal/stp_downloader.go)), tagging packages with `"stable"` to auto-install on physical lab devices ([`stp_downloader.go:235`](../../../../pinpoint/go/workflows/internal/stp_downloader.go#L235)).
  3. Triggers `CbbRunnerWorkflow` ([`cbb_runner.go`](../../../../pinpoint/go/workflows/internal/cbb_runner.go)) across target platforms.

### 1.2 Dedicated Hardware Lab Fleet
- CBB runs on specialized bare-metal test pools defined in [`pinpoint/go/bot_configs/internal.json`](../../../../pinpoint/go/bot_configs/internal.json):
  - `mac-m3-pro-perf-cbb`, `mac-m4-mini-perf-cbb`, `mac-m5-pro-perf-cbb`
  - `win-victus-perf-cbb`, `win-arm64-snapdragon-elite-perf-cbb`
  - `android-pixel10-perf-cbb`, `android-pixel-tangor-perf-cbb`
- Benchmarks executed include Speedometer 3, JetStream 2/3, and MotionMark.
- Results are uploaded to GCS and visualized on dedicated CBB dashboard sections in `chrome-internal.json` ([`perf/configs/spanner/chrome-internal.json:75-120`](../../configs/spanner/chrome-internal.json#L75-L120)).

---

## 2. Bot Fleet Capacity & Swarming Allocation

A constant bottleneck in running automatic bisections and try jobs is physical bot fleet capacity.

### 2.1 Bot Selection & Load Balancing
- Implemented in `FindAvailableBotsActivity` ([`pinpoint/go/workflows/internal/pairwise_runner.go:160-190`](../../../../pinpoint/go/workflows/internal/pairwise_runner.go#L160-L190)).
- Queries Swarming for bots that are:
  - Free (not executing a task).
  - Alive (heartbeating within threshold).
  - Non-quarantined (passing hardware diagnostics).
- Shuffles available bot IDs using a deterministic seed-based pseudo-random algorithm ([`pairwise_runner.go:184-187`](../../../../pinpoint/go/workflows/internal/pairwise_runner.go#L184-L187)) to distribute wear evenly and prevent hotspotting on the first returned bot.

### 2.2 Starvation Prevention & Priority Tiers
- CQ tryjobs take precedence over continuous bisections.
- Bot pools are segregated in dimensions:
  - `chrome.tests.pinpoint-cq`: Dedicated to presubmit commit-queue trybots.
  - `chrome.tests.pinpoint`: Dedicated to post-commit bisections.
  - `chrome.tests.pinpoint-cbb`: Dedicated to comparative benchmarking.
- This prevents a spike in long-running waterfall bisections from halting developer merges.

---

## 3. Task Backlog Audit & Workflow Health

Maintaining healthy queue throughput requires ongoing monitoring of workflow queues and task latency:
1. **Temporal Task Queues**:
   - Workflows are segregated into specific task queues (e.g. `perf.bisect`, `perf.pairwise`, `perf.cbb`).
   - Worker concurrency is tuned in [`pinpoint/go/workflows/worker/main.go`](../../../../pinpoint/go/workflows/worker/main.go) to match GKE pod memory and CPU limits.
2. **Activity Timeouts & Zombie Job Recovery**:
   - Every Swarming task request includes explicit execution and expiration timeouts.
   - If a device drops off the network or panics during a benchmark, Temporal activity retry policies catch timeout errors and reschedule the task on an alternative bot from the available pool.

---

## 4. Architectural Parallels with Skia Gold

Skia Gold is Skia's automated visual regression detection service. While Perf tracks scalar numerical time series (e.g., milliseconds, bytes), Gold tracks rasterized image digests. Both systems share core architectural foundations:

| Architecture Layer | Skia Perf | Skia Gold | Shared Value & Synergy |
| :--- | :--- | :--- | :--- |
| **Ingestion Event Model** | Test outputs $\to$ GCS $\to$ Pub/Sub notification $\to$ Ingestion Worker | Rendered images $\to$ GCS $\to$ Pub/Sub $\to$ Gold Ingestion Worker | Identical Pub/Sub batch pull and retry mechanics |
| **Commit Ordering** | Linearized `types.CommitNumber` resolved from Gitiles / Git | Linearized commit positions resolved via `git.Git` | Shared commit cache and indexing infrastructure |
| **Storage Backend** | Cloud Spanner (`Traces`, `Postings`, `Anomalies`) | Cloud Spanner / CockroachDB (`Expectations`, `Digests`) | Shared SQL migration framework and tooling |
| **Web UI Architecture** | LitElement Web Components (`cluster-summary2-sk`, `perf-scaffold-sk`) | LitElement Web Components (`cluster-digests-sk`, `cluster-page-sk`) | Shared Design System and UI component libraries (`elements-sk`) |
| **Triage Paradigm** | Triaging step-fits (Untriaged, Positive/Improvement, Negative/Regression) | Triaging image diffs (Untriaged, Positive/Approved, Negative/Bug) | Common mental model for sheriffs across visual and performance regressions |
