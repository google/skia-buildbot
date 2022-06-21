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

Take the following steps to upgrade to a new Bazel version or if rolling out a new version of the
toolchain, e.g. rebuilding to include updated tools.

### Step 1 - Making changes

#### Updating Bazel Version
If necessary, update the `//.bazelversion` file with the new Bazel version. This file is read by
[Bazelisk](https://github.com/bazelbuild/bazelisk) (for those engineers who use `bazelisk`
as a replacement for `bazel`).

If not updating Bazel, merely note what version is there.

#### Rebuild Toolchain Container

If necessary, make changes to `//bazel/rbe/gce_linux_container/Dockerfile` and run
`make -C bazel/rbe/gce_linux_container push` from the root directory, e.g.:

```
$ make -C bazel/rbe/gce_linux_container push
...
Successfully tagged gcr.io/skia-public/infra-rbe-linux:2022-06-17T17_13_12Z-somegoogler-16372d5-dirty
docker push gcr.io/skia-public/infra-rbe-linux:2022-06-17T17_13_12Z-somegoogler-16372d5-dirty
The push refers to repository [gcr.io/skia-public/infra-rbe-linux]
...
2022-06-17T17_13_12Z-somegoogler-16372d5-dirty: digest: sha256:0b12571d1befe54e9300711c8519535d635b867c317b2098915c7733fd65b833 size: 2007
make: Leaving directory '/usr/local/google/home/somegoogler/buildbot/bazel/rbe/gce_linux_container'
```

Make note of the image SHA256 hash printed out by the above command.

If not rebuilding the container, merely note what SHA256 hash is specified in the `container-image`
exec_property of the platform defined in `//bazel/rbe/generated/config/BUILD`
([example](https://skia.googlesource.com/buildbot/+/bb3604fd9a57bb20d799341b50f616af9e0062d4/bazel/rbe/generated/config/BUILD#43)).

### Step 2 - Generate Bazel Files

Note: We frequently skip this step when making changes to the RBE toolchain container or when
upgrading to a new Bazel version, and things usually continue to work. If you wish to skip
regenerating these files, just update the `container-image` exec_property of the platform defined
in `//bazel/rbe/generated/config/BUILD`
([example](https://skia.googlesource.com/buildbot/+/bb3604fd9a57bb20d799341b50f616af9e0062d4/bazel/rbe/generated/config/BUILD#43)).

Regenerate the `//bazel/rbe/generated` directory with the `rbe_configs_gen` CLI tool (installation
instructions below):

```
# Replace the <PLACEHOLDERS> as needed.
$ rbe_configs_gen \
      --bazel_version=<BAZEL VERSION> \
      --toolchain_container=gcr.io/skia-public/rbe-container-skia-infra@sha256:<HASH OF MOST RECENT IMAGE> \
      --output_src_root=<PATH TO REPOSITORY CHECKOUT> \
      --output_config_path=bazel/rbe/generated \
      --exec_os=linux \
      --target_os=linux
```

Example:

```
$ rbe_configs_gen \
      --bazel_version=4.1.0 \
      --toolchain_container=gcr.io/skia-public/rbe-container-skia-infra@sha256:5418d876cd8bb7cdbd05b834254038744d15f2f38327dfa46ff8dc9f83355260 \
      --output_src_root=$HOME/buildbot \
      --output_config_path=bazel/rbe/generated \
      --exec_os=linux \
      --target_os=linux
```

If `rbe_configs_gen` fails, try deleting all files under `//bazel/rbe/generated` (except for this
file) and re-run `rbe_configs_gen`.

### Step 3 - Cleanup

Run `bazel run //:buildifier` and fix any linting errors on the generated files (e.g. move load
statements at the top of the file, etc.)

### Step 4 - Manual Updates

If updating the Bazel version, update the
[bazel-toolchains](https://github.com/bazelbuild/bazel-toolchains) repository version imported from
`//WORKSPACE` to match the new Bazel version.

If updating the image version, the generated changes should have properly updated
`//bazel/rbe/generated/config/BUILD` to refer to the new image, so there is nothing else to do.

### Step 5 - Test out the changes
Try running various bazel commands with `--config remote` to make use of the new image.

When uploading the change as a CL, any tasks with the `-RBE` suffix will automatically use the
image specified in `//bazel/rbe/generated/config/BUILD`.

## How to install the `rbe_configs_gen` CLI tool

Clone the [bazel-toolchains](https://github.com/bazelbuild/bazel-toolchains) repository outside of
the Buildbot repository checkout, build the `rbe_configs_gen` binary, and place it on your $PATH:

```
$ git clone https://github.com/bazelbuild/bazel-toolchains

$ cd bazel-toolchains

# This assumes that $HOME/bin is in your $PATH.
$ go build -o $HOME/bin/rbe_configs_gen ./cmd/rbe_configs_gen/rbe_configs_gen.go
```
