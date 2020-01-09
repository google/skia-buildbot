#/bin/bash

# Creates the service account used by AutoRoll Backend, and export a key for it
# into the kubernetes cluster as a secret.

set -e -x

PROJECT="skia-corp"
SERVICE_ACCOUNT_NAME="chrome-internal-release-roll"

../kube/secrets/add-service-account.sh \
  google.com:${PROJECT} \
  ${PROJECT} \
  ${SERVICE_ACCOUNT_NAME} \
  "Service account for AutoRolls into Chromium Internal Release Branches." \
  roles/datastore.user

gsutil iam ch serviceAccount:${SERVICE_ACCOUNT_NAME}@${PROJECT}.google.com.iam.gserviceaccount.com:objectAdmin gs://skia-autoroll
