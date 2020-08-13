#!/bin/bash

# Authenticate using the credentials provided at GOOGLE_APPLICATION_CREDENTIALS.
gcloud auth activate-service-account --key-file=$GOOGLE_APPLICATION_CREDENTIALS

DATABASE=('skia' 'android' 'android_x' 'ct' 'flutter_flutter' 'flutter_engine')

# Dump the tables we want backed up and copy the gzipped output to Google Cloud Storage.
for database in "${DATABASE[@]}"
do
    echo "Backing up $database"
    cockroach dump $database alerts --insecure --host=perf-cockroachdb-public \
    | gzip --stdout \
    | gsutil cp - gs://skia-public-backup/cockroachdb/perf/$database/$(date +%Y)/$(date +%m)/$(date +%d)/alerts.gz
done