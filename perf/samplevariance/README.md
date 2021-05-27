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

If you want to specify the files then pass in a GCS path via the --prefix flag:

    $ bazel run :samplevariance -- --prefix=gs://skia-perf/nano-json-v1/2021/05/23/02/

## Usage

    $  bazel run :samplevariance samplevariance -- -h
    Usage of samplevariance:
      -alsologtostderr
            log to standard error as well as files
      -filter string
            A query to filter the traces. (default "source_type=skp&sub_result=min_ms")
      -log_backtrace_at value
            when logging hits line file:N, emit a stack trace
      -log_dir string
            If non-empty, write log files in this directory
      -logtostderr
            log to standard error instead of files
      -out string
            Output filename. If not supplied then CSV file is written to stdout.
      -prefix string
            GCS location to search for files. E.g. gs://skia-perf/nano-json-v1/2021/05/23/02/. If not
            supplied then all the files from yesterday are queried.
      -stderrthreshold value
            logs at or above this threshold go to stderr
      -top int
            The top number of CSV rows to report. Set to -1 to return all of them. (default 100)
      -v value
            log level for V logs
      -vmodule value
            comma-separated list of pattern=N settings for file-filtered logging
