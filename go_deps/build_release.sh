#!/bin/bash

set -x -e

NAME="gcr.io/skia-public/go_deps"
ROOT="$(git rev-parse --show-toplevel)"
BASE="$(cat ${ROOT}/go_deps/base.sha256)"

docker build -f ${ROOT}/go_deps/Dockerfile --tag ${NAME} --build-arg base=${BASE} ${ROOT}
docker push ${NAME}
docker inspect --format='{{index .RepoDigests 0}}' ${NAME} > ${ROOT}/go_deps/image.sha256