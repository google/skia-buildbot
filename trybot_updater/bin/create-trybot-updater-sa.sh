#/bin/bash

# Creates the service account used by trybot updater, and export a key for it
# into the kubernetes cluster as a secret.

../kube/secrets/add-service-account.sh \
  skia-public \
  skia-corp \
  skia-trybot-updater \
  "Service account for Trybot updater for access to Git."
