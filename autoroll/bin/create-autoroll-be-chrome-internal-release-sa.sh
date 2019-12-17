#/bin/bash

# Creates the service account used by AutoRoll Backend, and export a key for it
# into the kubernetes cluster as a secret.

set -e -x

../kube/secrets/add-service-account.sh \
  google.com:skia-corp \
  skia-corp \
  chrome-internal-release-roll \
  "Service account for AutoRolls into Chromium Internal Release Branches." \
  roles/datastore.user

gsutil iam ch serviceAccount:${SA_EMAIL}:objectAdmin gs://skia-autoroll
