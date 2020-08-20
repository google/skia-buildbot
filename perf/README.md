# SkiaPerf Server

Reads Skia performance data from databases and serves interactive dashboards
for easy exploration and annotations.

# Developing

The easiest way to develop should be to do:

    go get -u go.skia.org/infra/perf/go/...

Which will fetch the repository into the right location and
download dependencies.

Then to build everything:

    cd $GOPATH/src/go.skia.org/infra/perf
    make

Make sure the \$GOPATH/bin is on your path so that you can easily run the
executables after they are generated.

The tests require the CockroachDB emulator to be running. To run the tests:

    make test

To run the application locally:

    perfserver frontend --local --config_filename=./configs/local.json \
      --do_clustering=false --noemail --port=:8001
