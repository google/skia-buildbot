#!/bin/bash

set -e -x

PROJECT="${PROJECT:-skia-public}"
INSTANCE="${INSTANCE:-task-driver}"
TABLE="${TABLE:-task-driver-runs}"
COLUMN_FAMILY="${COLUMN_FAMILY:-MSGS}"

go get -u cloud.google.com/go/bigtable/cmd/cbt
cbt --project=${PROJECT} --instance=${INSTANCE} createtable ${TABLE}
cbt --project=${PROJECT} --instance=${INSTANCE} createfamily ${TABLE} ${COLUMN_FAMILY}
