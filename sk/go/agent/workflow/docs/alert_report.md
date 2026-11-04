# Skia Infrastructure Active Alert Report

This document contains instructions for agents to retrieve active alerts from
Alert Manager (`am.skia.org`), bundle correlated signals to reduce volume,
evaluate recurrence patterns, and generate a prioritized triage report.

## General Rules

- **CRITICAL:** This is strictly an alert retrieval, correlation, and reporting
  workflow. Do **not** attempt to read source code, debug issues, or formulate
  code fixes during this workflow unless explicitly asked to deep-dive into a
  specific alert.
- Because hundreds of alerts may be active simultaneously, **always** use the
  `--output-file` flag when calling `get_active_alerts` to save the output to a
  file and read it in chunks, rather than consuming the entire output on stdout
  at once.
- Preserve all distinct signals when bundling: correlated alerts trigger on
  different criteria and their combination often reveals more than any single
  alert on its own.

## Phase 1: Retrieve Active Alerts

1. Run `sk agent tool get_active_alerts --output-file=<temp_file>`, passing any
   filter flags relevant to the user's request (`--category`, `--severity`,
   `--alertname`, `--owner`, `--assigned_to`, `--include_silenced`,
   `--include_assigned`).
2. Read the entire output file in chunks so that no rows are missed.
   - `get_active_alerts` pre-groups alerts into multiple Markdown tables based on
     their set of non-empty parameter keys. Each table includes standard columns
     (`ID`, `AlertName`, `Abbr`, `Severity`, `Category`, `Started`, `Owner`,
     `AssignedTo`, `Flags`) followed by dynamic columns for each alert parameter
     present in that group.
   - When a metric is emitted by an external checker service about another
     resource, Prometheus prefixes conflicting target labels with `exported_`
     (e.g., `exported_app`, `exported_pod`, `exported_container`,
     `exported_namespace`), and `get_active_alerts` omits the overshadowed
     unprefixed scraper labels.

## Phase 2: Bundle Correlated Alerts

While the multi-table output naturally groups alerts that share identical
parameter key sets, **correlated alerts for the same entity frequently land in
different tables** when one alert rule attaches extra metric labels (or uses
`exported_*` labels). Different alert rules also format `Abbr` differently (for
example, one alert may put a pod or branch name in `Abbr`, while another puts an
app or job name in `Abbr`).

Before presenting any findings, group the active alerts into focused bundles
across all tables using the following rules:

1. **By Specific Target Instance (Within and Across Tables):**
   - Group alerts that share parameter values identifying the **exact same
     specific instance or entity**—such as a specific pod (`pod` /
     `exported_pod`), workload or roller instance (`app` / `exported_app`,
     `roller`), bot (`bot`), CI job or hardware dimension (`job_name`),
     configuration file, database, or service account—even if they appear in
     different tables, have different `AlertName`s, or have differently
     formatted `Abbr` values.
   - Note that when `exported_*` columns are present, any remaining unprefixed
     scraper columns on that row belong to the checker service that emitted the
     metric, whereas the `exported_*` columns identify the actual target
     resource (and should be matched with the corresponding unprefixed columns
     in other tables).
   - When multiple alerts fire for the same specific entity, combine their
     signals so their joint meaning is clear.
2. **By Homogeneous Pattern:**
   - Roll up alerts into a single bundle when:
     - The **same `AlertName`** (or closely paired alert set) fires across
       multiple similar targets (e.g., `GoldSQLBackupError` across several
       databases, `AutoRollGetReviewersFailed` across rollers hitting the same
       rotation proxy, `OverdueJobSpec` across jobs on the same hardware pool,
       or `Bot*` alerts across machines on the same host/rack).
     - The **same failure mode** begins simultaneously across closely related
       workloads in the same namespace/cluster (e.g., multiple pods of the same
       service family starting to crash-loop at the exact same time).
