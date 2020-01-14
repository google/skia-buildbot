#/bin/bash

# Creates the service account for androidingest.

set -e -x

EMAIL=$(../../kube/secrets/add-service-account.sh \
  skia-public \
  skia-public \
  skia-android-ingest \
  "Service account for androidingest." \
  roles/storage.objectAdmin)

gsutil iam ch "serviceAccount:${EMAIL}" gs://skia-perf