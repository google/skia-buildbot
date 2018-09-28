#!/bin/bash

set -x -e

export GOPATH="$(pwd)/go_deps"
export GOROOT="$(pwd)/go/go"
go build -o ${1} ./test_drivers/*
