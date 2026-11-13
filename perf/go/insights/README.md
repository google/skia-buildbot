# perfinsights (proof of concept)

perfinsights is a small, read-only service that lets LLM agents (Gemini CLI, Jetski, ...) and
scripts get Skia Perf anomaly insights. It serves the same six operations over REST
(`/api/v1`) and MCP (`/mcp`, Streamable HTTP, stateless), plus a landing page (`/`) and
`/llms.txt`.

It reads the Perf database through narrow read-only interfaces of the existing stores, so it
can't write anything. It is meant to become the implementation behind the perf.v1 `/api/v1`
endpoints and the `/mcp` endpoint in perf-fe.

## Layout

| Package          | What it does                                                                 |
| ---------------- | ---------------------------------------------------------------------------- |
| `insights`       | The service: validation, store reads, LLM-friendly models, summaries.        |
| `rest`           | REST handlers, the C1 error envelope, C4 headers and `openapi.json`.         |
| `mcptools`       | MCP tools, resources (`perf://glossary`, `perf://instance`) and prompts.     |
| `site`           | Landing page, trace viewer page and `llms.txt`, with a strict CSP.           |
| `tracereader`    | Reads one trace's values from `TraceValues` with a single bounded query.     |
| `insightsserver` | The binary: wiring, auth, rate limits, timeouts and host/origin checks.      |
| `localonly`      | Guards that keep the proof of concept on loopback and the demo database.     |
| `devseed`        | Adds the demo alert and sheriff to the local demo database (no psql needed). |

## Tools

| MCP tool                      | REST                                                                   |
| ----------------------------- | ---------------------------------------------------------------------- |
| `list_sheriffs`               | `GET /api/v1/sheriffs`                                                 |
| `summarize_anomalies`         | `GET /api/v1/insights:summarize?sheriff=...&days=7`                    |
| `list_anomalies`              | `GET /api/v1/anomalies?sheriff=...`                                    |
| `get_anomalies`               | `GET /api/v1/anomalies/{id}`, `POST /api/v1/anomalies:batchGet`        |
| `get_anomaly_trace`           | `GET /api/v1/anomalies/{id}/trace?commits_before=100&commits_after=50` |
| `suggest_triage`              | `GET /api/v1/insights:suggestTriage?sheriff=...&days=1`                |
| `get_anomaly_group`           | `GET /api/v1/anomalyGroups/{id}`                                       |
| `find_anomalies_for_revision` | `GET /api/v1/revisions/{rev}/anomalies`                                |

The trace viewer page, `/trace?id=<anomaly id>`, charts an anomaly's trace (its measurements
around the commit range, the range itself and the medians before and after) and shows its
details: the change, the test, the alert that flagged it and the suspect commits. The **Trace**
links next to regressions in the summary and the triage suggestions open it in a new tab; the page
URL keeps the id and window (`&before=100&after=50`), so it can be shared. Traces come from the
`TraceValues` table through `tracereader`, which runs one parameterized query for the anomaly's
own trace (the trace key comes from the stored anomaly, never from the caller), at most 500
commits on each side. It doesn't use the trace store, which loads an index of every trace when it
starts.

The triage assistant (`suggest_triage`, and the "Triage assistant" section of the landing page)
proposes how to triage a sheriff's untriaged anomalies of the last days (default 1), from the
data Perf stored with each anomaly:

- `file_one_bug`: regressions whose commit ranges intersect, which may share one cause;
- `file_bug`: clean regressions on their own (the median moved at least 4 times the robust
  standard deviation before the step, and stayed there);
- `ignore_likely_noise`: steps under 2 times the noise before them, or that went back to the
  earlier level;
- `review_improvements` and `review` for the rest.

Each suggestion links to Perf's report page (`/u/?anomalyIDs=...`, or `/u/?rev=...` for big
groups), where people apply triage with their own identity and permissions, and bug
suggestions include a draft. perfinsights never changes triage.

## Run it locally

All commands run from `perf/`, with the local demo database up (`make run-demo-instance`).

1. Build:

   ```bash
   bazelisk build --config=mayberemote //perf/go/insights/...
   ```

2. Add the demo alert and its sheriff (`test_subscription`) to the demo database. This is
   idempotent:

   ```bash
   ../_bazel_bin/perf/go/insights/devseed/devseed_/devseed
   ```

3. Let Perf detect regressions: start (or restart) the frontend with clustering _after_ step 2,
   e.g. `make run_frontend_with_clustering`. A frontend loads alert configs at startup and then
   only every 10 minutes. With the demo data it finds 2 anomalies within a minute.

   Don't re-run `make run-demo-instance` or `make run-spanner-emulator` afterwards: they delete
   the Docker containers, which wipes the demo database.

4. Start perfinsights:

   ```bash
   ../_bazel_bin/perf/go/insights/insightsserver/insightsserver_/insightsserver \
     --config_filename=./configs/demo_spanner.json \
     --connection_string='postgresql://root@127.0.0.1:5432/demo?sslmode=disable' \
     --instance_name=local-demo
   ```

5. Open http://127.0.0.1:8090. From a laptop, forward the port first:
   `ssh -L 8090:127.0.0.1:8090 <cloudtop>`, then open http://localhost:8090.

