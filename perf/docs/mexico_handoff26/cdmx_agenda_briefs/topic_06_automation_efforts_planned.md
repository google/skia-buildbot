# Topic 6: Automation Efforts Planned

## Executive Summary
This brief reviews the strategic automation efforts planned across Skia Perf and Pinpoint engineering teams. It explores the implementation of dynamic website deployment pipelines, compares TypeScript integration and Puppeteer end-to-end testing strategies, examines the architecture of Pinpoint on CQ (Continuous Quality) with CABE analysis, and investigates AI-assisted regression detection through the Gemini assistant side panel and Model Context Protocol (MCP).

---

## 1. Dynamic Website Deployment Pipeline

Deploying updates across dozens of multi-tenant Perf instances (e.g. `chrome-internal`, `v8-internal`, `android`, `fuchsia`, `skia-public`) has historically required manual configuration rollouts or static image tagging.

### 1.1 GitOps & Kubernetes Architecture
- Services are declared as Kubernetes deployments running under GKE.
- Configuration is driven by the JSON files in [`perf/configs/spanner/`](../../configs/spanner/).
- The target dynamic deployment pipeline leverages:
  1. Automated container builds via Bazel rules (`//perf:perfserver`, `//pinpoint:pinpoint`).
  2. Centralized schema validation using `config.validate` ([`perf/go/config/validate/validate.go`](../../go/config/validate/validate.go)) to catch configuration syntax errors before deployment.
  3. Continuous rollout through staging/autopush instances (`chrome-internal-autopush`, `v8-internal-autopush`) before promoting images to production tiers.

### 1.2 Spanner Schema Migration Automation
- Schema changes are maintained in version-controlled migration files in [`perf/go/sql/expectedschema/migrations/`](../../go/sql/expectedschema/migrations/).
- The `perf-spanner-migrations` automation ensures that schema updates (such as adding columns to `Autobisections` or creating new indexes on `Culprits`) are validated against local Spanner emulators in Bazel tests before applying to production databases.

---

## 2. Testing Strategy: TypeScript Integration vs. E2E Tests

The frontend of Perf and Pinpoint combines custom elements, Web Components (LitElement), and Angular. Maintaining reliability across this surface requires a clear division between test layers:

```
┌────────────────────────────────────────────────────────┐
│             Puppeteer End-to-End Tests                 │
│  - Full browser automation (*_puppeteer_test.ts)       │
│  - Visual screenshots, dialog flows, UI screenshots    │
└───────────────────────────┬────────────────────────────┘
                            │
┌───────────────────────────▼────────────────────────────┐
│          TypeScript Component Integration Tests        │
│  - Mocha/Chai/Karma (*_test.ts)                        │
│  - Mocked fetch requests (fetch-mock)                  │
│  - DOM state, event firing, attribute validation       │
└───────────────────────────┬────────────────────────────┘
                            │
┌───────────────────────────▼────────────────────────────┐
│               Go Backend Unit & Contract Tests         │
│  - Go test suites (*_test.go)                          │
│  - In-memory database pools, Spanner emulators         │
└────────────────────────────────────────────────────────┘
```

### 2.1 Component-Level Integration Tests (`*_test.ts`)
- Implemented using `@open-wc/testing`, Mocha, and Chai.
- Example: [`perf/modules/cluster-summary2-sk/cluster-summary2-sk_test.ts`](../../modules/cluster-summary2-sk/cluster-summary2-sk_test.ts) and [`perf/modules/bisect-dialog-sk/bisect-dialog-sk_test.ts`](../../modules/bisect-dialog-sk/bisect-dialog-sk_test.ts).
- Mock HTTP endpoints using `fetch-mock`, testing how components render given complex `ClusterSummary` payloads without requiring live backend servers. Fast, reliable, run on every pre-submit.

### 2.2 Puppeteer E2E Tests (`*_puppeteer_test.ts`)
- Example: [`perf/modules/cluster-page-sk/cluster-page-sk_puppeteer_test.ts`](../../modules/cluster-page-sk/cluster-page-sk_puppeteer_test.ts) and [`perf/modules/perf-scaffold-sk/perf-scaffold-sk_puppeteer_test.ts`](../../modules/perf-scaffold-sk/perf-scaffold-sk_puppeteer_test.ts).
- Spin up real Chromium browser instances via testbeds, taking visual screenshots (Gold diffs) and validating modal interactions (e.g. clicking `#bisect-dialog-submit`, verifying toast popups).

---

## 3. Pinpoint on CQ (Continuous Quality)

Shifting regression detection "left"—from post-commit waterfall alerts to pre-commit commit-queue (CQ) trybots—is a major focus for preventing performance degradation before code lands.

### 3.1 Bot Fleet Allocation for CQ
- In [`pinpoint/go/bot_configs/external.json:440-455`](../../../../pinpoint/go/bot_configs/external.json#L440-L455), dedicated CQ bot pools are configured:
  - `linux-perf-cq` (dimensions: `chrome.tests.pinpoint-cq`)
  - `win-10-perf-cq`
- These bots are segregated from long-running post-commit bisections to guarantee low queuing latency for developer tryjobs.

### 3.2 Statistical Analysis via CABE
- Pinpoint try jobs on CQ run pairwise A/B tests between the candidate CL and base revision.
- Results are fed to CABE (Categorical Analysis of Benchmark Experiments), located in [`cabe/go/cmd/cabeserver/main.go`](../../../../cabe/go/cmd/cabeserver/main.go) and [`cabe/go/analyzer/analyzer.go`](../../../../cabe/go/analyzer/analyzer.go).
- `computeCQCabeAnalysisResults` ([`cabeserver/main.go:292`](../../../../cabe/go/cmd/cabeserver/main.go#L292)) evaluates statistical confidence intervals. If a CL causes a statistically significant regression exceeding critical thresholds, CABE marks the build as failed, preventing merge.

---

## 4. AI-Assisted Regression Detection & Exploration

Skia Perf is incorporating Large Language Models (LLMs) to accelerate triage, root cause analysis, and natural language trace queries.

### 4.1 The Gemini Side Panel (`gemini-side-panel-sk`)
- Component: [`perf/modules/gemini-side-panel-sk/gemini-side-panel-sk.ts`](../../modules/gemini-side-panel-sk/gemini-side-panel-sk.ts).
- Integrated into the primary application shell [`perf/modules/perf-scaffold-sk/perf-scaffold-sk.ts:380-410`](../../modules/perf-scaffold-sk/perf-scaffold-sk.ts#L380-L410).
- Users can click "Ask Gemini" to open a slide-out assistant that accepts conversational prompts regarding the active graph (e.g., *"Summarize when this regression started"*, *"Compare commit range diffs"*).
- The frontend posts queries to `/_/chat`, which routes to backend Gemini AI endpoints ([`perf/go/frontend/frontend.go:1555`](../../go/frontend/frontend.go#L1555)).

### 4.2 Model Context Protocol (MCP) Integration
- Located in [`mcp/services/pinpoint/client.go`](../mcp/services/pinpoint/client.go).
- Exposes structured tools to AI agents:
  - Querying trace metrics across specific time ranges.
  - Automatically creating Pinpoint try or bisect jobs with `{"origin": "gemini"}` tags ([`client.go:103`](../mcp/services/pinpoint/client.go#L103)).
  - Analyzing culprit commits and summarizing changelog messages for sheriffs.
