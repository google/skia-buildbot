# Skia Perf Architecture, Struct Inventory & End-to-End Interaction Flows

Comprehensive technical reference manual and architecture specification for Skia Perf (`go.skia.org/infra/perf`). Grounded strictly in codebase source files with verified symbol locations, data contracts, and call stacks.

---

## 1. System Topology & Subsystem Architecture

Skia Perf is a multi-tier distributed telemetry ingestion, storage, anomaly detection, visualization, and bisection platform.

```
                              +---------------------------------------+
                              |   Web Browser / Frontend UI Clients   |
                              |  (59 Lit-based Custom Web Components)  |
                              +-------------------+-------------------+
                                                  | HTTP / REST / SSE / WASM
                                                  v
+---------------------------------------------------------------------------------------------------+
|                                  perfserver frontend Process                                       |
|                                                                                                   |
|  +---------------------------+  +-------------------------------+  +---------------------------+  |
|  |     Frontend Router       |  |     14 API Controllers        |  |   Background Daemons      |  |
|  |   (chi Mux, Security,     |  | (GraphApi, TriageApi,         |  | - ParamSetRefresher (1h)  |  |
|  |    Static Dist Handler)   |  |  AnomaliesApi, WasmApi, etc.) |  | - Continuous Clustering   |  |
|  +-------------+-------------+  +---------------+---------------+  +-------------+-------------+  |
|                |                                |                                |                |
|                +--------------------------------+--------------------------------+                |
|                                                 |                                                 |
|                                                 v                                                 |
|               +------------------------------------------------------------------+                |
|               | DataFrameBuilder Engine (Query Chunking, Caching, Filtering)     |                |
|               +--------------------+---------------------------------------------+                |
+------------------------------------|--------------------------------------------------------------+
                                     |
           +-------------------------+-------------------------+
           |                                                   |
           v                                                   v
+----------------------+                            +----------------------+
|  Google Cloud Spanner|                            |   Redis Cache        |
|  (or CockroachDB)    |                            |   (Optional Cluster) |
|                      |                            |                      |
| - Postings           |                            | - ParamSets Tile     |
| - TraceValues2       |                            |   Cache              |
| - TraceParams        |                            | - Trace Data Cache   |
| - Regressions2       |                            +----------------------+
| - AnomalyGroups      |
| - Shortcuts / Favs   |
+----------^-----------+
           |
           | WriteTraces (Batched)
+----------+----------------------------------------------------------------------------------------+
|                                    perfserver ingest Process                                      |
|                                                                                                   |
|  +--------------------+    +--------------------+    +--------------------+    +---------------+  |
|  |  Pub/Sub Listener  | -> |   GCS File Fetch   | -> |  Format Parser     | -> | VCS Commit    |  |
|  |  (File Arrival)    |    |   (file.File)      |    |  (Nano/Legacy/Fuchs)|    | Resolver (Git)|  |
|  +--------------------+    +--------------------+    +--------------------+    +---------------+  |
+---------------------------------------------------------------------------------------------------+
           |
           | Pub/Sub Ingestion Notification
           v
+---------------------------------------------------------------------------------------------------+
|                                    perfserver cluster Process                                     |
|                                                                                                   |
|  +--------------------+    +--------------------+    +--------------------+    +---------------+  |
|  | Alert Config Loop  | -> | DataFrame Window   | -> | Step Detection     | -> | Regr Refiner  |  |
|  | (Periodic / Event) |    | Query (dfBuilder)  |    | (StepFit / K-Means)|    | & Notification|  |
|  +--------------------+    +--------------------+    +--------------------+    +-------+-------+  |
+----------------------------------------------------------------------------------------|----------+
                                                                                         |
                                                                    RegressionFound Event|
                                                                                         v
+---------------------------------------------------------------------------------------------------+
|                                 perfserver maintenance Process                                    |
|                                                                                                   |
|  - Expected Schema Verifier & Migrator (`ValidateAndMigrateNewSchema`)                            |
|  - Trace Visibility Checker & Promoter (`checker.Check`, `promoter.Promote`)                      |
|  - Gitiles/LUCI Sheriff Config Continuous Importer (`sheriffconfig.StartImportRoutine` - 10m)      |
|  - Redis Cache Periodic Tile Refresher (`cacheParamSetRefresher.StartRefreshRoutine` - 4h)        |
|  - Expired Shortcuts & Regressions Deleter (`deletion.RunPeriodicDeletion` - 15m)                 |
+---------------------------------------------------------------------------------------------------+
                                                                                         |
                                                                                         v
+---------------------------------------------------------------------------------------------------+
|                                Temporal Distributed Orchestration                                 |
|                                                                                                   |
|  +---------------------------------------------------+  +--------------------------------------+  |
|  | MaybeTriggerBisectionWorkflow                     |  | ProcessCulpritWorkflow               |  |
|  | (TaskQueue: perf.grouping)                        |  | (TaskQueue: perf.grouping)           |  |
|  | - 30m Clustering Window Delay                     |  | - Converts Pinpoint Commits          |  |
|  | - Candidate Selection                             |  | - Persists Culprit in DB             |  |
|  | - Dispatches Bisect to Pinpoint                   |  | - Updates IssueTracker & Comments    |  |
|  | - Polls Pinpoint Job Completion (every 30m, 10h)  |  +--------------------------------------+  |
|  +---------------------------------------------------+                                             |
+---------------------------------------------------------------------------------------------------+
```

---

## 2. Comprehensive Go Struct & Interface Inventory

The table below catalogs the primary Go structs, interfaces, and abstractions across `perf/go/...`.

