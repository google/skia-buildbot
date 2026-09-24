# Topic 4: Demystifying Legacy Pinpoint and Scoping the Q4/Q1 Migration to Skia Infra

## Executive Summary
This brief deconstructs the architecture of legacy Catapult Pinpoint and contrasts it with the new Go-based Skia Pinpoint implementation. It articulates the technical sequencing rationale for migrating workflow execution before frontend cutover, inventories legacy Appspot functional boundaries, and details the remaining feature parity gaps required to complete the migration during Q4/Q1.

---

## 1. Architectural Contrast: Legacy Catapult vs. Skia Pinpoint

| System Dimension | Legacy Catapult Pinpoint (`catapult/dashboard/pinpoint`) | New Skia Pinpoint (`pinpoint/go/...`) |
| :--- | :--- | :--- |
| **Language & Runtime** | Python 2.7 / Python 3 on Google App Engine (Standard) | Go 1.23+ compiled binaries on Google Kubernetes Engine (GKE) |
| **Workflow Engine** | Custom App Engine TaskQueues with Datastore entity mutations | Temporal durable execution workflows (`go.temporal.io/sdk/workflow`) |
| **Storage Layer** | Google Cloud Datastore / NDB (`Job`, `JobState`, `Quest`) | Google Cloud Spanner (`Autobisections`, `Culprits`, `Jobs`) + GCS |
| **Build Dispatch** | Buildbucket v1/v2 API calls interleaved with datastore retries | Structured Go client in [`pinpoint/go/backends/buildbucket.go`](../../../../pinpoint/go/backends/buildbucket.go) |
| **Test Execution** | Swarming REST API polling via App Engine cron handlers | Swarming Go client in [`pinpoint/go/backends/swarming.go`](../../../../pinpoint/go/backends/swarming.go) with CAS digest tracking |
| **Statistical Analysis** | Python `scipy.stats` routines embedded in App Engine instances | Native Go statistical libraries in [`pinpoint/go/compare/stats/`](../../../../pinpoint/go/compare/stats/) |
| **Frontend UI** | Polymer 1.x / 2.x web components (`job.html`, `job-page.html`) | Modern Angular WebUI in [`pinpoint/webui/`](../../../../pinpoint/webui/) with zoneless change detection |

---

## 2. Sequencing Rationale: Execution Before UI Migration

Migrating a mission-critical bisect service that handles hundreds of daily bisections across Chrome, Android, and WebRTC carries severe risk of developer disruption. The team adopted a staged migration strategy:

### Phase 1: Re-platforming Execution (Temporal Workflows)
- Rather than trying to migrate UI and execution simultaneously, the backend execution engine was rewritten first in Go on Temporal ([`pinpoint/go/workflows/internal/bisect.go`](../../../../pinpoint/go/workflows/internal/bisect.go)).
- **Why Temporal?**:
  - Legacy Pinpoint suffered from stuck taskqueue tasks, datastore transaction conflicts, and inability to resume complex multi-day bisections after transient App Engine restarts.
  - Temporal provides durable event sourcing: if a worker pod crashes while waiting for an 8-hour Swarming build, execution resumes transparently at the exact state without re-running finished tests.
