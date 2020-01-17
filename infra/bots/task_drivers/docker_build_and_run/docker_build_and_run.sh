#!/bin/bash

# We need a wrapper script because Swarming doesn't run the command in a shell,
# which means we can't use environment variables directly in the command.
set -e -x

docker run \
  -v .:/SRC \
  -v ${SWARMING_OUT_DIR}:/OUT \
  --env LUCI_CONTEXT=/SRC${LUCI_CONTEXT#$(pwd)} \
  --env SWARMING_BOT_ID=${SWARMING_BOT_ID} \
  --env SWARMING_SERVER=${SWARMING_SERVER} \
  --env SWARMING_TASK_ID=${SWARMING_TASK_ID} \
  gcr.io/skia-public/docker-build-and-run:prod \
  ./docker_build_and_run $@
