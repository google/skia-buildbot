# Skia Perf developer guide

## Table of contents

- [First Time Setup](#first-time-setup)
- [Build](#build)
- [Test](#test)
- [Running locally](#running-locally)
- [Troubleshooting](#troubleshooting)
- [Creating a CL](#creating-a-cl)
- [Debugging and Profiling](#debugging-and-profiling)

---

## First Time Setup

First check out this repo and get all the dependencies as described in the
[top-level README.md](../../README.md), including the Cloud SDK, which is needed to
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

- `--test_output=streamed`: A bazel flag forcing tests to run locally and display logs directly to your terminal as they happen.
- `--test_arg=--no-single-run`: Passes --no-single-run to the Karma test runner. It keeps the Karma server running after the initial test execution, useful for debugging in a browser.
- `--test_arg=--auto-watch`: Passes --auto-watch to Karma. Makes Karma watch for file changes and automatically re-run tests when changes are detected.

### Debug Karma tests

1. Run a test with the debug arguments
   `bazelisk test //perf/modules/regressions-page-sk:regressions-page-sk_test --test_output=streamed --test_arg=--no-single-run --test_arg=--auto-watch`
2. Open the application in your browser on port `9876`. [Screenshot 1](https://screenshot.googleplex.com/7K5Jr4PBz5hCB3d.png).
3. See assertion errors in the console if any. Find your sources, add a debug breakpoint. Rerun the test by refreshing the page. [Screenshot 2](https://screenshot.googleplex.com/Bq4QfPLMrBdAbyT.png).

### Debug Puppeteer tests

Through screenshots:

1. Add `await takeScreenshot(testBed.page, 'test_name', 'step_name');` from `puppeteer-tests/util`
2. After test run copy screenshots from the remote machine to local machine, unzip it. Example of query:
   `scp -r $USER@$USER.c.googlers.com:/usr/local/google/home/$USER/.cache/bazel/_bazel_$USER/b94e5a721a59a936c04032522dbb25a3/execroot/_main/bazel-out/k8-fastbuild/testlogs/perf/modules/explore-multi-sk/explore-multi-sk_puppeteer_test/test.outputs/ ~/Downloads  && unzip ~/Downloads/test.outputs/outputs.zip -d ~/Downloads`

   You can find out the output path by running: `bazelisk info output_path`

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
production database. The startup script will automatically check for valid Google Cloud Application Default Credentials (ADC) and prompt you to log in if they are missing or expired. It also automatically spins up the local authentication proxy in the background.

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

After the server starts, navigate to http://localhost:8003.

### Rebuilding frontend only (for faster development)

You don't need to restart the Go server each time you made a change to web pages.
Rebuild them on the fly with

```
bazelisk build --config=mayberemote -c dbg //perf/pages/...
```

#### Hot Reloading

To automatically rebuild the frontend and refresh your browser when files change, you can use the hot-reload script. First, ensure you have `entr` installed (`sudo apt install entr`), then run:

```
./perf/hot-reload.sh
```

When you edit a file in `perf/modules`, the script will automatically rebuild the pages and trigger either a CSS hot-swap or a full page reload in the DevMode browser.

If `entr` does not work in your environment (e.g. over NFS or within some VMs, or inside a Cider-G workspace), the script will automatically fallback to polling mode. You can also explicitly run the script in polling mode by providing the `--poll` or `-p` argument:

```
./perf/hot-reload.sh --poll
```

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
ENV_REMOTE_ENDPOINT='[https://v8-perf.skia.org](https://v8-perf.skia.org)' ./demopage.sh perf/modules/day-range-sk
```

or

```
ENV_REMOTE_ENDPOINT='[https://v8-perf.skia.org](https://v8-perf.skia.org)' bazelisk run //perf/modules/plot-summary-sk:demo_page_server
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

- **Google Cloud Credentials (ADC) Issues:**
  The `run_with_spanner.sh` script automatically checks for valid credentials and will prompt you to log in if they are missing or expired. However, if you still encounter errors like `INVALID_ARGUMENT: Invalid credentials path specified: /acct_credentials.json`, you can manually force a new login:

  ```
  gcloud auth application-default login
  ```

  _(Note: don't confuse this with `gcloud auth login` - that generates credentials solely for the gcloud CLI. `gcloud auth application-default login` generates credentials used by Client Libraries.)_

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

See [top-level STYLEGUIDE.md](../../STYLEGUIDE.md) for information about registering
components so that lit-plugin sees them.

## Cider-G Workspace Setup

Please go to the [go/browser-perf-engprod](http://go/browser-perf-engprod).

## Debugging and Profiling

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
