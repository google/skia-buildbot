# SampleVariance

A Go program and a bash script for analyzing sample variance of nanobench
results stored in Google Cloud Storage (GCS). The nanobench program usually
stores 10 samples from each run of a benchmark, and this tool allows looking at
those samples across a wide number of runs stored on GCS.

## Prerequisites

- Read-only access to the GCS bucket.
- The following installed:

  - Go
  - gsutil
  - miller

You can usually install miller via:

    sudo apt install miller

## Running

By default this will run over all JSON files from yesterday:

    $ bazel run //perf/samplevariance:run

If you want to specify the files then pass in a GCS path with wildcards as an
argument:

    $ bazel run //perf/samplevariance:run -- "gs://skia-perf/nano-json-v1/2021/05/23/02/**"

By default `main.go` will only process traces of source_type=skp and
sub_result=min_ms. Modify `main.go` if you need to analyze other traces.

The output will CSV files placed in `/tmp/variance-XXXX/`:

- `variance.csv` - The full set of data.
- `sorted.csv` - Which is the full set of data sorted in numerical descending
  order of $ratio.
- `short-sorted.csv` - The first 100 entries of `sorted.csv`.
