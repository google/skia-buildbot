#!/bin/bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

NAME="$(cat ${DIR}/image.sha256 | cut -d'@' -f1)"

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