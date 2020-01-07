#/bin/bash

# Creates the service account used by the auto-deploy server.
../kube/secrets/add-service-account.sh \
  skia-public \
  skia-public \
  skia-auto-deploy \
  "The auto-deploy service account."
