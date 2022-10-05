# User-specific Bazel configuration file

Users may create a `bazelrc` file in this directory with their personal Bazel configurations. This
file is loaded from `//.bazelrc` via a `try-import` line (https://bazel.build/run/bazelrc#imports).

Note that the `bazelrc` file is gitignored.

## The "mayberemote" configuration

One notable use case for a user-specific `bazelrc` file is to override the "mayberemote"
configuration, which is used e.g. from Makefile targets that *may* perform RBE builds with Bazel.

By default, `bazel build --config=mayberemote //path/to:target` will be equivalent to
`bazel build //path/to:target`, that is, it will perform a local (non-RBE) build. In order for the
former invocation to be equivalent to `bazel build --config=remote //path/to:target`, please create
a `bazelrc` file in this directory if you haven't already, then add the following line:

```
build:mayberemote --config=remote
```

To learn more about the "mayberemote" configuration, please read the comments in `//.bazelrc`.
