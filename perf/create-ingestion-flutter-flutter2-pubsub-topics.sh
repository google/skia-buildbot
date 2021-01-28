#/bin/bash
# Creates the PubSub topic for Flutter Perf files and then ties it to GCS
# notifications.

set -e -x

PROJECT_ID=skia-public
TOPIC=perf-ingestion-flutter-flutter2

perf-tool config create-pubsub-topics --config_filename=./configs/flutter-flutter2.json
gsutil notification create -f json -e OBJECT_FINALIZE -t projects/${PROJECT_ID}/topics/${TOPIC} -p flutter-flutter gs://flutter-skia-perf-prod
