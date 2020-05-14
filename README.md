Skia-Buildbot Repository
========================

This repo contains infrastructure code for Skia.


Getting the Source Code
=======================

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

Using `go get` will fetch the repository into your GOPATH directory along with
all the Go dependencies. You will need to set GOPATH and GO111MODULE=on. E.g.:

```
$ export GOPATH=${WORKDIR}
$ export GO111MODULE=on
$ go get -u -t go.skia.org/infra/...
$ cd ${GOPATH}/src/go.skia.org/infra/
```

Note: go.skia.org is a custom import path and will only work if used like the
examples [here](http://golang.org/cmd/go/#hdr-Remote_import_paths).

Install [Node.js](https://nodejs.org/en/download/) (not as root) and add the bin
dir to your path. Optionally run `npm install npm -g`, as suggested by the
[npm getting started doc](https://docs.npmjs.com/getting-started/installing-node#updating-npm).

Install other dependencies:

```
$ sudo apt-get install python-django
$ go get -u github.com/kisielk/errcheck \
  golang.org/x/tools/cmd/goimports \
  go.chromium.org/luci/client/cmd/isolate
$ npm install -g polylint bower
```

Build ~everything:

```
$ make all
```

Generated Code
==============

Some code is generated using `go generate` with external binaries. First,
install the version of protoc referenced in the [asset creation
script](https://skia.googlesource.com/skia/+show/master/infra/bots/assets/protoc/create.py)
and ensure it is on your PATH before other versions of protoc.

Install the necessary go packages:
```
$ go get -u \
  github.com/golang/protobuf/protoc-gen-go \
  golang.org/x/tools/cmd/stringer \
  google.golang.org/grpc \
  github.com/vektra/mockery/...
```

To generate code run in this directory:

```
$ go generate ./...
```


Running unit tests
==================

Install [Cloud SDK](https://cloud.google.com/sdk/).

Use this command to run the presubmit tests:

```
$ ./run_unittests --small
```
