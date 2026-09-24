# Skia Pinpoint Architecture, Struct Inventory & End-to-End Interaction Flows

Comprehensive technical reference manual and architecture specification for Skia Pinpoint (`go.skia.org/infra/pinpoint`). Grounded strictly in codebase source files with verified symbol locations, data contracts, mathematical models, and call stacks.

---

## 1. System Topology & Subsystem Architecture

Skia Pinpoint is a distributed performance bisection, A/B try-job, and regression verification platform designed to pinpoint performance regressions in Chromium and related dependencies down to the individual culprit commit or DEPS roll.

```
+---------------------------------------------------------------------------------------------------+
|                                  Frontend UI Layer Clients                                        |
|                                                                                                   |
|  +----------------------------------------------+  +-------------------------------------------+  |
|  | Modern Angular 17+ SPA (pinpoint/webui/app) |  | Legacy Lit Web Components (pinpoint/ui)   |  |
|  | - JobListComponent, JobTableComponent       |  | - <pinpoint-landing-page-sk>              |  |
|  | - NewJobComponent (PatchParser, Form)       |  | - <pinpoint-new-job-sk>                   |  |
|  | - GatewayService (HTTP client /proxy)        |  | - <commit-run-overview-sk>, <job-overview>|  |
|  +----------------------+-----------------------+  +---------------------+---------------------+  |
|                         |                                                |                        |
+-------------------------|------------------------------------------------|------------------------+
                          | HTTP / REST (JSON)                             | HTTP / REST (JSON)
                          v                                                v
+---------------------------------------------------------------------------------------------------+
|                             Pinpoint HTTP Gateway & Ingestion Layer                               |
|                                                                                                   |
|  +------------------------------------------+    +---------------------------------------------+  |
|  | pinpoint-webui Gateway Process           |    | pinpoint-jobs Frontend Service              |  |
|  | (pinpoint/go/webui/main.go)              |    | (pinpoint/go/frontend/cmd/main.go)          |  |
|  | - Reverse Proxy & Static Asset Server     |    | - chi HTTP Mux (/benchmarks, /bots, /jobs)  |  |
|  | - PinpointGateway gRPC-Gateway Proxy      |    | - Spanner JobStore Query Engine             |  |
|  |   (/pinpoint/v1/jobs, /user, /new, etc.) |    | - gRPC-Gateway Pinpoint JSON Handler        |  |
|  +--------------------+---------------------+    +----------------------+----------------------+  |
+-----------------------|-------------------------------------------------|-------------------------+
                        |                                                 |
                        +-----------------------+-------------------------+
                                                |
                                                v
+---------------------------------------------------------------------------------------------------+
|                         Pinpoint Core gRPC API & Translation Service                              |
|                          (pinpoint/go/service/service_impl.go & pinpoint/go/pinpoint)             |
|                                                                                                   |
|  +---------------------------+  +---------------------------+  +-------------------------------+  |
|  | ScheduleBisection RPC     |  | SchedulePairwise RPC      |  | ScheduleCulpritFinder RPC     |  |
|  | - RateLimiter (1 per 30m) |  | - RateLimiter (1 per 30m) |  | - RateLimiter (1 per 30m)    |  |
|  | - Input Validation        |  | - Input Validation        |  | - Catapult compatibility field|  |
|  | - Temporal Client Exec    |  | - Temporal Client Exec    |  | - Temporal Client Exec       |  |
|  +-------------+-------------+  +-------------+-------------+  +---------------+---------------+  |
|                |                              |                                |                  |
|                v                              v                                v                  |
|  +---------------------------------------------------------------------------------------------+  |
|  | Legacy Chromeperf / Catapult Bridge (pinpoint/go/pinpoint/internal/legacy_client.go)        |  |
|  | - Talks to pinpoint-dot-chromeperf.appspot.com & chromeperf.appspot.com                     |  |
|  | - Mimics legacy jobs, queries legacy datastore, creates try/bisect jobs on legacy Catapult  |  |
|  +---------------------------------------------------------------------------------------------+  |
+------------------------------------------------|--------------------------------------------------+
                                                 |
                                                 v
+---------------------------------------------------------------------------------------------------+
|                         Temporal Distributed Workflow Orchestration Engine                        |
|                                 (Namespace: perf-internal)                                        |
|                                                                                                   |
|  +-----------------------------------------+     +---------------------------------------------+  |
|  | BisectWorkflow (perf.bisect)            |     | PairwiseWorkflow (perf.pairwise)            |  |
|  | - Adaptive sample sizing:               |     | - PairwiseCommitsRunnerWorkflow             |  |
|  |   10 -> 20 -> 40 -> 80 -> 160           |     | - Randomized AB / BA Ordering               |  |
|  | - Non-blocking channel & selector loop  |     | - Permutation Balancing & Drop Logic        |  |
|  | - Concurrent binary search on commits   |     | - Wilcoxon Signed-Rank Test Evaluation      |  |
|  +--------------------+--------------------+     +---------------------+-----------------------+  |
|                       |                                                |                          |
|                       v                                                v                          |
|  +-----------------------------------------+     +---------------------------------------------+  |
|  | CatapultBisectWorkflow                  |     | CulpritFinderWorkflow (Sandwich Workflow)   |  |
|  | (perf.catapult.bisect)                  |     | (perf.culprit_finder)                       |  |
|  | - Wraps BisectWorkflow                  |     | 1. Pairwise Regression Verification         |  |
|  | - Converts to Catapult LegacyJobResponse|     | 2. Bisection to Find Culprits               |  |
|  | - Writes back to Catapult Datastore     |     | 3. Pairwise Culprit Verification (A-1 vs A) |  |
|  |                                         |     | 4. Invoke Culprit Processing Callback       |  |
|  +--------------------+--------------------+     +---------------------+-----------------------+  |
+-----------------------|------------------------------------------------|--------------------------+
                        |                                                |
                        v                                                v
+---------------------------------------------------------------------------------------------------+
|                                  Child Workflows & Activity Workers                               |
|                                                                                                   |
|  +--------------------------------+  +-------------------------------+  +----------------------+  |
|  | SingleCommitRunner             |  | RunBenchmarkWorkflow          |  | BuildChromeWorkflow  |  |
|  | - Orchestrates Build + Swarm   |  | - Schedules task on Swarming  |  | - Search/Build on BB |  |
|  | - Distributes across Bot IDs   |  | - Polls Pending & Running     |  | - Waits completion   |  |
|  | - Collects CAS histogram values|  | - Fetches CAS Output Root     |  | - Fetches CAS isolate|  |
|  +--------------------------------+  +-------------------------------+  +----------------------+  |
+------------------------------------------------|--------------------------------------------------+
                                                 |
         +---------------------------------------+---------------------------------------+
         |                                       |                                       |
         v                                       v                                       v
+----------------------+               +----------------------+               +----------------------+
| LUCI Buildbucket API |               | LUCI Swarming Server |               | Google RBE-CAS       |
| (cr-buildbucket)     |               | (chrome-swarming)    |               | (remotebuildexecution|
|                      |               |                      |               |  .googleapis.com)    |
| - SearchBuilds       |               | - TriggerTask        |               |                      |
| - ScheduleBuild      |               | - FetchFreeBots      |               | - DialRBECAS         |
| - GetBuildStatus     |               | - GetStatus          |               | - ReadValuesByChart  |
| - Output CAS digest  |               | - CancelTasks        |               | - ReadValuesForAll   |
+----------------------+               +----------------------+               +----------------------+
         |                                       |                                       |
         +---------------------------------------+---------------------------------------+
                                                 |
                                                 v
+---------------------------------------------------------------------------------------------------+
|                         External Storage, Persistence & Integration Layer                         |
|                                                                                                   |
|  +-------------------------+  +--------------------------+  +----------------------------------+  |
|  | Spanner Pinpoint DB     |  | Perf Subsystem Bridge    |  | Google IssueTracker (Buganizer)  |  |
|  | (JobStore)              |  | (MaybeTriggerBisection)  |  | (issuetracker.googleapis.com)    |  |
|  | - Jobs table            |  | - Autobisections Table   |  | - Post Culprit Detected Comment  |  |
|  | - CommitRunData (CAS)   |  | - AnomalyGroup Callback  |  | - Formatted with Gitiles links   |  |
|  | - Wilcoxon Results JSON |  | - perf.process_culprit   |  | - secretAPIKey authenticated     |  |
|  +-------------------------+  +--------------------------+  +----------------------------------+  |
+---------------------------------------------------------------------------------------------------+
```

---

## 2. Comprehensive Go Struct & Interface Inventory

The table below catalogs every primary Go struct, interface, and data abstraction across `pinpoint/go/...` and `pinpoint/proto/v1`.

