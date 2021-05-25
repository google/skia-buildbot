# SampleVariance

A Go program and a bash script for analyzing sample variance of nanobench
results stored in Google Cloud Storage (GCS).

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

    readarray -t files <<< $(gsutil ls "gs://skia-perf/nano-json-v1/2021/05/24/**")

Then:

    $ ./run.sh

The output will CSV files placed in `/tmp/variance-XXXX/`.
