# Skia-Buildbot Repository

This repo contains infrastructure code for Skia.

# Supported Infrastucture Platforms

The infrastructure code is generally built to run on x86 linux. Running on other
platforms may be possible but is not officially supported and success will vary
depending on the command.

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

## Add bazelisk to path

```
export PATH=$PATH:$(go env GOPATH)/bin
```

## Install Node.js and npm

You will need Node.js and npm installed to run web infrastructure tests and linters.
We recommend using [nvm](https://github.com/nvm-sh/nvm) to manage Node versions.

After installing Node.js, run the following command to install repository dependencies (including
linter tools):

```bash
npm install
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

## Set up environ variables

This step might be an optional step, but some test requires these enviornment variables.

Runs

```
./scripts/run_emulators/run_emulators start
```

The following are example of environment variables.

```
Emulators started. Set environment variables as follows:
export DATASTORE_EMULATOR_HOST=localhost:8891
export BIGTABLE_EMULATOR_HOST=localhost:8892
export PUBSUB_EMULATOR_HOST=localhost:8893
export FIRESTORE_EMULATOR_HOST=localhost:8894
export COCKROACHDB_EMULATOR_HOST=localhost:8895
```

And stores these environment variables to `~/.bashrc` file.

## Execute tests

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
