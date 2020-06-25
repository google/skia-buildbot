#!/bin/sh

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

NAME="gcr.io/skia-public/run_emulators"

# Kill any running containers.
for CONTAINER in $(docker container ls --quiet --filter ancestor="${NAME}"); do
  docker container kill ${CONTAINER} > /dev/null
done

# Clean up any stopped containers.
for CONTAINER in $(docker container ls --quiet --filter ancestor="${NAME}"); do
  docker container rm ${CONTAINER} > /dev/null
done

# Dump the new environment variables.
cat ${DIR}/env-unset.sh