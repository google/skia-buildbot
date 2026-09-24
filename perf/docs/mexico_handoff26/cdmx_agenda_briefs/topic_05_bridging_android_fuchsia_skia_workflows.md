# Topic 5: Bridging Siloed Workflows Across Android, Fuchsia, and Skia Partners

## Executive Summary
Skia Perf and Pinpoint serve diverse partner teams across Google, each with distinct repository layouts, artifact build pipelines, and regression triage expectations. This brief examines how the platform bridges historically siloed workflows for Android (AndroidX), Fuchsia, and Skia, detailing custom ingestion adapters, notification formatters, and the partner validation gates required prior to shutting down legacy Appspot systems.

---

## 1. Android & AndroidX Ingestion and Triage Architecture

The Android platform introduces specific architectural divergence from Chromium's Git-centric model:

### 1.1 Synthetic Commit Identifiers vs. Git Hashes
- In standard Skia and Chrome instances, performance traces are indexed against upstream Git commits (`git.Git` provider, [`perf/go/git/git.go`](../../go/git/git.go)).
- In Android instances ([`perf/configs/spanner/android.json`](../../configs/spanner/android.json)), data originates from internal Android build pipelines where benchmark points correspond to **Build IDs** (e.g., `10768667`) rather than linear Git hashes.
- Ingestion is handled via specialized microservices in [`android_ingest/go/androidingest/main.go`](../android_ingest/go/androidingest/main.go) and parser adapters in [`android_ingest/go/parser/parser.go`](../android_ingest/go/parser/parser.go).
- `parser.convertFromAndroidSpecificFormat` ([`parser.go:130`](../android_ingest/go/parser/parser.go#L130)) maps incoming Android testing JSONs—such as `LauncherJankTests#testAppSwitchGMailtoHome`—into standardized Skia benchmark traces.

### 1.2 The `AndroidNotificationProvider`
- In [`perf/go/notify/android_notification_provider.go`](../../go/notify/android_notification_provider.go), Perf defines custom bug generation logic:
  - Context struct: `AndroidBugTemplateContext` ([`android_notification_provider.go:21`](../../go/notify/android_notification_provider.go#L21)).
  - Helper `GetBuildIdUrlDiff()` ([`android_notification_provider.go:68-80`](../../go/notify/android_notification_provider.go#L68-L80)) inspects `RegressionCommitLinks["Build ID"]` and `PreviousCommitLinks["Build ID"]`, assembling a range search query:
    ```
    https://android-build.corp.google.com/range_search/cls/from_id/{from}/to_id/{to}/?s=menu&includeTo=0&includeFrom=1
    ```
  - Formats test descriptions for IssueTracker bugs into human-readable strings:
    `{{test_class}}#{{test_method}} ({{device_name}} {{os_version}})` ([`android_notification_provider.go:63`](../../go/notify/android_notification_provider.go#L63)).
  - Links to triage playbooks: `http://go/androidx-bench-triage` ([`perf/configs/spanner/android.json:17`](../../configs/spanner/android.json#L17)).

---

## 2. Fuchsia Integration and Internal Bisection Cutover

Fuchsia operates public and internal instances ([`fuchsia-public.json`](../../configs/spanner/fuchsia-public.json), [`fuchsia-internal.json`](../../configs/spanner/fuchsia-internal.json)) tracking performance across the Fuchsia OS tree and WebEngine.

### 2.1 JSON Format Translation Pipeline
- Fuchsia performance tests output a schema distinct from Catapult histograms and standard Skia Perf JSONs.
- Skia provides a converter in [`perf/go/fuchsia_to_skia_perf/`](../../go/fuchsia_to_skia_perf/):
  - Struct `FuchsiaPerfResults` ([`convert/types.go:24`](../../go/fuchsia_to_skia_perf/convert/types.go#L24)) captures test suite names, components, and metric arrays.
  - `PopulateResults` ([`convert/lib.go:214`](../../go/fuchsia_to_skia_perf/convert/lib.go#L214)) transforms Fuchsia records into `SkiaResultItem` structures for ingestion into Cloud Spanner.
  - Calculates mean, median, and variance metrics (`CalculateStats`, [`convert/lib.go:266`](../../go/fuchsia_to_skia_perf/convert/lib.go#L266)).

### 2.2 Pinpoint Bisection Cutover for Fuchsia
- In [`pinpoint/go/bot_configs/isolate_targets.yaml:31`](../../../../pinpoint/go/bot_configs/isolate_targets.yaml#L31):
  ```yaml
  # WebEngine tests are specific to Fuchsia devices only.
  fuchsia-perf: performance_web_engine_test_suite
  ```
- Fuchsia WebEngine performance regressions bisect against `performance_web_engine_test_suite`.
- Transition goal: Completely retire the standalone internal Fuchsia bisection scripts in favor of Pinpoint's `BisectWorkflow`, running WebEngine isolate builds across Swarming device pools.

---

## 3. Skia Core Partner Workflows

The Skia repository itself (`skia.googlesource.com/skia`) is the founding tenant of the infrastructure:
- Direct integration with Git repository commit numbers (`perfGit`, [`perf/go/git/git.go`](../../go/git/git.go)).
- Nanobench and GM benchmarks ingest sub-millisecond drawing and GPU rasterization timings.
- Direct integration with Skia Gold for visual regression detection across identical Git revisions.

---

## 4. Appspot Shutdown Gates for Partner Workflows

To ensure zero regressions in operational capability when `chromeperf.appspot.com` is shut down, each partner workflow must pass explicit verification gates:

| Partner | Current Reliance on Appspot | Shutdown Verification Gate | Target State |
| :--- | :--- | :--- | :--- |
| **Chrome Waterfall** | Catapult UI viewing, Datastore job history | Pinpoint Angular WebUI feature complete; all sheriffs triaging via `/r2/` or new UI | Direct Spanner reads, Temporal bisection |
| **V8** | Legacy `/a/` anomaly page, Catapult trace URLs | Ensure `SheriffConfigService` and `keys_for_commit_range` multi-repo linking function natively in Skia Perf without Appspot forwarding | Native Skia Perf `/a/` page, Pinpoint Go |
| **Android / AndroidX** | Historically utilized Catapult dashboards for some OS benchmarks | `AndroidNotificationProvider` fully generating Buganizer issues with valid `GetBuildIdUrlDiff()` links directly from Spanner | Independent Spanner instance (`android.json`) |
| **Fuchsia** | Custom ingestion script bridging to legacy dashboards | `fuchsia_to_skia_perf` pipeline automated on GCS upload; `fuchsia-perf` isolate target bisecting cleanly in Pinpoint | Full Skia Perf + Pinpoint WebEngine bisection |

### Final Decommissioning Step
Once all partner gates pass:
1. Revoke App Engine cron jobs and task queues.
2. Route DNS records (`chromeperf.appspot.com`, `pinpoint-dot-chromeperf.appspot.com`) to Cloud CDN / GKE ingress proxies that redirect to `perf.luci.app` and `pinpoint.luci.app`.
3. Drop the App Engine Datastore instances.
