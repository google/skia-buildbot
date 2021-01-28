#/bin/bash
# Creates the PubSub topic for Flutter Perf files and then ties it to GCS
# notifications.

set -e -x

PROJECT_ID=skia-public
TOPIC=perf-ingestion-flutter-engine2

perf-tool config create-pubsub-topics --config_filename=./configs/flutter-engine2.json
gsutil notification create -f json -e OBJECT_FINALIZE -t projects/${PROJECT_ID}/topics/${TOPIC} -p flutter-engine gs://flutter-skia-perf-prod
