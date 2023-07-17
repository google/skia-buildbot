The `cd` image produced in this package encapsulates the `build-images`
executable (`./go/build-images`) which is used in numerous Louhi stages.

These stages are configured in https://louhi-config-internal.googlesource.com/skia-infra/+/refs/heads/master/.louhi/stage-types/
and used to do things like "Build a Docker container using the Bazel target //foo:bar".
