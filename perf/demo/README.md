This directory contains data used to demo Perf.

The `perf/demo/data` directory contains ingestion files in
[format.Format](https://pkg.go.dev/go.skia.org/infra/perf/go/ingest/format?tab=doc#Format)
that can be used with an ingester of type 'dir'. See `/perf/configs/demo.json`.

The data files were written by `generate_data.go` which is preserved in case the
data needs to be changed/expanded.

The generated files are to be used with the
https://github.com/skia-dev/perf-demo-repo.git repo.