3. **Avoid Over-Bundling & Unverified Causality (CRITICAL):**
   - **Do NOT use broad organizational labels (`appgroup`, `namespace`,
     `cluster`, or `category`) to merge different alert types or unrelated
     workloads into a single mega-bundle.** Unrelated failure modes within the
     same product area—such as a container image build failure vs. a PubSub
     ingestion backlog vs. a pod crash-loop, or an unrelated workload that
     merely shares an `appgroup` label—are **separate issues** and must be
     reported as separate items.
   - **Do NOT speculate on upstream/downstream root causes without explicit
     shared parameters.** Only link different alerts across resources when their
     parameters explicitly reference the same target, endpoint, or hardware
     constraint (for example, matching hardware/device dimensions between
     `CQTrybotRunningTooLong` and `OverdueJobSpec`). Do not invent causal
     narratives across services that would require log inspection to verify.
   - **Beware of simultaneous `Started` timestamps from checker/monitoring
     restarts:** When a metric emitter (such as `datahopper` or `k8s-checker`)
     or Prometheus restarts, many unrelated alerts emitted by that service may
     reset and show the exact same `Started` timestamp. Do not treat a shared
     `Started` timestamp across unrelated alert types as proof of a single
     shared incident; check `get_alert_details` history to see if the alerts
     were already firing prior to a brief reset.

## Phase 3: Evaluate Details, Recurrence & History (`get_alert_details`)

Because `get_active_alerts` omits verbose fields like `description`, notes,
silence details, and historical occurrences to keep the overview compact, run
`sk agent tool get_alert_details --alert_id=<id>` on a representative alert for
each high-priority or actionable bundle. This provides:

- **Full Parameters (including `description`):** Diagnostic messages, playbook
  links (`PROD.md`), and log/dashboard URLs.
- **Notes & Silences:** Human commentary and details on active or recently
  expired silences.
- **Active Duration & Recent History:** Past incident start/end times and
  durations.

**CRITICAL — Do NOT rely on the boolean `Flaky` field to dismiss an alert:**

- **Intermittent Signals != Harmless Flakes:** A signal that repeatedly fires
  and auto-resolves often indicates a **persistent, critical problem** whose
  metric oscillates across a threshold (for example, a crash-looping service
  that takes several minutes to start up, briefly reports healthy, and crashes
  again; or a periodic background task that fails on every cycle).
- **Sustained Outages with Historical Flaps:** An alert that has been
  continuously active for hours or days may still report `Flaky: true` if it
  flapped briefly before locking into a hard failure.

Instead of relying on `Flaky`, examine the **Active Duration** and the **Recent
History** table to describe the alert's actual temporal pattern:

- **Newly Firing / Sustained:** Continuously active since `Started` with no (or
  few) recent resolutions.
- **Oscillating / Repeating:** Firing and resolving repeatedly. Report the
  observed cadence, typical duration, and gap between occurrences.
- **Chronic:** Continuously active for an extended period (days or weeks).

## Phase 4: Generate the Triage Report

Present a structured report organized by priority:

1. **Actionable Issues (Recent or Actively Oscillating):**
   - Highlight the bundles that most likely require immediate attention.
   - For each bundle, include:
     - **Target & Scope:** Affected service, host/pool, or dependency.
     - **Alert Count & Combined Signals:** Which `AlertName`s are firing
       together and what their combination indicates.
     - **Timing & Cadence:** How long the current incident has been active and
       (where inspected) whether it is sustained or oscillating.
     - **Owner / AssignedTo:** Any owner or assignee, plus any
       `RECENTLY_EXPIRED_SILENCE` or `NOTES` flags.
     - **Representative IDs & Links:** 1–3 alert IDs and relevant playbook or
       log URLs from the representative alert's `description` in
       `get_alert_details`.
2. **Broader Fleet / Capacity Roll-Ups:**
   - Summarized pattern bundles covering multiple machines, jobs, or ongoing
     degradations.
3. **Chronic & Hygiene Backlog:**
   - A compact summary table of long-standing or static policy alerts showing
     alert counts and age so they remain visible without burying actionable
     issues.
