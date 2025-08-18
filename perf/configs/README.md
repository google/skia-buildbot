These files are configurations for known instances of Perf.

The `local.json` config is used for local manual testing. It points to the
`integration/data` directory which is also used for unit tests.

Each file should serialize to/from a
[config.InstanceConfig](https://pkg.go.dev/go.skia.org/infra/perf/go/config?tab=doc#InstanceConfig).
See the Go struct for detailed documentation of what each field means.

`chrome-internal-lts` is a manually pushed instance based on a specific hash.
This acts as a stable version used for backup purposes.
See: https://skia.googlesource.com/k8s-config/+/refs/heads/main/pipeline/stages.json