| Package | Symbol Name | Type | Source File | Core Responsibility |
| :--- | :--- | :--- | :--- | :--- |
| `alerts` | [`Alert`](../../go/alerts/alert.go) | `struct` | `perf/go/alerts/alert.go` | Alert definition model containing query criteria, step detection rules, action triggers, and notification routing. |
| `alerts` | [`Store`](../../go/alerts/store.go) | `interface` | `perf/go/alerts/store.go` | Persistence contract for Alert configurations (`SaveAlert`, `DeleteAlert`, `ListAlerts`). |
| `alerts` | [`ConfigProvider`](../../go/alerts/configprovider.go) | `interface` | `perf/go/alerts/configprovider.go` | Cached, thread-safe in-memory provider of active alert definitions with periodic TTL refreshes. |
| `anomalies` | [`Store`](../../go/anomalies/anomalies.go) | `interface` | `perf/go/anomalies/anomalies.go` | Abstraction for retrieving anomalies across commits, revisions, and test paths. |
| `anomalies/sql` | [`SqlAnomaliesStore`](../../go/anomalies/sql/sql.go) | `struct` | `perf/go/anomalies/sql/sql.go` | Implementation of `anomalies.Store` reading directly from Spanner `Regressions2` table. |
| `anomalies/chromeperf` | [`ChromePerfAnomaliesStore`](../../go/anomalies/chromeperf/chromeperf.go) | `struct` | `perf/go/anomalies/chromeperf/chromeperf.go` | Implementation of `anomalies.Store` forwarding queries to legacy ChromePerf REST API. |
| `anomalygroup` | [`Store`](../../go/anomalygroup/store.go) | `interface` | `perf/go/anomalygroup/store.go` | CRUD interface for grouping co-occurring anomalies under a unified bisection candidate. |
| `anomalygroup/utils` | [`AnomalyGrouper`](../../go/anomalygroup/utils/anomalygrouputils.go) | `interface` | `perf/go/anomalygroup/utils/anomalygrouputils.go` | Coordinates anomaly matching against existing groups and invokes Temporal bisection workflows. |
| `backend/client` | [`PinpointClient`](../../go/backend/client/client.go) | `struct` | `perf/go/backend/client/client.go` | gRPC client wrapper connecting Perf to Pinpoint bisection and pairwise try-job services. |
| `clustering2` | [`ClusterSummary`](../../go/clustering2/clustering.go) | `struct` | `perf/go/clustering2/clustering.go` | Encapsulates trace cluster statistics, centroid vector, keys list, and mathematical step fit. |
| `config` | [`InstanceConfig`](../../go/config/config.go) | `struct` | `perf/go/config/config.go` | Primary runtime configuration model parsed from instance JSON (data store, Git, ingestion, alerts, notify). |
| `culprit` | [`Store`](../../go/culprit/store.go) | `interface` | `perf/go/culprit/store.go` | Persistence contract for storing culprit git revisions identified by Pinpoint bisections. |
| `dataframe` | [`DataFrame`](../../go/dataframe/dataframe.go) | `struct` | `perf/go/dataframe/dataframe.go` | Core tabular matrix representation: `Header` (commits/timestamps), `ParamSet`, and `TraceSet` (float32 values). |
| `dataframe` | [`DataFrameBuilder`](../../go/dataframe/dataframe.go) | `interface` | `perf/go/dataframe/dataframe.go` | Primary query engine interface (`NewNFromQuery`, `NewFromQueryAndRange`, `NewFromKeysAndRange`). |
| `dfbuilder` | [`DataFrameBuilderFromTraceStore`](../../go/dfbuilder/dfbuilder.go) | `struct` | `perf/go/dfbuilder/dfbuilder.go` | Production implementation of `DataFrameBuilder` querying Spanner postings, trace chunks, and Redis cache. |
| `dryrun` | [`Requests`](../../go/dryrun/dryrun.go) | `struct` | `perf/go/dryrun/dryrun.go` | Asynchronous coordinator for evaluating hypothetical alert rules against historical telemetry. |
| `favorites` | [`Store`](../../go/favorites/favorites.go) | `interface` | `perf/go/favorites/favorites.go` | Storage contract for user-personalized dashboards and pinned graph queries. |
| `frontend` | [`Frontend`](../../go/frontend/frontend.go) | `struct` | `perf/go/frontend/frontend.go` | Main HTTP application server coordinating template rendering, routing, middleware, and sub-APIs. |
| `frontend/api` | [`FrontendApi`](../../go/frontend/api/api.go) | `interface` | `perf/go/frontend/api/api.go` | Common interface for modular HTTP controllers registering endpoints on `chi.Mux`. |
| `frontend/api` | [`TriageBackend`](../../go/frontend/api/triageBackend.go) | `interface` | `perf/go/frontend/api/triageBackend.go` | Interface abstracting bug filing, anomaly status updates, and bug associations between Spanner and ChromePerf. |
| `git` | [`Git`](../../go/git/git.go) | `interface` | `perf/go/git/git.go` | VCS abstraction translating between dense integer `CommitNumber`s, Git hashes, and timestamps. |
| `graphsshortcut` | [`Store`](../../go/graphsshortcut/graphsshortcut.go) | `interface` | `perf/go/graphsshortcut/graphsshortcut.go` | Storage contract for multi-graph dashboard layout shortcuts. |
| `ingest/process` | [`workerInfo`](../../go/ingest/process/process.go) | `struct` | `perf/go/ingest/process/process.go` | Ingestion pipeline worker parsing incoming telemetry files, resolving commit numbers, and writing to store. |
| `issuetracker` | [`IssueTracker`](../../go/issuetracker/issuetracker.go) | `interface` | `perf/go/issuetracker/issuetracker.go` | Client abstraction for Google IssueTracker (Buganizer) API (`FileBug`, `ListIssues`, `AddComment`). |
| `notify` | [`Notifier`](../../go/notify/notify.go) | `interface` | `perf/go/notify/notify.go` | Notification delivery contract (`RegressionFound`, `RegressionMissing`) across Email, IssueTracker, and AnomalyGroup. |
| `psrefresh` | [`ParamSetRefresher`](../../go/psrefresh/psrefresh.go) | `interface` | `perf/go/psrefresh/psrefresh.go` | Background daemon periodically building and caching unified `ParamSet`s from recent database tiles. |
| `progress` | [`Tracker`](../../go/progress/progress.go) | `interface` | `perf/go/progress/progress.go` | State tracker for long-running asynchronous HTTP operations polled via `/_/status/{id}`. |
| `regression` | [`Store`](../../go/regression/continuous.go) | `interface` | `perf/go/regression/continuous.go` | Storage contract for persisting and retrieving regression detection records in `Regressions2`. |
| `regression/continuous` | [`Continuous`](../../go/regression/continuous/continuous.go) | `struct` | `perf/go/regression/continuous/continuous.go` | Continuous background engine executing alert queries, clustering, step fitting, and notification dispatches. |
| `sheriffconfig` | [`SheriffConfig`](../../go/sheriffconfig/service/service.go) | `struct` | `perf/go/sheriffconfig/service/service.go` | Background syncer importing sheriff alert configurations from Gitiles/LUCI Config repositories. |
| `shortcut` | [`Store`](../../go/shortcut/shortcut.go) | `interface` | `perf/go/shortcut/shortcut.go` | Storage contract for persisting sets of trace keys associated with user plots. |
| `stepfit` | [`StepFit`](../../go/stepfit/stepfit.go) | `struct` | `perf/go/stepfit/stepfit.go` | Mathematical regression fit calculations (Least Squares Step Function, Turning Point, Regression Score). |
| `subscription` | [`Store`](../../go/subscription/store.go) | `interface` | `perf/go/subscription/store.go` | Persistence contract for sheriff subscriptions linking alert rules to components and notification channels. |
| `trace_visibility` | [`Store`](../../go/trace_visibility/store/store.go) | `interface` | `perf/go/trace_visibility/store/store.go` | Storage contract for trace access-control rules governing public vs internal visibility. |
| `tracecache` | [`TraceCache`](../../go/tracecache/tracecache.go) | `struct` | `perf/go/tracecache/tracecache.go` | Multi-tier LRU and Redis cache for raw telemetry trace arrays. |
| `tracestore` | [`TraceStore`](../../go/tracestore/tracestore.go) | `interface` | `perf/go/tracestore/tracestore.go` | Primary low-level database contract (`WriteTraces`, `QueryTracesIDOnly`, `ReadTraces`). |
| `tracestore` | [`MetadataStore`](../../go/tracestore/metadata.go) | `interface` | `perf/go/tracestore/metadata.go` | Contract for storing and querying diagnostic links and swarming artifacts per trace. |
| `userissue` | [`Store`](../../go/userissue/store.go) | `interface` | `perf/go/userissue/store.go` | Contract for storing user-submitted issue annotations attached to specific trace keys and commits. |
| `workflows` | [`MaybeTriggerBisectionParam`](../../go/workflows/workflows.go) | `struct` | `perf/go/workflows/workflows.go` | Parameter envelope for Temporal bisection workflow execution. |
| `workflows/internal` | [`CulpritServiceActivity`](../../go/workflows/internal/culprit_service_activity.go) | `struct` | `perf/go/workflows/internal/culprit_service_activity.go` | Temporal activity struct executing culprit persistence and Buganizer issue updates. |

---

## 3. Comprehensive TypeScript Component & Controller Inventory

The table below catalogs all **59** Lit-based custom elements (`-sk`) and key controller modules in `perf/modules/...`.

| Directory | Custom Element Tag / Class | Primary Role & User Interaction |
| :--- | :--- | :--- |
| `alert-config-sk` | `<alert-config-sk>` | Interactive editor for alert rules, query filters, step detection algorithms, and dry-run triggers. |
| `alerts-page-sk` | `<alerts-page-sk>` | Legacy administrative alerts catalog page listing, creating, and deleting alert configurations. |
| `algo-select-sk` | `<algo-select-sk>` | Dropdown component for selecting anomaly detection algorithms (StepFit, K-Means). |
| `anomalies-table-sk` | `<anomalies-table-sk>` | Tabular display of detected anomalies supporting multi-selection and bulk triage operations. |
| `anomaly-playground-sk` | `<anomaly-playground-sk>` | Interactive experimentation workspace for tuning step detection parameters against live trace data. |
| `bisect-dialog-sk` | `<bisect-dialog-sk>` | Modal dialog for configuring and launching Pinpoint bisections (benchmark, story, commits, patch). |
| `bug-tooltip-sk` | `<bug-tooltip-sk>` | Floating tooltip displaying Buganizer issue details, assignee, status, and summary upon hover. |
| `calendar-input-sk` | `<calendar-input-sk>` | Input field with attached date picker popup for selecting query time boundaries. |
| `calendar-sk` | `<calendar-sk>` | Calendar grid view for browsing historical telemetry dates and selecting commit ranges. |
| `chart-tooltip-sk` | `<chart-tooltip-sk>` | Interactive plot tooltip rendering commit details, test parameters, anomaly markers, and action links. |
| `cluster-lastn-page-sk` | `<cluster-lastn-page-sk>` | Cluster view rendering regression summaries across the last N commits. |
| `cluster-page-sk` | `<cluster-page-sk>` | Main clustering explorer page rendering step detection clusters and triage controls. |
| `cluster-summary2-sk` | `<cluster-summary2-sk>` | Detailed visual card of a single regression cluster with sparkline, centroid plot, and trace count. |
| `commit-detail-panel-sk` | `<commit-detail-panel-sk>`| Side panel displaying VCS commit metadata, author, commit message, and diff links. |
| `commit-detail-picker-sk` | `<commit-detail-picker-sk>`| Search and selection modal for choosing exact repository revisions. |
| `commit-detail-sk` | `<commit-detail-sk>` | Inline display for commit hashes with quick navigation links to Gitiles / Gerrit. |
| `commit-range-sk` | `<commit-range-sk>` | Visual commit range selector with dual inputs for beginning and ending revisions. |
| `day-range-sk` | `<day-range-sk>` | Range slider and date picker for querying telemetry over sliding day windows. |
| `domain-picker-sk` | `<domain-picker-sk>` | Toggle selector switching graph axes between Commit Number, Timestamp, and Date. |
| `existing-bug-dialog-sk` | `<existing-bug-dialog-sk>` | Modal allowing users to associate selected anomalies with an already open Buganizer issue. |
| `explore-multi-sk` | `<explore-multi-sk>` | Multi-graph synchronized explorer rendering multiple trace plots side by side. |
| `explore-multi-v2-sk` | `<explore-multi-v2-sk>` | Accelerated multi-graph UI utilizing WASM and Web Workers for high-density rendering. |
| `explore-simple-sk` | `<explore-simple-sk>` | Core plot explorer element coordinating query dialogs, plot rendering, zooming, and triage. |
| `explore-sk` | `<explore-sk>` | Top-level container page hosting navigation and the active explore component. |
| `extra-links-sk` | `<extra-links-sk>` | Configurable navigation links panel displaying external dashboards and doc links. |
| `favorites-dialog-sk` | `<favorites-dialog-sk>` | Modal dialog for saving, editing, and categorizing favorite dashboard links. |
| `favorites-sk` | `<favorites-sk>` | Personalized favorites manager displaying user-saved and system-wide dashboard views. |
| `gemini-side-panel-sk` | `<gemini-side-panel-sk>` | AI-assisted side panel for conversational queries and natural-language trace investigation. |
| `graph-list-sk` | `<graph-list-sk>` | Ordered list of active plot cards in multi-graph view with re-ordering and removal controls. |
| `graph-title-sk` | `<graph-title-sk>` | Editable header element for individual plots supporting custom titles and shortcut links. |
| `json-source-sk` | `<json-source-sk>` | Diagnostic modal viewing the raw JSON telemetry payload stored in GCS for a specific datapoint. |
| `keyboard-shortcuts-help-sk`| `<keyboard-shortcuts-help-sk>`| Modal overlay displaying keyboard navigation and shortcut documentation. |
| `new-bug-dialog-sk` | `<new-bug-dialog-sk>` | Interactive form filing new Buganizer issues with automatic component, title, and body prefill. |
| `perf-scaffold-sk` | `<perf-scaffold-sk>` | Main application shell providing the header, navigation drawer, theme toggle, and login status. |
| `picker-field-sk` | `<picker-field-sk>` | Autocomplete search field used within query builders and commit selectors. |
| `pinpoint-dialog-sk` | `<pinpoint-dialog-sk>` | Legacy Pinpoint bisection initiation modal. |
| `pinpoint-try-job-dialog-sk`| `<pinpoint-try-job-dialog-sk>`| Dialog for initiating pairwise try-jobs with custom patches against baseline commits. |
| `pivot-query-sk` | `<pivot-query-sk>` | Multi-dimensional pivot table query builder selecting group-by keys and aggregation metrics. |
| `pivot-table-sk` | `<pivot-table-sk>` | High-density data grid rendering aggregated pivot results across trace dimensions. |
| `plot-google-chart-sk` | `<plot-google-chart-sk>` | Chart renderer using Google Charts for summary and cluster representations. |
| `plot-summary-sk` | `<plot-summary-sk>` | Overview mini-map plot showing full-timeframe context below zoomed charts. |
| `point-links-sk` | `<point-links-sk>` | Contextual links renderer displaying swarming task logs, isolates, and build artifacts for a point. |
| `query-chooser-sk` | `<query-chooser-sk>` | Faceted query builder displaying keys, counts, and parameter values. |
| `query-count-sk` | `<query-count-sk>` | Real-time counter badge showing the number of traces matching current query parameters. |
| `regressions-page-sk` | `<regressions-page-sk>` | Sheriff alert triage page displaying detected regressions grouped by subscription and alert rule. |
| `report-page-sk` | `<report-page-sk>` | Multi-anomaly inspection report page rendering synchronized graphs for a group or bug. |
| `revision-info-sk` | `<revision-info-sk>` | Revision comparison element displaying commit ranges and anomaly clusters across a revision window. |
| `sheriff-configs-dry-run-sk`| `<sheriff-configs-dry-run-sk>`| Validation UI testing proposed Sheriff Config changes against historical data. |
| `split-chart-menu-sk` | `<split-chart-menu-sk>` | Context menu enabling splitting of multi-trace plots by parameter dimensions. |
| `subscription-table-sk` | `<subscription-table-sk>` | Management table for sheriff subscriptions, auto-triage settings, and notification channels. |
| `test-picker-sk` | `<test-picker-sk>` | Hierarchical test suite and metric selector for ChromePerf benchmark hierarchies. |
| `triage-menu-sk` | `<triage-menu-sk>` | Action menu offering triage operations (Ignore, Reset, Nudge, File Bug, Associate Bug). |
| `triage-page-sk` | `<triage-page-sk>` | Legacy triage dashboard organizing regressions across commits. |
| `triage-panel-sk` | `<triage-panel-sk>` | Embedded panel providing quick triage buttons inside graph tooltip and cluster views. |
| `triage-status-sk` | `<triage-status-sk>` | Badge icon rendering the current triage state (Untriaged, Positive, Negative, Ignored). |
| `triage2-sk` | `<triage2-sk>` | Modern multi-anomaly triage controller coordinating bulk triage operations. |
| `tricon2-sk` | `<tricon2-sk>` | Multi-state icon button used for fast triage decisions in tables. |
| `user-issue-sk` | `<user-issue-sk>` | Interactive dialog for viewing, adding, and deleting user bug annotations on traces. |
| `word-cloud-sk` | `<word-cloud-sk>` | Visual word cloud rendering dominant parameter keys and values across clustered traces. |

