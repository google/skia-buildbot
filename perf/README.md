# SkiaPerf Server

Reads Skia performance data from databases and serves interactive dashboards
for easy exploration and annotations.

# Developing

First check out this repo and get all the dependencies [per this
README.md](../README.md), including the Cloud SDK, which is needed to run tests
locally.

All building and testing is done by Bazel, but there is a Makefile
that records regularly used commands.

## Build

To build the full project:

    bazelisk build --config=mayberemote //perf/...

## Test

To run all the tests:

    bazelisk test --config=mayberemote //perf/...

Note the first time you run this it will fail and inform you of the gcloud
simulators that need to be running in the background and how to start them.

## Running locally

To run a local instance of Perf against a fake dataset:

1.  Run with a fresh database (demo data ingested)

        make run-demo-instance

2.  Run without tearing down database (expects a running database on the machine)

        make run-demo-instance-db-persist

After the local server has started, navigate to http://localhost:8002 in a
browser.

If you are interested in reading more about the local database, refer to [this doc](Spanner.md)

You can also view demo/test pages of a web components by running
`demopage.sh` from the root of the repo and passing in the relative path
of the web component you want to view, for example:

    ./demopage.sh perf/modules/day-range-sk

Additionally, the remote backend can be reverse-proxied such that the demo page
server will forward APIs under `/_/` to the remote backend (`ENV_REMOTE_ENDPOINT`)

    ENV_REMOTE_ENDPOINT='https://v8-perf.skia.org' ./demopage.sh perf/modules/day-range-sk

or

    ENV_REMOTE_ENDPOINT='https://v8-perf.skia.org' bazelisk run //perf/modules/plot-summary-sk:demo_page_server

This will allow the demo page to fetch the real data.

Note you need to have `entr` installed for this to work:

    sudo apt install entr
