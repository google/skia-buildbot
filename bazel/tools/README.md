# Tools

This directory defines tools that can be executed via `bazel run`. For example, it contains tools
that are invoked from `go:generate` comments in Go code (`//go:generate bazelisk run <label>`).

To make `bazel run` invocations shorter, it is recommended to define aliases in the top-level
`//BUILD.bazel` file.
