#!/bin/bash

set -e -x

IMAGE="${1}"
OUTPUT_PATH="$(realpath ${2})"

TMP="$(mktemp -d)"

for PLATFORM in ${@:3}; do
  PLATFORM_OUT="${OUTPUT_PATH}/${PLATFORM}"
  mkdir -p "${PLATFORM_OUT}"
  GOOS="$(echo $PLATFORM | cut -d "-" -f 1)"
  GOARCH="$(echo $PLATFORM | cut -d "-" -f 2)"
  docker run \
    --env GOOS="${GOOS}" \
    --env GOARCH="${GOARCH}" \
    --mount type=bind,src=$(pwd),dst=/repo \
    --mount type=bind,src=${PLATFORM_OUT},dst=/out \
    --workdir /repo \
    "${IMAGE}" \
    /repo/infra/bots/go_build_in_docker.sh
done

rm -rf "${TMP}"