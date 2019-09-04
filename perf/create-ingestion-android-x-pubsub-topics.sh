#/bin/bash
# Creates the PubSub topic for AndroidX Perf files and then ties it to GCS
# notifications.

set -e -x

source ../kube/config.sh

TOPIC=perf-ingestion-android-x-production

perf-tool config create-pubsub-topics --big_table_config=android-x
gsutil notification create -f json -e OBJECT_FINALIZE -t projects/${PROJECT_ID}/topics/${TOPIC} -p android-master-ingest gs://skia-perf