- **Backwards Compatibility Bridge**:
  - The team implemented [`CatapultBisectWorkflow`](../../../../pinpoint/go/workflows/catapult/catapult_bisect.go#L140).
  - When invoked, Temporal runs the bisection in Go, converts the results via `ConvertToCatapultResponseWorkflow` ([`catapult_bisect.go:63`](../../../../pinpoint/go/workflows/catapult/catapult_bisect.go#L63)), and invokes `WriteBisectToCatapultActivity` ([`catapult_bisect.go:178`](../../../../pinpoint/go/workflows/catapult/catapult_bisect.go#L178)) to write back to legacy Datastore (`https://pinpoint-dot-chromeperf.appspot.com/api/job`).
  - Users continue viewing results in familiar Catapult dashboards while running on modern Go/Temporal execution.

### Phase 2: Frontend Migration & WebUI Cutover
- With execution proven stable on Temporal, build the standalone Angular WebUI ([`pinpoint/webui/`](../../../../pinpoint/webui/)) and Pinpoint API service ([`pinpoint/go/service/`](../../../../pinpoint/go/service/)).
- Cut over the browser interface, deprecate writebacks, and shut down App Engine.

---

## 3. Legacy Boundaries: What Appspot Still Does

Despite the majority of bisections running through Temporal, legacy Appspot continues to handle several peripheral responsibilities:
1. **Legacy Datastore as the Source of Record for Old Jobs**: Decades of historical Pinpoint bisection runs remain stored in App Engine Datastore.
2. **IssueTracker Auto-Filing for Catapult Jobs**: Some legacy sheriffing automations read Datastore job statuses to auto-comment on Buganizer issues.
3. **Catapult UI Rendering**: Users navigating to `pinpoint-dot-chromeperf.appspot.com/job/<job_id>` rely on Python App Engine handlers reading NDB entities.
4. **Sandwich Verification Pre-Filtering**: Certain heuristic sandwich tryjobs were historically initiated by Catapult cron handlers.

---

## 4. Remaining Parity Gaps for Q4/Q1 Migration

To complete the cutover and permanently decommission App Engine, the following functional gaps must be fully resolved:

### 4.1 Pairwise Try Job Parity
- **Status**: Implemented in [`pinpoint/go/workflows/internal/pairwise_runner.go`](../../../../pinpoint/go/workflows/internal/pairwise_runner.go).
- **Mechanism**:
  - Compares two commits (Control vs. Treatment) with interleaved execution (`workflows.PairwiseOrder`, [`pairwise_runner.go:199-210`](../../../../pinpoint/go/workflows/internal/pairwise_runner.go#L199-L210)).
  - Runs the **Pairwise Wilcoxon Signed-Rank Test** ([`pinpoint/go/compare/compare.go:141-211`](../../../../pinpoint/go/compare/compare.go#L141-L211), [`pinpoint/go/compare/stats/wilcoxon_signed_rank_test.go`](../../../../pinpoint/go/compare/stats/wilcoxon_signed_rank_test.go)).
  - Applies logarithmic or normalization transforms to defend against zero-valued test metrics ([`compare.go:168-182`](../../../../pinpoint/go/compare/compare.go#L168-L182)).
- **Remaining Task**: Ensure database writebacks into Spanner are fully validated for all try job invocations ([`pinpoint/go/workflows/worker/main.go:36`](../../../../pinpoint/go/workflows/worker/main.go#L36)).

### 4.2 Telemetry Command-Line Argument Parity
- Legacy Pinpoint had hundreds of special-cased quest arguments for different benchmark types.
- Parity is maintained in [`pinpoint/go/run_benchmark/telemetry.go:35-180`](../../../../pinpoint/go/run_benchmark/telemetry.go#L35-L180), matching arguments line-by-line with Catapult's `run_telemetry_test.py`.
- **Remaining Task**: Final audit of obscure benchmark story tags and system health arguments (`system_health.common_desktop`, etc.).

### 4.3 Pinpoint WebUI Feature Completeness
- The new UI in [`pinpoint/webui/`](../../../../pinpoint/webui/) must support:
  - Bisection tree graph visualization (rendering commit nodes, status colors, p-values, sample distribution histograms).
  - Manual job creation form with bot config autocomplete, benchmark picker, and patch upload.
  - User job list with filtering by author, benchmark, status, and repository.
- Integration tests in [`pinpoint/webui/tsconfig.test.json`](../../../../pinpoint/webui/tsconfig.test.json) validate these views.

---

## 5. Q4/Q1 Migration Roadmap & Exit Criteria

```
Q4 Milestone 1: Spanner Writeback Default
├── Enable databaseWriteback = true in production worker
└── Validate parity of Autobisections / Culprits records against legacy Datastore

Q4 Milestone 2: WebUI Beta Launch
├── Deploy pinpoint/go/webui to GKE
└── Enable parallel UI evaluation for Chrome/V8 sheriffs

Q1 Milestone 3: Writeback Cutoff & Redirects
├── Remove WriteBisectToCatapultActivity from CatapultBisectWorkflow
├── Redirect pinpoint-dot-chromeperf.appspot.com to new WebUI
└── Deprecate Python App Engine service
```
