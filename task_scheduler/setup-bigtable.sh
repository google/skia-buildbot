#!/bin/bash

set -e -x

# Set up BigTable tables and column families.
go get -u cloud.google.com/go/bigtable/cmd/cbt

BIGTABLE_TABLE="tasks-cfg"
BIGTABLE_COLUMN_FAMILY="CFGS"
PROJECT="skia-public"
BIGTABLE_INSTANCE="production"
cbt --project=${PROJECT} --instance=${BIGTABLE_INSTANCE} createtable ${BIGTABLE_TABLE}
cbt --project=${PROJECT} --instance=${BIGTABLE_INSTANCE} createfamily ${BIGTABLE_TABLE} ${BIGTABLE_COLUMN_FAMILY}

BIGTABLE_INSTANCE="staging"
cbt --project=${PROJECT} --instance=${BIGTABLE_INSTANCE} createtable ${BIGTABLE_TABLE}
cbt --project=${PROJECT} --instance=${BIGTABLE_INSTANCE} createfamily ${BIGTABLE_TABLE} ${BIGTABLE_COLUMN_FAMILY}

PROJECT="skia-corp"
BIGTABLE_INSTANCE="internal"
cbt --project=${PROJECT} --instance=${BIGTABLE_INSTANCE} createtable ${BIGTABLE_TABLE}
cbt --project=${PROJECT} --instance=${BIGTABLE_INSTANCE} createfamily ${BIGTABLE_TABLE} ${BIGTABLE_COLUMN_FAMILY}
