#!/bin/bash

set -x -e

export GOCACHE="$(pwd)/cache/go_cache"
export GOROOT="$(pwd)/go/go"

cd buildbot
go get

task_drivers_dir=infra/bots/task_drivers
for td in $(cd ${task_drivers_dir} && ls); do
  go build -o ${1}/${td} ${task_drivers_dir}/${td}/${td}.go
done
