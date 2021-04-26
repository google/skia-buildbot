#/bin/bash

# Creates the service account for SkCQ.

../../kube/secrets/add-service-account.sh \
  skia-public \
  skia-public \
  skia-commit-queue \
  "Service account for SkCQ."
