#!/bin/sh

set -e

# Datastore.
curl ${DATASTORE_EMULATOR_HOST}

# Bigtable.
cbt --project=fake --instance=fake createtable faketable && \
    cbt --project=fake --instance=fake deletetable faketable

# Pubsub.
curl ${PUBSUB_EMULATOR_HOST}

# Firestore.
curl ${FIRESTORE_EMULATOR_HOST}

# CockroachDB.
cockroach node status --insecure --url postgresql://${COCKROACHDB_EMULATOR_HOST}