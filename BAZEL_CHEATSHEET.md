# Bazel cheatsheet

This cheatsheet provides tips on how to build and test software using Bazel.

## Building

How to build a specific target:

```
$ bazel build //go/util:util
```

How to build all targets under a specific directory, and all subdirectories:

```
$ bazel build //go/util/...
```

How to build all code in the workspace:

```
$ bazel build ...
```

### Building on RBE

By default, Bazel will build targets on the host system. To build on RBE, add flag `--config=remote`
to your `bazel build` invocation, e.g.:

```
$ bazel build ... --config=remote
```

## Testing

The tips below include examples using `go_test` targets, but most Bazel flags mentioned here apply
to other types of targets as well (e.g. `python_test`, `karma_test`, `sk_element_puppeteer_test`,
`nodejs_mocha_test`, etc.).

### Running tests

How to run a specific test target:

```
$ bazel test //go/util:util_test
```

How to run all test targets under a specific directory, and all subdirectories:

```
$ bazel test //go/...
```

How to run all tests in the workspace:

```
$ bazel test ...
```

### Running tests on RBE

By default, Bazel will run test targets on the host system. To run test targets on RBE, add flag
`--config=remote` to your `bazel test` invocation, e.g.:

```
$ bazel test ... --config=remote
```

### Test output

Bazel does not print the output of a test (e.g. any `fmt.Printf` statements) unless flag
`--test_output=all` is passed (e.g. `bazel test //go/util/... --test_output=all`). This will run all
requested targets in parallel, and print out their outputs after all tests finish running.

As an alternative, `--test_output=streamed` will print out the test output in real time, rather than
waiting until the test finishes running. However, if the `bazel test` invocation targets multiple
tests, this will force tests to execute one at a time.

### Caching

Bazel caches successful test runs, and reports `(cached) PASSED` on subsequent `bazel test`
invocations, e.g.:

```
$ bazel test //go/util:util_test
...
//go/util:util_test                                                      PASSED in 0.1s

$ bazel test //go/util:util_test
...
//go/util:util_test                                             (cached) PASSED in 0.1s
```

To disable caching, use flag `--nocache_test_results`, e.g.

```
$ bazel test //go/util:util_test
...
//go/util:util_test                                             (cached) PASSED in 0.1s

$ bazel test //go/util:util_test --nocache_test_results
...
//go/util:util_test                                                      PASSED in 0.1s
```

### Flaky tests

While `--nocache_test_results` can be useful for debugging flaky tests, flag `--runs_per_test` was
specifically added for this purpose. Example:

```
$ bazel test //path/to:my_flaky_test --runs_per_test=10
...
//path/to:my_flaky_test                                                 FAILED in 4 out of 10 in 0.1s
```

### Test timeouts

By default, Bazel will report test timeout if the test does not finish within 5 minutes. This can be
overridden via the `--test_timeout` flag, e.g.

`$ bazel test //go/util:util_test --test_timeout=20`

This can also be overridden via the `timeout` and `size` arguments of the test target, e.g.

```
go_test(
    name = "my_test",
    srcs = ["my_test.go"],
    timeout = "long",
    ....
)
```

See the
[documentation](https://docs.bazel.build/versions/master/be/common-definitions.html#test.timeout)
for more.

### Custom command-line flags

Some of our `go_test` targets defines custom command-line flags
(e.g `flag.Bool("logtostderr", ...)`), either directly or via their dependencies. Such flags can be
passed to a Bazel test target via the `--test_arg` flag, e.g.

```
$ bazel test //go/util:util_test --test_arg=-logtostderr
```

As an alternative, command-line flags can be specified via the `args` argument of the Bazel test
target, as follows:

```
go_test(
    name = "my_test",
    srcs = ["my_test.go],
    args = ["-logtostderr"],
    ...
)
```

See the
[documentation](https://docs.bazel.build/versions/master/be/common-definitions.html#test.args) for
more.

### `go test` flags under Bazel

The `go test` command supports flags such as `-v` to print verbose outputs, `-run` to run a specific
test case, etc. These flags can be passed to a `go_test` test target via `--test_arg`, but need to
be prefixed with `-test.`, e.g.:

```
$ bazel test //go/util:util_test --test_arg=-test.v            # Equivalent to "go test -v"

$ bazel test //go/util:util_test --test_arg=-test.run=TestFoo  # Equivalent to "go test -run=TestFoo"
```

### Overriding environment variables

Use flag `--test_env` to specify any environment variables, e.g.

```
$ bazel test //path/to:my_cockroachdb_test --test_env=COCKROACHDB_EMULATOR_STORE_DIR=/tmp/crdb
```

To pipe through an environment variable from the host's system:

```
$ export COCKROACHDB_EMULATOR_STORE_DIR=/tmp/crdb
$ bazel test //path/to:my_cockroachdb_test --test_env=COCKROACHDB_EMULATOR_STORE_DIR
```

### Example `bazel test` invocation for Go tests

The following example shows what a typical `bazel test` invocation might look like while debugging
a `go_test` target locally.

```
# Equivalent to "$ MY_ENV_VAR=foo go test ./go/my_pkg -v -logtostderr"
$ bazel test //go/my_pkg:my_pkg_test \
             --test_output=streamed \
             --nocache_test_results \
             --test_arg=-test.v \
             --test_arg=-logtostderr \
             --test_env=MY_ENV_VAR=foo
```
