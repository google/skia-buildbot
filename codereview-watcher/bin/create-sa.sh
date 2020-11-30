#/bin/bash

# Creates the service account for codereview-watcher.

../../kube/secrets/add-service-account.sh \
  skia-public \
  skia-public \
  skia-codereview-watcher \
  "Service account for Codereview Watcher."
