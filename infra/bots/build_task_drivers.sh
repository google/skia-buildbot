#!/bin/bash

set -x -e

export GOPATH="$(pwd)/go_deps"
export GOROOT="$(pwd)/go/go"

# This is kind of dumb, but the easiest way to get actual desired current state
# of the repo. Replace the go_deps version of the infra repo with the isolated
# version, which will be more up-to-date and include any applied patch.
rm -rf ${GOPATH}/src/go.skia.org/infra
cp -r ./buildbot ${GOPATH}/src/go.skia.org/infra

task_drivers_dir=${GOPATH}/src/go.skia.org/infra/infra/bots/task_drivers
for td in $(cd ${task_drivers_dir} && ls); do
  go build -o ${1}/${td} ${task_drivers_dir}/${td}/${td}.go
done
