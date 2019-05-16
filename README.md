Skia-Buildbot Repository
========================

This repo contains infrastructure code for Skia.


Getting the Source Code
=======================

The main source code repository is a Git repository hosted at
[https://skia.googlesource.com/buildbot.git](https://skia.googlesource.com/buildbot.git).
It is possible to check out this repository directly with `git clone` or via
`go get`:

```
$ cd ${WORKDIR}
$ git clone https://skia.googlesource.com/buildbot.git
```

or

```
$ go get -u -t go.skia.org/infra/...
```

The latter fetches the repository into your $GOPATH directory along with all the
Go dependencies, while the former allows you to work in whatever directory you
want. If you're working within GOPATH, you probably want to set this variable:

```
export GO111MODULE=on
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
script](https://skia.googlesource.com/skia/+/master/infra/bots/assets/protoc/create.py)
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

The installed python-django version must be >= 1.7. Run the following to update:

```
$ sudo pip install Django --upgrade
```

Use this command to run the presubmit tests:

```
$ ./run_unittests --small
```
