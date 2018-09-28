#!/bin/bash

set -x -e

export GOPATH="$(pwd)/go_deps"
export GOROOT="$(pwd)/go/go"

# This is kind of dumb, but the easiest way to get actual desired current state
# of the repo.
rm -rf $GOPATH/src/go.skia.org/infra
cp -r ./buildbot $GOPATH/src/go.skia.org/infra

go build -o ${1} ./buildbot/infra/bots/task_drivers/*