| Package | Symbol Name | Type | Source File | Core Responsibility |
| :--- | :--- | :--- | :--- | :--- |
| `pinpointpb` | [`PinpointServer`](../../../pinpoint/proto/v1/service_grpc.pb.go#L28) | `interface` | `pinpoint/proto/v1/service.proto` | Core gRPC server interface (`ScheduleBisection`, `QueryBisection`, `SchedulePairwise`, `QueryPairwise`, `ScheduleCulpritFinder`, `CancelJob`, `LegacyJobQuery`). |
| `pinpointpb` | [`PinpointGatewayServer`](../../../pinpoint/proto/v1/gateway_grpc.pb.go#L29) | `interface` | `pinpoint/proto/v1/gateway.proto` | WebUI gateway gRPC server interface (`QueryJobList`, `GetUserInfo`, `CreateTryJob`, `CancelJob`, `ListBotConfigurations`, `ListBenchmarks`, `GetBenchmark`, `ListRecentBuilds`, `GetCommit`, `GetPatch`). |
| `pinpointpb` | [`ScheduleBisectRequest`](../../../pinpoint/proto/v1/service.pb.go#L34) | `struct` | `pinpoint/proto/v1/service.proto` | Protobuf payload defining bisection input parameters (git hashes, bot configuration, benchmark, story, chart, magnitude, pin, aggregation). |
| `pinpointpb` | [`SchedulePairwiseRequest`](../../../pinpoint/proto/v1/service.pb.go#L370) | `struct` | `pinpoint/proto/v1/service.proto` | Protobuf payload defining pairwise try-job parameters (start/end `CombinedCommit` or CAS builds, bot name, benchmark, story, initial attempts). |
| `pinpointpb` | [`ScheduleCulpritFinderRequest`](../../../pinpoint/proto/v1/service.pb.go#L644) | `struct` | `pinpoint/proto/v1/service.proto` | Protobuf payload initiating sandwich verification workflow across a commit range. |
| `pinpointpb` | [`CombinedCommit`](../../../pinpoint/proto/v1/service.pb.go#L231) | `struct` | `pinpoint/proto/v1/service.proto` | Represents a composite commit: base Chromium commit, optional list of modified DEPS revisions, and optional Gerrit patch. |
| `pinpointpb` | [`Commit`](../../../pinpoint/proto/v1/service.pb.go#L115) | `struct` | `pinpoint/proto/v1/service.proto` | Metadata model for a single Git commit (git hash, repo URL, author, timestamp, subject, commit position, review URL, change ID). |
| `pinpointpb` | [`GerritChange`](../../../pinpoint/proto/v1/service.pb.go#L198) | `struct` | `pinpoint/proto/v1/service.proto` | Encapsulates a Gerrit patchset reference (host, project, change number, patchset number, patchset git hash). |
| `pinpointpb` | [`Culprit`](../../../pinpoint/proto/v1/service.pb.go#L265) | `struct` | `pinpoint/proto/v1/service.proto` | Pair of commits representing the identified culprit (`Culprit`) and its immediate predecessor (`Prior`) for verification. |
| `pinpointpb` | [`CASReference`](../../../pinpoint/proto/v1/service.pb.go#L292) | `struct` | `pinpoint/proto/v1/service.proto` | RBE-CAS content locator containing CAS instance string and `Digest` (hash and byte size). |
| `pinpointpb` | [`PairwiseExecution`](../../../pinpoint/proto/v1/service.pb.go#L532) | `struct` | `pinpoint/proto/v1/service.proto` | Output model of pairwise run containing Wilcoxon results map per chart and left/right swarming task statuses. |
| `pinpointpb` | [`BisectExecution`](../../../pinpoint/proto/v1/service.pb.go#L341) | `struct` | `pinpoint/proto/v1/service.proto` | Output model of bisection workflow containing job ID, identified culprits, and detailed prior/culprit pairs. |
| `pinpointpb` | [`CulpritProcessingCallbackParams`](../../../pinpoint/proto/v1/service.pb.go#L705) | `struct` | `pinpoint/proto/v1/service.proto` | Parameters allowing Pinpoint to dispatch callback workflows (`perf.process_culprit`) to Perf's Temporal task queue. |
| `pinpointpb` | [`LegacyJobResponse`](../../../pinpoint/proto/v1/service.pb.go#L756) | `struct` | `pinpoint/proto/v1/service.proto` | Catapult-compatible schema mirroring `/api/job` response format for backward compatibility with the legacy Catapult dashboard. |
| `service` | [`server`](../../../pinpoint/go/service/service_impl.go#L25) | `struct` | `pinpoint/go/service/service_impl.go` | Core implementation of `pb.PinpointServer` managing rate limiting, Temporal client dispatch, and workflow lifecycle. |
| `pinpoint` | [`Client`](../../../pinpoint/go/pinpoint/pinpoint.go#L21) | `struct` | `pinpoint/go/pinpoint/pinpoint.go` | High-level client wrapping `LegacyClient` and authenticated Gerrit HTTP client. |
| `pinpoint` | [`pinpointClient`](../../../pinpoint/go/pinpoint/gateway.go#L15) | `interface` | `pinpoint/go/pinpoint/gateway.go` | Internal contract defining data operations needed by the WebUI gateway server. |
| `pinpoint` | [`gatewayServer`](../../../pinpoint/go/pinpoint/gateway.go#L27) | `struct` | `pinpoint/go/pinpoint/gateway.go` | Implementation of `pb.PinpointGatewayServer` handling user authentication headers and proxying to `pinpointClient`. |
| `internal` | [`LegacyClient`](../../../pinpoint/go/pinpoint/internal/legacy_client.go#L68) | `struct` | `pinpoint/go/pinpoint/internal/legacy_client.go` | HTTP client communicating with `pinpoint-dot-chromeperf.appspot.com` and `chromeperf.appspot.com`. |
| `jobsservice` | [`Service`](../../../pinpoint/go/frontend/service/jobs.go#L41) | `struct` | `pinpoint/go/frontend/service/jobs.go` | HTTP controller serving HTML templates, benchmark/bot metadata, and Spanner job queries. |
| `jobstore` | [`JobStore`](../../../pinpoint/go/sql/jobs_store/jobs_store.go#L29) | `interface` | `pinpoint/go/sql/jobs_store/jobs_store.go` | Database persistence abstraction for Pinpoint jobs (`AddInitialJob`, `UpdateJobStatus`, `GetJob`, `AddResults`, `SetErrors`, `AddCommitRuns`, `ListJobs`). |
| `jobstore` | [`jobStoreImpl`](../../../pinpoint/go/sql/jobs_store/jobs_store.go#L53) | `struct` | `pinpoint/go/sql/jobs_store/jobs_store.go` | Implementation of `JobStore` persisting records into Spanner SQL tables. |
| `schema` | [`JobSchema`](../../../pinpoint/go/sql/schema/schema.go#L11) | `struct` | `pinpoint/go/sql/schema/schema.go` | Database row mapping for the `Jobs` table in Spanner. |
| `schema` | [`CommitRunData`](../../../pinpoint/go/sql/schema/schema.go#L36) | `struct` | `pinpoint/go/sql/schema/schema.go` | Storage model for build parameters, commit info, CAS references, and test runs. |
| `backends` | [`BuildbucketClient`](../../../pinpoint/go/backends/buildbucket.go#L69) | `interface` | `pinpoint/go/backends/buildbucket.go` | Contract for managing LUCI Buildbucket builds (`StartChromeBuild`, `GetSingleBuild`, `GetBuildStatus`, `GetCASReference`, `CancelBuild`). |
| `backends` | [`buildbucketClient`](../../../pinpoint/go/backends/buildbucket.go#L112) | `struct` | `pinpoint/go/backends/buildbucket.go` | Production implementation of `BuildbucketClient` utilizing gRPC `bgrpcpb.BuildsClient`. |
| `backends` | [`SwarmingClient`](../../../pinpoint/go/backends/swarming.go#L25) | `interface` | `pinpoint/go/backends/swarming.go` | Contract for managing LUCI Swarming benchmark tasks (`TriggerTask`, `FetchFreeBots`, `GetStatus`, `GetCASOutput`, `CancelTasks`, `GetBotTasksBetweenTwoTasks`). |
| `backends` | [`SwarmingClientImpl`](../../../pinpoint/go/backends/swarming.go#L52) | `struct` | `pinpoint/go/backends/swarming.go` | Implementation of `SwarmingClient` wrapping `swarmingv2.SwarmingV2Client`. |
| `backends` | [`IssueTracker`](../../../pinpoint/go/backends/issuetracker.go#L31) | `interface` | `pinpoint/go/backends/issuetracker.go` | Interface for filing culprit detection reports to Google IssueTracker. |
| `backends` | [`issueTrackerTransport`](../../../pinpoint/go/backends/issuetracker.go#L36) | `struct` | `pinpoint/go/backends/issuetracker.go` | Production IssueTracker client using Google API secret key and text templates. |
| `bot_configs` | [`BotConfig`](../../../pinpoint/go/bot_configs/bot_configs.go#L72) | `struct` | `pinpoint/go/bot_configs/bot_configs.go` | Model defining bot parameters: browser, builder, bucket, repo, swarming server, dimensions, and alias. |
| `bot_configs` | [`TargetMaps`](../../../pinpoint/go/bot_configs/isolate_targets.go#L17) | `struct` | `pinpoint/go/bot_configs/isolate_targets.go` | YAML configuration mapping benchmarks, bot exact names, and regex patterns to build isolate target binaries. |
| `common` | [`CombinedCommit`](../../../pinpoint/go/common/combined_commit.go#L11) | `struct` | `pinpoint/go/common/combined_commit.go` | Go native representation of composite commits with hashing, cloning, and DEPS modification helpers. |
| `compare` | [`CompareResults`](../../../pinpoint/go/compare/compare.go#L220) | `struct` | `pinpoint/go/compare/compare.go` | Results model of statistical bisection comparison (`Verdict`, `PValue`, `PValueKS`, `PValueMWU`, `LowThreshold`, `HighThreshold`, `MeanDiff`). |
| `compare` | [`ComparePairwiseResult`](../../../pinpoint/go/compare/compare.go#L131) | `struct` | `pinpoint/go/compare/compare.go` | Results model of pairwise try-job comparison embedding `PairwiseWilcoxonSignedRankedTestResult`. |
| `stats` | [`PairwiseWilcoxonSignedRankedTestResult`](../../../pinpoint/go/compare/stats/wilcoxon_signed_rank.go#L29) | `struct` | `pinpoint/go/compare/stats/wilcoxon_signed_rank.go` | Mathematical output: Hodges-Lehmann estimate, confidence intervals (`LowerCi`, `UpperCi`), p-value, control/treatment medians. |
| `midpoint` | [`MidpointHandler`](../../../pinpoint/go/midpoint/midpoint.go#L31) | `struct` | `pinpoint/go/midpoint/midpoint.go` | Evaluates Gitiles commit logs and DEPS files to calculate binary search midpoints and resolve DEPS rolls. |
| `midpoint` | [`CommitRange`](../../../pinpoint/go/midpoint/midpoint.go#L25) | `struct` | `pinpoint/go/midpoint/midpoint.go` | Tuple holding left and right `CombinedCommit`s being bisected. |
| `read_values` | [`Client`](../../../pinpoint/go/read_values/read_values.go#L23) | `struct` | `pinpoint/go/read_values/read_values.go` | Client fetching benchmark histogram outputs from RBE-CAS digests and parsing measurements. |
| `run_benchmark` | [`RunBenchmark`](../../../pinpoint/go/run_benchmark/run_benchmark.go#L33) | `struct` | `pinpoint/go/run_benchmark/run_benchmark.go` | Coordinates task creation parameters and builds swarming `NewTaskRequest` payloads. |
| `workflows` | [`BisectParams`](../../../pinpoint/go/workflows/workflows.go#L152) | `struct` | `pinpoint/go/workflows/workflows.go` | Input parameter envelope for `BisectWorkflow` and `CatapultBisectWorkflow`. |
| `workflows` | [`PairwiseParams`](../../../pinpoint/go/workflows/workflows.go#L213) | `struct` | `pinpoint/go/workflows/workflows.go` | Input parameter envelope for `PairwiseWorkflow`. |
| `workflows` | [`CulpritFinderParams`](../../../pinpoint/go/workflows/workflows.go#L256) | `struct` | `pinpoint/go/workflows/workflows.go` | Input parameter envelope for `CulpritFinderWorkflow` including callback URLs and task queue. |
| `workflows` | [`BuildParams`](../../../pinpoint/go/workflows/workflows.go#L48) | `struct` | `pinpoint/go/workflows/workflows.go` | Parameters required to compile a binary on Buildbucket (commit, device, target, project, patch). |
| `workflows` | [`Build`](../../../pinpoint/go/workflows/workflows.go#L70) | `struct` | `pinpoint/go/workflows/workflows.go` | Represents a completed build: CAS isolate digest, build ID, and Buildbucket status. |
| `workflows` | [`TestRun`](../../../pinpoint/go/workflows/workflows.go#L94) | `struct` | `pinpoint/go/workflows/workflows.go` | Represents an executed benchmark swarming task: CAS results digest, values map, units, task ID, status. |
| `workflows` | [`PairwiseTestRun`](../../../pinpoint/go/workflows/workflows.go#L142) | `struct` | `pinpoint/go/workflows/workflows.go` | Pair of test runs (`FirstTestRun`, `SecondTestRun`) executed in randomized order (`Permutation`). |
| `internal` | [`BisectExecution`](../../../pinpoint/go/workflows/internal/bisect.go#L101) | `struct` | `pinpoint/go/workflows/internal/bisect.go` | Internal execution state tracking comparisons and run data across the bisection tree. |
| `internal` | [`BisectRun`](../../../pinpoint/go/workflows/internal/bisect_run.go#L21) | `struct` | `pinpoint/go/workflows/internal/bisect_run.go` | Manages scheduled and completed benchmark runs for a specific commit during bisection. |
| `internal` | [`CommitRun`](../../../pinpoint/go/workflows/internal/commits_runner.go#L69) | `struct` | `pinpoint/go/workflows/internal/commits_runner.go` | Encapsulates the Build and slice of `TestRun`s for a single revision. |
| `internal` | [`PairwiseRun`](../../../pinpoint/go/workflows/internal/pairwise_runner.go#L50) | `struct` | `pinpoint/go/workflows/internal/pairwise_runner.go` | Encapsulates Left and Right `CommitRun`s along with their execution `Order` permutation slice. |
| `internal` | [`CbbRunnerParams`](../../../pinpoint/go/workflows/internal/cbb_runner.go#L29) | `struct` | `pinpoint/go/workflows/internal/cbb_runner.go` | Parameter model for Chrome Browser Benchmarking runs (browser, channel, bot config, benchmarks). |
| `internal` | [`JobStoreActivities`](../../../pinpoint/go/workflows/internal/database_activities.go#L23) | `struct` | `pinpoint/go/workflows/internal/database_activities.go` | Temporal activity struct executing database mutations against Spanner `JobStore`. |
| `catapult` | [`DatastoreResponse`](../../../pinpoint/go/workflows/catapult/write.go#L18) | `struct` | `pinpoint/go/workflows/catapult/write.go` | Response model returned from writing bisection results back to Catapult Datastore. |

---

## 3. Comprehensive TypeScript / Angular Component & Service Inventory

Pinpoint has two frontend implementations:
1. **Modern Angular 17+ SPA** in `pinpoint/webui/app/...` (the active, forward-looking Pinpoint Web UI adhering to strict Angular Material design guidelines).
2. **Lit-based Custom Web Components** in `pinpoint/ui/modules/...` (the intermediate micro-frontend UI used for embeds and legacy views).

### 3.1 Modern Angular WebUI (`pinpoint/webui/app/...`)

| Directory / File | Component / Service Class | Type | Primary Role & Interaction |
| :--- | :--- | :--- | :--- |
| `app/app.component.ts` | [`AppComponent`](../../../pinpoint/webui/app/app.component.ts#L10) | Component | Root application shell rendering `<app-header>` and `<router-outlet>`. |
| `app/app.config.ts` | `appConfig` | Config | Application-wide DI providers: `provideRouter`, `provideAnimationsAsync`, `provideHttpClient`. |
| `app/header/header.component.ts` | [`HeaderComponent`](../../../pinpoint/webui/app/header/header.component.ts#L22) | Component | Top navigation toolbar displaying Pinpoint branding, current user email (`GetUserInfo`), theme toggle (dark/light), and settings dialog trigger. |
| `app/job-list/job-list.component.ts` | [`JobListComponent`](../../../pinpoint/webui/app/job-list/job-list.component.ts#L20) | Component | Top-level job explorer container hosting search filters, user filter, bot configuration selector, job table, and column customizer. |
| `app/job-list/job-table/job-table.component.ts` | [`JobTableComponent`](../../../pinpoint/webui/app/job-list/job-table/job-table.component.ts#L29) | Component | High-density data table (`mat-table`) rendering job ID, user, bot, benchmark, created time, status badges, and action buttons (Cancel, View). |
| `app/job-list/column-selector/column-selector.component.ts` | [`ColumnSelectorComponent`](../../../pinpoint/webui/app/job-list/column-selector/column-selector.component.ts#L19) | Component | Interactive column configuration dropdown allowing users to show, hide, and reorder table columns. |
| `app/new-job/new-job.component.ts` | [`NewJobComponent`](../../../pinpoint/webui/app/new-job/new-job.component.ts#L71) | Component | Comprehensive form dialog for creating try-jobs: selects bot, benchmark, story, base commit/patch, experiment commit/patch, and extra browser args. |
| `app/new-job/patch-parser.ts` | [`PatchParser`](../../../pinpoint/webui/app/new-job/patch-parser.ts#L22) | Utility | Parses Gerrit code-review URLs (`chromium-review.googlesource.com/c/project/+/12345/2`) into structured host, project, change ID, and patchset number. |
| `app/cancel-job-dialog/cancel-job-dialog.component.ts` | [`CancelJobDialogComponent`](../../../pinpoint/webui/app/cancel-job-dialog/cancel-job-dialog.component.ts#L20) | Component | Confirmation modal capturing user cancellation reason and dispatching `CancelJob` API call. |
| `app/gateway/gateway.service.ts` | [`GatewayService`](../../../pinpoint/webui/app/gateway/gateway.service.ts#L33) | Service | Angular HTTP client wrapper implementing `PinpointGateway` interface via REST endpoints (`/pinpoint/v1/...`). |
| `app/job-list/jobs.service.ts` | [`JobsService`](../../../pinpoint/webui/app/job-list/jobs.service.ts#L26) | Service | Reactive state store for job listings, active search queries, cursor pagination (`nextCursor`, `prevCursor`), and real-time polling. |
| `app/job-list/job-table-columns.service.ts` | [`JobTableColumnsService`](../../../pinpoint/webui/app/job-list/job-table-columns.service.ts#L21) | Service | Manages column visibility state and persists user preferences into `localStorage`. |
| `app/settings/settings.service.ts` | [`SettingsService`](../../../pinpoint/webui/app/settings/settings.service.ts#L18) | Service | Manages user application preferences (default bot pool, auto-refresh intervals). |
| `app/theme/theme.service.ts` | [`ThemeService`](../../../pinpoint/webui/app/theme/theme.service.ts#L15) | Service | Controls light/dark theme toggles and synchronizes theme classes with document root. |

### 3.2 Lit-based Custom Web Components (`pinpoint/ui/...`)

| Directory / File | Element Tag / Symbol | Type | Primary Role & Interaction |
| :--- | :--- | :--- | :--- |
| `modules/pinpoint-scaffold-sk` | `<pinpoint-scaffold-sk>` | Element | Application shell providing layout navigation, header, login status, and responsive container. |
| `modules/pinpoint-landing-page-sk`| `<pinpoint-landing-page-sk>`| Element | Landing dashboard rendering search bar, job filters, and `<jobs-table-sk>`. |
| `modules/jobs-table-sk` | `<jobs-table-sk>` | Element | Table rendering historic Pinpoint runs fetched from `/json/jobs/list`. |
| `modules/pinpoint-new-job-sk` | `<pinpoint-new-job-sk>` | Element | Interactive form element initiating new try and bisect jobs against `/pinpoint/v1/schedule`. |
| `modules/pinpoint-results-page-sk`| `<pinpoint-results-page-sk>`| Element | Detailed execution view rendering job status, commit comparisons, and test histograms. |
| `modules/job-overview-sk` | `<job-overview-sk>` | Element | High-level summary card displaying benchmark name, configuration, author, and verdict. |
| `modules/commit-run-overview-sk` | `<commit-run-overview-sk>` | Element | Visual comparison card contrasting build isolates and swarming task outputs for a commit pair. |
| `modules/wilcoxon-results-sk` | `<wilcoxon-results-sk>` | Element | Chart and table component visualizing Wilcoxon signed-rank confidence intervals, p-values, and medians. |
| `services/api.ts` | `fetchJobs`, `getJob`, etc. | Module | Client API module executing HTTP requests against `/json/jobs/list` and `/json/job/{jobID}`. |

---

## 4. Bot Configurations, Isolate Target Mapping & Benchmark Support Matrix

### 4.1 Bot Configuration Architecture (`bot_configs.go`)
Pinpoint uses two embedded JSON catalogs:
1. `external.json`: Public builders and bot configurations (e.g. `linux-perf`, `win-11-perf`, `mac-m2-pro-perf`).
2. `internal.json`: Internal Google-proprietary test hardware (e.g. `android-pixel*-perf`, `internal-linux-perf`).

Each `BotConfig` defines:
- **`browser`**: Target browser binary (`chrome`, `android-chrome-bundle`, etc.).
- **`bucket`**: LUCI pool used to compile binaries (typically `try` or `luci.chrome.try`).
- **`builder`**: Compile builder on Buildbucket (e.g. `linux-perf-builder`, `android-perf-builder`).
- **`repository`**: Repository to build (`chromium`).
- **`swarming_server`**: Swarming instance (default `https://chrome-swarming.appspot.com`).
- **`dimensions`**: Swarming bot selector tags (`os`, `device_type`, `pool:chrome.tests.perf`).

### 4.2 Isolate Target Mapping Matrix (`isolate_targets.yaml`)
To run benchmarks on Swarming, Pinpoint must compile the correct isolate binary on Buildbucket. Target resolution is performed by `bot_configs.GetIsolateTarget(bot, benchmark)`:

| Benchmark / Bot Criteria | Resolved Build Isolate Target | Underlying Target Rationale |
| :--- | :--- | :--- |
| **Benchmark Exact:** `webrtc_perf_tests` | `webrtc_perf_tests` | Standalone WebRTC performance benchmark runner binary. |
| **Bot Exact:** `android-go-wembley-perf` | `performance_test_suite_android_chrome_google_bundle` | Android Go Low-RAM Chrome bundle. |
| **Bot Exact:** `android-new-pixel-perf` | `performance_test_suite_android_chrome_google_bundle` | Modern Pixel device Google Chrome application bundle. |
| **Bot Exact:** `android-new-pixel-pro-perf` | `performance_test_suite_android_chrome_google_bundle` | Pixel Pro flagship bundle. |
| **Bot Exact:** `android-pixel-fold-perf` | `performance_test_suite_android_chrome_google_bundle` | Pixel Foldable bundle. |
| **Bot Exact:** `android-pixel-tangor-perf[-cbb]` | `performance_test_suite_android_chrome_google_bundle` | Pixel Tablet bundle. |
| **Bot Exact:** `android-pixel10-perf[-cbb]` | `performance_test_suite_android_chrome_google_bundle` | Next-gen Pixel bundle. |
| **Bot Exact:** `android-pixel4-perf[-pgo]` | `performance_test_suite_android_chrome_google_bundle` | Legacy Pixel 4 hardware bundle. |
| **Bot Exact:** `android-pixel6-perf[-pgo]` | `performance_test_suite_android_chrome_google_bundle` | Pixel 6 Tensor v1 bundle. |
| **Bot Exact:** `android-pixel6-pro-perf` | `performance_test_suite_android_chrome_google_bundle` | Pixel 6 Pro Tensor v1 bundle. |
| **Bot Exact:** `android-pixel9-perf` | `performance_test_suite_android_chrome_google_bundle` | Pixel 9 Tensor v4 bundle. |
| **Bot Exact:** `android-pixel9-pro[-xl]-perf` | `performance_test_suite_android_chrome_google_bundle` | Pixel 9 Pro flagship bundles. |
| **Bot Exact:** `android-samsung-foldable-perf` | `performance_test_suite_android_chrome_google_bundle` | Samsung Galaxy Fold bundle. |
| **Regex Match:** `.*webview.*` | `performance_webview_test_suite` | Android System WebView shell and driver tests. |
| **Regex Match:** `.*eve.*` | `performance_test_suite_eve` | ChromeOS Google Pixelbook (Eve) Chromebook target. |
| **Regex Match:** `.*fuchsia-perf.*` | `performance_web_engine_test_suite` | Fuchsia OS WebEngine headless runner. |
| **Default Fallback:** (All other desktop bots) | `performance_test_suite` | Standard desktop Linux, Windows, and macOS Chrome performance harness. |

---

## 5. Exhaustive End-to-End Interaction Flows Catalog

Every interaction across Pinpoint is cataloged under its respective trigger category.

### Category 1: UI_INTERACTIVE

#### Action 1.1: WebUI Try Job Scheduling (Pairwise A/B Analysis)
- **Level 1: Simplistic View**
```
[User configures Try Job in NewJobComponent]
                    |
                    v
[POST /pinpoint/v1/new -> GatewayService.CreateTryJob]
                    |
                    v
[PinpointGateway.CreateTryJob -> extracts user & validates]
                    |
                    v
[LegacyClient.CreatePinpointTryJob / POST /api/new]
                    |
                    v
[Spanner JobStore: AddInitialJob (status: 'Pending')]
                    |
                    v
[Dispatches Temporal PairwiseWorkflow (perf.pairwise)]
                    |
                    v
[Returns JobID to Frontend UI]
```
- **Level 2: Detailed Call-Stack Flow**
  1. User fills in benchmark, bot configuration, story, base commit/patch, and experiment commit/patch in `NewJobComponent` ([`new-job.component.ts:350`](../../../pinpoint/webui/app/new-job/new-job.component.ts#L350)) and clicks "Submit".
  2. If a patch URL was provided, `PatchParser.parsePatch` ([`patch-parser.ts:22`](../../../pinpoint/webui/app/new-job/patch-parser.ts#L22)) decomposes the URL into host, project, change ID, and patchset number.
  3. `GatewayService.CreateTryJob` ([`gateway.service.ts:69`](../../../pinpoint/webui/app/gateway/gateway.service.ts#L69)) dispatches `POST /pinpoint/v1/new` with a `CreateTryJobRequest` JSON payload.
  4. Reverse proxy passes request to `gatewayServer.CreateTryJob` ([`gateway.go:87`](../../../pinpoint/go/pinpoint/gateway.go#L87)).
  5. `getEmailFromContext` ([`gateway.go:63`](../../../pinpoint/go/pinpoint/gateway.go#L63)) extracts the authenticated user email from incoming metadata header `x-webauth-user`.
  6. Invokes `Client.CreatePinpointTryJob` ([`pinpoint.go:94`](../../../pinpoint/go/pinpoint/pinpoint.go#L94)), forwarding to `LegacyClient.CreatePinpointTryJob` ([`internal/legacy_client.go:486`](../../../pinpoint/go/pinpoint/internal/legacy_client.go#L486)).
  7. Alternatively, when invoking via Go Pinpoint gRPC directly, `server.SchedulePairwise` ([`service_impl.go:221`](../../../pinpoint/go/service/service_impl.go#L221)) is called.
  8. `server.SchedulePairwise` enforces rate limiting (`s.limiter.Allow()`, 1 request per 30 minutes) and runs `validatePairwiseRequest` ([`validation.go:12`](../../../pinpoint/go/service/validation.go#L12)).
  9. Connects to Temporal via `s.temporal.NewClient(s.hostPort, s.namespace)`.
  10. Sets `client.StartWorkflowOptions` with `WorkflowExecutionTimeout: 2 * time.Hour` and `MaximumAttempts: 1`.
  11. Dispatches `c.ExecuteWorkflow(ctx, workflowOptions, workflows.PairwiseWorkflow, &workflows.PairwiseParams{...})` ([`service_impl.go:254`](../../../pinpoint/go/service/service_impl.go#L254)).
  12. Returns `PairwiseExecution{JobId: workflowRun.GetID()}` to caller.
- **Level 3: Supported Configurations Matrix**
  Supported by all bots listed in `bot_configs/external.json` and `bot_configs/internal.json` running Telemetry, Crossbench, and WebRTC benchmarks.

---

#### Action 1.2: Bisection Scheduling via API / Perf UI
- **Level 1: Simplistic View**
```
[Perf UI / User invokes POST /pinpoint/v1/bisection]
                        |
                        v
[server.ScheduleBisection -> Rate Limiter (30m)]
                        |
                        v
[validateBisectRequest (bot, benchmark, commits)]
                        |
                        v
[Temporal Client connects to namespace 'perf-internal']
                        |
                        v
[c.ExecuteWorkflow: CatapultBisectWorkflow (12h timeout)]
                        |
                        v
[Returns BisectExecution{JobId: uuid}]
```
- **Level 2: Detailed Call-Stack Flow**
  1. Triggered from Skia Perf's `bisect-dialog-sk` or via cURL `POST /pinpoint/v1/bisection` with `ScheduleBisectRequest` payload.
  2. Handled by `server.ScheduleBisection` ([`service_impl.go:133`](../../../pinpoint/go/service/service_impl.go#L133)).
  3. Evaluates rate limiter `s.limiter.Allow()`. If exhausted, rejects with `codes.ResourceExhausted`.
  4. Calls `updateFieldsForCatapult(req)` for field compatibility.
  5. Executes `validateBisectRequest(req)` ([`validation.go:42`](../../../pinpoint/go/service/validation.go#L42)), verifying non-empty start/end git hashes, valid bot config, benchmark, chart, and story.
  6. Creates Temporal client via `s.temporal.NewClient(s.hostPort, s.namespace)`.
  7. Configures `client.StartWorkflowOptions` with `WorkflowExecutionTimeout: 12 * time.Hour`, `TaskQueue: s.taskQueue`, and `MaximumAttempts: 1`.
  8. Calls `c.ExecuteWorkflow(ctx, wo, workflows.CatapultBisect, &workflows.BisectParams{Request: req, Production: !s.devMode})` ([`service_impl.go:165`](../../../pinpoint/go/service/service_impl.go#L165)).
  9. Returns `&pb.BisectExecution{JobId: wf.GetID()}` to HTTP client.
- **Level 3: Supported Configurations Matrix**
  Supported across all Chromium performance bots. When invoked from Perf, enabled on the 18 Spanner instances configured with `show_bisect_btn: true` (e.g. `chrome-internal`, `chrome-public-autopush`, `fuchsia-internal`, `v8-public`).

---

#### Action 1.3: Job Querying & Status Monitoring
- **Level 1: Simplistic View**
```
[Client sends GET /pinpoint/v1/query?job_id=xxx or /query-pairwise]
                                  |
                                  v
[server.QueryPairwise / server.QueryBisection]
                                  |
                                  v
[Temporal: DescribeWorkflowExecution (status check)]
                                  |
         +------------------------+------------------------+
         |                                                 |
         v (RUNNING)                                       v (COMPLETED)
[Return Status: RUNNING]                   [c.GetWorkflow -> workflowRun.Get]
                                                           |
                                                           v
                                           [Deserialize PairwiseExecution]
                                                           |
                                                           v
                                           [Return Status: COMPLETED + Results]
```
- **Level 2: Detailed Call-Stack Flow**
  1. Frontend polls `GET /pinpoint/v1/query-pairwise?job_id={id}` or `GET /json/job/{jobID}`.
  2. For pairwise queries: `server.QueryPairwise` ([`service_impl.go:268`](../../../pinpoint/go/service/service_impl.go#L268)) validates `job_id`.
  3. Dials Temporal client and executes `c.DescribeWorkflowExecution(ctx, req.JobId, "")` ([`service_impl.go:282`](../../../pinpoint/go/service/service_impl.go#L282)).
  4. Inspects `resp.GetWorkflowExecutionInfo().GetStatus()`:
     - `WORKFLOW_EXECUTION_STATUS_COMPLETED`: Invokes `c.GetWorkflow(ctx, req.JobId, "")`, calls `workflowRun.Get(ctx, &pairwiseExecution)` to deserialize results, and returns `QueryPairwiseResponse{Status: COMPLETED, Execution: &pairwiseExecution}` ([`service_impl.go:303`](../../../pinpoint/go/service/service_impl.go#L303)).
     - `WORKFLOW_EXECUTION_STATUS_FAILED` / `TIMED_OUT` / `TERMINATED`: Returns `QueryPairwiseResponse{Status: FAILED}`.
     - `WORKFLOW_EXECUTION_STATUS_CANCELED`: Returns `QueryPairwiseResponse{Status: CANCELED}`.
     - `WORKFLOW_EXECUTION_STATUS_RUNNING`: Returns `QueryPairwiseResponse{Status: RUNNING}`.
  5. For database queries: `Service.GetJobHandler` ([`jobs.go:177`](../../../pinpoint/go/frontend/service/jobs.go#L177)) queries Spanner: `s.jobStore.GetJob(ctx, jobID)`.
- **Level 3: Supported Configurations Matrix**
  Supported by all jobs and instances.

---

#### Action 1.4: Legacy Catapult Job Bridge & Datastore Sync
- **Level 1: Simplistic View**
```
[Catapult UI requests GET /api/job/xxx or LegacyJobQuery]
                            |
                            v
[CatapultBisectWorkflow completes internal.BisectWorkflow]
                            |
                            v
[ConvertToCatapultResponseWorkflow: builds LegacyJobResponse]
                            |
                            v
[WriteBisectToCatapultActivity: POST /api/job -> Catapult Datastore]
```
- **Level 2: Detailed Call-Stack Flow**
  1. Catapult bisection jobs run via `CatapultBisectWorkflow` ([`catapult_bisect.go:140`](../../../pinpoint/go/workflows/catapult/catapult_bisect.go#L140)).
  2. After `internal.BisectWorkflow` completes, executes child workflow `ConvertToCatapultResponseWorkflow` ([`catapult_bisect.go:63`](../../../pinpoint/go/workflows/catapult/catapult_bisect.go#L63)).
  3. `parseArguments` maps `ScheduleBisectRequest` fields into `LegacyJobResponse_Argument` ([`parsers.go:52`](../../../pinpoint/go/workflows/catapult/parsers.go#L52)).
  4. `parseRawDataToLegacyObject` parses `Comparisons` and `RunData` into legacy `LegacyJobResponse_State` slices.
  5. `updateStatesWithComparisons` ([`catapult_bisect.go:28`](../../../pinpoint/go/workflows/catapult/catapult_bisect.go#L28)) computes `Prev` and `Next` verdict strings for each commit state.
  6. Workflow invokes activity `WriteBisectToCatapultActivity` ([`write.go:34`](../../../pinpoint/go/workflows/catapult/write.go#L34)).
  7. Formats URL: `https://pinpoint-dot-chromeperf.appspot.com/api/job/{job_id}` (or `staging-dot-chromeperf...`).
  8. Serializes JSON and sends HTTP `POST` using authorized OAuth2 token (`ScopeUserinfoEmail`).
- **Level 3: Supported Configurations Matrix**
  Supported by all legacy bisection jobs triggered from Chromeperf.

---

#### Action 1.5: Job Cancellation
- **Level 1: Simplistic View**
```
[User clicks "Cancel" in CancelJobDialogComponent]
                        |
                        v
[POST /pinpoint/v1/job/cancel or GET /pinpoint/v1/cancel]
                        |
                        v
[server.CancelJob -> c.CancelWorkflow(ctx, jobID)]
                        |
                        v
[Workflow catches ErrCanceled -> DisconnectedContext Cleanup]
                        |
                        v
[CleanupBuildActivity / CleanupBenchmarkRunActivity / UpdateJobStatus]
```
- **Level 2: Detailed Call-Stack Flow**
  1. User enters reason in `CancelJobDialogComponent` ([`cancel-job-dialog.component.ts:40`](../../../pinpoint/webui/app/cancel-job-dialog/cancel-job-dialog.component.ts#L40)).
  2. Dispatches `POST /pinpoint/v1/job/cancel` via `GatewayService.CancelJob`.
  3. Calls `server.CancelJob` ([`service_impl.go:88`](../../../pinpoint/go/service/service_impl.go#L88)).
  4. Connects to Temporal and invokes `c.CancelWorkflow(ctx, req.JobId, "")`.
  5. In `BuildWorkflow`, the deferred cleanup block detects `errors.Is(ctx.Err(), workflow.ErrCanceled)`.
  6. Creates disconnected context: `workflow.NewDisconnectedContext(ctx)`.
  7. Executes `bca.CleanupBuildActivity` ([`build_workflow.go:42`](../../../pinpoint/go/workflows/internal/build_workflow.go#L42)) to cancel running Buildbucket builds.
  8. In `RunBenchmarkWorkflow`, executes `rba.CleanupBenchmarkRunActivity` ([`run_benchmark.go:77`](../../../pinpoint/go/workflows/internal/run_benchmark.go#L77)) calling `sc.CancelTasks(ctx, []string{taskID})`.
  9. In `PairwiseWorkflow`, executes `UpdateJobStatus(jobID, "Canceled")` against Spanner.
- **Level 3: Supported Configurations Matrix**
  Supported by all active workflows.

---

### Category 2: TASK_DISPATCH_AND_EXECUTION

#### Action 2.1: Chrome Build Orchestration via Buildbucket
- **Level 1: Simplistic View**
```
[SingleCommitRunner / PairwiseCommitsRunner requires Build]
                             |
                             v
[bot_configs.GetIsolateTarget(bot, benchmark) -> Target name]
                             |
                             v
[ExecuteChildWorkflow: BuildWorkflow (BuildChrome)]
                             |
                             v
[SearchOrBuildActivity: Search for existing build with same commit/deps/patch]
                             |
            +----------------+----------------+
            | Found ?                         | Not Found
            v                                 v
[Reuse existing Build ID]           [StartBuild: POST to Buildbucket]
            |                                 |
            +----------------+----------------+
                             |
                             v
[WaitBuildCompletionActivity: Poll GetStatus every 15s (heartbeat)]
                             |
                             v (SUCCESS)
[RetrieveBuildArtifactActivity: Extract CAS Reference from swarm_hashes_refs]
```
- **Level 2: Detailed Call-Stack Flow**
  1. `buildChrome` ([`commits_runner.go:121`](../../../pinpoint/go/workflows/internal/commits_runner.go#L121)) invokes `bot_configs.GetIsolateTarget(bot, benchmark)`.
  2. Resolves isolate target from `isolate_targets.yaml` (e.g. `performance_test_suite_android_chrome_google_bundle` or `performance_test_suite`).
  3. Dispatches child workflow `workflows.BuildChrome` (`BuildWorkflow`, [`build_workflow.go:20`](../../../pinpoint/go/workflows/internal/build_workflow.go#L20)).
  4. Workflow invokes activity `bca.SearchOrBuildActivity` ([`build_workflow.go:85`](../../../pinpoint/go/workflows/internal/build_workflow.go#L85)).
  5. `buildClient.CreateFindBuildRequest` prepares search filters using `buildset` tag (`commit/gitiles/chromium.googlesource.com/chromium/src/+/{hash}`) and `deps_revision_overrides`.
  6. Calls `buildClient.FindBuild` ([`backends/buildbucket.go:178`](../../../pinpoint/go/backends/buildbucket.go#L178)) querying `SearchBuilds` RPC on Buildbucket.
  7. If build exists and is younger than `CasExpiration` (30 days), returns existing `buildID`.
  8. If not found, calls `buildClient.CreateStartBuildRequest` and `buildClient.StartBuild` ([`backends/buildbucket.go:487`](../../../pinpoint/go/backends/buildbucket.go#L487)), triggering `ScheduleBuild` RPC.
  9. Workflow executes `bca.WaitBuildCompletionActivity` ([`build_workflow.go:131`](../../../pinpoint/go/workflows/internal/build_workflow.go#L131)), polling `buildClient.GetStatus(buildID)` with heartbeats until `buildbucketpb.Status_SUCCESS`.
  10. Workflow executes `bca.RetrieveBuildArtifactActivity` ([`build_workflow.go:175`](../../../pinpoint/go/workflows/internal/build_workflow.go#L175)), reading `swarm_hashes_refs` output property from Buildbucket response.
  11. Returns `workflows.Build` containing `*apipb.CASReference{CasInstance, Digest{Hash, SizeBytes}}`.
- **Level 3: Supported Configurations Matrix**
  Supported across all Chromium try builders (`luci.chrome.try`, `luci.chromium.try`).

---

#### Action 2.2: Swarming Task Dispatch & Bot Allocation
- **Level 1: Simplistic View**
```
[SingleCommitRunner / PairwiseCommitsRunner launches runBenchmark]
                             |
                             v
[RunBenchmarkWorkflow (perf.run_benchmark)]
                             |
                             v
[ScheduleTaskActivity: run_benchmark.Run -> Swarming TriggerTask]
                             |
                             v
[WaitTaskPendingActivity: Poll task status until PENDING -> RUNNING]
                             |
                    (If NO_RESOURCE / Bot died)
                             v
[Retry loop (up to 3x): Clear bot dimension & reschedule across pool]
                             |
                             v
[WaitTaskFinishedActivity: Poll task until COMPLETED]
                             |
                             v
[RetrieveTestCASActivity: Fetch CasOutputRoot digest]
```
- **Level 2: Detailed Call-Stack Flow**
  1. `runBenchmark` ([`commits_runner.go:169`](../../../pinpoint/go/workflows/internal/commits_runner.go#L169)) constructs `RunBenchmarkParams` with `BuildCAS`, `BotConfig`, `Benchmark`, `Story`, and specific `Dimensions` (`key: "id", value: botId`).
  2. Dispatches child workflow `workflows.RunBenchmark` (`RunBenchmarkWorkflow`, [`run_benchmark.go:59`](../../../pinpoint/go/workflows/internal/run_benchmark.go#L59)).
  3. Workflow enters retry loop: `canRetry(state, attempt)` (up to `maxRetry = 3`).
  4. Executes `rba.ScheduleTaskActivity` ([`run_benchmark.go:293`](../../../pinpoint/go/workflows/internal/run_benchmark.go#L293)).
  5. `run_benchmark.Run` ([`run_benchmark/run_benchmark.go:64`](../../../pinpoint/go/run_benchmark/run_benchmark.go#L64)) constructs swarming `NewTaskRequest`:
     - Sets task command lines from `telemetry.go` or `benchmark_test_factory.go`.
     - Appends isolate input `CASReference`.
     - Sets dimensions: pool, os, device, id.
  6. Calls `sc.TriggerTask(ctx, req)` against `chrome-swarming.appspot.com:443`.
  7. Workflow executes `rba.WaitTaskPendingActivity` ([`run_benchmark.go:374`](../../../pinpoint/go/workflows/internal/run_benchmark.go#L374)), polling every 15s.
  8. If state returns `NO_RESOURCE` (e.g. target bot died), loop clears specific `Dimensions` bot ID to allow fallback to any bot in the pool, and retries scheduling.
  9. Executes `rba.WaitTaskFinishedActivity` ([`run_benchmark.go:420`](../../../pinpoint/go/workflows/internal/run_benchmark.go#L420)), polling every 30s.
  10. Executes `rba.RetrieveTestCASActivity` ([`run_benchmark.go:470`](../../../pinpoint/go/workflows/internal/run_benchmark.go#L470)), calling `sc.GetCASOutput(taskID)`.
  11. Returns `workflows.TestRun{TaskID, Status, CAS: CasOutputRoot}`.
- **Level 3: Supported Configurations Matrix**
  Supported by all bare-metal bot pools connected to `chrome-swarming.appspot.com`.

---

#### Action 2.3: CAS Result Fetching & Value Reading
- **Level 1: Simplistic View**
```
[Swarming Task Completes -> returns TestRun.CAS Digest]
                             |
                             v
[CollectValuesActivity / CollectAllValuesActivity]
                             |
                             v
[read_values.DialRBECAS(cas_instance)]
                             |
                             v
[Client.ReadValuesByChart / ReadValuesForAllCharts]
                             |
                             v
[Downloads histogram.json / test output files from RBE-CAS]
                             |
                             v
[Parses chart measurements, applies aggregation (mean), returns float64 array]
```
- **Level 2: Detailed Call-Stack Flow**
  1. In `SingleCommitRunner` ([`commits_runner.go:199`](../../../pinpoint/go/workflows/internal/commits_runner.go#L199)), after `runBenchmark` returns `TestRun`, calls activity `CollectValuesActivity` or `CollectAllValuesActivity`.
  2. `CollectValuesActivity` ([`commits_runner.go:280`](../../../pinpoint/go/workflows/internal/commits_runner.go#L280)) calls `read_values.DialRBECAS(ctx, run.CAS.CasInstance)`.
  3. `DialRBECAS` ([`read_values/read_values.go:43`](../../../pinpoint/go/read_values/read_values.go#L43)) establishes authenticated gRPC connection to `remotebuildexecution.googleapis.com:443`.
  4. Invokes `client.ReadValuesByChart` ([`read_values/read_values.go:68`](../../../pinpoint/go/read_values/read_values.go#L68)) passing CAS digest and target chart.
  5. CAS client locates and downloads test output files (such as `perf_results.json` or Telemetry histograms).
  6. Deserializes histogram samples and executes specified `aggMethod` (e.g. `mean`, `sum`, `min`, `max`, `std`, or raw unaggregated array).
  7. Returns `workflows.TestResults{Values, Units, Architecture, OSName}`.
  8. `SingleCommitRunner` populates `tr.Values` and `tr.Units`.
- **Level 3: Supported Configurations Matrix**
  Compatible with all Telemetry, Crossbench, and Google test benchmarks that produce RBE-CAS output digests.

---

#### Action 2.4: Pairwise Randomized Execution & Bot Locking
- **Level 1: Simplistic View**
```
[PairwiseCommitsRunnerWorkflow initiates pairs]
                        |
                        v
[FindAvailableBotsActivity: Fetch alive, non-quarantined bots]
                        |
                        v
[Deterministic Bot & Pair Permutation Shuffling via Seed]
                        |
                        v
[generatePairOrderIndices: [0, 1, 0, 1, ...]]
 (0 = LeftThenRight; 1 = RightThenLeft)
                        |
                        v
[RunBenchmarkPairwiseWorkflow: Dispatches paired tasks to SAME bot]
                        |
                        v
[Check Task Order & Contiguity: IsTaskPairOrdered & IsTaskPairContinuous]
                        |
                        v
[Remove Missing Data & Balance Pairs: Equalize LeftThenRight and RightThenLeft]
```
- **Level 2: Detailed Call-Stack Flow**
  1. `PairwiseCommitsRunnerWorkflow` ([`pairwise_runner.go:256`](../../../pinpoint/go/workflows/internal/pairwise_runner.go#L256)) executes activity `FindAvailableBotsActivity` ([`pairwise_runner.go:165`](../../../pinpoint/go/workflows/internal/pairwise_runner.go#L165)).
  2. Calls `sc.FetchFreeBots(botConfig)` and shuffles the bot IDs using `rand.New(rand.NewSource(seed))`.
  3. `generatePairOrderIndices(seed, iterations)` ([`pairwise_runner.go:199`](../../../pinpoint/go/workflows/internal/pairwise_runner.go#L199)) produces an equal-length sequence of `LeftThenRight` (0) and `RightThenLeft` (1) permutations.
  4. Concurrent builds for Left and Right commits are executed via `buildChrome`.
  5. Workflow loops over iterations, assigning each pair to a specific bot from `botIds`:
     `botDimension := map[string]string{"key": "id", "value": botIds[i % len(botIds)]}`.
  6. Dispatches `RunBenchmarkPairwiseWorkflow` ([`run_benchmark.go:142`](../../../pinpoint/go/workflows/internal/run_benchmark.go#L142)).
  7. First task is scheduled and waits for Swarming acceptance: `WaitTaskAcceptedActivity` ([`run_benchmark.go:324`](../../../pinpoint/go/workflows/internal/run_benchmark.go#L324)).
  8. Once accepted, second task is immediately scheduled on the **same bot**.
  9. Waits for both tasks to complete: `WaitTaskFinishedActivity`.
  10. Verifies pairing integrity via activities:
      - `IsTaskPairOrderedActivity`: Confirms task 1 started before task 2.
      - `IsTaskPairContinuousActivity`: Calls `sc.GetBotTasksBetweenTwoTasks` to verify no unrelated task ran on that bot in between.
  11. Back in `PairwiseCommitsRunnerWorkflow`, data balancing is enforced:
      - `removeMissingDataFromPairs(chart)` ([`pairwise_runner.go:131`](../../../pinpoint/go/workflows/internal/pairwise_runner.go#L131)): If either run in a pair failed, discards data for both.
      - `removeDataUntilBalanced(chart)` ([`pairwise_runner.go:141`](../../../pinpoint/go/workflows/internal/pairwise_runner.go#L141)): Drops excess pairs until the count of `LeftThenRight` equals `RightThenLeft`, neutralizing thermal throttling and background bot bias.
- **Level 3: Supported Configurations Matrix**
  Supported by all performance try-job bots.

---

### Category 3: STATISTICAL_ANALYSIS_AND_MIDPOINT

#### Action 3.1: Adaptive Performance Bisection Statistical Engine
- **Level 1: Simplistic View**
```
[BisectWorkflow compares Lower vs Higher Commit Runs]
                         |
                         v
[compareRuns -> compare.ComparePerformance(valuesA, valuesB, magnitude, dir)]
                         |
       +-----------------+-----------------+
       | |diff| < 0.1 * rawMagnitude ?     | Normal diff
       v (YES)                             v
[Verdict: Same (IsTooSmall: true)]   [Compute IQR & Normalized Magnitude]
                                           |
                                           v
                                     [HighThresholdPerformance lookup]
                                           |
                                           v
                             [PValueKS = KolmogorovSmirnov(A, B)]
                             [PValueMWU = MannWhitneyU(A, B)]
                             [PValue = min(PValueKS, PValueMWU)]
                                           |
                   +-----------------------+-----------------------+
                   | PValue <= 0.01        | PValue <= HighThresh  | PValue > HighThresh
                   v                       v                       v
          [Verdict: Different]     [Verdict: Unknown]      [Verdict: Same]
```
- **Level 2: Detailed Call-Stack Flow**
  1. `BisectWorkflow` ([`bisect.go:256`](../../../pinpoint/go/workflows/internal/bisect.go#L256)) calls `compareRuns(ctx, lower, higher, chart, magnitude, improvementDir)`.
  2. Gathers sample arrays: `valuesA = lower.AllValues(chart)` and `valuesB = higher.AllValues(chart)`.
  3. If all runs failed (`len == 0`), falls back to functional comparison via `compareErrorRuns` / `CompareFunctional` ([`compare.go:251`](../../../pinpoint/go/compare/compare.go#L251)).
  4. Otherwise, calls `compare.ComparePerformance` ([`compare.go:281`](../../../pinpoint/go/compare/compare.go#L281)):
     - Checks `isSmallDiff(mean(B) - mean(A), rawMagnitude)` ([`compare.go:125`](../../../pinpoint/go/compare/compare.go#L125)). If `< 0.1 * magnitude`, returns `Verdict: Same, IsTooSmall: true` to halt bisection on sub-threshold regressions.
     - Sorts concatenated samples to calculate Interquartile Range (`iqr = Q3 - Q1`).
     - Calculates `normalizedMagnitude = math.Abs(rawMagnitude / iqr)`.
     - Calls `thresholds.HighThresholdPerformance(normalizedMagnitude, avgSampleSize)` ([`thresholds/thresholds.go:28`](../../../pinpoint/go/compare/thresholds/thresholds.go#L28)) to get adaptive upper significance boundary.
     - Calls internal `compare(valuesA, valuesB, lowThreshold=0.01, highThreshold, direction)` ([`compare.go:323`](../../../pinpoint/go/compare/compare.go#L323)).
     - Executes `KolmogorovSmirnov(valuesA, valuesB)` ([`kolmogorov_smirnov.go:16`](../../../pinpoint/go/compare/kolmogorov_smirnov.go#L16)).
     - Executes `MannWhitneyU(valuesA, valuesB)` ([`mann_whitney_u.go:38`](../../../pinpoint/go/compare/mann_whitney_u.go#L38)).
     - Selects `PValue = min(PValueKS, PValueMWU)`:
       - `PValue <= 0.01`: `Verdict = Different` (Reject null hypothesis).
       - `PValue <= HighThreshold`: `Verdict = Unknown` (Ambiguous, schedule additional runs).
       - `PValue > HighThreshold`: `Verdict = Same` (Accept null hypothesis, distributions match).
- **Level 3: Supported Configurations Matrix**
  Used across all Pinpoint performance bisections.

---

#### Action 3.2: Pairwise Wilcoxon Signed-Rank Test
- **Level 1: Simplistic View**
```
[PairwiseWorkflow finishes paired benchmark runs]
                        |
                        v
[comparePairwiseRuns -> compare.ComparePairwise(valuesA, valuesB, dir)]
                        |
     +------------------+------------------+
     | valuesA == valuesB ?                | Distinct values
     v (YES)                               v
[Return Verdict: Same, Estimate: 0]   [handlePairwiseEdgeCase: nudge identical values]
                                           |
                                           v
                             [Select Transform: LogTransform vs NormalizeResult]
                                           |
                                           v
                             [stats.PairwiseWilcoxonSignedRankedTest(B, A)]
                                           |
                                           v
                             [Computes: PValue, Estimate, LowerCi, UpperCi]
                                           |
                    +----------------------+----------------------+
                    | PValue < 0.05 AND LowerCi * UpperCi > 0     | Otherwise
                    v                                             v
           [Verdict: Different (Significant)]             [Verdict: Same]
```
- **Level 2: Detailed Call-Stack Flow**
  1. `PairwiseWorkflow` ([`pairwise.go:177`](../../../pinpoint/go/workflows/internal/pairwise.go#L177)) calls `comparePairwiseRuns(ctx, pr, dir)`.
  2. For each common chart in `pr.GetCommonCharts()`, calls `compare.ComparePairwise(valuesA, valuesB, dir)` ([`compare.go:141`](../../../pinpoint/go/compare/compare.go#L141)).
  3. If all pairs are identical (`slices.Equal`), returns `Verdict: Same, PValue: 1.0, Estimate: 0.0`.
  4. Calls `handlePairwiseEdgeCase` ([`compare.go:380`](../../../pinpoint/go/compare/compare.go#L380)): If all values in A are identical and all values in B are identical (e.g. A=[3,3,3], B=[4,4,4]), nudges `valuesB[0] += valuesB[0] * 1e-9` to prevent NaN confidence intervals in the zero-in root solver.
  5. Determines transformation: If any value is `<= 1e-6`, uses `stats.NormalizeResult`; otherwise `stats.LogTransform`.
  6. Calls `stats.PairwiseWilcoxonSignedRankedTest(valuesB, valuesA, stats.TwoSided, transform)` ([`stats/wilcoxon_signed_rank.go:94`](../../../pinpoint/go/compare/stats/wilcoxon_signed_rank.go#L94)):
     - Computes pair differences: `diffs[i] = y[i] - x[i]`.
     - Calculates Walsh averages: `(diffs[i] + diffs[j]) / 2`.
     - Executes Hodges-Lehmann location estimator (`Estimate`).
     - Computes exact p-value via `ExactSignRankTest` or normal approximation for large sample sizes.
     - Computes 95% confidence intervals (`LowerCi`, `UpperCi`) using Brent's method root-finding algorithm in `zeroin.go`.
  7. Sanitizes floats (`sanitizeFloat` removes `NaN` / `Inf`).
  8. If `PValue < 0.05` and confidence intervals don't cross zero (`LowerCi * UpperCi > 0`), returns `Verdict: Different` (`Significant: true`).
- **Level 3: Supported Configurations Matrix**
  Supported by all Pairwise A/B Try Jobs and Culprit Finder verification phases.

---

#### Action 3.3: Midpoint Commit Resolution & DEPS Roll Unraveling
- **Level 1: Simplistic View**
```
[BisectWorkflow detects Verdict: Different between Start & End]
                             |
                             v
[FindMidCommitActivity: MidpointHandler.FindMidCombinedCommit]
                             |
        +--------------------+--------------------+
        | Main commits not adjacent               | Main commits ADJACENT
        v                                         v
[Gitiles LogFirstParent(start, end)]     [Assume DEPS Roll: fetchGitDeps(start, end)]
        |                                         |
        v                                         v
[Compute median index commit in Chromium] [Identify modified repository (e.g. V8)]
        |                                         |
        v                                         v
[Return CombinedCommit{Main: mid}]        [Gitiles LogFirstParent on V8 depot]
                                                  |
                                                  v
                                          [Return CombinedCommit{Main: start,
                                            ModifiedDeps: [V8@mid]}]
```
- **Level 2: Detailed Call-Stack Flow**
  1. In `BisectWorkflow` ([`bisect.go:308`](../../../pinpoint/go/workflows/internal/bisect.go#L308)), when a pair is `Different`, invokes activity `FindMidCommitActivity` calling `MidpointHandler.FindMidCombinedCommit(ctx, lower, higher)` ([`midpoint.go:355`](../../../pinpoint/go/midpoint/midpoint.go#L355)).
  2. If `startCommit.Key() == endCommit.Key()`, errors out (identical commits).
  3. **Case A: No Modified DEPS (Main repository bisection)**:
     - `m.findMidCommit(ctx, start.Main, end.Main)` calls `m.findMidpoint` ([`midpoint.go:69`](../../../pinpoint/go/midpoint/midpoint.go#L69)).
     - Retrieves `gitiles.GitilesRepo` client via `m.getOrCreateRepo(url)`.
     - Calls `gc.LogFirstParent(ctx, startGitHash, endGitHash)` ([`midpoint.go:84`](../../../pinpoint/go/midpoint/midpoint.go#L84)) returning commit slice in reverse chronological order.
     - If `len(lc) > 1`: Commits are not adjacent. Takes `lc[len(lc)/2]`, returning `common.NewCombinedCommit(midCommit)`.
  4. **Case B: Commits are Adjacent (DEPS Roll Detection)**:
     - If `len(lc) == 1 && lc[0].Hash == endGitHash`: Commits are adjacent.
     - Calls `m.findMidCommitInDEPS(ctx, startCommit, endCommit)` ([`midpoint.go:156`](../../../pinpoint/go/midpoint/midpoint.go#L156)).
     - Calls `m.fetchGitDeps` ([`midpoint.go:116`](../../../pinpoint/go/midpoint/midpoint.go#L116)) for each commit:
       - Calls `gc.ReadFileAtRef(ctx, "DEPS", commit.GitHash)`.
       - Calls `deps_parser.ParseDeps(content)` ([`midpoint.go:132`](../../../pinpoint/go/midpoint/midpoint.go#L132)).
     - Compares dependency maps to identify which dependency repository URL has mismatched git hashes (e.g. `https://chromium.googlesource.com/v8/v8.git`).
     - Queries Gitiles on the dependency repository between the rolled git hashes.
     - Returns `CombinedCommit{Main: startCommit.Main, ModifiedDeps: []*pb.Commit{v8MidCommit}}`.
  5. **Case C: Multi-tier Nested DEPS (ModifiedDeps already present)**:
     - `m.fillModifiedDeps` ([`midpoint.go:250`](../../../pinpoint/go/midpoint/midpoint.go#L250)) synchronizes dependency depths across both arms before calculating midpoint.
  6. Back in `BisectWorkflow`, checks `CheckCombinedCommitEqualActivity(lower, mid)`:
     - If `equal`: Bisection has terminated at adjacent commits. Appends `mid` to `be.Culprits` and `be.DetailedCulprits{Prior: lower, Culprit: mid}` ([`bisect.go:323`](../../../pinpoint/go/workflows/internal/bisect.go#L323)).
- **Level 3: Supported Configurations Matrix**
  Supports any Git-based repository configured in `DEPS` (e.g. V8, WebRTC, Skia, Dawn, ANGLE). CIPD dependencies are ignored.

---

#### Action 3.4: Functional Bisection / Flakiness Analysis
- **Level 1: Simplistic View**
```
[All benchmark runs fail or produce errors on a commit]
                          |
                          v
[compareRuns detects len(valuesA) == 0 || len(valuesB) == 0]
                          |
                          v
[Pivots to compareErrorRuns -> Binary failure values (0.0 vs 1.0)]
                          |
                          v
[compare.CompareFunctional(valuesA, valuesB, expectedErrRate=1.0)]
                          |
                          v
[HighThresholdFunctional lookup -> Evaluates KS & MWU tests on failure rates]
```
- **Level 2: Detailed Call-Stack Flow**
  1. In `compareRuns` ([`workflows/internal/compare.go:37`](../../../pinpoint/go/workflows/internal/compare.go#L37)), when `len(valuesA) == 0 || len(valuesB) == 0`, checks if error runs exist.
  2. Calls `lower.AllErrorValues(chart)` and `higher.AllErrorValues(chart)` ([`commits_runner.go:91`](../../../pinpoint/go/workflows/internal/commits_runner.go#L91)), generating binary arrays where `1.0` indicates task failure or missing CAS and `0.0` indicates success.
  3. Calls `compare.CompareFunctional(valuesA, valuesB, expectedErrRate=1.0)` ([`compare.go:251`](../../../pinpoint/go/compare/compare.go#L251)).
  4. Queries `thresholds.HighThresholdFunctional(expectedErrRate, avgSampleSize)` ([`thresholds/thresholds.go:37`](../../../pinpoint/go/compare/thresholds/thresholds.go#L37)).
  5. Evaluates KS and MWU tests using `compare(valuesA, valuesB, lowThreshold=0.01, highThreshold, Down)`.
  6. If failure rate is statistically higher on commit B, flags commit B as culprit introducing flakiness or build crashes.
- **Level 3: Supported Configurations Matrix**
  Active across all performance bisection jobs as an automated fallback.

---

### Category 4: WORKFLOW_ORCHESTRATED (TEMPORAL)

#### Action 4.1: Adaptive Bisection Workflow (`BisectWorkflow`)
- **Level 1: Simplistic View**
```
[Start BisectWorkflow (perf.bisect)]
                  |
                  v
[Initialize tracker & schedule initial pair: Lower (C_start) vs Higher (C_end)]
                  |
                  v
[SingleCommitRunner executes 10 iterations each]
                  |
                  v
[comparisons.Send(CommitRangeTracker{Lower, Higher})]
                  |
                  v
+=================+=========================================================+
| Selector Loop: while pendings > 0                                         |
|                                                                           |
| 1. comparisons channel receives (Lower, Higher)                           |
| 2. compareRuns evaluates PValue:                                          |
|    - If SAME: Sub-tree discarded, branches terminate.                     |
|    - If UNKNOWN:                                                          |
|        if runs < 160:                                                     |
|          Adaptive ladder: 10 -> 20 -> 40 -> 80 -> 160 iterations          |
|          schedulePairRuns schedules next iteration chunk                  |
|          re-queues (Lower, Higher) comparison                             |
|    - If DIFFERENT:                                                        |
|        mid = FindMidCommitActivity(Lower, Higher)                         |
|        if mid == Lower (adjacent):                                        |
|          Higher is Culprit! Append to be.Culprits                         |
|        else:                                                              |
|          midRun = tracker.newRun(mid)                                     |
|          Schedule runs for midRun                                         |
|          Enqueues TWO new comparisons: (Lower, Mid) AND (Mid, Higher)     |
+===========================================================================+
                  |
                  v
[Returns BisectExecution with all identified culprits and run histories]
```
- **Level 2: Detailed Call-Stack Flow**
  1. `BisectWorkflow` ([`bisect.go:116`](../../../pinpoint/go/workflows/internal/bisect.go#L116)) is launched on `perf-internal` Temporal namespace.
  2. Activity `FindAvailableBotsActivity` populates `p.BotIds`.
  3. `schedulePairRuns` initiates `minSampleSize = 10` iterations for `lower` and `higher` commits.
  4. Waits for initial runs: `lower.updateRuns` and `higher.updateRuns`.
  5. Seeds `comparisons` channel (`workflow.NewBufferedChannel(ctx, 100)`).
  6. Non-blocking `selector` event loop runs while `pendings > 0`:
     - Reads `CommitRangeTracker` from channel.
     - Calls `compareRuns(ctx, lower, higher, chart, magnitude, improvementDir)`.
     - Appends `compareResult` to `be.Comparisons`.
     - **Case Verdict == Unknown**:
       - Checks `if len(lower.Runs) >= 160`: Halts further runs to prevent infinite loops.
       - Otherwise, calls `nextRunSize(lower, higher, minSampleSize)` ([`bisect_run.go:156`](../../../pinpoint/go/workflows/internal/bisect_run.go#L156)), advancing along `benchmarkRunIterations = [10, 20, 40, 80, 160]`.
       - Calls `schedulePairRuns` to trigger additional child workflows via `SingleCommitRunner`.
       - When child futures complete, re-sends `cr` to `comparisons` channel.
     - **Case Verdict == Different**:
       - Calls `FindMidCommitActivity`.
       - Calls `CheckCombinedCommitEqualActivity`.
       - If equal: Appends `higher` to `be.Culprits` and records `DetailedCulprits`.
       - If not equal: Allocates new run index `midRunIdx, midRun := tracker.newRun(mid)`.
       - Calls `midRun.scheduleRuns` for `expectedSize`.
       - When `midRun` future finishes, forks bisection by sending two ranges: `cr.CloneWithHigher(midRunIdx)` and `cr.CloneWithLower(midRunIdx)` ([`bisect.go:354`](../../../pinpoint/go/workflows/internal/bisect.go#L354)).
  7. When `pendings == 0`, copies `tracker.runs` into `be.RunData` and returns `be`.
- **Level 3: Supported Configurations Matrix**
  Supported by all performance bisection configurations.

---

#### Action 4.2: Pairwise A/B Try Job Workflow (`PairwiseWorkflow`)
- **Level 1: Simplistic View**
```
[PairwiseWorkflow (perf.pairwise) starts]
                    |
                    v
[AddInitialJob: records job in Spanner]
                    |
                    v
[ExecuteChildWorkflow: PairwiseCommitsRunnerWorkflow]
                    |
                    v
[AddCommitRuns: records Left & Right CAS / Task runs in Spanner]
                    |
                    v
[comparePairwiseRuns: Computes Wilcoxon Signed-Rank Test]
                    |
                    v
[AddResults & UpdateJobStatus: Writes p-values & medians to Spanner]
```
- **Level 2: Detailed Call-Stack Flow**
  1. `PairwiseWorkflow` ([`pairwise.go:27`](../../../pinpoint/go/workflows/internal/pairwise.go#L27)) receives `PairwiseParams`.
  2. Converts CAS references via `convertCas`.
  3. Executes activity `AddInitialJob` ([`pairwise.go:61`](../../../pinpoint/go/workflows/internal/pairwise.go#L61)) writing to Spanner `Jobs` table.
  4. Parses extra arguments for control and experiment via `splitExtraArgs`.
  5. Executes activity `UpdateJobStatus(jobID, "Running", 0)`.
  6. Dispatches child workflow `workflows.PairwiseCommitsRunner` (`PairwiseCommitsRunnerWorkflow`, [`pairwise.go:158`](../../../pinpoint/go/workflows/internal/pairwise.go#L158)).
  7. When completed, executes activity `AddCommitRuns` ([`pairwise.go:172`](../../../pinpoint/go/workflows/internal/pairwise.go#L172)), serializing commit metadata and CAS digests.
  8. Calls `comparePairwiseRuns(ctx, pr, compare.UnknownDir)`.
  9. Converts Wilcoxon results into proto format: `protoResults[chart]`.
  10. Deferred completion handler executes activity `AddResults` ([`pairwise.go:146`](../../../pinpoint/go/workflows/internal/pairwise.go#L146)) and `UpdateJobStatus(jobID, "Completed", duration)`.
- **Level 3: Supported Configurations Matrix**
  Supported by all try-job bots.

---

#### Action 4.3: Sandwich Verification / Culprit Finder Workflow (`CulpritFinderWorkflow`)
- **Level 1: Simplistic View**
```
[CulpritFinderWorkflow (perf.culprit_finder) starts]
                         |
                         v
[Phase 1: Regression Verification via PairwiseWorkflow (Start vs End, 30 runs)]
                         |
      +------------------+------------------+
      | Significant regression ?            | Not significant
      v (YES)                               v
[Compute observed magnitude:        [Return CulpritFinderExecution{
  TreatmentMedian - ControlMedian]    RegressionVerified: false}]
         |
         v
[Phase 2: Bisection via CatapultBisectWorkflow (InitialAttempts: 20)]
         |
      +--+------------------+
      | Culprits found ?    | No culprits
      v (YES)               v
[Phase 3: Culprit      [Return RegressionVerified: true, Culprits: nil]
 Verification (Pairwise
 of Prior vs Culprit)]
         |
         v
[Phase 4: InvokeCulpritProcessingWorkflow -> perf.process_culprit on perf.grouping]
```
- **Level 2: Detailed Call-Stack Flow**
  1. `CulpritFinderWorkflow` ([`catapult/culprit_finder.go:23`](../../../pinpoint/go/workflows/catapult/culprit_finder.go#L23)) receives `CulpritFinderParams`.
  2. **Phase 1: Regression Verification**:
     - Executes child workflow `workflows.PairwiseWorkflow` with 30 iterations between `StartGitHash` and `EndGitHash`.
     - Inspects `pe.Results[chart].Significant`: If not significant, returns `RegressionVerified: false` immediately, avoiding expensive bisection on phantom anomalies.
  3. **Phase 2: Bisection**:
     - Computes `magnitude = fmt.Sprintf("%f", res.TreatmentMedian - res.ControlMedian)`.
     - Executes child workflow `workflows.CatapultBisect` with `ComparisonMagnitude: magnitude` and `InitialAttemptCount: 20`.
     - If `len(be.Culprits) == 0`, returns `RegressionVerified: true, Culprits: nil`.
  4. **Phase 3: Culprit Verification**:
     - Calls `verifyCulprits(ctx, be, cfp)` ([`catapult/culprit_finder.go:93`](../../../pinpoint/go/workflows/catapult/culprit_finder.go#L93)).
     - For each culprit in `be.DetailedCulprits`, launches parallel child `workflows.PairwiseWorkflow` comparing `Prior` commit vs `Culprit` commit with `CulpritVerify: true`.
     - Only culprits verified to have a statistically significant difference are retained.
  5. **Phase 4: Culprit Processing Callback**:
     - Calls `InvokeCulpritProcessingWorkflow(ctx, cfp, verifiedCulprits)` ([`catapult/culprit_finder.go:109`](../../../pinpoint/go/workflows/catapult/culprit_finder.go#L109)).
     - Dispatches child workflow `perf_wf.ProcessCulprit` to task queue `cfp.CallbackParams.TemporalTaskQueueName` with `PARENT_CLOSE_POLICY_ABANDON`.
- **Level 3: Supported Configurations Matrix**
  Supported across Chromium automated regression pipelines (e.g. `chrome-internal`, `chrome-public`).

---

#### Action 4.4: Cross-Browser Benchmark Runner (`CbbRunnerWorkflow`)
- **Level 1: Simplistic View**
```
[CbbRunnerWorkflow (perf.cbb_runner) starts]
                        |
                        v
[validateParameters: browser (chrome, edge, safari) & channel (stable, dev, tp)]
                        |
                        v
[Setup benchmarks: speedometer3, jetstream2, motionmark, etc.]
                        |
                        v
[Loops benchmarks -> calls SingleCommitRunner on bot (e.g. mac-m3-pro-perf-cbb)]
                        |
                        v
[CollectValuesActivity / read_values: parses results]
                        |
                        v
[format.Format -> GCS Upload Activity -> gs://{bucket}/cbb/... json format]
```
- **Level 2: Detailed Call-Stack Flow**
  1. `CbbRunnerWorkflow` ([`cbb_runner.go:132`](../../../pinpoint/go/workflows/internal/cbb_runner.go#L132)) receives `CbbRunnerParams`.
  2. `validateParameters` checks browser support (`chrome`, `edge`, `safari`) and release channels (`stable`, `dev`, `technology-preview`).
  3. `setupBenchmarks` ([`cbb_runner.go:99`](../../../pinpoint/go/workflows/internal/cbb_runner.go#L99)) loads default benchmark suite if nil (e.g. `speedometer3`, `jetstream2`, `motionmark1.3`).
  4. For desktop Chrome, handles `--disable-field-trial-config` when `SkipFinch: true`.
  5. Iterates through configured benchmarks, invoking `SingleCommitRunner` across target hardware bots.
  6. Collects sampled values from RBE-CAS.
  7. Formats telemetry data into `perfresults` and Skia Perf format (`format.Format`).
  8. Executes `UploadToGCSActivity` writing results into Cloud Storage bucket for ingestion by Skia Perf's `perfserver ingest`.
- **Level 3: Supported Configurations Matrix**
  Supported by CBB dedicated bots (e.g. `mac-m3-pro-perf-cbb`, `android-pixel10-perf-cbb`).

---

#### Action 4.5: Catapult Legacy Bisect Adapter (`CatapultBisectWorkflow`)
- **Level 1: Simplistic View**
```
[CatapultBisectWorkflow (perf.catapult.bisect) starts]
                         |
                         v
[Dispatches child internal.BisectWorkflow with specified JobID]
                         |
                         v
[ExecuteChildWorkflow: ConvertToCatapultResponseWorkflow]
                         |
                         v
[ExecuteActivity: WriteBisectToCatapultActivity -> Catapult Datastore API]
                         |
                         v
[Returns BisectExecution to Catapult Chromeperf callers]
```
- **Level 2: Detailed Call-Stack Flow**
  1. `CatapultBisectWorkflow` ([`catapult_bisect.go:140`](../../../pinpoint/go/workflows/catapult/catapult_bisect.go#L140)) receives `BisectParams`.
  2. Ensures deterministic `workflowID = p.JobID` or generates UUID via `workflow.SideEffect`.
  3. Executes child workflow `internal.BisectWorkflow` ([`catapult_bisect.go:164`](../../../pinpoint/go/workflows/catapult/catapult_bisect.go#L164)).
  4. Passes completed `BisectExecution` into `ConvertToCatapultResponseWorkflow`.
  5. Executes activity `WriteBisectToCatapultActivity` ([`catapult_bisect.go:178`](../../../pinpoint/go/workflows/catapult/catapult_bisect.go#L178)):
     - Sends HTTP POST payload to `https://pinpoint-dot-chromeperf.appspot.com/api/job/{job_id}`.
     - On non-production environments (`!p.Production`), ignores write errors to allow smooth local testing.
  6. Returns `&pb.BisectExecution{JobId, Culprits, DetailedCulprits}`.
- **Level 3: Supported Configurations Matrix**
  Active for all Catapult/Chromeperf backward-compatibility endpoints.

---

### Category 5: RESULT_REPORTING_AND_INTEGRATION

#### Action 5.1: Culprit Reporting to Perf Spanner Autobisections
- **Level 1: Simplistic View**
```
[Pinpoint Bisection finishes -> returns verified culprit commits]
                              |
                              v
[Perf MaybeTriggerBisectionWorkflow: processBisectJobResults]
                              |
                              v
[AutobisectionService.SaveAutobisection -> Spanner Autobisections table]
                              |
                              v
[Stores: JobID, AnomalyGroupID, Culprit Git Hash, RegressionStatus]
```
- **Level 2: Detailed Call-Stack Flow**
  1. In Perf's `MaybeTriggerBisectionWorkflow` ([`perf/go/workflows/internal/maybe_trigger_bisection.go:514`](../../go/workflows/internal/maybe_trigger_bisection.go#L514)), after `waitPinpointJobCompletion` returns, calls `processBisectJobResults`.
  2. Extracts culprit hashes from `culpritCommits`: `culprits[i] = c.GitHash`.
  3. Constructs `b_pb.SaveAutobisectionRequest`:
     - `JobId: jobState.JobID`
     - `WorkflowId: workflow.GetInfo(ctx).WorkflowExecution.ID`
     - `AnomalyGroupId: anomalyGroupId`
     - `AnomalyId: anomaly.Id`
     - `RegressionStatus: extractRegressionStatus(jobState)`
  4. Executes activity `bsaToken().SaveAutobisection` calling Autobisection service URL.
  5. Persists record into Spanner database table `Autobisections`.
- **Level 3: Supported Configurations Matrix**
  Active on all Perf instances with `pinpoint_task_queue` and `grouping_task_queue` configured (e.g. `chrome-internal`, `chrome-public-autopush`).

---

#### Action 5.2: Perf Process Culprit Callback
- **Level 1: Simplistic View**
```
[CulpritFinderWorkflow identifies verified culprits]
                         |
                         v
[InvokeCulpritProcessingWorkflow]
                         |
                         v
[Dispatches perf.process_culprit on TaskQueue: perf.grouping]
                         |
                         v
[ProcessCulpritWorkflow in Perf executes:]
  1. CulpritServiceActivity.PersistCulprit (Spanner Culprits table)
  2. IssueTrackerServiceActivity.AddComment (Notifies sheriff on Buganizer)
```
- **Level 2: Detailed Call-Stack Flow**
  1. In `CulpritFinderWorkflow` ([`catapult/culprit_finder.go:99`](../../../pinpoint/go/workflows/catapult/culprit_finder.go#L99)), calls `InvokeCulpritProcessingWorkflow(ctx, cfp, verifiedCulprits)`.
  2. Iterates over `verified_combined_culprits`, calling `findLastDepCommit` ([`catapult/culprit_finder.go:145`](../../../pinpoint/go/workflows/catapult/culprit_finder.go#L145)) to extract the specific leaf culprit commit (either from `ModifiedDeps` or `Main`).
  3. Constructs `workflow.ChildWorkflowOptions`:
     - `TaskQueue: cfp.CallbackParams.TemporalTaskQueueName` (Perf grouping queue).
     - `ParentClosePolicy: enums.PARENT_CLOSE_POLICY_ABANDON`.
  4. Calls `workflow.ExecuteChildWorkflow(c_ctx, perf_wf.ProcessCulprit, perf_wf.ProcessCulpritParam{CulpritServiceUrl, Commits, AnomalyGroupId})` ([`catapult/culprit_finder.go:134`](../../../pinpoint/go/workflows/catapult/culprit_finder.go#L134)).
  5. Perf worker picks up task on `perf.grouping` queue:
     - Persists culprit into Spanner `Culprits` table.
     - Adds comment and updates status on linked Google IssueTracker bug.
- **Level 3: Supported Configurations Matrix**
  Supported whenever `CulpritProcessingCallbackParams` are passed from Perf.

---

#### Action 5.3: Legacy Catapult Datastore Writeback
- **Level 1: Simplistic View**
```
[CatapultBisectWorkflow finishes ConvertToCatapultResponseWorkflow]
                                  |
                                  v
[WriteBisectToCatapultActivity(resp, isProduction)]
                                  |
                                  v
[Dials Authorized HTTP Client (ScopeUserinfoEmail)]
                                  |
                                  v
[POST https://pinpoint-dot-chromeperf.appspot.com/api/job/{job_id}]
                                  |
                                  v
[Catapult Datastore saves Job entity with full state & comparisons]
```
- **Level 2: Detailed Call-Stack Flow**
  1. `CatapultBisectWorkflow` ([`catapult_bisect.go:178`](../../../pinpoint/go/workflows/catapult/catapult_bisect.go#L178)) executes `WriteBisectToCatapultActivity`.
  2. Activity function ([`catapult/write.go:34`](../../../pinpoint/go/workflows/catapult/write.go#L34)) formats target endpoint based on `isProduction` flag:
     - Prod: `https://pinpoint-dot-chromeperf.appspot.com/api/job/%s`
     - Staging: `https://staging-dot-pinpoint-dot-chromeperf.appspot.com/api/job/%s`
  3. Obtains OAuth2 token source via `google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)`.
  4. Marshals `LegacyJobResponse` into JSON payload.
  5. Sends HTTP `POST` request.
  6. Catapult backend receives payload and deserializes into its App Engine Datastore `Job` model, making results viewable on `pinpoint-dot-chromeperf.appspot.com`.
- **Level 3: Supported Configurations Matrix**
  Supported by all Catapult bisection jobs.

---

#### Action 5.4: Google IssueTracker (Buganizer) Commenting & Culprit Tagging
- **Level 1: Simplistic View**
```
[Culprit identified and bug_id is present]
                    |
                    v
[BugUpdateWorkflow (perf.bug_update)]
                    |
                    v
[IssueTrackerActivity.ReportCulpritActivity(bug_id, culprits)]
                    |
                    v
[issueTrackerTransport.fillTemplate: renders culprit_detected.tmpl]
                    |
                    v
[Google IssueTracker API: ModifyIssue (adds markdown comment with Gitiles links)]
```
- **Level 2: Detailed Call-Stack Flow**
  1. When a bisection or culprit verification finishes with a non-empty `bug_id`, `BugUpdateWorkflow` ([`bug_update.go:21`](../../../pinpoint/go/workflows/internal/bug_update.go#L21)) is executed.
  2. Executes activity `ita.ReportCulpritActivity(bug_id, culprits)` ([`bug_update.go:48`](../../../pinpoint/go/workflows/internal/bug_update.go#L48)).
  3. Instantiates `backends.NewIssueTrackerTransport(ctx)` ([`backends/issuetracker.go:62`](../../../pinpoint/go/backends/issuetracker.go#L62)):
     - Fetches API key from GCP Secret Manager in project `skia-infra-public` secret `perf-issue-tracker-apikey`.
     - Initializes authorized client using `https://www.googleapis.com/auth/buganizer`.
  4. Calls `fillTemplate(culprits)` parsing embedded template `culprit_detected.tmpl` ([`backends/culprit_detected.tmpl`](../../../pinpoint/go/backends/culprit_detected.tmpl)):
     - Renders commit links (`{repository}/+/{githash}`) for both base Chromium and modified DEPS.
  5. Calls `client.Issues.Modify(issueID, &issuetracker.IssueComment{Comment: comment}).Do()`.
- **Level 3: Supported Configurations Matrix**
  Active on all jobs configured with a valid Buganizer issue ID.

---

#### Action 5.5: Pinpoint Spanner JobsStore Lifecycle Persistence
- **Level 1: Simplistic View**
```
[PairwiseWorkflow Lifecycle Milestones]
                    |
                    +---> 1. AddInitialJob: row created (status: 'Pending', config, user)
                    |
                    +---> 2. UpdateJobStatus: status set to 'Running'
                    |
                    +---> 3. AddCommitRuns: Left & Right CAS / Swarming TestRun JSON written
                    |
                    +---> 4. AddResults: Wilcoxon statistics (p-values, medians, CI) written
                    |
                    +---> 5. UpdateJobStatus: status set to 'Completed' (or 'Failed' / 'Canceled')
```
- **Level 2: Detailed Call-Stack Flow**
  1. `JobStoreActivities` ([`database_activities.go:23`](../../../pinpoint/go/workflows/internal/database_activities.go#L23)) wraps Spanner `JobStore` methods.
  2. **Job Creation**: `AddInitialJob` ([`jobs_store.go:98`](../../../pinpoint/go/sql/jobs_store/jobs_store.go#L98)) executes:
     `INSERT INTO Jobs (job_id, name, user, bot_name, benchmark, story, ... status) VALUES (@job_id, ..., 'Pending')`.
  3. **Status Updates**: `UpdateJobStatus` ([`jobs_store.go:189`](../../../pinpoint/go/sql/jobs_store/jobs_store.go#L189)) executes:
     `UPDATE Jobs SET status = @status, duration = @duration, last_updated = CURRENT_TIMESTAMP() WHERE job_id = @job_id`.
  4. **Build & Test Output Persistence**: `AddCommitRuns` ([`jobs_store.go:275`](../../../pinpoint/go/sql/jobs_store/jobs_store.go#L275)) JSON-encodes `left` and `right` `CommitRunData` (build IDs, isolate CAS digests, and swarming task IDs) into the `commit_runs` column.
  5. **Statistical Results Persistence**: `AddResults` ([`jobs_store.go:217`](../../../pinpoint/go/sql/jobs_store/jobs_store.go#L217)) serializes the Wilcoxon result map into the `comparison_results` JSON column.
  6. **Error Logging**: On workflow error, `SetErrors` ([`jobs_store.go:250`](../../../pinpoint/go/sql/jobs_store/jobs_store.go#L250)) records error strings into the `errors` column.
- **Level 3: Supported Configurations Matrix**
  Supported by all Go native Pinpoint jobs managed by `perfserver`.
