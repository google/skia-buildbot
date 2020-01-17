#!/bin/bash

# We need a wrapper script because Swarming doesn't run the command in a shell,
# which means we can't use environment variables directly in the command.
set -e -x

env
docker run \
  -v $(pwd):/SRC \
  -v $(dirname ${LUCI_CONTEXT}):/AUTH
  -v ${SWARMING_OUT_DIR}:/OUT \
  --env LUCI_CONTEXT=/AUTH/$(basename ${LUCI_CONTEXT}) \
  --env SWARMING_BOT_ID \
  --env SWARMING_SERVER \
  --env SWARMING_TASK_ID \
  gcr.io/skia-public/docker-build-and-run:prod \
  $@
