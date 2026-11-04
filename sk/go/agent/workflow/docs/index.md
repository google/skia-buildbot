# Skia Infra Agents Guide

This document serves as the top-level router for agents operating within Skia
repositories. It contains brief descriptions of workflows and is designed to
keep your primary context window lean.

**CRITICAL:** Do NOT perform complex infrastructure analysis or workflow
orchestration using your default behaviors. You MUST consult any relevant
workflows listed below before attempting these tasks. When executing these
workflows, you are acting as a data-gathering analyst, NOT a developer. Do NOT
attempt to read source code, debug, or fix issues unless explicitly instructed
to do so.

## Workflows

Depending on your task, read details for the corresponding workflow by running
`sk agent workflow <name>`:

### task_failure_analysis

**Command:** `sk agent workflow task_failure_analysis`
**File:** `sk/go/agent/workflow/docs/task_failure_analysis.md`
**Trigger:** Use this workflow when asked to:

- Summarize recent task failures.
- Investigate "what commit broke task X?"
- Find the root cause of recent regressions.
- Determine if a failing task is a hard regression or flaky infrastructure.

### task_drilldown

**Command:** `sk agent workflow task_drilldown`
**File:** `sk/go/agent/workflow/docs/task_drilldown.md`
**Trigger:** Use this workflow when asked to:

- Provide detailed analysis of a single task.
- Find actual build or test failure(s), error messages, etc.
- Investigate flaky tasks.

### alert_report

**Command:** `sk agent workflow alert_report`
**File:** `sk/go/agent/workflow/docs/alert_report.md`
**Trigger:** Use this workflow when asked to:

- Quickly summarize or list active alerts on `am.skia.org` without deep-diving into logs or root causes.
- Generate a fast surface-level alert overview (overall, by category, or for a specific owner/assignee).

### alert_drilldown

**Command:** `sk agent workflow alert_drilldown`
**File:** `sk/go/agent/workflow/docs/alert_drilldown.md`
**Trigger:** Use this workflow when asked to:

- Deeply investigate a specific alert or bundle of related alerts.
- Determine the root cause of a firing alert by inspecting logs, cluster state, source code, or repository refs.

---

_Note: As more complex, agent-specific workflows are developed for this
repository, they should be added as separate markdown files in this directory
and linked here in this document._
