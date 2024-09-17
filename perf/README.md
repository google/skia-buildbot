# SkiaPerf Server

Reads Skia performance data from databases and serves interactive dashboards
for easy exploration and annotations.

# Developing

First check out this repo and get all the dependencies [per this
README.md](../README.md), including the Cloud SDK, which is needed to run tests
locally.

In addition you will also need to install the full executable `cockroach`. In order to successfully install and work on the command, here are a set of cautions when installing coakroach on your machine:

- Make sure to download the Full Binary version : https://www.cockroachlabs.com/docs/releases#v22-1. Choose the v22-1 full binary version for compatibility with Schema validation on your machine. Linux version is for cloudtop, and ARM 64-bit is for Mac.

  - After downloading the cockroach version, we extract the download archive anywhere(default is from Downloads), and then unzip the archive file to export that directory into the path variable.

  - Alternative, we can unzip the downloaded cockroach file, open the lib folder, and copy the libgeos.so and libgeos_c.so files from current path into the /usr/local/lib/cockroach folder, eg.

        cp -i /usr/local/google/home/username/Downloads/cockroach-v22.1.22.linux-amd64/lib/libgeos.so /usr/local/lib/cockroach

  - Then export path

        export PATH=$PATH:/path/to/cockroach

  - Then add this path into the .bashrc file. Also make sure to source the .bashrc file under the current path to refresh the active terminal, or open a new terminal.

  - After that, run

        cockroach start-single-node --insecure ---listen-addr=127.0.0.1:26257

    (nit: listen-address is set by default)

  - Then type dt, you will see a list of local tables with no data.

  - To confirm that the cockroach installation has been completed, we should open a SQL shell to this running database, by running

        cockroach sql --insecure --host=localhost:26257

  - Moreover, we can run the local instance of Perf to prove that it is working as intended:

        make run-demo-chromeperf

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

You can also view demo/test pages of a web components by running
`demopage.sh` from the root of the repo and passing in the relative path
of the web component you want to view, for example:

    ./demopage.sh perf/modules/day-range-sk

Note you need to have `entr` installed for this to work:

    sudo apt install entr