---

## 4. 31 Spanner Instances Configuration Matrix

This matrix maps all 31 configuration files in `perf/configs/spanner/*.json` against key architecture toggles.

| Instance Name | SQL Anomalies (`fetch_anomalies_from_sql`) | ChromePerf Anomalies (`fetch_chrome_perf_anomalies`) | New Alerts Page (`new_alerts_page`) | Enable V2 UI (`enable_v2_ui`) | Show Bisect Button (`show_bisect_btn`) | Pinpoint Task Queue (`pinpoint_task_queue`) | Grouping Task Queue (`grouping_task_queue`) | Notification Transport (`notify_config`) | IssueTracker Secret Project |
| :--- | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :--- | :---: |
| `android` | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | `markdown_issuetracker` | ✅ |
| `android2-autopush` | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | `markdown_issuetracker` | ✅ |
| `angle` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | `none` | ❌ |
| `chrome-internal-autopush` | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ | ✅ | `anomalygroup` | ✅ |
| `chrome-internal-ng` | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ | ✅ | `anomalygroup` | ✅ |
| `chrome-internal-secondary` | ❌ | ✅ | ❌ | ❌ | ✅ | ❌ | ❌ | `none` | ✅ |
| `chrome-internal` | ❌ | ✅ | ✅ | ❌ | ✅ | ✅ | ✅ | `anomalygroup` | ✅ |
| `chrome-public-autopush` | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ | ✅ | `anomalygroup` | ✅ |
| `chrome-public-exp` | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | `none` | ✅ |
| `chrome-public` | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | `none` | ✅ |
| `crystalball` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | `none` | ❌ |
| `devtools-frontend` | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | `none` | ✅ |
| `emscripten` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | `none` | ❌ |
| `eskia-internal` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | `none` | ✅ |
| `flutter-engine` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | `html_email` | ❌ |
| `flutter-flutter` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | `html_email` | ❌ |
| `fuchsia-exp-internal` | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | `anomalygroup` | ✅ |
| `fuchsia-exp-public` | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | `html_email` | ✅ |
| `fuchsia-internal-autopush` | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | `html_email` | ✅ |
| `fuchsia-internal` | ❌ | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | `anomalygroup` | ✅ |
| `fuchsia-public` | ❌ | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | `html_email` | ✅ |
| `germanium-internal` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | `none` | ✅ |
| `germanium-public` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | `none` | ✅ |
| `skia-public` | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | `none` | ❌ |
| `v8-internal-autopush` | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | `none` | ✅ |
| `v8-internal` | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | `none` | ✅ |
| `v8-public` | ❌ | ❌ | ✅ | ❌ | ✅ | ❌ | ❌ | `html_email` | ❌ |
| `webrtc-public-ng` | ✅ | ✅ | ❌ | ❌ | ✅ | ❌ | ❌ | `html_email` | ✅ |
| `webrtc-public` | ❌ | ✅ | ❌ | ❌ | ✅ | ❌ | ❌ | `html_email` | ✅ |
| `widevine-cdm` | ❌ | ✅ | ❌ | ❌ | ✅ | ❌ | ❌ | `markdown_issuetracker` | ✅ |
| `widevine-whitebox` | ❌ | ✅ | ❌ | ❌ | ✅ | ❌ | ❌ | `none` | ✅ |

---

## 5. Exhaustive End-to-End Interaction Flows Catalog

Every interaction flow across the system is cataloged under its respective trigger category.

### Category 1: UI_INTERACTIVE

