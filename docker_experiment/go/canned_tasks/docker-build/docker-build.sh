#!/bin/bash

# We need a wrapper script because Swarming doesn't run the command in a shell,
# which means we can't use environment variables directly in the command.
set -e -x

docker run \
  -v $(pwd):/SRC \
  -v $(dirname ${LUCI_CONTEXT}):/AUTH \
  -v ${SWARMING_OUT_DIR}:/OUT \
  -v /var/run/docker.sock:/var/run/docker.sock \
  --env LUCI_CONTEXT=/AUTH/$(basename ${LUCI_CONTEXT}) \
  --env SWARMING_BOT_ID \
  --env SWARMING_SERVER \
  --env SWARMING_TASK_ID \
  --network host \
  gcr.io/skia-public/docker-build@sha256:2d5c3abba4f9acae8179f82bd46ceae8489221e91d82bad7859f25a4a51a7077 \
  $@
