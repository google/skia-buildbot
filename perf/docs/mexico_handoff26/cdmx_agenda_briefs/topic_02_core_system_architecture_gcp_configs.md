# Topic 2: Core System Architecture, GCP Hosting, and Configuration Sprawl

## Executive Summary
This brief outlines the infrastructure architecture of Skia Perf and Pinpoint, tracing the complete lifecycle of performance data from Swarming bot execution in Chrome/Android lab fleets to Google Cloud Platform (GCP) ingestion pipelines, Cloud Spanner storage, and interactive web visualization. It details the containerized microservices hosted on Google Kubernetes Engine (GKE), evaluates multi-tenant instance isolation, and provides an architectural reference for configuration management.

---

## 1. End-to-End Data Pipeline

The data ingestion path transforms raw benchmark test output into indexed, queryable time-series traces:

```
[Swarming Bot Fleets]
         │ (Executes benchmark runner, writes results JSON)
         ▼
[Google Cloud Storage (GCS)]
  gs://<bucket>/ingest/...
         │ (GCS Object Finalize Notification)
         ▼
[Google Cloud Pub/Sub Topic]
  perf-ingestion-<instance>-spanner
         │ (Pulls batch notifications)
         ▼
[Perf Ingestion Pods (GKE)]
  perf/go/ingest/process/process.go
  ├── worker.processSingleFile()
  ├── Parser.Parse() (Extracts commit, params, metrics)
  ├── Git Provider (Resolves Git hash -> integer CommitNumber)
  └── TraceStore.WriteTraces()
         │
         ├──► [Cloud Spanner Database] (Traces, Commits, Postings tables)
         │
         ▼ (Publishes IngestEvent)
[Pub/Sub: FileIngestionTopicName]
  perf-cluster-<instance>
         │
         ▼
[Perf Backend Pods (GKE)]
  perf/go/regression/continuous/
  ├── Sliding window step detection & K-Means clustering
  └── AnomalyStore.Persist() -> Spanner (Anomalies, AnomalyGroups)
         │
         ▼
[Perf Frontend Pods (GKE)]
  perf/go/frontend/ -> LitElement Web Components (UI)
```

### 1.1 Test Execution & File Storage
- Swarming bots in device labs run Telemetry benchmarks, generating JSON result files adhering to the histogram or legacy format ([`perf/go/ingest/format/format.go`](../../go/ingest/format/format.go)).
- Test runners upload artifacts to GCS buckets specified in instance configurations (e.g., `gs://v8-perf-prod/ingest`, `gs://chrome-perf-non-public/ingest`).

