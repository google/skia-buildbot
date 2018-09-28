#!/bin/bash

set -x -e

export GOPATH="$(pwd)/go_deps"
export GOROOT="$(pwd)/go/go"

# This is kind of dumb, but the easiest way to get actual desired current state
# of the repo.
# TODO

go build -o ${1} ./buildbot/infra/bots/task_drivers/*
