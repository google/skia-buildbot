# SampleVariance

A Go program for analyzing sample variance of nanobench results stored in Google
Cloud Storage (GCS). The nanobench program usually stores 10 samples from each
run of a benchmark, and this tool allows looking at those samples across a wide
number of runs stored on GCS.

## Prerequisites

- Read-only access to the GCS bucket.

## Running

By default this will run over all JSON files from yesterday:

    $ bazel run :samplevariance -- --logtostderr > top100.csv

If you want to specify the files, then pass in a GCS path via the --prefix flag:

    $ bazel run :samplevariance -- --prefix=gs://skia-perf/nano-json-v1/2021/05/23/02/

You can also change the tests being considered using the --filter flag:

    $ bazel run :samplevariance -- --filter=source_type=bench&sub_result=min_ms

See `bazel run :samplevariance -- --help` for all the options.
