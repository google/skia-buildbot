#!/bin/sh

set -e
gcloud beta emulators datastore start --no-store-on-disk --host-port=0.0.0.0:$(echo ${DATASTORE_EMULATOR_HOST} | cut -d':' -f2) --project=test-project &
gcloud beta emulators bigtable start --host-port=0.0.0.0:$(echo ${BIGTABLE_EMULATOR_HOST} | cut -d':' -f2) --project=test-project &
gcloud beta emulators pubsub start --host-port=0.0.0.0:$(echo ${PUBSUB_EMULATOR_HOST} | cut -d':' -f2) --project=test-project &
gcloud beta emulators firestore start --host-port=0.0.0.0:$(echo ${FIRESTORE_EMULATOR_HOST} | cut -d':' -f2) &
cockroach start-single-node --insecure --listen-addr=0.0.0.0:$(echo ${COCKROACHDB_EMULATOR_HOST} | cut -d':' -f2) --store=/tmp/cockroach &
sleep infinity