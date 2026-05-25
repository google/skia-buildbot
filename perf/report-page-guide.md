# Report Page Triage & Analysis Guide

> **Bug Filing & Feedback**: Found an issue with the Perf Report Page,
> or have a feature request? File a ticket in the
> [Perf 2.0 Buganizer Component](https://b.corp.google.com/issues/new?component=1989668).

This guide explains how to navigate, triage, and analyze performance regressions
(anomalies) using the **Perf Report Page** (`/u`).

While the **Explore** (`/e`) view is designed for deep-diving into a single
comprehensive graph, and the [Multigraph](./multigraph-guide.md) (`/m`) view is
optimized for viewing and synchronizing multiple metrics, the **Report Page**
(`/u`) is the dedicated workspace for triaging anomalies and regressions.

All examples and links in this guide use the Chrome Perf instance
([https://chrome-perf.corp.goog/u/](https://chrome-perf.corp.goog/u/)), but the
core concepts, layouts, and user flows apply universally to all **Perf 2.0**
dashboard instances that have activated the `"show_triage_link"` configuration
flag (defined in [config.go](./go/config/config.go#L1153)).

---

# Table of Contents

- [Accessing the Report Page](#accessing-the-report-page)
- [The Anomalies Table (Triage Dashboard)](#the-anomalies-table-triage-dashboard)
  - [Columns & Indicators](#columns--indicators)
  - [Smart Grouping & Expandable Summary Rows](#smart-grouping--expandable-summary-rows)
  - [Sorting Columns](#sorting-columns)
- [Suspect Range & Bypassing Dependency Rolls](#suspect-range--bypassing-dependency-rolls)
  - [The Suspect Commits List](#the-suspect-commits-list)
  - [Camera-Roll Feature: Dependency Roll Bypasses](#camera-roll-feature-dependency-roll-bypasses)
- [Analyzing Interactive Regression Graphs](#analyzing-interactive-regression-graphs)
  - [Symmetric History Windows](#symmetric-history-windows)
  - [Graph Synchronization & Interactive Controls](#graph-synchronization--interactive-controls)
- [The Action Tooltip & Triage Menu](#the-action-tooltip--triage-menu)
  - [Filing a New Buganizer Issue](#filing-a-new-buganizer-issue)
  - [Associating with an Existing Bug](#associating-with-an-existing-bug)
  - [Ignoring Anomalies](#ignoring-anomalies)
  - [Nudging Anomalies](#nudging-anomalies)
- [Automated Bisection & Debugging (Pinpoint)](#automated-bisection--debugging-pinpoint)
  - [Triggering a Pinpoint Bisection](#triggering-a-pinpoint-bisection)
  - [Generating Debug Traces (Try Jobs)](#generating-debug-traces-try-jobs)

---

## Accessing the Report Page

The Report Page (`/u`) is **query-parameter driven** and is not intended to be
accessed directly without parameters. If you navigate to
`https://chrome-perf.corp.goog/u/` directly, you will see an empty state stating
"All anomalies triaged" or a red "group report is invalid" error message.

Instead, users are typically routed to this page from external sources, including:

- **Automated Alert Emails / Alerts (`/a`)**: Triggered by regression detection.
- **Buganizer Comments**: Posted automatically or manually to track performance bugs.
- **Chromium Dash / Build Pipelines**: Linking directly to regression ranges.

### URL Query Parameters

When routed here, the URL is pre-populated with specific parameters that the
page reads to build the workspace:

- **Bug ID** (`?bugID=123456`): Loads all anomalies associated with the
  specified Buganizer ID. The page title updates to "Report for bug: 123456".
- **Anomaly IDs** (`?anomalyIDs=987,988`): Loads the specified anomaly IDs. The
  page title becomes "Report for anomalies: 987,988".
- **Anomaly Group ID** (`?anomalyGroupID=abc-123`): Loads all anomalies
  belonging to the pre-computed anomaly group. The page title becomes
  "Report for anomaly group: abc-123".
- **Saved Session ID** (`?sid=xyz789`): Restores a saved workspace state,
  checking and rendering charts for the anomalies saved in that session. The
  page title becomes "Report for selected: xyz789".
- **Revision** (`?rev=12345`): Filters the loaded alerts around revision
  `12345`.

---

## The Anomalies Table (Triage Dashboard)

The top section of the workspace renders the **Anomalies Table**. It aggregates
all loaded regressions and improvements, presenting critical metadata in a
single, high-level view.

![Anomalies Table](./images/report_page_guide/anomalies-table.png 'Anomalies Table')

### Columns & Indicators

- **Group Toggle**: Visible on nested anomalies. Displays the count of
  regressions and improvements in that cluster (e.g., `3 | 1`).
- **Checkbox**: Selects anomalies for charting. Checking a row adds its
  interactive graph below.
- **Chart (Trending Up Icon)**: Next to the checkbox in every row. Clicking this
  button opens the trace's [Multigraph](./multigraph-guide.md) (`/m`) view in a
  new tab.
- **Bug ID**: Shows the primary associated Buganizer issue ID. Clicking the
  link opens the bug in a new tab. A small **Close (X)** icon next to the ID
  unassociates the bug. Hovering over this cell displays a tooltip listing all
  the distinct bug IDs associated with the underlying anomalies in that row.
- **Revisions**: The suspect range of commits where the regression occurred.
- **Bot**: The name of the hardware configuration or bot that ran the test.
- **Test Suite**: The parent benchmark suite name.
- **Test**: The specific test leaf path.
- **Delta %**: The percentage change in performance.
  - **Red percentages** represent performance regressions (labeled with [Red]).
  - **Green percentages** represent performance improvements (labeled with [Green]).

### Smart Grouping & Expandable Summary Rows

To prevent dashboard clutter, anomalies with identical test hierarchies or
overlapping revision ranges are nested inside parent **Group Rows**.

- Click the regressions/improvements count button on the far left of a row
  (e.g. `5 | 0`) to expand and inspect every individual trace in that group.
- Toggle the master checkbox in a collapsed summary row to simultaneously check
  or uncheck all child anomalies.

### Sorting Columns

Sheriffs can sort the entire list of anomalies by clicking on any header column
button: **Bug ID**, **Revisions**, **Bot**, **Test Suite**, **Test**, and
**Delta %**. An arrow icon will indicate ascending or descending order.

---

## Suspect Range & Bypassing Dependency Rolls

### The Suspect Commits List

Directly below the anomalies table, the workspace lists up to **10 Common
Commits** representing the suspect range where the performance change was
detected.

- Clicking a shortened commit hash opens its Git repository hosting page (e.g.,
  Gitiles/Gerrit) in a new browser tab to inspect the code diff.
- Scanning this list helps you identify suspect changes in configuration,
  dependency bumps, or code refactors without leaving the Perf dashboard.

![Common Commits Range](./images/report_page_guide/common-commits.png 'Common Commits Range')

### Camera-Roll Feature: Dependency Roll Bypasses

Dependency roll commits (e.g., rolling V8 or WebRTC) have messages starting with
`Roll` or `Manual roll`. **Perf 2.0** detects these commits and displays a
**Film-Roll icon** next to the commit hash.

- **Commit Hash Link**: Opens the generic roll CL.
- **Film-Roll Icon**: Bypasses the roll wrapper and opens the underlying rolled
  CL repository history or rolled CLs directly in a new tab.

---

## Analyzing Interactive Regression Graphs

Checking an anomaly row (or multiple rows) in the table immediately draws
interactive charts below the list.

![Interactive Graphs](./images/report_page_guide/interactive-graphs.png 'Interactive Graphs')

### Symmetric History Windows

Report Page graphs render data in a symmetric window: **1 week before and 1 week
after** the detected regression point.

This window displays:

- Whether the regression represents a temporary spike or a sustained step-change.
- Whether subsequent commits have already mitigated the regression.
- The baseline noise and variance of the metric before the regression occurred.

### Graph Synchronization & Interactive Controls

- **Zoom & Pan Sync:** Timeline dragging, zooming, or panning on any chart
  automatically synchronizes all other sister charts on the page, allowing for
  easy side-by-side correlation.
- **Hover detail:** Hovering over any data point displays its exact commit
  position, date, and value details in a tooltip.
- **Toggle X-Axis Domain:** Click the **Settings Gear** in the top right of any
  chart to toggle between **Commit Position** (default) or **Commit Date** on
  the X-axis.

---

## The Action Tooltip & Triage Menu

Clicking any data point on an interactive graph opens a details **Tooltip**
containing the triage menu:

![Triage Actions Dialog](./images/report_page_guide/triage-popup.png 'Triage Actions Dialog')

### Filing a New Buganizer Issue

1. Click **New Bug** to open the filing form.
2. Fill in the component and title fields.
3. Submit the form. The table automatically links the new Bug ID to the
   selected anomalies.

### Associating with an Existing Bug

1. Click **Existing Bug**.
2. Enter the Buganizer ID or select from the active performance bugs list.
3. Click **Associate**.

### Ignoring Anomalies

1. Click **Ignore**.
2. This marks the anomalies as ignored.

### Nudging Anomalies

Automated alert detectors sometimes place the regression anchor slightly before
or after the actual step change.

- Use the **Nudge** buttons (**-2, -1, +1, +2**) in the graph tooltip triage
  section to shift the anomaly's suspect start and end revision boundaries
  visually on the chart.
- This updates the database to ensure alert tracking matches the true visual
  step change.

---

## Automated Bisection & Debugging (Pinpoint)

For supported instances, you can trigger downstream performance debugging
directly from the graph tooltip under the **Pinpoint** section.

### Triggering a Pinpoint Bisection

To run a bisection job across a suspect commit range:

1. Open the **Bisection Dialog** via the graph tooltip.
2. The dialog pre-populates the **Test Path**, **Bug ID**, **Start Commit**,
   **End Commit**, and **Story** from the selected anomaly.
3. (Optional) Enter a Gerrit **Patch** hash to apply across the bisection run.
4. Click **Bisect** to submit the Pinpoint job. The tooltip renders a tracking
   link for the active job.

![Pinpoint Bisection Form](./images/report_page_guide/bisect-dialog.png 'Pinpoint Bisection Form')

### Generating Debug Traces (Try Jobs)

If you need to gather deep trace event categories to debug rendering or memory
behavior:

1. Open the **Debug Traces (Try Job) Dialog**.
2. Adjust your **Base Commit**, **Experiment Commit**, and customize the
   **Tracing Arguments** (pre-populated with standard categories like
   `toplevel,toplevel.flow`).
3. Click **Generate** to run an A/B trace gathering run on Pinpoint hardware.

![Try Job Form](./images/report_page_guide/try-job-dialog.png 'Try Job Form')
