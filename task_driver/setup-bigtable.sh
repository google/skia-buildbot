#!/bin/bash

set -e -x

PROJECT="${PROJECT:-skia-public}"
PUBSUB_TOPIC="${PUBSUB_TOPIC:-task-driver-logs}"
LOG_NAME="${LOG_NAME:-task-driver}"

# Set up BigTable tables and column families.
go get -u cloud.google.com/go/bigtable/cmd/cbt

PROJECT="skia-public"
BIGTABLE_INSTANCE="production"

BIGTABLE_TABLE="task-driver-runs"
BIGTABLE_COLUMN_FAMILY="MSGS"
cbt --project=${PROJECT} --instance=${BIGTABLE_INSTANCE} createtable ${BIGTABLE_TABLE}
cbt --project=${PROJECT} --instance=${BIGTABLE_INSTANCE} createfamily ${BIGTABLE_TABLE} ${BIGTABLE_COLUMN_FAMILY}

BIGTABLE_TABLE="task-driver-logs"
BIGTABLE_COLUMN_FAMILY="LOGS"
cbt --project=${PROJECT} --instance=${BIGTABLE_INSTANCE} createtable ${BIGTABLE_TABLE}
cbt --project=${PROJECT} --instance=${BIGTABLE_INSTANCE} createfamily ${BIGTABLE_TABLE} ${BIGTABLE_COLUMN_FAMILY}

BIGTABLE_INSTANCE="staging"

BIGTABLE_TABLE="task-driver-runs"
BIGTABLE_COLUMN_FAMILY="MSGS"
cbt --project=${PROJECT} --instance=${BIGTABLE_INSTANCE} createtable ${BIGTABLE_TABLE}
cbt --project=${PROJECT} --instance=${BIGTABLE_INSTANCE} createfamily ${BIGTABLE_TABLE} ${BIGTABLE_COLUMN_FAMILY}

BIGTABLE_TABLE="task-driver-logs"
BIGTABLE_COLUMN_FAMILY="LOGS"
cbt --project=${PROJECT} --instance=${BIGTABLE_INSTANCE} createtable ${BIGTABLE_TABLE}
cbt --project=${PROJECT} --instance=${BIGTABLE_INSTANCE} createfamily ${BIGTABLE_TABLE} ${BIGTABLE_COLUMN_FAMILY}

PROJECT="skia-corp"
BIGTABLE_INSTANCE="internal"

BIGTABLE_TABLE="task-driver-runs"
BIGTABLE_COLUMN_FAMILY="MSGS"
cbt --project=${PROJECT} --instance=${BIGTABLE_INSTANCE} createtable ${BIGTABLE_TABLE}
cbt --project=${PROJECT} --instance=${BIGTABLE_INSTANCE} createfamily ${BIGTABLE_TABLE} ${BIGTABLE_COLUMN_FAMILY}

BIGTABLE_TABLE="task-driver-logs"
BIGTABLE_COLUMN_FAMILY="LOGS"
cbt --project=${PROJECT} --instance=${BIGTABLE_INSTANCE} createtable ${BIGTABLE_TABLE}
cbt --project=${PROJECT} --instance=${BIGTABLE_INSTANCE} createfamily ${BIGTABLE_TABLE} ${BIGTABLE_COLUMN_FAMILY}


# Set up logs export to pubsub.
gcloud --project=${PROJECT} logging sinks create task-driver-logs-to-pubsub \
    pubsub.googleapis.com/projects/${PROJECT}/topics/${PUBSUB_TOPIC} \
    --log-filter="logName=\"projects/${PROJECT}/logs/${LOG_NAME}\""

PUBSUB_SERVICE_ACCOUNT=$(gcloud logging --project=${PROJECT} sinks describe task-driver-logs-to-pubsub | grep writerIdentity | sed -e 's/writerIdentity: serviceAccount://g')

gcloud projects add-iam-policy-binding ${PROJECT} --member serviceAccount:${PUBSUB_SERVICE_ACCOUNT} --role roles/pubsub.publisher
