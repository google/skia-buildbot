# Bazel examples

This directory contains examples of tests where the test environment is modified in various ways.

# Test wrapped by a runner script

```
$ bazel test //switchboard/bazelexamples/wrapper:wrapper_test
```

This example shows how one might execute arbitrary commands before or after running a `go_test`.

# Running a test alongside another binary

```
$ bazel test //switchboard/bazelexamples/test_on_env:test_on_env_test
```

This example leverages the `test_on_env` rule to run a binary (i.e. the test environment) alongside
a `go_test`.
