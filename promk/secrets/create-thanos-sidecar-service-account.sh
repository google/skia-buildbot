#/bin/bash

# Creates the service account key used by the thanos-bounce image.

set -e -x

../../kube/secrets/add-service-account.sh \
  skia-public \
  skolo-rack4 \
  thanos-sidecar-service-account \
  "Service account for thanos sidecar running in the skolo-rack4 k8s cluster." \
  roles/storage.objectAdmin
