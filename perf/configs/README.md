These files are configurations for known instances of Perf.

The `local.json` config is used for local manual testing. It points to the
`integration/data` directory which is also used for unit tests.

Each file should serialize to/from a
[config.InstanceConfig](https://pkg.go.dev/go.skia.org/infra/perf/go/config?tab=doc#InstanceConfig).
See the Go struct for detailed documentation of what each field means.
