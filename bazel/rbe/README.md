# Bazel toolchain configuration for RBE

---

**DO NOT EDIT THIS DIRECTORY BY HAND.**

All files in this directory (excluding this file) are generated with the `rbe_configs_gen` CLI
tool. Keep reading for details.

---

This directory contains a Bazel toolchain configuration for RBE. It is generated with the
`rbe_configs_gen` CLI tool from the
[bazel-toolchains](https://github.com/bazelbuild/bazel-toolchains) repository.

This directory is referenced from `//.bazelrc`.

## Upgrading to a new Bazel version

Take the following steps to upgrade to a new Bazel version.

### Step 1

Update the `//.bazelversion` file with the new Bazel version. This file is read by
[Bazelisk](https://github.com/bazelbuild/bazelisk) (for those engineers who use `bazelisk`
as a replacement for `bazel`).

### Step 2

Regenerate the `//bazel/rbe` directory with the `rbe_configs_gen` CLI tool (installation
instructions below):

```
# Replace the <PLACEHOLDERS> as needed.
$ rbe_configs_gen \
      --bazel_version=<BAZEL VERSION> \
      --toolchain_container=gcr.io/skia-public/rbe-container-skia-infra@sha256:<HASH OF MOST RECENT IMAGE> \
      --output_src_root=<PATH TO REPOSITORY CHECKOUT> \
      --output_config_path=bazel/rbe \
      --exec_os=linux \
      --target_os=linux
```

Example:

```
$ rbe_configs_gen \
      --bazel_version=4.1.0 \
      --toolchain_container=gcr.io/skia-public/rbe-container-skia-infra@sha256:5deebf6be17a276a44580301daacdbb580c0b2b25eb4b0dea7e51f8011ec12ff \
      --output_src_root=$HOME/buildbot \
      --output_config_path=bazel/rbe \
      --exec_os=linux \
      --target_os=linux
```

If `rbe_configs_gen` fails, try deleting all files under `//bazel/rbe` (except for this file) and
re-run `rbe_configs_gen`.

### Step 3

Run `bazel run //:buildifier` and fix any linting errors on the generated files (e.g. move load
statements at the top of the file, etc.)

### Step 4

Update the [bazel-toolchains](https://github.com/bazelbuild/bazel-toolchains) repository version
imported from `//WORKSPACE` to match the new Bazel version.

## How to install the `rbe_configs_gen` CLI tool

Clone the [bazel-toolchains](https://github.com/bazelbuild/bazel-toolchains) repository outside of
the Buildbot repository checkout, build the `rbe_configs_gen` binary, and place it on your $PATH:

```
$ git clone https://github.com/bazelbuild/bazel-toolchains

$ cd bazel-toolchains

# This assumes that $HOME/bin is in your $PATH.
$ go build -o $HOME/bin/rbe_configs_gen ./cmd/rbe_configs_gen/rbe_configs_gen.go
```
