luci-go: LUCI services and tools in Go
======================================

[![GoDoc](https://godoc.org/github.com/luci/luci-go?status.svg)](https://godoc.org/github.com/luci/luci-go)
[![Build Status](https://travis-ci.org/luci/luci-go.svg?branch=master)](https://travis-ci.org/luci/luci-go)
[![Coverage Status](https://coveralls.io/repos/luci/luci-go/badge.svg?branch=master&service=github)](https://coveralls.io/github/luci/luci-go?branch=master)

Installing
----------

    go get -u github.com/luci/luci-go/client/cmd/...
    go get -u github.com/luci/luci-go/server/cmd/...


Code layout
-----------

  * [/appengine/...](https://github.com/luci/luci-go/tree/master/appengine)
    contains [AppEngine](https://cloud.google.com/appengine/docs/go/) server
    code. It imports packages from `/common/...` and `/server/...`.
  * [/client/...](https://github.com/luci/luci-go/tree/master/client) contains
    all client code.
  * [/common/...](https://github.com/luci/luci-go/tree/master/common) contains
    code and structures shared between all of `/appengine/...`, `/client/...`
    and `/server/...`; for example, the structures used by the server APIs.
    These are inherently APIs.
  * [/deploytool/...](https://github.com/luci/luci-go/tree/master/deploytool)
    contains the LUCI cloud services deployment tool.
  * [/logdog/...](https://github.com/luci/luci-go/tree/master/logdog) contains
    LogDog client and server code, as well as APIs, protobufs, and support
    libraries.
  * [/server/...](https://github.com/luci/luci-go/tree/master/server) contains
    standalone server code. Its packages are reusable by `/appengine/...`.
  * [/tools/...](https://github.com/luci/luci-go/tree/master/tools) contains
    support tools used by other LUCI components.


Versioning
----------

  * Branch `go1` contains the stable code.
  * Branch `master` constains the latest code.


Contributing
------------

  * Sign the [Google CLA](https://cla.developers.google.com/clas).
  * Make sure your `user.email` and `user.name` are configured in `git config`.
  * Install test-only packages:
    `go get -u -t github.com/luci/luci-go/client/...`
  * Install the [pcg](https://github.com/maruel/pre-commit-go) git hook:
    `go get -u github.com/maruel/pre-commit-go/cmd/... && pcg`

Run the following to setup the code review tool and create your first review:

    git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git $HOME/src/depot_tools
    export PATH="$PATH:$HOME/src/depot_tools"
    cd $GOROOT/github.com/luci/luci-go
    git checkout -b work origin/master

    # hack hack

    git commit -a -m "This is awesome\nR=joe@example.com"
    # This will ask for your Google Account credentials.
    git cl upload -s
    # Wait for LGTM over email.
    # Check the commit queue box in codereview website.
    # Wait for the change to be tested and landed automatically.

Use `git cl help` and `git cl help <cmd>` for more details.
