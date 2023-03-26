# SkiaPerf Server

Reads Skia performance data from databases and serves interactive dashboards
for easy exploration and annotations.

# Developing

First check out this repo and get all the dependencies [per this
README.md](../README.md), including the Cloud SDK, which is needed to run tests
locally.

In addition you will also need to install the `cockroach` binary. [Download the
full executable from
here.](https://www.cockroachlabs.com/docs/releases/v22.1#v22-1-16-downloads).

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

    make run-demo-instance

You can also view demo/test pages of a  web components by running
`demopage.sh` from the root of the repo and passing in the relative path
of the web component you want to view, for example:

    ./demopage.sh perf/modules/day-range-sk

Note you need to have `entr` installed for this to work:

    sudo apt install entr