#### Action 1.1: Trace Query Execution & Plot Rendering
- **Level 1: Simplistic View**
```
[User Formulates Query in ExploreSimpleSk]
                    |
                    v
[POST /_/frame/start -> Progress Token Issued]
                    |
                    v (Background Goroutine)
[FrameRequestProcess.Run: DataFrameBuilder.NewNFromQuery]
                    |
                    v
[Spanner Postings & TraceValues2 Query -> DataFrame Joined]
                    |
                    v
[Attach Trace Metadata Links & Overlay Anomaly Map]
                    |
                    v
[Client Polls /_/status/{id} -> Renders Plot in ExploreSimpleSk]
```
- **Level 2: Detailed Call-Stack Flow**
  1. User selects parameters in `ExploreSimpleSk` ([`explore-simple-sk.ts`](../../modules/explore-simple-sk/explore-simple-sk.ts#L650)) and clicks "Plot".
  2. Frontend sends `POST /_/frame/start` with a `FrameRequest` JSON body.
  3. `graphApi.frameStartHandler` ([`perf/go/frontend/api/graphApi.go:128`](../../go/frontend/api/graphApi.go#L128)) validates queries and registers `fr.Progress` with `api.progressTracker`.
  4. Spawns background goroutine calling `frame.ProcessFrameRequest` ([`perf/go/ui/frame/frame.go:137`](../../go/ui/frame/frame.go#L137)).
  5. `frameRequestProcess.run` ([`perf/go/ui/frame/frame.go:214`](../../go/ui/frame/frame.go#L214)) iterates over queries, calling `p.doSearch` which invokes `dfBuilder.NewNFromQuery` ([`perf/go/dfbuilder/dfbuilder.go:375`](../../go/dfbuilder/dfbuilder.go#L375)).
  6. `dfBuilder` resolves commit ranges via `perfGit.CommitSliceFromCommitNumberRange`, queries Spanner `Postings` table to find matching trace IDs, and queries `TraceValues2` to fetch float32 values.
  7. If formulas exist, `p.doCalc` executes mathematical expressions using `calc.Context`.
  8. `dataframe.GetMetadataForTraces` ([`perf/go/ui/frame/frame.go:179`](../../go/ui/frame/frame.go#L179)) queries `metadataStore` to attach diagnostic links.
  9. `addRevisionBasedAnomaliesToResponse` or `addTimeBasedAnomaliesToResponse` queries `anomalyStore.GetAnomaliesForTraces` to attach anomaly markers.
  10. `ret.request.Progress.Results(resp)` stores completed response.
  11. Frontend polls `GET /_/status/{id}` ([`perf/go/progress/progress.go`](../../go/progress/progress.go)) and retrieves the completed `FrameResponse`.
- **Level 3: Instance Support Matrix**
  Supported by **all 31 instances**.

---

#### Action 1.2: Anomaly Triage & Bug Creation (Dual-Backend Architecture)
- **Level 1: Simplistic View**
```
[User Clicks "File Bug" on Anomaly in NewBugDialogSk]
                         |
                         v
              [POST /_/triage/file_bug]
                         |
           +-------------+-------------+
           | preferLegacy(r) ?         |
        YES|                           |NO
           v                           v
[ChromeperfTriageBackend]     [SqlTriageBackend]
           |                           |
           v (POST file_bug_skia)      v (issuetracker.FileBug)
   [ChromePerf API]             [Google IssueTracker]
                                       |
                                       v
                                [Update DB: RegStore.SetBugID]
```
- **Level 2: Detailed Call-Stack Flow**
  1. User fills bug details in `NewBugDialogSk` ([`new-bug-dialog-sk.ts`](../../modules/new-bug-dialog-sk/new-bug-dialog-sk.ts#L180)) and clicks submit.
  2. Dispatches `POST /_/triage/file_bug` with `FileBugRequest` payload.
  3. `triageApi.FileNewBug` ([`perf/go/frontend/api/triageApi.go:126`](../../go/frontend/api/triageApi.go#L126)) enforces `roles.Editor`.
  4. Calls `api.getTriageBackend(r)` ([`perf/go/frontend/api/triageApi.go:36`](../../go/frontend/api/triageApi.go#L36)).
  5. Routing logic invokes `preferLegacy(r)` ([`perf/go/frontend/api/common.go:27`](../../go/frontend/api/common.go#L27)):
     - Checks if `config.Config.SwitchBetweenAnomalySources && FetchAnomaliesFromSql && FetchChromePerfAnomalies`.
     - Inspects cookie `r.Cookie("fetch_anomalies_from_sql")`.
     - Defaults to `!config.Config.FetchAnomaliesFromSql`.
  6. **If Legacy Backend**:
     - `ChromeperfTriageBackend.FileBug` ([`perf/go/frontend/api/chromeperfTriageBackend.go:49`](../../go/frontend/api/chromeperfTriageBackend.go#L49)) converts string keys to integer slice.
     - Calls `b.chromeperfClient.SendPostRequest(ctx, "file_bug_skia", ...)` via HTTP to ChromePerf endpoint.
  7. **If SQL Backend**:
     - `triageBackend.FileBug` ([`perf/go/frontend/api/triageBackend.go:32`](../../go/frontend/api/triageBackend.go#L32)) calls `t.issueTracker.FileBug(ctx, req)`.
     - On success, writes to Spanner: `t.regStore.SetBugID(ctx, req.Keys, bugId)` ([`perf/go/regression/sqlregressionstore/sqlregressionstore.go`](../../go/regression/sqlregressionstore/sqlregressionstore.go)).
     - Executes SQL: `UPDATE Regressions2 SET bug_id = @bug_id WHERE commit_number = @commit_number AND alert_id = @alert_id`.
- **Level 3: Instance Support Matrix**
  - **SQL Backend**: Active on 9 instances (`chrome-internal-autopush`, `chrome-internal-ng`, `chrome-public-autopush`, `fuchsia-exp-internal`, `fuchsia-exp-public`, `fuchsia-internal-autopush`, `v8-internal-autopush`, `v8-internal`, `webrtc-public-ng`).
  - **ChromePerf Backend**: Active on 17 instances where `fetch_chrome_perf_anomalies: true`.
  - **Disabled**: Instances without IssueTracker configured (`angle`, `crystalball`, `emscripten`, `flutter-*`, `skia-public`).

---

#### Action 1.3: Anomaly Triage State Mutations (Ignore, Reset, Nudge)
- **Level 1: Simplistic View**
```
[User Selects Triage Action (Ignore / Reset / Nudge) in TriageMenuSk]
                                |
                                v
                   [POST /_/triage/edit_anomalies]
                                |
             +------------------+------------------+
             | preferLegacy(r) ?                   |
          YES|                                     |NO
             v                                     v
  [ChromeperfTriageBackend]               [SqlTriageBackend]
  (POST edit_anomalies_skia)                       |
                                  +----------------+----------------+
                                  |                |                |
                                  v                v                v
                              [IGNORE]          [RESET]          [NUDGE]
                              t.regStore.      t.regStore.      t.regStore.
                              IgnoreAnomalies  ResetAnomalies   NudgeAndReset
```
- **Level 2: Detailed Call-Stack Flow**
  1. User triggers action in `TriageMenuSk` ([`triage-menu-sk.ts`](../../modules/triage-menu-sk/triage-menu-sk.ts#L110)).
  2. Sends `POST /_/triage/edit_anomalies` with `EditAnomaliesRequest` JSON.
  3. `triageApi.EditAnomalies` ([`perf/go/frontend/api/triageApi.go:174`](../../go/frontend/api/triageApi.go#L174)) verifies `roles.Editor`.
  4. Routes to `TriageBackend.EditAnomalies` ([`perf/go/frontend/api/triageBackend.go:51`](../../go/frontend/api/triageBackend.go#L51)):
     - **IGNORE**: calls `t.regStore.IgnoreAnomalies(ctx, req.Keys)`. Sets `triage_status = 'ignored'`.
     - **RESET**: calls `t.regStore.ResetAnomalies(ctx, req.Keys)`. Clears bug association and resets status to `untriaged`.
     - **NUDGE**: calls `t.regStore.NudgeAndResetAnomalies(ctx, req.Keys, displayCommit)`. Updates the commit offset associated with the anomaly and resets triage state.
- **Level 3: Instance Support Matrix**
  Supported on all instances with an active triage backend (24 instances).

---

#### Action 1.4: Interactive Pinpoint Bisection & Try-Job Dispatch
- **Level 1: Simplistic View**
```
[User Configures Bisect in BisectDialogSk]    [User Submits Try Job in PinpointTryJobDialogSk]
                     |                                              |
                     v                                              v
           [POST /_/bisect/create]                              [POST /_/try]
                     |                                              |
                     +----------------------+-----------------------+
                                            |
                         +------------------+------------------+
                         | Use New Pinpoint / Backend Routing? |
                      YES|                                     |NO
                         v                                     v
             [gRPC PinpointClient]                  [Legacy Pinpoint Client]
          ScheduleBisection / SchedulePairwise       CreateBisect / CreateTryJob
                         |                                     |
                         v                                     v
           [Temporal pinpoint_task_queue]            [Legacy ChromePerf API]
```
- **Level 2: Detailed Call-Stack Flow**
  1. User fills parameters in `BisectDialogSk` ([`bisect-dialog-sk.ts`](../../modules/bisect-dialog-sk/bisect-dialog-sk.ts#L220)) or `PinpointTryJobDialogSk` ([`pinpoint-try-job-dialog-sk.ts`](../../modules/pinpoint-try-job-dialog-sk/pinpoint-try-job-dialog-sk.ts#L150)).
  2. Invokes `POST /_/bisect/create` or `POST /_/try`.
  3. `pinpointApi.checkAuthorized` ([`perf/go/frontend/api/pinpointApi.go:70`](../../go/frontend/api/pinpointApi.go#L70)) validates `roles.Bisecter` using OAuth identity headers extracted by `getContextWithAuthHeaders`.
  4. For bisection: `createBisectHandler` ([`perf/go/frontend/api/pinpointApi.go:250`](../../go/frontend/api/pinpointApi.go#L250)):
     - If benchmark routes to backend (`backendClient.ShouldRouteToBackend`) or `PinpointTaskQueue` is set: constructs `pinpoint_pb.ScheduleBisectRequest` and calls `newClient.ScheduleBisection(ctx, newReq)`.
     - Else: calls legacy `api.pinpointClient.CreateBisect(ctx, &cbr, isNewAnomaly)`.
  5. For try jobs: `createTryJobHandler` ([`perf/go/frontend/api/pinpointApi.go:133`](../../go/frontend/api/pinpointApi.go#L133)):
     - If `cbr.UseNewPinpoint`: builds `pinpoint_pb.SchedulePairwiseRequest` and calls `newClient.SchedulePairwise(ctx, newReq)`.
     - Else: calls legacy `api.pinpointClient.CreateTryJob(ctx, &cbr)`.
  6. Returns job ID and Temporal UI URL (`/namespaces/{namespace}/workflows/{job_id}`).
- **Level 3: Instance Support Matrix**
  Supported on instances with `show_bisect_btn: true` (16 instances), with Temporal queue routing active on `chrome-internal`, `chrome-internal-autopush`, `chrome-internal-ng`, and `chrome-public-autopush`.

---

#### Action 1.5: Faceted Cascade Autocomplete & ParamSet Counts
- **Level 1: Simplistic View**
```
[User Focuses Query Parameter in QueryChooserSk]
                       |
                       +-----------------------+
                       |                       |
                       v                       v
               [POST /_/count]         [POST /_/nextParamList]
                       |                       |
                       +-----------+-----------+
                                   |
                                   v
             [queryApi.PreflightQuery -> ParamSetRefresher.GetParamSetForQuery]
                                   |
                                   v
       [Evaluates Subqueries / Postings Intersections -> Returns Count & ParamSet]
```
- **Level 2: Detailed Call-Stack Flow**
  1. `QueryChooserSk` ([`query-chooser-sk.ts`](../../modules/query-chooser-sk/query-chooser-sk.ts#L340)) intercepts user keystrokes in faceted dropdowns.
  2. Fires `POST /_/count` or `POST /_/nextParamList` (`queryApi.nextParamListHandler` [`perf/go/frontend/api/queryApi.go:101`](../../go/frontend/api/queryApi.go#L101)).
  3. `findNextParamInQueryString` evaluates `Config.QueryConfig.IncludedParams` to determine the next cascading dimension.
  4. Calls `api.PreflightQuery` ([`perf/go/frontend/api/queryApi.go:68`](../../go/frontend/api/queryApi.go#L68)) which invokes `paramsetRefresher.GetParamSetForQuery(ctx, q, u)`.
  5. Evaluates preflight postings intersections against Spanner/Redis cache.
  6. Returns filtered `ReadOnlyParamSet` and integer trace match count.
- **Level 3: Instance Support Matrix**
  Supported by **all 31 instances**.

---

#### Action 1.6: In-Browser Accelerated Rendering via WebAssembly
- **Level 1: Simplistic View**
```
[ExploreMultiV2Sk Loads Page]
             |
             +-----------------------+-----------------------+
             |                       |                       |
             v                       v                       v
  [GET /_/wasm/meta.json] [GET /_/wasm/params.json] [GET /_/wasm/traces.bin]
             |                       |                       |
             +-----------------------+-----------------------+
                                     |
                                     v
         [Web Worker WASM Execution (Direct IEEE 754 Float32 Decoding)]
```
- **Level 2: Detailed Call-Stack Flow**
  1. `ExploreMultiV2Sk` initializes and requests pre-computed tile binaries.
  2. Hits `wasmApi` endpoints registered in [`perf/go/frontend/api/wasmApi.go:89`](../../go/frontend/api/wasmApi.go#L89):
     - `GET /_/wasm/meta.json` (`metaHandler`): tile number and schema version.
     - `GET /_/wasm/params.json` (`paramsHandler`): dictionary-encoded parameter strings.
     - `GET /_/wasm/traces.bin` (`tracesHandler`): packed IEEE 754 binary trace array.
  3. Backend serves from in-memory `wasmCache` or disk cache (`/tmp/wasm_cache`).
  4. Browser transfers binary buffer directly to WebAssembly Web Worker, rendering thousands of series with zero JSON decoding overhead.
- **Level 3: Instance Support Matrix**
  Supported on all instances, primarily leveraged by `android` and `android2-autopush` where `enable_v2_ui: true`.

---

#### Action 1.7: User Issue Buganizer Annotations
- **Level 1: Simplistic View**
```
[User Right-Clicks Datapoint in ExploreSimpleSk -> "Annotate Issue"]
                                |
                                v
                  [POST /_/user_issue/create]
                                |
             +------------------+------------------+
             |                                     |
             v                                     v
  [IssueTracker.FileBug]                [UserIssueStore.Save]
  (Creates Buganizer Ticket)            (Saves Trace Key & Commit Mapping)
```
- **Level 2: Detailed Call-Stack Flow**
  1. User triggers issue annotation modal via `<user-issue-sk>`.
  2. Sends `POST /_/user_issue/create` to `userIssueApi.createUserIssueHandler` ([`perf/go/frontend/api/userIssueApi.go:120`](../../go/frontend/api/userIssueApi.go#L120)).
  3. Verifies `roles.Editor`.
  4. Calls `ui.issueTracker.FileBug(ctx, req)` creating Buganizer issue.
  5. Calls `ui.userIssueStore.Save(ctx, issue)` persisting mapping in Spanner `UserIssues` table.
- **Level 3: Instance Support Matrix**
  Supported on all instances configuring `IssueTrackerConfig` (24 instances).

---

#### Action 1.8: Saved Dashboard Shortcuts & Graph Links
- **Level 1: Simplistic View**
```
[User Clicks "Copy Link" or "Share Dashboard"]
                     |
                     v
          [POST /_/shortcut/update]
                     |
                     v
   [GraphsShortcutStore.Insert / Update]
                     |
                     v (Spanner GraphsShortcuts Table)
   [Returns Unique Shortcut ID (e.g. /m?sid=...)]
```
- **Level 2: Detailed Call-Stack Flow**
  1. Frontend captures state of all open charts.
  2. Sends `POST /_/shortcut/update` to `shortcutsApi.createGraphsShortcutHandler` ([`perf/go/frontend/api/shortcutsApi.go:34`](../../go/frontend/api/shortcutsApi.go#L34)).
  3. Deserializes graphs payload and invokes `graphsShortcutStore.Insert(ctx, graphsData)`.
  4. Writes serialized configuration to Spanner `GraphsShortcuts` table.
  5. Returns unique identifier for URL sharing.
- **Level 3: Instance Support Matrix**
  Supported by **all 31 instances**.

---

#### Action 1.9: Alert Rule Authoring & Notification Dry-Run
- **Level 1: Simplistic View**
```
[User Configures Alert Rule in AlertConfigSk]
                     |
                     v
           [POST /_/dryrun/start]
                     |
                     v (Background Goroutine)
     [DryrunRequests.Start: DataFrame Evaluation]
                     |
                     v
[Runs StepFit & Refiner -> Populates Progress Results]
                     |
                     v
[Client Polls /_/status/{id} -> Displays Detected Regressions]
```
- **Level 2: Detailed Call-Stack Flow**
  1. User edits query and thresholds in `AlertConfigSk` ([`alert-config-sk.ts`](../../modules/alert-config-sk/alert-config-sk.ts#L450)).
  2. Clicks "Dry Run", sending `POST /_/dryrun/start`.
  3. Handled by `alertsApi.dryrunRequests.StartHandler` ([`perf/go/frontend/api/alertsApi.go:55`](../../go/frontend/api/alertsApi.go#L55)).
  4. Calls `dryrun.Requests.Start` ([`perf/go/dryrun/dryrun.go:85`](../../go/dryrun/dryrun.go#L85)).
  5. Spawns background goroutine building DataFrame over the test window, running `StepFit` and regression refinement without committing to DB.
  6. Frontend polls `GET /_/status/{id}` and displays hypothetical anomalies on graph.
- **Level 3: Instance Support Matrix**
  Supported by **all 31 instances**.

---

#### Action 1.10: Personalized Favorites Management
- **Level 1: Simplistic View**
```
[User Stars Dashboard View in FavoritesSk]
                     |
                     v
          [POST /_/favorites/new]
                     |
                     v
           [favStore.Save Favorite]
                     |
                     v (Spanner Favorites Table)
    [Returns Updated User Favorites Sections]
```
- **Level 2: Detailed Call-Stack Flow**
  1. User clicks star icon in UI, invoking `FavoritesApi.newFavoriteHandler` ([`perf/go/frontend/api/favoritesApi.go:40`](../../go/frontend/api/favoritesApi.go#L40)).
  2. Verifies user identity via `loginProvider.LoggedInAs(r)`.
  3. Calls `f.favStore.Save(ctx, userEmail, favoriteItem)`.
  4. Writes to Spanner `Favorites` table with composite key `(user_id, id)`.
- **Level 3: Instance Support Matrix**
  Supported by **all 31 instances**.

---

#### Action 1.11: Anomaly Algorithm Playground Testing
- **Level 1: Simplistic View**
```
[User Pastes Raw Numbers in AnomalyPlaygroundSk]
                        |
                        v
     [POST /_/playground/anomaly/v1/detect]
                        |
                        v
           [stepfit.StepFit Algorithm]
                        |
                        v
     [Returns Detected Turning Points & Fit Graph]
```
- **Level 2: Detailed Call-Stack Flow**
  1. User enters custom time-series data in `AnomalyPlaygroundSk`.
  2. Submits to `POST /_/playground/anomaly/v1/detect` handled by `anomaly.Handler` ([`perf/go/frontend/frontend.go:1436`](../../go/frontend/frontend.go#L1436)).
  3. Executes `stepfit.StepFit` mathematical evaluation without accessing database.
  4. Returns turning points, step magnitude, and regression status.
- **Level 3: Instance Support Matrix**
  Supported by **all 31 instances**.

---

#### Action 1.12: Model Context Protocol (MCP) AI Query
- **Level 1: Simplistic View**
```
[LLM Agent Queries Perf Data via MCP]
                   |
                   v
           [GET /mcp/data]
                   |
                   v
     [mcpApi.getTraceDataHandler]
                   |
                   v
[dfBuilder.NewFromQueryAndRange -> JSON Response]
```
- **Level 2: Detailed Call-Stack Flow**
  1. External AI assistant issues `GET /mcp/data?query=...&begin=...&end=...&metadata=true`.
  2. Handled by `mcpApi.getTraceDataHandler` ([`perf/go/frontend/api/mcpApi.go:43`](../../go/frontend/api/mcpApi.go#L43)).
  3. Invokes `dfBuilder.NewFromQueryAndRange(ctx, q, beginTime, endTime)`.
  4. If `metadata=true`, enriches traces with diagnostic links via `metadataStore`.
  5. Returns structured JSON containing Header, ParamSet, and TraceSet.
- **Level 3: Instance Support Matrix**
  Supported by **all 31 instances**.

---

#### Action 1.13: Sheriff Config LUCI Validation Integration
- **Level 1: Simplistic View**
```
[LUCI Config Service Validates Config Change in Gerrit]
                           |
                           v
              [POST /_/configs/validate]
                           |
                           v
       [sheriffConfigApi.validateConfigHandler]
                           |
                           v
        [Parses & Validates Alert Definitions]
```
- **Level 2: Detailed Call-Stack Flow**
  1. LUCI Config service queries `GET /_/configs/metadata` ([`perf/go/frontend/api/sheriffConfigApi.go:52`](../../go/frontend/api/sheriffConfigApi.go#L52)) to match owned paths.
  2. On pending Gerrit CL, sends `POST /_/configs/validate`.
  3. `sheriffConfigApi.validateConfigHandler` verifies `roles.LuciConfig`.
  4. Validates configuration syntax against `alerts.Config` schema, returning validation errors.
- **Level 3: Instance Support Matrix**
  Active on instances with `enable_sheriff_config: true` (`chrome-internal`, `chrome-internal-autopush`, `chrome-internal-ng`).

---

### Category 2: INGESTION_STREAM

#### Action 2.1: Pub/Sub Notification & Ingestion Worker Dispatch
- **Level 1: Simplistic View**
```
[Swarming Bot Uploads JSON to GCS] -> [Cloud Pub/Sub Message]
                                             |
                                             v
                             [file.GCSSource.Start Polling]
                                             |
                                             v
                           [workerInfo.processSingleFile Dispatch]
```
- **Level 2: Detailed Call-Stack Flow**
  1. Test runner writes telemetry file to Google Cloud Storage.
  2. GCS Pub/Sub notification triggers `file.GCSSource.Start` ([`perf/go/file/gcs.go`](../../go/file/gcs.go)).
  3. Worker pool claims message, invoking `workerInfo.processSingleFile(f)` ([`perf/go/ingest/process/process.go:118`](../../go/ingest/process/process.go#L118)).
- **Level 3: Instance Support Matrix**
  Supported by all 31 instances during ingestion operations.

---

#### Action 2.2: Payload Parsing & Format Normalization
- **Level 1: Simplistic View**
```
[workerInfo.processSingleFile] -> [parser.Parser.Parse]
                                         |
            +----------------------------+----------------------------+
            |                            |                            |
            v                            v                            v
     [Nano-JSON v1]             [Legacy BenchData]             [FuchsiaPerf]
```
- **Level 2: Detailed Call-Stack Flow**
  1. In `processSingleFile`, calls `w.p.Parse(ctx, f)` ([`perf/go/ingest/process/process.go:142`](../../go/ingest/process/process.go#L142)).
  2. Determines format from config (`Format: "nano"`, `"legacy"`, `"fuchsia"`).
  3. Returns extracted `params` (`[]paramtools.Params`), `values` (`[]float32`), `gitHash`, and `fileLinks`.
- **Level 3: Instance Support Matrix**
  Supported by all 31 instances based on configured `ingestion_config.parser`.

---

#### Action 2.3: Commit Number Resolution & Race Condition Nack
- **Level 1: Simplistic View**
```
[Extract Git Hash / Commit Number] -> [perfGit.GetCommitNumber]
                                              |
                          +-------------------+-------------------+
                          | Commit Found in DB?                   |
                       YES|                                       |NO
                          v                                       v
                  [Proceed to Write]                  [Nack Message (Retry 10m)]
```
- **Level 2: Detailed Call-Stack Flow**
  1. In `processSingleFile`, checks `w.g.GetCommitNumber(ctx, gitHash, commitNumberFromFile)` ([`perf/go/ingest/process/process.go:191`](../../go/ingest/process/process.go#L191)).
  2. If missing, attempts on-demand Git repo sync: `w.g.Update(ctx)`.
  3. If still missing and `time.Since(f.PubSubMsg.PublishTime) < 10*time.Minute`:
     - Calls `f.PubSubMsg.Nack()` to allow redelivery once Git poller catches up.
     - Prevents dropped telemetry caused by out-of-order arrival between Swarming and Gitiles.
- **Level 3: Instance Support Matrix**
  Supported by all 31 instances.

---

#### Action 2.4: High-Throughput Batch Ingestion Transaction
- **Level 1: Simplistic View**
```
[workerInfo.processSingleFile]
              |
              v
[SQLTraceStore.WriteTraces (Retried up to 3x)]
              |
              +-------------------+-------------------+
              |                   |                   |
              v                   v                   v
      [Batch Insert]      [Batch Upsert]       [Batch Upsert]
       SourceFiles         TraceParams          TraceValues2
```
- **Level 2: Detailed Call-Stack Flow**
  1. In `processSingleFile`, invokes `w.store.WriteTraces` ([`perf/go/ingest/process/process.go:235`](../../go/ingest/process/process.go#L235)).
  2. `SQLTraceStore.WriteTraces` ([`perf/go/tracestore/sqltracestore/sqltracestore.go`](../../go/tracestore/sqltracestore/sqltracestore.go)):
     - Computes trace IDs using MD5 hash of normalized parameter key-value pairs.
     - Executes batched SQL statements inserting into `SourceFiles`, `TraceParams`, and writing values into `TraceValues2` (`trace_id`, `commit_number`, `val`).
  3. On success: calls `f.PubSubMsg.Ack()`.
- **Level 3: Instance Support Matrix**
  Supported by all 31 instances.

---

#### Action 2.5: Auxiliary Diagnostic Links Persistence
- **Level 1: Simplistic View**
```
[Ingested File Has fileLinks?] -> [metadataStore.InsertMetadata]
                                           |
                                           v
                             [Spanner Metadata Table]
```
- **Level 2: Detailed Call-Stack Flow**
  1. In `processSingleFile`, checks `if fileLinks != nil` ([`perf/go/ingest/process/process.go:273`](../../go/ingest/process/process.go#L273)).
  2. Calls `w.metadataStore.InsertMetadata(ctx, f.Name, fileLinks)`.
  3. Persists diagnostic Swarming/isolate URLs for tooltip drilldown.
- **Level 3: Instance Support Matrix**
  Supported on instances capturing links metadata.

---

#### Action 2.6: Pub/Sub Broadcast for Event-Driven Anomaly Detection
- **Level 1: Simplistic View**
```
[Successful Batch Write] -> [sendPubSubEvent]
                                  |
                                  v
           [Pub/Sub Topic: FileIngestionTopicName]
                                  |
                                  v
   [Wakes Event-Driven Clustering Loop in perfserver cluster]
```
- **Level 2: Detailed Call-Stack Flow**
  1. In `processSingleFile`, calls `sendPubSubEvent(ctx, w.pubSubClient, w.instanceConfig.IngestionConfig.FileIngestionTopicName, ...)` ([`perf/go/ingest/process/process.go:267`](../../go/ingest/process/process.go#L267)).
  2. Serializes `ingestevents.IngestEvent` with affected trace IDs and publishes to Cloud Pub/Sub.
  3. Downstream cluster daemons react immediately to newly arrived traces.
- **Level 3: Instance Support Matrix**
  Active on instances configuring `file_ingestion_topic_name`.

---

### Category 3: DAEMON_SCHEDULED

#### Action 3.1: Continuous Git / Gitiles Repository Poller
- **Level 1: Simplistic View**
```
[1-Minute Periodic Ticker] -> [perfGit.StartBackgroundPolling]
                                     |
                                     v
                       [git.Git.Update / Poll Gitiles]
                                     |
                                     v
                     [Insert New Commits into DB]
```
- **Level 2: Detailed Call-Stack Flow**
  1. Initialized in `frontend.go` ([`perf/go/frontend/frontend.go:100`](../../go/frontend/frontend.go#L100)) and `maintenance.go:106`.
  2. Runs background goroutine ticking every 60 seconds (`gitRepoUpdatePeriod`).
  3. Queries origin Gitiles repository, fetches new commits, resolves parent relationships, and assigns dense integer `CommitNumber`s.
- **Level 3: Instance Support Matrix**
  Active on **all 31 instances**.

---

#### Action 3.2: Continuous Regression Detection Loop
- **Level 1: Simplistic View**
```
[Clustering Goroutine Pool] -> [Continuous.Run]
                                     |
                 +-------------------+-------------------+
                 |                                       |
                 v (Event Driven)                        v (Continuous Polling)
      [RunEventDrivenClustering]               [RunContinuousClustering]
                 |                                       |
                 +-------------------+-------------------+
                                     |
                                     v
                      [Continuous.ProcessAlertConfig]
```
- **Level 2: Detailed Call-Stack Flow**
  1. Triggered if `flags.DoClustering` is true (`frontend.go:971`).
  2. Runs `c.Run(context.Background())` ([`perf/go/regression/continuous/continuous.go:153`](../../go/regression/continuous/continuous.go#L153)).
  3. Iterates over active alert definitions from `configProvider.GetAllAlertConfigs`.
  4. Calls `c.ProcessAlertConfig` to search for steps.
- **Level 3: Instance Support Matrix**
  Active on all clustering deployments.

---

#### Action 3.3: Step Detection & K-Means Clustering
- **Level 1: Simplistic View**
```
[Continuous.ProcessAlertConfig] -> [dfBuilder.NewNFromQuery]
                                            |
                                            v
                                  [StepFit / K-Means]
                                            |
                                            v
                            [RegressionRefiner.Process]
                                            |
                                            v
                                  [Save to Regressions2]
```
- **Level 2: Detailed Call-Stack Flow**
  1. In `ProcessAlertConfig`, loads trace data via `dfBuilder.NewNFromQuery`.
  2. Runs mathematical step detection: fits least-squares step function to trace centroids.
  3. If regression exceeds threshold: passes candidate to `regressionRefiner.Process`.
  4. Writes regression record to Spanner `Regressions2` table.
- **Level 3: Instance Support Matrix**
  Active on all instances performing regression detection.

---

#### Action 3.4: Automated Alert Notification Dispatcher & Temporal Handoff
- **Level 1: Simplistic View**
```
[Regression Detected] -> [notifier.RegressionFound]
                               |
            +------------------+------------------+
            |                                     |
            v                                     v
  [IssueTracker / Email]               [AnomalyGroupNotifier]
  (Direct Bug / Email Notification)               |
                                                  v
                                     [anomalygrouputils.ProcessRegression]
                                                  |
                                                  v
                                     [temporalClient.ExecuteWorkflow]
                                     (MaybeTriggerBisectionWorkflow)
```
- **Level 2: Detailed Call-Stack Flow**
  1. Detected regression invokes `notifier.RegressionFound` ([`perf/go/regression/continuous/continuous.go`](../../go/regression/continuous/continuous.go)).
  2. If `notifications: "anomalygroup"`, invokes `AnomalyGroupNotifier.RegressionFound` ([`perf/go/anomalygroup/notifier/anomalygroupnotifier.go:83`](../../go/anomalygroup/notifier/anomalygroupnotifier.go#L83)).
  3. Calls `n.grouper.ProcessRegressionInGroup` -> `anomalygrouputils.ProcessRegression` ([`perf/go/anomalygroup/utils/anomalygrouputils.go:60`](../../go/anomalygroup/utils/anomalygrouputils.go#L60)).
  4. Queries backend gRPC to find existing group; if none found, creates group.
  5. Initializes Temporal client (`tpr_client.DefaultTemporalProvider`) and calls:
     ```go
     temporalClient.ExecuteWorkflow(ctx, wo, workflows.MaybeTriggerBisection, &workflows.MaybeTriggerBisectionParam{...})
     ```
     on task queue `config.Config.TemporalConfig.GroupingTaskQueue` ([`anomalygrouputils.go:140`](../../go/anomalygroup/utils/anomalygrouputils.go#L140)).
- **Level 3: Instance Support Matrix**
  - **Temporal Handoff (`anomalygroup`)**: Active on `chrome-internal`, `chrome-internal-autopush`, `chrome-internal-ng`, `chrome-public-autopush`, `fuchsia-exp-internal`, and `fuchsia-internal`.
  - **Direct IssueTracker**: Active on `android`, `android2-autopush`, `widevine-cdm`.
  - **Email**: Active on `flutter-*`, `fuchsia-public`, `v8-public`, `webrtc-public`.

---

#### Action 3.5: Tile ParamSet Refresher Daemon
- **Level 1: Simplistic View**
```
[1-Hour Periodic Ticker] -> [ParamSetRefresher.Start]
                                  |
                                  v
                  [Query Recent Database Tiles]
                                  |
                                  v
                  [Update In-Memory & Redis ParamSet]
```
- **Level 2: Detailed Call-Stack Flow**
  1. Runs hourly loop in `ParamSetRefresher` ([`perf/go/psrefresh/psrefresh.go:821`](../../go/frontend/frontend.go#L821)).
  2. Queries recent tiles from `TraceStore` and constructs consolidated `ReadOnlyParamSet`.
  3. Updates atomic memory reference used by all query autocompletes.
- **Level 3: Instance Support Matrix**
  Active on **all 31 instances**.

---

#### Action 3.6: Trace Visibility Checker & Promoter Daemons
- **Level 1: Simplistic View**
```
[1-Hour Ticker: Visibility Checker]       [12-Hour Ticker: Visibility Promoter]
                |                                         |
                v                                         v
   [Query Gerrit / Git Commits]              [Promote Eligible Traces to Public]
                |                                         |
                v                                         v
[Update Trace Visibility Store]             [Write Public Access Rules to DB]
```
- **Level 2: Detailed Call-Stack Flow**
  1. Running in `perfserver maintenance` ([`perf/go/maintenance/maintenance.go:85`](../../go/maintenance/maintenance.go#L85)).
  2. `startVisibilityChecker`: runs every 1 hour, querying Gerrit to verify if commits and telemetry paths have been published.
  3. `startVisibilityPromoter`: runs every 12 hours, updating Spanner access-control tables to allow public instance read access.
- **Level 3: Instance Support Matrix**
  Active on instances with `visibility_config` enabled.

---

#### Action 3.7: Sheriff Config Gitiles / LUCI Sync Daemon
- **Level 1: Simplistic View**
```
[10-Minute Periodic Ticker] -> [SheriffConfig.StartImportRoutine]
                                      |
                                      v
                      [Fetch Configs from Gitiles / LUCI]
                                      |
                                      v
                   [Sync Subscriptions & Alerts in DB]
```
- **Level 2: Detailed Call-Stack Flow**
  1. In `maintenance.Start` ([`perf/go/maintenance/maintenance.go:143`](../../go/maintenance/maintenance.go#L143)).
  2. Ticks every 10 minutes (`configImportPeriod`).
  3. Fetches protobuf/yaml definitions from Gitiles repository (`MaintenanceConfig.GitilesRepoUrl`).
  4. Parses alert rules and syncs records into `AlertStore` and `SubscriptionStore`.
- **Level 3: Instance Support Matrix**
  Active on instances configuring `enable_sheriff_config: true`.

---

#### Action 3.8: Redis Cache Tile Refresher Daemon
- **Level 1: Simplistic View**
```
[4-Hour Periodic Ticker] -> [cacheParamSetRefresher.StartRefreshRoutine]
                                           |
                                           v
                             [Pre-Compute ParamSets for Tiles]
                                           |
                                           v
                             [Write Serialized Cache to Redis]
```
- **Level 2: Detailed Call-Stack Flow**
  1. In `maintenance.Start` ([`perf/go/maintenance/maintenance.go:177`](../../go/maintenance/maintenance.go#L177)).
  2. Ticks every 4 hours (`redisCacheRefreshPeriod`).
  3. Pre-computes ParamSets across historical tiles and warms Redis cache clusters.
- **Level 3: Instance Support Matrix**
  Active on maintenance deployments with `flags.RefreshQueryCache: true`.

---

#### Action 3.9: Expired Shortcuts & Regressions Deleter Daemon
- **Level 1: Simplistic View**
```
[15-Minute Periodic Ticker] -> [deleter.RunPeriodicDeletion]
                                           |
                                           v
                          [Purge Shortcuts Older than TTL]
                                           |
                                           v
                          [Delete Expired Regressions (1000/batch)]
```
- **Level 2: Detailed Call-Stack Flow**
  1. In `maintenance.Start` ([`perf/go/maintenance/maintenance.go:185`](../../go/maintenance/maintenance.go#L185)).
  2. Ticks every 15 minutes (`deletionPeriod`).
  3. Queries Spanner for shortcuts and regressions older than retention limits and deletes in batches of 1,000.
- **Level 3: Instance Support Matrix**
  Active on maintenance deployments with `flags.DeleteShortcutsAndRegressions: true`.

---

### Category 4: WORKFLOW_ORCHESTRATED

#### Action 4.1: Automated Bisection Orchestration (`MaybeTriggerBisectionWorkflow`)
- **Level 1: Simplistic View**
```
[Temporal Triggers MaybeTriggerBisectionWorkflow]
                        |
                        v
[Wait 30-Minute Clustering Window (waitForAnomalyClusteringWindow)]
                        |
                        v
[Load AnomalyGroup by ID -> Check GroupAction]
                        |
       +----------------+----------------+
       | GroupAction == BISECT           | GroupAction == REPORT
       v                                 v
[isBisectionAllowed?]           [processAnomaliesAsReporting]
       |                                 |
   YES | NO (Fallback)                   v
       v                               [File Buganizer Issue]
[processAnomaliesAsBisection]
       |
       v
[Dispatch Pinpoint Bisect & Poll Completion]
```
- **Level 2: Detailed Call-Stack Flow**
  1. Worker executes `MaybeTriggerBisectionWorkflow` ([`perf/go/workflows/internal/maybe_trigger_bisection.go:51`](../../go/workflows/internal/maybe_trigger_bisection.go#L51)).
  2. Calls `waitForAnomalyClusteringWindow` ([`line 59`](../../go/workflows/internal/maybe_trigger_bisection.go#L59)) pausing execution for 30 minutes to allow related regressions to aggregate into the group.
  3. Calls `loadAnomalyGroupByID` (`agsa.LoadAnomalyGroupByIDActivity`).
  4. Evaluates `GroupAction`:
     - If `GroupActionType_BISECT`:
       - Checks `isBisectionAllowed(ctx)` (verifies quotas and active runs).
       - Calls `processAnomaliesAsBisection` ([`line 96`](../../go/workflows/internal/maybe_trigger_bisection.go#L96)).
       - Finds candidate: `workflow.ExecuteActivity(ctx, agsa.FindBisectionCandidateActivity, ...)`.
       - Schedules bisection: dispatches activity to Pinpoint queue (`CreateBisectJobActivity`).
       - Polls status: `waitPinpointJobCompletion` every 30 minutes up to 10 hours.
       - If culprits identified: executes child workflow `ProcessCulpritWorkflow`!
       - If bisection fails or disallowed: falls back to `processAnomaliesAsReporting`.
     - If `GroupActionType_REPORT`:
       - Calls `processAnomaliesAsReporting`: files a Buganizer report via `agsa.ReportAnomaliesActivity`.
- **Level 3: Instance Support Matrix**
  Supported on instances configuring `TemporalConfig.GroupingTaskQueue` (Chrome and Fuchsia instances).

---

#### Action 4.2: Culprit Commit Processing & User Notification (`ProcessCulpritWorkflow`)
- **Level 1: Simplistic View**
```
[Pinpoint Identifies Culprit Commit]
                 |
                 v
   [Temporal: ProcessCulpritWorkflow]
                 |
                 v
   [convertPinpointCommits -> Parse Host/Repo/Revision]
                 |
                 v
   [csa.PeristCulprit -> Save in Spanner Culprits Table]
                 |
                 v
   [csa.NotifyUserOfCulprit -> Post Comments to IssueTracker]
```
- **Level 2: Detailed Call-Stack Flow**
  1. Triggered as child workflow from bisection or directly via Temporal client ([`perf/go/workflows/internal/process_culprit.go:17`](../../go/workflows/internal/process_culprit.go#L17)).
  2. Calls `convertPinpointCommits` ([`line 47`](../../go/workflows/internal/process_culprit.go#L47)) parsing `https://{host}/{project}.git` and commit revisions.
  3. Executes activity `csa.PeristCulprit` persisting culprit IDs into Spanner `Culprits` table.
  4. Executes activity `csa.NotifyUserOfCulprit` updating IssueTracker tickets with commit links, author notifications, and bisection graphs.
- **Level 3: Instance Support Matrix**
  Active in conjunction with `MaybeTriggerBisectionWorkflow`.

---

### Category 5: ADMIN_CLI

#### Action 5.1: Instance Config Validation & Pub/Sub Provisioning
- **Level 1: Simplistic View**
```
[Operator Runs: perf-tool config validate] -> [InstanceConfigFromFile Schema Check]
                                                      |
[Operator Runs: perf-tool config create-pubsub] ----> [Creates Cloud Pub/Sub Topics & Subscriptions]
```
- **Level 2: Detailed Call-Stack Flow**
  1. Operator invokes `perf-tool config validate --config <file>` ([`perf/go/perf-tool/main.go:231`](../../go/perf-tool/main.go#L231)).
  2. Runs `validate.InstanceConfigFromFile` validating JSON schema definitions.
  3. Running `create-pubsub-topics-and-subscriptions` creates Google Cloud Pub/Sub topics matching `ingestion_config`.
- **Level 3: Instance Support Matrix**
  Available for all 31 configuration targets.

---

#### Action 5.2: Diagnostic Trace Queries & Offline Export
- **Level 1: Simplistic View**
```
[Operator Runs: perf-tool traces export] -> [TracesExport: QueryTracesIDOnly]
                                                    |
                                                    v
                                  [Writes Offline JSON Dataset]
```
- **Level 2: Detailed Call-Stack Flow**
  1. Operator runs `perf-tool traces export --query <q> --begin <c1> --end <c2> --out <file>` ([`perf/go/perf-tool/main.go:273`](../../go/perf-tool/main.go#L273)).
  2. Connects directly to Spanner `TraceStore`, reads all matching traces between commit bounds, and outputs a JSON file for local debugging.
- **Level 3: Instance Support Matrix**
  Operational against all 31 databases.

---

#### Action 5.3: Force Re-ingestion of Historical Telemetry
- **Level 1: Simplistic View**
```
[Operator Runs: perf-tool ingest force-reingest]
                       |
                       v
[Verifies Breakglass Role -> Scans GCS Date Hierarchy]
                       |
                       v
[Re-publishes Ingestion Pub/Sub Notifications for Missing Files]
```
- **Level 2: Detailed Call-Stack Flow**
  1. Operator executes `perf-tool ingest force-reingest --start <t1> --stop <t2>` ([`perf/go/perf-tool/main.go:304`](../../go/perf-tool/main.go#L304)).
  2. Checks breakglass policy prompt (`skia-infra-breakglass-policy`).
  3. Scans GCS hourly directories, identifies missing files, and republishes ingestion messages to Pub/Sub.
- **Level 3: Instance Support Matrix**
  Operational across all instances.

---

#### Action 5.4: Database Disaster Recovery (Backups & Restores)
- **Level 1: Simplistic View**
```
[Operator Runs: perf-tool database backup alerts/shortcuts/regressions]
                                 |
                                 v
        [Dumps Spanner Table Rows to Serialized Backup File]
```
- **Level 2: Detailed Call-Stack Flow**
  1. Operator executes `perf-tool database backup <subcommand>` ([`perf/go/perf-tool/main.go:357`](../../go/perf-tool/main.go#L357)).
  2. Connects to Spanner/CockroachDB and exports `Alerts`, `Shortcuts`, or `Regressions2` data.
  3. Supports complementary `restore` commands to recover from corruption.
- **Level 3: Instance Support Matrix**
  Operational across all instances.

---

#### Action 5.5: Spanner Database Schema Migrations
- **Level 1: Simplistic View**
```
[CI/CD or Operator Runs: migrate.sh] -> [expectedschema.ValidateAndMigrateNewSchema]
                                                    |
                                                    v
                               [Applies DDL Schema Deltas to Spanner]
```
- **Level 2: Detailed Call-Stack Flow**
  1. Executed during deployment via `migrate.sh` ([`perf/migrate.sh`](../../migrate.sh)) or `maintenance.Start`.
  2. `expectedschema.ValidateAndMigrateNewSchema` checks `GetCurrentVersion` from database.
  3. Applies incremental DDL migrations bringing tables up to `schemaVersion`.
- **Level 3: Instance Support Matrix**
  Used across all Spanner instances.

---

#### Action 5.6: Android Raw Telemetry Re-transmission
- **Level 1: Simplistic View**
```
[Operator Runs: reingest-android-data.sh] -> [Pulls Swarming Outputs & Pushes to GCS]
```
- **Level 2: Detailed Call-Stack Flow**
  1. Operator executes `perf/reingest-android-data.sh` ([`perf/reingest-android-data.sh`](../../reingest-android-data.sh)).
  2. Re-extracts Android test outputs from Swarming servers and re-uploads to the Android GCS ingestion bucket.
- **Level 3: Instance Support Matrix**
  Dedicated to `android` and `android2-autopush`.

---

## 6. Challenging & Correcting Prior Worker Findings

| Item | Claim in Prior Investigation | Evidence Cited | What the Code Actually Shows | Corrected Finding |
| :--- | :--- | :--- | :--- | :--- |
| **1. Artifact File Path** | Written to `/usr/local/.../brain/c5d257de-b782-4bd5-a7a5-473a980b1ff4/perf_architecture_and_interaction_flows.md` | Author's report header | File did not exist on filesystem in requested path `/brain/512acb76-d3b0-446b-a564-59cf9982bbb7/...` or session directory | The artifact was missing; it has now been authored and written to the exact designated paths. |
| **2. TypeScript Component Inventory** | "Complete table of all 42 custom elements and controller classes" | modules summary | Inspection of `perf/modules/` reveals **59** `-sk` custom elements and 8 core controller modules | Prior worker missed 17 custom elements (e.g. `pinpoint-try-job-dialog-sk`, `existing-bug-dialog-sk`, `gemini-side-panel-sk`, `user-issue-sk`, `sheriff-configs-dry-run-sk`). |
| **3. Dual-Source Routing Mechanism** | "A client header toggle (preferLegacy) decides whether bug filing routes..." | Section 1 & Gaps | `perf/go/frontend/api/common.go:27-37` inspects `r.Cookie("fetch_anomalies_from_sql")` gated by `config.Config.SwitchBetweenAnomalySources` | Routing between legacy and SQL triage is governed by a **cookie**, NOT an HTTP client header. |
| **4. Spanner Migration Instances Count** | "10 instances have completed migration to Spanner (`fetch_anomalies_from_sql: true`)" | Section 4 & Gaps | Direct inspection of `perf/configs/spanner/*.json` identifies exactly 9 instances with `fetch_anomalies_from_sql: true` | There are **9** instances, not 10 (`chrome-internal-autopush`, `chrome-internal-ng`, `chrome-public-autopush`, `fuchsia-exp-internal`, `fuchsia-exp-public`, `fuchsia-internal-autopush`, `v8-internal-autopush`, `v8-internal`, `webrtc-public-ng`). |
| **5. Graph Query Execution Call-Stack** | Skipped `frame.ProcessFrameRequest` and claimed direct call from handler to `dfBuilder` | Action 1.1 | `graphApi.go:185` invokes `frame.ProcessFrameRequest`, which manages formula execution (`p.doCalc`), trace limits, metadata links, and anomaly overlays | `ProcessFrameRequest` is the central coordinator of frame execution and contains vital data transformations. |
| **6. Pinpoint API Route Coverage** | Only listed `POST /_/bisect/create` | Action 1.4 | `pinpointApi.go:63-67` registers `POST /_/bisect/create`, `POST /_/try`, and `/p` | Prior investigation omitted pairwise try-job scheduling (`/_/try`) and bisection querying (`/p`). |
| **7. Query API Route Coverage** | Only listed `POST /_/count` and `POST /_/nextParamList` | Action 1.6 | `queryApi.go:38` registers `/_/initpage` | `/_/initpage` is essential for initial page loads and bootstrap paramset retrieval. |
| **8. Omission of Entire Subsystems** | Completely omitted `SheriffConfigApi` and `UserIssueApi` | Subsystem list | Handlers exist in `perf/go/frontend/api/sheriffConfigApi.go` and `userIssueApi.go` | Added complete flows for LUCI Config validation and user bug annotations. |
| **9. Maintenance Daemons Breadth** | Only cited git poller, continuous clustering, paramset refresher, and deletion | Category 3 | `maintenance.go` initializes trace visibility checker/promoter, sheriff config sync, redis cache refresh, and regression migration | Documented all 9 active background ticker routines. |
| **10. End-to-End Workflow Trigger Link** | Fragmented description of how continuous clustering relates to Temporal | Category 4 | `AnomalyGroupNotifier.RegressionFound` calls `anomalygrouputils.ProcessRegression` which invokes `temporalClient.ExecuteWorkflow(workflows.MaybeTriggerBisection)` | Provided the complete, unbroken chain from trace ingestion to regression detection to Temporal worker execution. |

---

## 7. Remaining Questions & Gaps

1. **Temporal Pinpoint Worker Implementation Details**:
   - The Temporal workflow coordinators (`MaybeTriggerBisectionWorkflow`, `ProcessCulpritWorkflow`) and their activities are located in `perf/go/workflows/internal`. However, the downstream Pinpoint worker execution code that spins up Swarming bots and runs benchmark pairwise comparisons resides in the separate `go.skia.org/infra/pinpoint` package. This constitutes the primary focus of upcoming **Task 3**.
2. **Dual-Backend Deprecation Timeline**:
   - 7 instances currently operate in dual-backend mode (`SwitchBetweenAnomalySources`). Tracking the final decommissioning of ChromePerf API dependencies will allow removing `chromeperfClient` and cookie inspection logic.
3. **Recommended Next Steps**:
   - Formulate slide presentations directly from the 5 Trigger Categories and Level 1 / Level 2 interaction flows provided in this reference.
   - Advance to Task 3: Complete Pinpoint architecture and interaction flow investigation.
