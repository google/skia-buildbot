# Gold Correctness

For information on setting up or using Gold, see [these docs](docs/README.md).

For an architectural overview, see:
<https://docs.google.com/document/d/1U7eBzYrZCPx24Lp9JH2scKj3G8Gr8GRtQJZRhdigyRQ/edit>

To run Gold locally, run a local target defined in BUILD.bazel.
E.g. `bazel run //golden:skia_infra_local`. Then run `make run_auth_proxy_before_local_instance`.
You can then access the local Gold instance through http://localhost:8003 with the current
user authenticated.

## Backend Storage

Gold uses [CockroachDB](https://www.cockroachlabs.com/get-cockroachdb/) to store all data necessary
for running the backend servers. (Caveat: We are in the middle of a migration towards this goal.)

For production-specific advice, see docs/PROD.md.
