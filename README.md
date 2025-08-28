# [[FORK]] This is a fork of the [Skia Buildbot](https://skia.googlesource.com/buildbot/) repo reset back to June of 2024

# Skia-Buildbot Repository

This repo contains infrastructure code for Skia.

# Getting the Source Code

The main source code repository is a Git repository hosted at
[https://skia.googlesource.com/buildbot.git](https://skia.googlesource.com/buildbot.git).
It is possible to check out this repository directly with `git clone` or via
`go get`.

Using `git clone` allows you to work in whatever directory you want. You will
still need to set GOPATH in order to build some apps (recommended to put this in
a cache dir). E.g.:

```
$ cd ${WORKDIR}
$ git clone https://skia.googlesource.com/buildbot.git
$ export GOPATH=${HOME}/.cache/gopath/$(basename ${WORKDIR})
$ mkdir $GOPATH
$ cd buildbot
```

# Install dependencies

Almost all applications are built with Bazel, and bazelisk is the recommended
tool to ensure you have the right version of bazel installed:

```
go install github.com/bazelbuild/bazelisk@latest
go install github.com/bazelbuild/buildtools/buildifier@latest
go install github.com/kisielk/errcheck@latest
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/mikefarah/yq/v4@latest
go install go.chromium.org/luci/client/cmd/...@latest
```

## Install other dependencies:

```
sudo apt-get install jq
```

# Build ~everything

```
bazelisk build --config=mayberemote //...
```

# Test everything

```
bazelisk test --config=mayberemote //...
```

# Generated Code

To update generated code run the following in any directory:

```
go generate ./...
```

# Running unit tests

Install [Cloud SDK](https://cloud.google.com/sdk/).

Use this command to run the presubmit tests:

```
./run_unittests --small
```

# VS Code Setup

Use `bazelisk` and `starpls` with the VS Code Bazel extension.
The install for `bazelisk` is listed above, and `starpls` can be
downloaded from:

    https://github.com/withered-magic/starpls/releases

```
    "bazel.executable": "bazelisk",
    "bazel.enableCodeLens": true,
    "bazel.lsp.command": "starpls"
```