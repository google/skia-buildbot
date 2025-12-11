# SkiaPerf Server

The Performance Dashboard reads performance data from databases and serves
interactive dashboards to highlight how a commit impacts performance, allowing
for easy exploration and annotation.

The product is particularly useful for:

- **Chrome Developers** who are interested in performance regressions.
- **Marketing Teams** who need to track, visualize, and communicate product's
  performance enhancements.

# Table of Contents

- [Developing](#developing)
  - [First Time Setup](#first-time-setup)
  - [Build](#build)
  - [Test](#test)
  - [Running locally](#running-locally)
  - [Troubleshooting](#troubleshooting)
  - [Creating a CL](#creating-a-cl)
- [Debugging and Profiling](#debugging-and-profiling)
- [About](#about)
  - [Production Instances](#production-instances)
  - [Terminology](#terminology)
  - [History](#history)
- [Other Documentation](#other-documentation)

# Developing

## First Time Setup

First check out this repo and get all the dependencies as described in the
[top-level README.md](../README.md), including the Cloud SDK, which is needed to
run tests locally.

## Build

All building and testing is done by Bazel (optionally wrapped with Bazelisk),
but there is a Makefile that records regularly used commands.

To build the full project:

```
bazelisk build --config=mayberemote //perf/...
```

## Test

To run all the tests:

```
bazelisk test --config=mayberemote //perf/...
```

Note the first time you run this it will fail and inform you of the gcloud
simulators that need to be running in the background and how to start them.

Most of the time, you will need a local spanner instance.
To set it up, run the following make command, and export the following:

```
make run-spanner-emulator
export PGADAPTER_HOST=localhost:5432
export SPANNER_EMULATOR_HOST=localhost:9010
```

Those are specific for this project, and top level scripts won't start them.

### Test parameters

- `--test_output=streamed`: A blaze flag forcing tests to run locally and display logs directly to your terminal as they happen.
- `--test_arg=--no-single-run`: Passes --no-single-run to the Karma test runner. It keeps the Karma server running after the initial test execution, useful for debugging in a browser.
- `--test_arg=--auto-watch`: Passes --auto-watch to Karma. Makes Karma watch for file changes and automatically re-run tests when changes are detected.

### Debug Karma tests

1. Run a test with the debug arguments
   `bazelisk test //perf/modules/regressions-page-sk:regressions-page-sk_test --test_output=streamed --test_arg=--no-single-run --test_arg=--auto-watch`
2. Open the application in your browser on port `9876`. [Screenshot 1](https://screenshot.googleplex.com/7K5Jr4PBz5hCB3d.png).
3. See assertion errors in the console if any. Find your sources, add a debug breakpoint. Rerun the test by refreshing the page. [Screenshot 2](https://screenshot.googleplex.com/Bq4QfPLMrBdAbyT.png).

## Running locally

There are several ways to run Perf locally depending on your needs.

### Running with a local demo database

This is the most common way to run Perf for frontend development. It uses a
local database with pre-populated demo data.

- To run with a fresh database (demo data is re-ingested on every run):

  ```
  make run-demo-instance
  ```

- To run without tearing down the database between runs:

  ```
  make run-demo-instance-db-persist
  ```

After the server starts, navigate to http://localhost:8002.

### Running against a production database

For backend changes, you may want to run your local instance against a copy of a
production database.

```
./run_with_spanner.sh p=<project> i=<instance> d=<database> config=<config_path>
```

For example, to run against the internal Chrome perf instance:

```
./run_with_spanner.sh p=skia-infra-corp i=tfgen-spanid-20241205020733610 d=chrome_int config=./configs/spanner/chrome-internal.json
```

or internal v8 perf instance (loads faster):

```
./run_with_spanner.sh p=skia-infra-corp i=tfgen-spanid-20241205020733610 d=v8_int config=./configs/spanner/v8-internal.json
```

**Parameters:**

- `p`: The GCP project. `skia-infra-corp` for internal instances,
  `skia-infra-public` for public ones.
- `i`: The Spanner instance ID.
- `d`: The Spanner database name.
- `config`: Path to the instance configuration file. See `perf/configs/` for
  examples.

### Rebuilding frontend only (for faster development)

You don't need to restart the Go server each time you made a change to web pages.
Rebuild them on the fly with

```
bazelisk build --config=mayberemote -c dbg //perf/pages/...
```

### Running with authentication

If you are working on features that require login, you can run a local
authentication proxy. This proxy will sit in front of the Perf server and handle
authentication before forwarding requests to it. To start this setup:

```
make run-auth-proxy-before-demo-instance
```

Keep it running. In a separate terminal run the server
[as described above](#running-against-a-production-database).

After the server starts, navigate to http://localhost:8003.

### Running individual component demos

You can view demo/test pages of a web components by running `demopage.sh` from
the root of the repo and passing in the relative path of the web component you
want to view, for example:

```
./demopage.sh perf/modules/day-range-sk
```

Additionally, the remote backend can be reverse-proxied such that the demo page
server will forward APIs under `/_/` to the remote backend
(`ENV_REMOTE_ENDPOINT`)

```
ENV_REMOTE_ENDPOINT='https://v8-perf.skia.org' ./demopage.sh perf/modules/day-range-sk
```

or

```
ENV_REMOTE_ENDPOINT='https://v8-perf.skia.org' bazelisk run //perf/modules/plot-summary-sk:demo_page_server
```

This will allow the demo page to fetch the real data.

Note you need to have `entr` installed for this to work:

```
sudo apt install entr
```

### Troubleshooting

- If you see errors related to ptrace scope, you may need to run:

  ```
  echo 0 | sudo tee /proc/sys/kernel/yama/ptrace_scope
  ```

- If you get `INVALID_ARGUMENT: Invalid credentials path specified:
/acct_credentials.json`, check your gcloud configuration and make sure you
  are logged in:

  ```
  gcloud auth application-default login
  ```

  (don't confuse this with `gcloud auth login` - it generates credentials to be used solely by
  gcloud. `gcloud auth application-default login` generates credentials to be used by Client
  Libraries in general).

## Adding External Dependencies

This project uses Bazel to manage dependencies, with `pnpm` to handle Node.js packages.

1.  **Add the Dependency to `package.json`**

    Manually add the new package to the `dependencies` section of the root `package.json` file.

2.  **Update the Lockfile with `pnpm`**

    This project uses `pnpm` to manage the Node.js dependency tree. It is critical to use `pnpm`
    instead of `npm` to ensure the `pnpm-lock.yaml` file is correctly updated. Bazel's rules are
    configured to read this specific lockfile.

    If you do not have `pnpm` installed, you can install it with `npm`:

    ```
    npm install -g pnpm@8
    ```

    Run from the root of the repository:

    ```
    pnpm install
    ```

    This will update `pnpm-lock.yaml` with the new dependency.

3.  **Synchronize Bazel's Dependency Graph**

    ```
    bazelisk sync --only=npm
    ```

4.  **Reference the New Dependency in `BUILD.bazel`**

    You can now use the new package as a dependency in your `BUILD.bazel` files.

    ```bazel
    [ "//:node_modules/package-name", ],
    ```

## Creating a CL

To create a new CL (Change List) in Gerrit, follow these steps. For more
detailed information and best practices, please refer to the
[Skia Gerrit documentation](https://skia.org/docs/dev/contrib/submit/).

1.  Create a new branch from `origin/main`:

    ```
    git checkout -b <your-branch-name> -t origin/main
    ```

2.  Make your changes and commit them.

3.  Upload for review:

    ```
    git cl upload
    ```

### Pulling changes from the CL

Use `git cl patch <change_number`.

## VSCode extensions

The following VSCode extensions might prove useful for development:

- [Golang support - Go](https://marketplace.visualstudio.com/items?itemName=golang.Go)
- [Bazel](https://marketplace.visualstudio.com/items?itemName=BazelBuild.vscode-bazel)
- [Formatter - Prettier](https://marketplace.visualstudio.com/items?itemName=esbenp.prettier-vscode)
- [lit-plugin](https://marketplace.visualstudio.com/items?itemName=runem.lit-plugin)

See [top-level STYLEGUIDE.md](../STYLEGUIDE.md) for information about registering
components so that lit-plugin sees them.

## Cider-G Workspace Setup

Please go to the [go/browser-perf-engprod](http://go/browser-perf-engprod).

# Debugging and Profiling

When running the application locally, a debug server is also started on port 9000. This server exposes Go's standard `pprof` profiling data, which is useful
for debugging performance issues and memory leaks.

To get meaningful profiles, you may want to generate some load on the server
first, for example by loading several different graphs in the UI.

### Profiling with pprof

You can connect to the profiling endpoints using `go tool pprof`. For example,
to inspect the heap profile:

```
go tool pprof "http://localhost:9000/debug/pprof/heap"
```

Once in the `pprof` interactive console, you can use commands like `top` to see
the top memory consumers or `web` to visualize the call graph.

For internal use, you can also use `pprof` to visualize the profile as a flame
graph and compare it with another profile:

```
pprof -flame -symbolize=none (profile)
```

### Checking for Goroutine Leaks

To get a full stack dump of all running goroutines, which can help identify
leaks, visit the following URL in your browser or use `curl`:

```
http://localhost:9000/debug/pprof/goroutine?debug=2
```

# About

## Production Instances

The following are some example production instances. For the complete list,
refer to the configurations in the `perf/configs/` directory.

- **Chrome Perf**: https://chrome-perf.corp.goog/m/
- **Skia Perf**: https://skia-perf.luci.app/
- **V8 Perf**: https://v8-perf.skia.org/
- ...

## Terminology

- **Benchmark**: A top-level test name.
- **Test**: A specific test case within a benchmark.
- **Subtest**: A further breakdown of a test.
- **Bot**: The device or machine that runs the tests.
- **Trace**: A single line on a graph, representing measurements for a single
  test over time. A trace has a unique key, which is a combination of its
  properties (e.g., benchmark, test, subtest, bot, etc.).
- **Traceset**: The set of key-value pairs that uniquely identifies a trace.
- **X-axis**: Always represents commit position or timestamp.
- **Anomaly**: A statistically significant change in a trace, which could be a
  regression or an improvement.
- **Frame**: A chunk of trace data stored in the database.
- **Sheriff**: A person or tool responsible for monitoring a set of tests for
  regressions.
- **ChromePerf**: The legacy implementation of Perf.
- **Catapult**: The repository for the legacy Perf implementation.

See also
[Chromium Infra Glossary](https://chromium.googlesource.com/chromium/src/+/HEAD/docs/infra/glossary.md).

## History

The current Perf infrastructure in this repo was originally developed for Skia.
In 2023, a project began to unify it with Chrome's performance tooling,
replacing a legacy Python-based system. This unification effort involves
consolidating features from both platforms onto this modern Go and TypeScript
stack, with the goal of eventually deprecating the older system.

# Other Documentation

| File                                                | Description                                                      |
| --------------------------------------------------- | ---------------------------------------------------------------- |
| [`ai_generated_doc.md`](./docs/ai_generated_doc.md) | Overview of the system by Gemini                                 |
| [`API.md`](./API.md)                                | How to use the HTTP/JSON API for alerts.                         |
| [`BACKUPS.md`](./BACKUPS.md)                        | Instructions for backing up regression and alert data.           |
| [`CHECKLIST.md`](./CHECKLIST.md)                    | A checklist for launching a new Perf instance.                   |
| [`DESIGN.md`](./DESIGN.md)                          | The design documentation for Perf.                               |
| [`FORMAT.md`](./FORMAT.md)                          | Details on the Skia Perf JSON data format.                       |
| [`PERFSERVER.md`](./PERFSERVER.md)                  | Documentation for the `perfserver` command-line tool.            |
| [`PERFTOOL.md`](./PERFTOOL.md)                      | Documentation for the `perf-tool` command-line tool.             |
| [`PROD.md`](./PROD.md)                              | A manual for operating Perf in a production environment.         |
| [`Spanner.md`](./Spanner.md)                        | Information on the Spanner integration and running the emulator. |
| [`TRIAGE.md`](./TRIAGE.md)                          | Design for the regression triage page.                           |
