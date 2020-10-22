#/bin/bash

# Creates the service account for bugs central.

../../kube/secrets/add-service-account.sh \
  skia-public \
  skia-public \
  skia-bugs-central \
  "Service account for Bugs Central."
