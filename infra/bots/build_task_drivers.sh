#!/bin/bash

# Takes a single argument that is the output directory where executables are to
# be placed.

set -x -e

export GOCACHE="$(pwd)/cache/go_cache"
export GOPATH="$(pwd)/cache/gopath"
export GOROOT="$(pwd)/go/go"

cd buildbot

task_drivers_dir=infra/bots/task_drivers
for td in $(cd ${task_drivers_dir} && ls); do
  go build -o ${1}/${td} ${task_drivers_dir}/${td}/${td}.go
done
