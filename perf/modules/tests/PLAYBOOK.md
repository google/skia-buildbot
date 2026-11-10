# Perf Playwright E2E test playbook

This directory contains the hermetic Playwright end-to-end test suite for Skia Perf (`/m`, `/e`, `/e2`, `/a`, `/u`). Every test target runs inside the Bazel sandbox against the in-memory Go mock server (`//perf/go/frontend/mock:frontend_mock_server`) via the `sk_element_playwright_test` macro in `//infra-sk:index.bzl`.

The 5 component test suites are:

- `//perf/modules/tests:simplegraph_playwright_test` (`/e`)
- `//perf/modules/tests:multigraph_playwright_test` (`/m`)
- `//perf/modules/tests:explore_multi_v2_playwright_test` (`/e2`)
- `//perf/modules/tests:regressions_page_playwright_test` (`/a`)
- `//perf/modules/tests:report_page_playwright_test` (`/u`)

## Running tests locally

Run all Playwright E2E tests in headless mode:

```bash
bazel test --workspace_status_command= --test_output=errors //perf/modules/tests/...
```

Run a single test target or filter by Mocha test title:

```bash
bazel test --workspace_status_command= --test_output=errors \
  --test_filter="Report page" \
  //perf/modules/tests:report_page_playwright_test
```

## Headful and interactive debugging

Every `sk_element_playwright_test(name = "<target>", ...)` generates three manual local helper targets alongside the default headless target:

- `//perf/modules/tests:<target>_headful`: launches Chrome with a visible window (`PLAYWRIGHT_TEST_SHOW_BROWSER=true`, automatically using `/usr/bin/google-chrome` when `CHROME_BIN` points to `chrome-headless-shell`).
- `//perf/modules/tests:<target>_debug`: waits for a Node.js inspector debugger on port 9229 before running tests.
- `//perf/modules/tests:<target>_debug_headful`: combines a visible Chrome window with the Node.js debugger wait.

On a Cloudtop workstation with an Xwayland/X11 desktop session, pass `--strategy=TestRunner=local` and forward `DISPLAY`, `XAUTHORITY`, and `SLOW_MO`:

```bash
bazel test --workspace_status_command= --test_output=streamed \
  --strategy=TestRunner=local \
  --test_env=DISPLAY=:0 \
  --test_env=XAUTHORITY=$XAUTHORITY \
  --test_env=SLOW_MO=250 \
  //perf/modules/tests:multigraph_playwright_test_headful
```

Setting `SLOW_MO=<ms>` introduces a delay between Playwright browser actions so you can visually follow test execution in headful mode. When `SLOW_MO` or `PLAYWRIGHT_TEST_SHOW_BROWSER` is set, `playwright_util.ts` automatically disables Mocha test timeouts (`this.timeout(0)`).

## Inspecting test artifacts and traces (`failure.png` and `trace.zip`)

`//puppeteer-tests/playwright_util.ts` automatically records a Playwright trace (`screenshots: true, snapshots: true`) for each test case.

- On test failure: `afterEach` writes `failure.png` and `trace.zip` into Bazel undeclared test outputs.
- On passing tests with `SAVE_TRACE=1`: pass `--test_env=SAVE_TRACE=1` to save `screenshot.png` and `trace.zip` for all passing test cases as well:

```bash
bazel test --workspace_status_command= --test_env=SAVE_TRACE=1 \
  //perf/modules/tests:simplegraph_playwright_test
```

After the run, inspect artifacts in Bazel undeclared outputs:

```bash
unzip -l _bazel_testlogs/perf/modules/tests/<target>/test.outputs/outputs.zip
```

Open the trace in the Playwright Trace Viewer:

```bash
npx playwright show-trace path/to/trace.zip
```

## Controlling mock server state and personas

Tests use `MockServerClient` (`perf/modules/tests/mock_client.ts`) to reset server state between test cases and inspect recorded mutation payloads:

1. Call `await mock.reset('chrome')` or `await mock.reset('v8')` in `beforeEach` (or at the start of a test) to restore default anomalies, shortcuts, and frames and switch `window.perf` between the Chrome (`show_bisect_btn: true`) and V8 (`show_bisect_btn: false`) personas.
2. Call `await mock.recorded('/_/triage/edit_anomalies')` (or `/_/triage/file_bug`, `/_/triage/associate_alerts`, `/_/bisect/create`, `/_/shortcut/update`, `/_/anomalies/calculate_regr_shortcut`) to assert the exact JSON payloads sent by the frontend.

## Adding new workflows and extending mock data

Reusable locators and page actions live in `perf/modules/tests/pages/` (`MultigraphPage`, `SimplegraphPage`, `ExploreV2Page`, `RegressionsPage`, `ReportPage`, `TriageMenu`, `BisectDialog`).

The 24 Chrome and V8 traces, 20 commit headers (`67120..67139`), anomalies (`123`, `456`, `789`, `101`, `1001`, `1002`), and pre-seeded shortcuts live in `perf/go/frontend/mock/testdata.go`. Whenever you add traces to `testdata.go`, `BuildWasmCache` in `perf/go/frontend/mock/wasm.go` encodes them into `/_/wasm/meta.json`, `/_/wasm/params.json`, and `/_/wasm/traces.bin` for `/e2`. Run `bazel test --workspace_status_command= //perf/go/frontend/mock:all` after modifying any mock server handler or dataset definition.
