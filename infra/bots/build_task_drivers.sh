#!/bin/bash

set -x -e

export GOPATH="$(pwd)/go_deps"
export GOROOT="$(pwd)/go/go"
go build -o ${1} ./buildbot/infra/bots/test_drivers/*
