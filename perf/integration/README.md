This directory contains data used to do integration tests on Perf.

The `perf/integration/data` directory contains ingestion files in
[format.Format](https://pkg.go.dev/go.skia.org/infra/perf/go/ingest/format?tab=doc#Format)
that can be used with an ingester of type 'dir'. See `/perf/configs/local.json`.

The data files were written by `generate_data.go` which is preserved in case the
data needs to be changed/expanded.

The generated files are to be used with the
https://github.com/skia-dev/perf-demo-repo.git repo.

The integration data set has 9 good files, 1 file with a bad commit, and 1
malformed JSON file.
