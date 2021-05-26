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

Modify `run.sh` to list the files of interest, you can use wildcards in the
gsutil command when specifying files, for example, to process all files of May
24, 2021 you would use:

    gsutil ls "gs://skia-perf/nano-json-v1/2021/05/24/**"

By default `main.go` will only process traces of source_type=skp and
sub_result=min_ms. Modify `main.go` if you need to analyze other traces.

Then:

    $ ./run.sh

The output will CSV files placed in `/tmp/variance-XXXX/`:

- `variance.csv` - The full set of data.
- `sorted.csv` - Which is the full set of data sorted in numerical descending
  order of $ratio.
- `short-sorted.csv` - The first 100 entries of `sorted.csv`.
