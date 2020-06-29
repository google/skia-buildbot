#!/bin/bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# Clean up any existing containers.
${DIR}/stop.sh > /dev/null

# Find the emulators.
source ${DIR}/env.sh
PORTS=""
ENV=""
for EMULATOR in $(env | grep _EMULATOR_HOST); do
  PORT="$(echo "${EMULATOR}" | cut -d':' -f2)"
  PORTS="${PORTS}-p ${PORT}:${PORT} "
  ENV="${ENV}--env ${EMULATOR} "
done

# Create the container.
IMAGE="$(cat ${DIR}/image.sha256)"
CONTAINER=$(docker container create --rm ${PORTS} ${ENV} ${IMAGE})
docker container start "${CONTAINER}" > /dev/null
while [ "`docker inspect -f {{.State.Health.Status}} ${CONTAINER}`" != "healthy" ]; do
  sleep 2
done

# Dump the new environment variables.
cat ${DIR}/env.sh