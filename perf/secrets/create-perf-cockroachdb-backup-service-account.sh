#/bin/bash

# Creates the service account used by the backup cronjob.
../../kube/secrets/add-service-account.sh \
  skia-public \
  skia-public \
  perf-cockroachdb-backup \
  "The perf cockroachdb backup service account." \
  roles/storage.objectAdmin