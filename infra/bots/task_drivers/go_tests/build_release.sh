#!/bin/bash

set -x -e

SDK_VERSION="298.0.0" # Latest at time of writing.

NAME="gcr.io/skia-public/go_tests"
ROOT="$(git rev-parse --show-toplevel)/infra/bots/task_drivers/go_tests"
BASE="$(cat ${ROOT}/base.sha256)"

docker build -f ${ROOT}/Dockerfile --tag ${NAME} \
    --build-arg base=${BASE} \
    --build-arg sdk_version=${SDK_VERSION} \
    ${ROOT}
docker push ${NAME}
docker inspect --format='{{index .RepoDigests 0}}' ${NAME} > ${ROOT}/image.sha256