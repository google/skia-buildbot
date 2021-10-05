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

## Upgrading to a new Bazel version or rebuilding the toolchain container

Take the following steps to upgrade to a new Bazel version or if rolling out a new version
of the toolchain, e.g. rebuilding to include updated tools.

### Step 1 - Making changes

#### Updating Bazel Version
If necessary, update the `//.bazelversion` file with the new Bazel version. This file is read by
[Bazelisk](https://github.com/bazelbuild/bazelisk) (for those engineers who use `bazelisk`
as a replacement for `bazel`).

If not updating Bazel, merely note what version is there.

#### Rebuild Toolchain Container
If necessary, make changes to the `rbe_container_skia_infra` rule in `//BUILD.bazel` and
run `bazel run :push_rbe_container_skia_infra` from the root directory. Make note of the tagged
name. Identify the sha256 hash of the image using `docker pull <TAGGED IMAGE>`.

If not rebuilding the container, merely note what sha256 hash is specified in ./config/BUILD
under container-version.

### Step 2 - Generate Bazel Files

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
      --toolchain_container=gcr.io/skia-public/rbe-container-skia-infra@sha256:5418d876cd8bb7cdbd05b834254038744d15f2f38327dfa46ff8dc9f83355260 \
      --output_src_root=$HOME/buildbot \
      --output_config_path=bazel/rbe \
      --exec_os=linux \
      --target_os=linux
```

If `rbe_configs_gen` fails, try deleting all files under `//bazel/rbe` (except for this file) and
re-run `rbe_configs_gen`.

### Step 3 - Cleanup

Run `bazel run //:buildifier` and fix any linting errors on the generated files (e.g. move load
statements at the top of the file, etc.)

### Step 4 - Manual Updates

If updating the Bazel version, update the [bazel-toolchains](https://github.com/bazelbuild/bazel-toolchains)
repository version imported from `//WORKSPACE` to match the new Bazel version.

If updating the image version, the generated changes should have properly updated
`//bazel/rbe/config/BUILD` to refer to the new image, so there is nothing else to do.

### Step 5 - Test out the changes
Try running various bazel commands with `--config remote` to make use of the new image.

When uploading the change as a CL, any tasks with the `-RBE` suffix will automatically use the
image specified in `//bazel/rbe/config/BUILD`.

## How to install the `rbe_configs_gen` CLI tool

Clone the [bazel-toolchains](https://github.com/bazelbuild/bazel-toolchains) repository outside of
the Buildbot repository checkout, build the `rbe_configs_gen` binary, and place it on your $PATH:

```
$ git clone https://github.com/bazelbuild/bazel-toolchains

$ cd bazel-toolchains

# This assumes that $HOME/bin is in your $PATH.
$ go build -o $HOME/bin/rbe_configs_gen ./cmd/rbe_configs_gen/rbe_configs_gen.go
```
