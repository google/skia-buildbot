#/bin/bash Creates the PubSub topic for comp-ui Perf files and then ties it to
# GCS notifications.

set -e -x

source ../kube/config.sh

TOPIC=perf-ingestion-compui

perf-tool config create-pubsub-topics --config_filename=./configs/comp-ui.json
gsutil notification create -f json -e OBJECT_FINALIZE -t projects/${PROJECT_ID}/topics/${TOPIC} -p ingest gs://chrome-comp-ui-perf-skia
