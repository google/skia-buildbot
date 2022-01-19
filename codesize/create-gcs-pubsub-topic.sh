#/bin/bash

# Configures GCS to publish events to a PubSub topic whenever files are added to the codesize GCS
# bucket. If the topic does not exist, it will be created.

set -e -x

source ../kube/config.sh

BUCKET=gs://skia-codesize
TOPIC=skia-codesize-files

gsutil notification create \
  -f json \
  -e OBJECT_FINALIZE \
  -t projects/${PROJECT_ID}/topics/${TOPIC} \
  ${BUCKET}
