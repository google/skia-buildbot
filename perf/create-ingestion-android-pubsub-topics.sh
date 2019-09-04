#/bin/bash
# Creates the PubSub topic for Android Perf files and then ties it to GCS
# notifications.

set -e -x

source ../kube/config.sh

TOPIC=perf-ingestion-android-production

perf-tool config create-pubsub-topics --big_table_config=android-prod
gsutil notification create -f json -e OBJECT_FINALIZE -t projects/${PROJECT_ID}/topics/${TOPIC} -p android-master-ingest gs://skia-perf