`_bazel_bin` points at the output of the last build mode used; if a binary is missing, look in
`bazel-out/k8-fastbuild/bin` or `bazel-out/k8-dbg/bin`.

## Demo against production V8 data (read-only)

[`demo_v8.sh`](demo_v8.sh) runs perfinsights on your Cloudtop against the production V8
database (`skia-infra-corp` / `tfgen-spanid-20241205020733610` / `v8_int`, the database behind
https://v8-perf.corp.goog). It starts a PGAdapter container on `127.0.0.1:5433` with your
Application Default Credentials, then the server:

```bash
gcloud auth application-default login     # once; refresh it on the day of the demo
perf/go/insights/demo_v8.sh               # http://127.0.0.1:8090, for this machine only
perf/go/insights/demo_v8.sh --share       # also for other Googlers, through UberProxy
perf/go/insights/demo_v8.sh --stop        # stops the PGAdapter container
```

`DATABASE=v8_int_autopush` uses the autopush database instead.

With `--share`, the team opens `http://<your cloudtop>.c.googlers.com:8090` in Chrome on their
corp laptops; UberProxy serves it as `https://<encoded>.proxy.googlers.com` after its login and
device checks. UberProxy doesn't forward who the caller is, so anyone with the URL can read the
data: share it only with the audience of v8-perf, and stop the server after the demo. Remote MCP
clients through UberProxy are untested; during the demo, run the agent on your Cloudtop against
`http://127.0.0.1:8090/mcp`.

Safety layers:

- Every connection is read-only (`default_transaction_read_only=on`, which PGAdapter enforces:
  writes fail with "Update statements are not allowed in read-only mode").
- Perf developers have read-only IAM access; writes need a breakglass grant, so don't run the
  demo with one active.
- perfinsights only calls read methods, and every query has a deadline.
- FIX-01 (Gate G1) hasn't landed, so the server needs `--allow_non_demo_db`: perfinsights
  validates every id as a UUID before it reaches `GetByIDs`. Get ggj@'s OK before demoing.
- Your queries run with your identity and show up in Spanner's Data Access logs.

Very busy sheriffs can have more than 2000 anomalies in a week (the scan limit); responses then
say `truncated: true`, and asking for fewer days gives complete counts.

## Connect an agent

Gemini CLI (`~/.gemini/settings.json`):

```json
{ "mcpServers": { "perfinsights": { "httpUrl": "http://127.0.0.1:8090/mcp" } } }
```

Jetski (`~/.gemini/config/mcp_config.json`):

```json
{ "mcpServers": { "perfinsights": { "url": "http://127.0.0.1:8090/mcp" } } }
```

Then ask, e.g., "Which regressions are untriaged for sheriff test_subscription this week?".

## Serving inside Perf

A Perf instance whose config sets `"enable_insights": true` serves perfinsights under `/insights`
(the page, `/insights/trace`, `/insights/api/v1/...`, `/insights/mcp`, `/insights/llms.txt`),
with the stores perf-fe already has; see `perf/go/frontend/insights.go`. Until FIX-01 (Gate G1)
lands, only V8 autopush (`configs/spanner/v8-internal-autopush.json`) sets it. Inside Perf:

- Every request needs a signed-in user with the viewer, editor or admin role, as reported by the
  auth proxy, which is checked again in case the proxy lets a request through unchecked.
- At most 8 requests are served at once (429 otherwise), each with a 15 s deadline.
- The pages keep their own CSP, and the OpenAPI document names `/insights` as its server.
- The database pool is perf-fe's, which can write: perfinsights only uses read-only interfaces of
  the stores.

## Security model

- Read-only: the service only depends on read methods; perfgit runs with `localToProd=true`, so
  it never updates the Commits table.
- Gate G1: `sqlregression2store.GetByIDs` still interpolates ids into SQL (FIX-01). Every id is
  validated as a canonical UUID before any store call, and the server refuses to start against
  anything but the local `demo` database unless `--allow_non_demo_db` is passed. Keep it that
  way until FIX-01 lands.
- The standalone server's database connections are always read-only
  (`default_transaction_read_only=on`).
- Listens on `127.0.0.1` by default. A non-loopback `--listen` turns on `--require_auth`, which
  requires Skia auth-proxy headers (`X-WEBAUTH-USER`, viewer role). The proxy must run with
  `--allow_post` for `/mcp`, and the server must only be reachable through it. The exception is
  `--uberproxy_pen` (Cloudtop demos), which accepts `*.proxy.googlers.com` hosts without auth
  headers, because a Cloudtop's ports are only reachable through UberProxy.
- Host and Origin checks (DNS rebinding and cross-origin protection), no CORS, per-caller and
  global rate limits, a deadline on every request, bounded request and result sizes.
- Errors use the C1 envelope (`code`, `message`, `hint`); store errors become a generic
  `INTERNAL` error and are only logged.
- The landing page uses a strict CSP without inline code, and only builds the DOM with
  `textContent`.
- The MCP instructions tell models that tool results are data, not instructions.

## Limitations

- There is no anomaly-to-group lookup yet: group ids come from bugs or the Perf UI.
- `list_anomalies` and `summarize_anomalies` scan at most 2000 regressions per call, newest
  first, and filter in memory; responses say when the scan was truncated.
- Value windows come from the data stored with each regression, not from the trace store.
- No per-sheriff authorization: any viewer can read every sheriff's anomalies.
