#!/bin/bash

set -x -e

export GOCACHE="$(pwd)/cache/go_cache"
export GOROOT="$(pwd)/go/go"

cd buildbot
go get
cp -r buildbot ${1}
