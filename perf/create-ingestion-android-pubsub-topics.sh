#/bin/bash
# Creates the PubSub topic for Android Perf files and then ties it to GCS
# notifications.

set -e -x

source ../kube/config.sh

TOPIC=perf-ingestion-android-production

gcloud pubsub topics create ${TOPIC} || true
gsutil notification create -f json -e OBJECT_FINALIZE -t projects/${PROJECT_ID}/topics/${TOPIC} -p android-master-ingest gs://skia-perf
