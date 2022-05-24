# External Bazel repositories

This directory contains Go functions to access binaries and other files located in
[external Bazel repositories](https://bazel.build/docs/external) (i.e. those defined in the
`//WORKSPACE` file), such as files in CIPD packages, the `go` binary provided by
[rules_go](https://github.com/bazelbuild/rules_go), etc.
