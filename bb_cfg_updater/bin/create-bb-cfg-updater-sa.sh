#/bin/bash

# Creates the service account used by buildbucket config updater, and export a
# key for it into the kubernetes cluster as a secret.

../kube/secrets/add-service-account.sh \
  skia-public \
  skia-corp \
  skia-bb-cfg-updater \
  "Service account for Buildbucket Config Updater for access to Git."
