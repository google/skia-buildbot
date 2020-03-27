#/bin/bash
# Creates the PubSub topic for AndroidX Perf files and then ties it to GCS
# notifications.

set -e -x

source ../kube/config.sh

TOPIC=perf-ingestion-android-x2-production

perf-tool config create-pubsub-topics --config_filename=./configs/android-x2.json
gsutil notification create -f json -e OBJECT_FINALIZE -t projects/${PROJECT_ID}/topics/${TOPIC} -p android-master-ingest gs://skia-perf
