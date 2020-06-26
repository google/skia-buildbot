#!/bin/bash

set -e -x

NAME="gcr.io/skia-public/run_emulators"
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
BASE="$(cat ${DIR}/base.sha256)"

# Use a temp dir to avoid copying the whole repo into the build context.
TMP="$(mktemp -d)"
cp ${DIR}/run_in_docker.sh ${TMP}
cp ${DIR}/healthcheck.sh ${TMP}
docker build \
    --build-arg base=${BASE} \
    --tag ${NAME} \
    -f ${DIR}/Dockerfile \
    ${TMP}
rm -rf ${TMP}
docker push ${NAME}
docker inspect --format='{{index .RepoDigests 0}}' ${NAME} > ${DIR}/image.sha256