### 1.2 Pub/Sub Event Ingestion
- GCS triggers an event on a configured Google Cloud Pub/Sub topic upon upload.
- Ingestion workers subscribe to the topic via `IngestionConfig.Subscription` ([`perf/go/config/config.go:406`](../../go/config/config.go#L406)).
- Worker routines in [`perf/go/ingest/process/process.go:118-281`](../../go/ingest/process/process.go#L118-L281) download and process files:
  1. `w.p.Parse()` unpacks the file, validating headers, metric values, and metadata ([`process.go:142`](../../go/ingest/process/process.go#L142)).
  2. `w.g.GetCommitNumber()` converts the Git hash (or repo-supplied build id) into a monotonically increasing integer `types.CommitNumber` ([`process.go:191`](../../go/ingest/process/process.go#L191)). If the commit is not yet cached locally, `w.g.Update()` pulls the upstream repository.
  3. `w.store.WriteTraces()` writes trace tiles and posting entries to Cloud Spanner with retry logic up to `writeRetries = 10` ([`process.go:235`](../../go/ingest/process/process.go#L235)).
  4. If successful, `f.PubSubMsg.Ack()` commits the message; if a transient database failure occurs, dead-letter collection or nack retries handle redelivery ([`process.go:256-260`](../../go/ingest/process/process.go#L256-L260)).

### 1.3 Downstream Triggering via `FileIngestionTopicName`
- Once data is committed to Spanner, `sendPubSubEvent` publishes an `ingestevents.IngestEvent` ([`process.go:40-60`](../../go/ingest/process/process.go#L40-L60), [`267`](../../go/ingest/process/process.go#L267)) containing the ingested trace IDs and parameter set to `FileIngestionTopicName`.
- Backend workers running continuous regression detectors subscribe to this event to immediately run step detection on the newly modified traces.

---

## 2. GCP Infrastructure & Kubernetes Hosting Architecture

The system is deployed on Google Kubernetes Engine (GKE) under Google-managed projects (`skia-public`, `skia-infra-corp`):

### 2.1 Component Microservices
- **Frontend Pods (`perf-frontend`)**:
  - Expose HTTP/HTTPS interfaces for user interaction, plotting, and anomaly triaging.
  - Implements API endpoints in [`perf/go/frontend/api/`](../../go/frontend/api/) for graph generation, trace slicing, and Pinpoint bisection creation.
  - Uses `tracestore.TraceStore` to read tiles from Spanner on demand.
- **Backend Pods (`perf-backend`)**:
  - Run continuous anomaly detection loops ([`perf/go/regression/continuous/`](../../go/regression/continuous/)).
  - Host gRPC services: `AnomalyGroupService`, `CulpritService`, `AutobisectionService`.
  - Listen on `FileIngestionTopicName` to trigger continuous detection without polling.
- **Ingestion Pods (`perf-ingestion`)**:
  - Dedicated GKE worker pods that continuously consume files from Pub/Sub and stream data to Spanner. Decoupled from user traffic to isolate spikes in bot benchmark uploads.
- **Temporal Orchestration Cluster**:
  - Manages durable workflow execution for Pinpoint and Perf auto-bisections (`BisectWorkflow`, `MaybeTriggerBisectionWorkflow`, `CBB`).
- **Cloud Spanner**:
  - Backing relational datastore for all high-volume telemetry.
  - Tables: `Traces`, `Postings`, `Commits`, `Alerts`, `Anomalies`, `AnomalyGroups`, `Culprits`, `Autobisections`.
- **Cloud Storage (GCS)**:
  - Raw benchmark JSON archive and CAS inputs/outputs for test artifacts.
- **Observability**:
  - OpenCensus distributed tracing integrated with Cloud Trace via [`perf/go/tracing/tracing.go`](../../go/tracing/tracing.go).
  - Prometheus metrics instrumentation (`metrics2`) reporting ingestion latency, write error rates, and cluster sizes.

---

## 3. Instance Priority Matrix & Multi-Tenant Isolation

Rather than running a monolithic multi-tenant database, Perf enforces tenant isolation by separating instances into discrete Spanner databases, Pub/Sub topics, and GKE deployments:

| Instance Tier | Target Environments | Hosting Project | Network / Auth Boundary | Key Configurations |
| :--- | :--- | :--- | :--- | :--- |
| **Tier 1 (Internal Production)** | `chrome-internal`<br>`v8-internal`<br>`fuchsia-internal` | `skia-infra-corp` | Google Corp Network (IAP), strict Googler OAuth, internal Buganizer integration | [`chrome-internal.json`](../../configs/spanner/chrome-internal.json)<br>[`v8-internal.json`](../../configs/spanner/v8-internal.json)<br>[`fuchsia-internal.json`](../../configs/spanner/fuchsia-internal.json) |
| **Tier 2 (Public Production)** | `chrome-public`<br>`v8-public`<br>`android` (AndroidX)<br>`fuchsia-public` | `skia-public` | Public Internet (`*.luci.app`, `*.skia.org`), public issue trackers | [`chrome-public.json`](../../configs/spanner/chrome-public.json)<br>[`v8-public.json`](../../configs/spanner/v8-public.json)<br>[`android.json`](../../configs/spanner/android.json) |
| **Tier 3 (Autopush / Staging)** | `chrome-internal-autopush`<br>`v8-internal-autopush`<br>`android2-autopush` | `skia-infra-corp` / `skia-public` | Continuous deployment target; automated integration tests and early schema migrations | [`chrome-internal-autopush.json`](../../configs/spanner/chrome-internal-autopush.json)<br>[`v8-internal-autopush.json`](../../configs/spanner/v8-internal-autopush.json) |

### 3.1 Isolation Guarantees
1. **Resource Decoupling**: Large ingestion workloads from Chrome Waterfall bots cannot exhaust Spanner read/write throughput for AndroidX or Fuchsia.
2. **Access Control**: Confidential benchmarks (e.g. unreleased hardware chips, pre-launch device targets) are restricted to corp instances backed by private GCS buckets and separate Spanner instances.
3. **Failover & Rollout Safety**: Autopush instances receive new binary deployments and schema migrations first, protecting production triage operations.

---

## 4. Configuration Sprawl & Schema Rationalization

Instance behaviors are declared in JSON configuration files located in [`perf/configs/spanner/`](../../configs/spanner/). These map to the Go struct [`config.InstanceConfig`](../../go/config/config.go).

### 4.1 Key Configuration Functional Blocks
- **Data Store (`DataStoreConfig`)**: Spanner database connection string, tile size (default 256 or 512 points), query timeout.
- **Ingestion (`IngestionConfig`)**:
  - GCS source bucket URLs and prefixes.
  - Pub/Sub subscription name and topic.
  - `FileIngestionTopicName` for downstream anomaly detector notifications.
  - `BranchConfig`: Git branches tracked for continuous indexing.
- **Git Repo (`GitRepoConfig`)**:
  - Upstream Git URL (e.g., `chromium.googlesource.com/chromium/src.git`).
  - Commit URL format and commit range templates.
- **Notification & Issue Tracker (`NotifyConfig`, `IssueTrackerConfig`)**:
  - IssueTracker API component ID, issue priority, default assignees/subscribers.
  - Bug template subject and body text with Go template interpolation.
  - Provider type (`android`, `chromeperf`, or default).
- **Sheriff & Alerts Routing**:
  - `NewAlertsPage`: Controls navigation between `/a/` (sheriff config) and `/r2/` (Skia alerts) ([`config.go:1165`](../../go/config/config.go#L1165)).
  - `ShowTriageLink`, `ShowBisectBtn`, `ShowPinpointLink`: Feature flags governing UI components per instance.

### 4.2 Maintenance and Convergence Strategy
The variety of optional flags in [`perf/configs/spanner/*.json`](../../configs/spanner/) reflects legacy compatibility constraints accumulated across migrations. As part of CDMX onsite objectives, configurations will converge toward:
1. Standardized Spanner schemas using unified migrations in [`perf/go/sql/expectedschema/migrations/`](../../go/sql/expectedschema/migrations/).
2. Uniform alerts and triage pages, deprecating legacy URL branches (`/a/` vs `/r2/`).
3. Single notification provider framework, standardizing commit metadata resolution across all partner instances.
