#/bin/bash
# Creates the PubSub topic for Perf files and then ties it to GCS
# notifications.

set -e -x

source ../kube/config.sh

TOPIC=perf-ingestion-skia

gcloud pubsub topics create ${TOPIC}
gsutil notification create -f json -e OBJECT_FINALIZE -t projects/${PROJECT_ID}/topics/${TOPIC} -p nano-json-v1  gs://skia-perf
