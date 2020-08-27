#!/bin/bash

set -e
set -x

# Authenticate using the credentials provided at GOOGLE_APPLICATION_CREDENTIALS.
gcloud auth activate-service-account --key-file=$GOOGLE_APPLICATION_CREDENTIALS

CONFIGS=(
  'cdb-android-prod.json'
  'cdb-ct-prod.json'
  'cdb-nano.json'
  'cdb-android-x.json'
  'cdb-flutter-engine.json'
  'flutter-flutter.json'
)

# Dump the tables we want backed up and copy the gzipped output to Google Cloud Storage.
for config in "${CONFIGS[@]}"
do
    echo "Backing up $config"
    /usr/loca//bin/perf-tool database backup alerts \
      --config_filename=/usr/local/share/skiaperf/configs/$config --out=/tmp/alerts.dat
    /usr/local/bin/perf-tool database backup regressions \
      --config_filename=/usr/local/share/skiaperf/configs/$config --out=/tmp/regressions.dat \
      --backup_to_date=$(date -d 'now - 2 weeks' +%Y)-$(date -d 'now - 2 weeks' +%m)-$(date -d 'now - 2 weeks' +%d)

    gsutil cp /tmp/alerts.dat      gs://skia-public-backup/perf/$(date +%Y)/$(date +%m)/$(date +%d)/$config/alerts.dat
    gsutil cp /tmp/regressions.dat gs://skia-public-backup/perf/$(date +%Y)/$(date +%m)/$(date +%d)/$config/regressions.dat
done