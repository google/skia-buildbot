#!/bin/bash

# Authenticate using the credentials provided at GOOGLE_APPLICATION_CREDENTIALS.
gcloud auth activate-service-account --key-file=$GOOGLE_APPLICATION_CREDENTIALS

# Dump the tables we want backed up and copy the gzipped output to Google Cloud Storage.
cockroach dump skia alerts --insecure --host=perf-cockroachdb-public --dump-mode=data | gzip --stdout | gsutil cp - gs://skia-public-backup/cockroachdb/perf/skia/$(date +%Y)/$(date +%m)/$(date +%d)/alerts.gz