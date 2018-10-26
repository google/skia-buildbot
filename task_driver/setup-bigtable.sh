#!/bin/bash

set -e -x

PROJECT="${PROJECT:-skia-public}"
INSTANCE="${INSTANCE:-task-driver}"

go get -u cloud.google.com/go/bigtable/cmd/cbt

TABLE="task-driver-runs"
COLUMN_FAMILY="MSGS"
cbt --project=${PROJECT} --instance=${INSTANCE} createtable ${TABLE}
cbt --project=${PROJECT} --instance=${INSTANCE} createfamily ${TABLE} ${COLUMN_FAMILY}

TABLE="task-driver-logs"
COLUMN_FAMILY="LOGS"
cbt --project=${PROJECT} --instance=${INSTANCE} createtable ${TABLE}
cbt --project=${PROJECT} --instance=${INSTANCE} createfamily ${TABLE} ${COLUMN_FAMILY}
