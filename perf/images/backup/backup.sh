#!/bin/bash

set -e

# Authenticate using the credentials provided at GOOGLE_APPLICATION_CREDENTIALS.
gcloud auth activate-service-account --key-file=$GOOGLE_APPLICATION_CREDENTIALS

CONFIGS=(
  'comp-ui.json'
  'cdb-ct-prod.json'
  'cdb-nano.json'
  'cdb-flutter-engine.json'
  'flutter-flutter.json'
  'flutter-flutter2.json'
  'flutter-engine2.json'
  'cdb-android-x.json'
  'cdb-android-prod.json'
  'v8'
)

# Dump the tables we want backed up and copy the gzipped output to Google Cloud Storage.
for config in "${CONFIGS[@]}"
do
    echo "Backing up $config"
    /usr/local/bin/perf-tool database backup alerts \
      --config_filename=/usr/local/share/skiaperf/configs/$config --out=/tmp/alerts.dat
    # Defaults to backing up one month.
    /usr/local/bin/perf-tool database backup regressions \
      --config_filename=/usr/local/share/skiaperf/configs/$config --out=/tmp/regressions.dat

    gsutil cp /tmp/alerts.dat      gs://skia-public-backup/perf/$(date +%Y)/$(date +%m)/$(date +%d)/$config/alerts.dat
    gsutil cp /tmp/regressions.dat gs://skia-public-backup/perf/$(date +%Y)/$(date +%m)/$(date +%d)/$config/regressions.dat
done